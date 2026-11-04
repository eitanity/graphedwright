// Package svg draws a graph document as SVG text and answers which
// element a point hits, from the same geometry. It writes no DOM: the
// text goes to any io.Writer, the same on every target, so a drawing
// can be frozen and diffed.
package svg

import (
	"io"
	"math"

	gw "code.eitanity.com/graphedwright"
)

// Mode is what the drawing is for: looking at, or editing.
type Mode uint8

// The modes.
const (
	View Mode = iota
	Edit
)

// KindClassPrefix goes before a kind's name in the class of each of its
// elements and arrowheads. None of the renderer's own classes starts
// with it, so a kind of any name, "port" or "overlay" among them, is
// styled and found as that kind alone.
const KindClassPrefix = "kind-"

// kindClass writes the class of a kind's elements: the prefix, then the
// name with anything but letters, digits, '-' and '_' as '_'.
func (r *Renderer) kindClass(name string) {
	r.o.str(KindClassPrefix)
	r.o.name(name)
}

// Annotation is a figure drawn on an element for one render: under a
// vertex's label, or at an edge's midpoint. Class joins the element's
// classes. On an edge, a Weight above zero sets the stroke width in
// place of the edge's own.
type Annotation struct {
	Text   string
	Class  string
	Weight float64
}

// Options shape one render. VertexNotes and EdgeNotes are indexed by
// vertex and edge; a missing or empty entry draws nothing. CharWidth is
// the estimated width of one character of a label at the default font,
// 7 units when not positive; FontSize the label's size, 12 when not
// positive. Selected vertices and edges are outlined in the overlay in
// edit mode.
type Options struct {
	Mode          Mode
	Title, Desc   string
	VertexNotes   []Annotation
	EdgeNotes     []Annotation
	CharWidth     float64
	FontSize      float64
	Selection     Selection   // what the overlay outlines, in edit mode; nil for nothing
	DefaultWeight float64     // the stroke width of an unweighted edge, 1.5 when zero
	Style         string      // a stylesheet embedded in the drawing; "" for none
	Ghost         [2]gw.Point // a line the overlay draws, from a port to the pointer, in edit mode
	HasGhost      bool
	Marquee       gw.Viewport // a box the overlay draws, in edit mode
	HasMarquee    bool
	Target        gw.VertexIndex // a vertex the overlay rings as where an edge can start or end, in edit mode
	HasTarget     bool
	Handle        gw.VertexIndex // a vertex the overlay draws the resize handle of, in edit mode
	HasHandle     bool
	Hover         gw.VertexIndex // a vertex the overlay draws the cut texts of in full, in either mode
	HasHover      bool
	// NoFullText writes a cut text without its full text after it, for
	// the stylesheet to show on hover; its title still holds it.
	NoFullText bool
	// IDPrefix goes before the IDs the renderer makes, the title's and
	// the arrowheads', so that two drawings in one page do not share
	// them; DocIDPrefix goes before each vertex's and edge's ID, so that
	// a document cannot take an ID the page uses. Both are written as
	// tokens: letters, digits, '-' and '_', anything else as '_'.
	IDPrefix    string
	DocIDPrefix string
}

// Selection is what the overlay outlines: the vertices and edges it
// names by position, from 0 to the counts. An edit.Editor is one; a
// caller that draws a selection of its own gives a Selected. The
// renderer reads it while it draws, and draws no more than the
// document's capacities.
type Selection interface {
	SelectedCount() (vertices, edges int)
	SelectedVertex(i int) gw.VertexIndex
	SelectedEdge(i int) gw.EdgeIndex
}

// Selected is a Selection the caller holds: the vertices and edges in V
// and E.
type Selected struct {
	V []gw.VertexIndex
	E []gw.EdgeIndex
}

// SelectedCount is the number of vertices and edges.
func (s Selected) SelectedCount() (vertices, edges int) { return len(s.V), len(s.E) }

// SelectedVertex is the ith vertex, or None out of range.
func (s Selected) SelectedVertex(i int) gw.VertexIndex {
	if i < 0 || i >= len(s.V) {
		return gw.None
	}
	return s.V[i]
}

// SelectedEdge is the ith edge, or None out of range.
func (s Selected) SelectedEdge(i int) gw.EdgeIndex {
	if i < 0 || i >= len(s.E) {
		return gw.None
	}
	return s.E[i]
}

// Renderer holds the output buffer; one can render any number of
// documents, one at a time, without allocating.
type Renderer struct {
	o *out
	// expand is set while the overlay writes the hovered vertex's cut
	// texts in full, at its centre: its texts are walked as the
	// vertex draws them, and only those cut are written.
	expand bool
	centre gw.Point
}

// Render writes the whole document.
func (r *Renderer) Render(d *gw.Document, opts Options, w io.Writer) error {
	o := r.o
	o.reset(w)
	opts = opts.filled()
	view := d.View()
	if !usable(view) {
		view = bounds(d)
	}
	o.str(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="`)
	o.point(view.X, view.Y)
	o.byte(' ')
	o.point(view.W, view.H)
	o.str(`" class="graphedwright" data-mode="`)
	if opts.Mode == Edit {
		o.str("edit")
	} else {
		o.str("view")
	}
	o.str(`" role="img" aria-labelledby="`)
	o.name(opts.IDPrefix)
	o.str(`graphedwright-title"><title id="`)
	o.name(opts.IDPrefix)
	o.str(`graphedwright-title">`)
	o.text(opts.Title)
	o.str(`</title><desc>`)
	o.text(opts.Desc)
	o.str(`</desc>`)
	if opts.Style != "" {
		o.str(`<style>`)
		o.text(opts.Style)
		o.str(`</style>`)
	}
	r.defs(d, opts.IDPrefix)
	maxW := maxWeight(d, opts)
	// Containers first, each before what it holds and the one beside it
	// with all it holds, so that what they hold and the edges between
	// draw over them, and two that overlap do not interleave.
	o.str(`<g class="containers">`)
	for at := range d.Containers().Len() {
		v := d.Containers().At(at)
		r.vertex(d, opts, v)
	}
	o.str(`</g><g class="edges">`)
	for i := range d.EdgeSlots() {
		if d.Edge(gw.EdgeIndex(i)).Live() { //nolint:gosec // bounded by the capacity
			r.edge(d, opts, gw.EdgeIndex(i), maxW) //nolint:gosec // bounded by the capacity
		}
	}
	o.str(`</g><g class="vertices">`)
	for i := range d.VertexSlots() {
		if d.Vertex(gw.VertexIndex(i)).Live() && !d.Kinds().Vertex(d.Vertex(gw.VertexIndex(i)).Kind()).Container() { //nolint:gosec // bounded by the capacity
			r.vertex(d, opts, gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		}
	}
	o.str(`</g>`)
	r.overlay(d, opts, maxW)
	o.str(`</svg>`)
	o.byte('\n')
	o.flush()
	return o.err
}

// RenderVertex writes one vertex's element alone, for a host that
// replaces it in place.
func (r *Renderer) RenderVertex(d *gw.Document, opts Options, v gw.VertexIndex, w io.Writer) error {
	r.o.reset(w)
	r.vertex(d, opts.filled(), v)
	r.o.flush()
	return r.o.err
}

// RenderOverlay writes the overlay group alone, for a host that
// replaces it after a selection change.
func (r *Renderer) RenderOverlay(d *gw.Document, opts Options, w io.Writer) error {
	r.o.reset(w)
	opts = opts.filled()
	r.overlay(d, opts, maxWeight(d, opts))
	r.o.flush()
	return r.o.err
}

// RenderEdge writes one edge's element alone.
func (r *Renderer) RenderEdge(d *gw.Document, opts Options, e gw.EdgeIndex, w io.Writer) error {
	r.o.reset(w)
	opts = opts.filled()
	r.edge(d, opts, e, maxWeight(d, opts))
	r.o.flush()
	return r.o.err
}

// defaultCharWidth is the estimated width of a character of a label when
// the host gives none.
const defaultCharWidth = 7

// filled puts the defaults in the zero options.
func (opts Options) filled() Options {
	if !(opts.CharWidth > 0) {
		opts.CharWidth = defaultCharWidth
	}
	if !(opts.FontSize > 0) {
		opts.FontSize = defaultFontSize
	}
	if !(opts.DefaultWeight > 0) {
		opts.DefaultWeight = defaultStroke
	}
	return opts
}

// defs writes an arrowhead marker for every edge kind with an arrow,
// and a tail marker for every one with a tail, named by the kind's
// index.
func (r *Renderer) defs(d *gw.Document, prefix string) {
	o := r.o
	o.str(`<defs>`)
	kinds := d.Kinds()
	// bound: the edge kinds
	for i := range kinds.EdgeKinds() {
		k := kinds.Edge(gw.KindIndex(i)) //nolint:gosec // bounded by the kinds
		r.tailMarker(i, k, prefix)
		if !k.Arrow() {
			continue
		}
		o.str(`<marker id="`)
		o.name(prefix)
		o.str(`arrow-`)
		o.int(i)
		o.str(`" class="arrowhead `)
		r.kindClass(k.Name())
		switch k.Head() {
		case gw.FilledHead:
			o.str(`" viewBox="0 0 10 10" refX="1" refY="5" markerWidth="10" markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/></marker>`)
		case gw.OpenHead:
			// The edge runs to the tip, so the tip is the reference.
			o.str(` open" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="10" markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10"/></marker>`)
		case gw.HollowHead:
			o.str(` hollow" viewBox="-1 -1 12 12" refX="1" refY="5" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/></marker>`)
		case gw.TypedHead:
			o.str(` hollow typed" viewBox="-1 -1 12 12" refX="1" refY="5" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/><circle cx="3" cy="3.5" r="1.1"/><circle cx="3" cy="6.5" r="1.1"/></marker>`)
		case gw.BarHead:
			// The bar crosses the stroke just before the base.
			o.str(` hollow bar" viewBox="-4 -1 15 12" refX="1" refY="5" markerWidth="15" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/><path d="M -3 0 L -3 10"/></marker>`)
		}
	}
	o.str(`</defs>`)
}

// tailMarker writes the marker of edge kind i's tail, if it has one. It
// is drawn at the stroke's start, which stops the tail's length short
// of the source (way.started), and points back along the edge, so the
// tail spans from there to the source's outline.
func (r *Renderer) tailMarker(i int, k gw.EdgeKindView, prefix string) {
	if k.Tail() == gw.NoTail {
		return
	}
	o := r.o
	o.str(`<marker id="`)
	o.name(prefix)
	o.str(`tail-`)
	o.int(i)
	o.str(`" class="tail `)
	r.kindClass(k.Name())
	switch k.Tail() {
	case gw.FilledDiamond:
		o.str(`" viewBox="-1 -1 14 12" refX="0" refY="5" markerWidth="14" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 5 L 6 0 L 12 5 L 6 10 z"/></marker>`)
	case gw.HollowDiamond:
		o.str(` hollow" viewBox="-1 -1 14 12" refX="0" refY="5" markerWidth="14" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 5 L 6 0 L 12 5 L 6 10 z"/></marker>`)
	case gw.CircledPlus:
		o.str(` plus" viewBox="-1 -1 12 12" refX="0" refY="5" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><circle cx="5" cy="5" r="5"/><path d="M 0 5 L 10 5 M 5 0 L 5 10"/></marker>`)
	case gw.NoTail: // returned above
	}
}

// maxWeight is the largest weight among live weighted edges and their
// annotations, for the stroke scale; 1 when there is none.
func maxWeight(d *gw.Document, opts Options) float64 {
	m := 0.0
	for i := range d.EdgeSlots() {
		e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		if !e.Live() || !d.Kinds().Edge(e.Kind()).Weighted() {
			continue
		}
		w := e.Weight()
		if i < len(opts.EdgeNotes) && noteWeight(opts.EdgeNotes[i].Weight) {
			w = opts.EdgeNotes[i].Weight
		}
		if w > m {
			m = w
		}
	}
	if m <= 0 {
		return 1
	}
	return m
}

// noteWeight reports whether an annotation's weight is one to draw:
// positive and within gw.MaxCoord, as an edge's own is. NaN and the
// infinities are not, and the edge keeps its own weight.
func noteWeight(w float64) bool { return w > 0 && w <= gw.MaxCoord }

// strokeWidth is an edge's stroke width: the default, or for a weighted
// edge 1 to 7 by its weight against the largest.
func strokeWidth(d *gw.Document, opts Options, i int, maxW float64) float64 {
	e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	if !d.Kinds().Edge(e.Kind()).Weighted() {
		return opts.DefaultWeight
	}
	w := e.Weight()
	if i < len(opts.EdgeNotes) && noteWeight(opts.EdgeNotes[i].Weight) {
		w = opts.EdgeNotes[i].Weight
	}
	if w <= 0 {
		return 1
	}
	return 1 + 6*w/maxW
}

// edge writes one edge: its group with the kind's class, the data
// attributes, the path with its arrow and width, and its label and
// annotation at the midpoint.
func (r *Renderer) edge(d *gw.Document, opts Options, i gw.EdgeIndex, maxW float64) {
	o := r.o
	e := d.Edge(i)
	kind := d.Kinds().Edge(e.Kind())
	note := noteOf(opts.EdgeNotes, int(i))
	o.str(`<g class="edge `)
	r.kindClass(kind.Name())
	if kind.Dashed() {
		o.str(" dashed")
	}
	if note.Class != "" {
		o.byte(' ')
		o.name(note.Class)
	}
	o.str(`" id="`)
	o.name(opts.DocIDPrefix)
	o.attr(string(e.ID()))
	o.str(`" data-kind="`)
	o.attr(kind.Name())
	o.str(`" data-from="`)
	o.attr(string(d.Vertex(e.From()).ID()))
	o.str(`" data-to="`)
	o.attr(string(d.Vertex(e.To()).ID()))
	o.byte('"')
	r.dataAttrs(e.Attrs())
	o.byte('>')
	var lane [1]gw.Point
	w := wayOf(d, i, &lane)
	f := w.f
	r.path(f, kind, w.started(kind), w.through, w.shortened(kind))
	width := strokeWidth(d, opts, int(i), maxW)
	o.numAttr("stroke-width", width)
	if kind.Tail() != gw.NoTail {
		o.str(` marker-start="url(#`)
		o.name(opts.IDPrefix)
		o.str(`tail-`)
		o.int(int(e.Kind()))
		o.str(`)"`)
	}
	if kind.Arrow() {
		o.str(` marker-end="url(#`)
		o.name(opts.IDPrefix)
		o.str(`arrow-`)
		o.int(int(e.Kind()))
		o.str(`)"`)
	}
	o.str(`/>`)
	mid := w.middle(kind.Route())
	if f.down {
		r.besideLabels(kind, e, note, mid, width, w.side(), opts)
		o.str(`</g>`)
		return
	}
	r.levelLabels(kind, e, note, mid, width, w.side(), opts)
	o.str(`</g>`)
}

// levelLabels writes an edge's texts for a left-to-right flow: the
// stereotype and then the label above the stroke at the middle, the
// annotation below it; on side -1, for a lane below the others, the
// other way about, so that they are outside the lanes.
func (r *Renderer) levelLabels(kind gw.EdgeKindView, e gw.EdgeView, note Annotation, mid gw.Point, width, side float64, opts Options) {
	above, below := mid.Y-float64(width/2)-4, mid.Y+float64(width/2)+opts.FontSize
	near, step, far := above, -opts.FontSize, below
	if side < 0 {
		near, step, far = below, opts.FontSize, above
	}
	if kind.Stereotype() != "" {
		r.label("stereotype", kind.Stereotype(), mid.X, near, 0, opts)
		near += step
	}
	if gw.LabelRunes(kind.LabelFormat(), e.Label(), e.Attrs()) > 0 {
		r.formatted("label", kind.LabelFormat(), e.Label(), e.Attrs(), mid.X, near, 0, "middle", opts)
	}
	if note.Text != "" {
		r.label("annotation", note.Text, mid.X, far, 0, opts)
	}
}

// besideLabels writes an edge's texts for a top-to-bottom flow, whose
// edges mostly run down: the label to the right of the stroke at the
// middle, the stereotype above the label, the annotation to the left;
// on side -1, for a lane left of the others, the other way about.
func (r *Renderer) besideLabels(kind gw.EdgeKindView, e gw.EdgeView, note Annotation, mid gw.Point, width, side float64, opts Options) {
	x, other, y := mid.X+float64(width/2)+4, mid.X-float64(width/2)-4, mid.Y+opts.FontSize/3
	anchor, otherAnchor := "start", anchorEnd
	if side < 0 {
		x, other, anchor, otherAnchor = other, x, otherAnchor, anchor
	}
	above := y
	if gw.LabelRunes(kind.LabelFormat(), e.Label(), e.Attrs()) > 0 {
		r.formatted("label", kind.LabelFormat(), e.Label(), e.Attrs(), x, y, 0, anchor, opts)
		above -= opts.FontSize
	}
	if kind.Stereotype() != "" {
		r.labelAt("stereotype", kind.Stereotype(), x, above, 0, anchor, opts)
	}
	if note.Text != "" {
		r.labelAt("annotation", note.Text, other, y, 0, otherAnchor, opts)
	}
}

// path writes the opening of an edge's path element with its route
// from a through the bends to b, leg by leg, each worked out in the
// frame and written back in the document's axes.
func (r *Renderer) path(f frame, kind gw.EdgeKindView, a gw.Point, bends gw.PointList, b gw.Point) {
	o := r.o
	o.str(`<path d="M `)
	o.point(a.X, a.Y)
	across, down := ` H `, ` V `
	if f.down {
		across, down = down, across
	}
	for k := range bends.Len() + 1 {
		p, q := leg(a, bends, b, k)
		p, q = f.t(p), f.t(q)
		switch legRoute(kind.Route(), bends.Len() > 0, p, q) {
		case gw.Straight:
			r.at(f, ` L `, q)
		case gw.Curve:
			c1, c2 := f.controls(p, q)
			r.at(f, ` C `, c1)
			r.at(f, `, `, c2)
			r.at(f, `, `, q)
		case gw.Orthogonal:
			o.str(across)
			o.num((p.X + q.X) / 2)
			o.str(down)
			o.num(q.Y)
			o.str(across)
			o.num(q.X)
		}
	}
	o.byte('"')
}

// at writes a separator and a point of the frame in the document's axes.
func (r *Renderer) at(f frame, sep string, p gw.Point) {
	p = f.t(p)
	r.o.str(sep)
	r.o.point(p.X, p.Y)
}

// noteOf is the annotation at index i, or none.
func noteOf(notes []Annotation, i int) Annotation {
	if i < len(notes) {
		return notes[i]
	}
	return Annotation{}
}

// vertex writes one vertex: its group translated to its centre, the
// shape, its texts and the ports.
func (r *Renderer) vertex(d *gw.Document, opts Options, i gw.VertexIndex) {
	o := r.o
	v := d.Vertex(i)
	kind := d.Kinds().Vertex(v.Kind())
	note := noteOf(opts.VertexNotes, int(i))
	o.str(`<g class="vertex `)
	r.kindClass(kind.Name())
	if kind.Container() {
		o.str(" container")
	}
	if kind.Dashed() {
		o.str(" dashed")
	}
	if note.Class != "" {
		o.byte(' ')
		o.name(note.Class)
	}
	if !v.Placed() {
		o.str(" unplaced")
	}
	o.str(`" id="`)
	o.name(opts.DocIDPrefix)
	o.attr(string(v.ID()))
	o.str(`" data-kind="`)
	o.attr(kind.Name())
	o.str(`" transform="translate(`)
	o.point(v.Pos().X, v.Pos().Y)
	o.str(`)"`)
	r.dataAttrs(v.Attrs())
	o.byte('>')
	r.shape(shapeClass, kind.Shape(), v.Size())
	r.texts(d, opts, i)
	r.ports(d, i)
	o.str(`</g>`)
}

// texts writes a vertex's texts, each cut at the room it has with the
// full text as its title: the keyword and label, the container's
// title or the compartments, and the annotation.
func (r *Renderer) texts(d *gw.Document, opts Options, i gw.VertexIndex) {
	v := d.Vertex(i)
	kind := d.Kinds().Vertex(v.Kind())
	note := noteOf(opts.VertexNotes, int(i))
	labelY, labelW := 4.0, v.Size().W-8
	if note.Text != "" {
		labelY = -2
	}
	if kind.Shape() == gw.Actor {
		// An actor's label is at the foot of its box, under the
		// figure, as wide as it needs.
		labelY, labelW = float64(v.Size().H/2)-4, 0
	}
	switch {
	case kind.Container():
		r.title(v, kind, opts)
	case d.ShowsCompartments(i):
		r.compartments(v, kind, labelW, opts)
	default:
		if kw := kind.Keyword(); kw != "" {
			// The keyword is the line above the label, the two centred
			// in the shape; an actor's, whose label is at its foot, is
			// above its head.
			half := float64(opts.FontSize/2) + 1
			kwY := labelY - half
			if kind.Shape() == gw.Actor {
				kwY = float64(-v.Size().H/2) - 4
			} else {
				labelY += half
			}
			r.label("keyword", kw, 0, kwY, labelW, opts)
		}
		r.formatted("label", kind.LabelFormat(), v.Label(), v.Attrs(), 0, labelY, labelW, "middle", opts)
	}
	if note.Text != "" {
		r.label("annotation", note.Text, 0, labelY+opts.FontSize, v.Size().W-8, opts)
	}
}

// compartments draws the label at the top of a vertex, under its
// keyword if it has one, and below it each compartment with a value:
// a rule across the vertex, then a line of text for each line of the
// value, cut at the vertex's width as a label is. The sizes are the
// document's, by which the vertex was sized to hold them.
func (r *Renderer) compartments(v gw.VertexView, kind gw.VertexKindView, labelW float64, opts Options) {
	o := r.o
	top, room := float64(-v.Size().H/2), gw.TitleRoom(kind)
	if kind.Keyword() != "" {
		r.label("keyword", kind.Keyword(), 0, top+gw.CompartmentTitle-8, labelW, opts)
	}
	r.formatted("label", kind.LabelFormat(), v.Label(), v.Attrs(), 0, top+room-8, labelW, "middle", opts)
	fixed := opts
	fixed.CharWidth = gw.CharWidth
	left, w, y := float64(-v.Size().W/2), v.Size().W, top+room
	// bound: the kind's compartments
	for c := range kind.Compartments().Len() {
		key := kind.Compartments().At(c)
		val := v.Attrs().Value(key)
		if val == "" {
			continue
		}
		if !r.expand {
			o.str(`<g class="compartment" data-key="`)
			o.attr(key)
			o.str(`"><line`)
			o.numAttr("x1", left)
			o.numAttr("y1", y)
			o.numAttr("x2", left+w)
			o.numAttr("y2", y)
			o.str(`/>`)
		}
		y += gw.CompartmentPad
		// bound: the value's lines, at most its length
		for more := true; more; {
			var line string
			line, val, more = gw.NextLine(val)
			y += gw.CompartmentLine
			r.labelAt("entry", line, left+gw.CompartmentInset, y-3, w-2*gw.CompartmentInset, "start", fixed)
		}
		y += gw.CompartmentPad
		if !r.expand {
			o.str(`</g>`)
		}
	}
}

// title draws a container's label, its title, in the room at its top,
// after its kind's keyword if it has one.
func (r *Renderer) title(v gw.VertexView, kind gw.VertexKindView, opts Options) {
	keyword := kind.Keyword()
	x, y, w := float64(-v.Size().W/2)+8, float64(-v.Size().H/2)+gw.ContainerTitle-6, v.Size().W-16
	if keyword != "" {
		r.labelAt("keyword", keyword, x, y, w, "start", opts)
		// A keyword is drawn smaller than a label, at about nine
		// tenths of its width a character, and half a character apart.
		kw := math.Min(w, float64(float64(runeCount(keyword))*float64(opts.CharWidth*0.9))+float64(opts.CharWidth/2))
		x, w = x+kw, w-kw
	}
	r.formatted("label", kind.LabelFormat(), v.Label(), v.Attrs(), x, y, w, "start", opts)
}

// shapeClass is the class of a vertex's own shape, as against the
// overlay's ring.
const shapeClass = "shape"

// shape writes the vertex's shape centred on the origin, with the class.
func (r *Renderer) shape(class string, shape gw.Shape, s gw.Size) {
	o := r.o
	w, h := s.W, s.H
	switch shape {
	case gw.Rect, gw.RoundRect, gw.Pill:
		o.str(`<rect class="`)
		o.str(class)
		o.byte('"')
		o.numAttr("x", -w/2)
		o.numAttr("y", -h/2)
		o.numAttr("width", w)
		o.numAttr("height", h)
		if shape == gw.RoundRect {
			o.numAttr("rx", 6)
		}
		if shape == gw.Pill {
			o.numAttr("rx", h/2)
		}
		o.str(`/>`)
	case gw.Diamond:
		o.str(`<polygon class="`)
		o.str(class)
		o.str(`" points="`)
		o.point(0, -h/2)
		o.byte(' ')
		o.point(w/2, 0)
		o.byte(' ')
		o.point(0, h/2)
		o.byte(' ')
		o.point(-w/2, 0)
		o.str(`"/>`)
	case gw.Circle:
		o.str(`<ellipse class="`)
		o.str(class)
		o.str(`" cx="0" cy="0"`)
		o.numAttr("rx", w/2)
		o.numAttr("ry", h/2)
		o.str(`/>`)
	case gw.Actor, gw.DataStore:
		// Their box, filled for the hit and the ring, with the
		// figure's strokes over it.
		o.str(`<rect class="`)
		o.str(class)
		if class == shapeClass && shape == gw.Actor {
			o.str(` box`)
		} else if class == shapeClass {
			o.str(` store`)
		}
		o.byte('"')
		o.numAttr("x", -w/2)
		o.numAttr("y", -h/2)
		o.numAttr("width", w)
		o.numAttr("height", h)
		o.str(`/>`)
		if class == shapeClass {
			r.figure(shape, s)
		}
	}
}

// actorLabel is the height at the foot of an actor's box kept for its
// label.
const actorLabel = 16

// figure writes the strokes of an actor or a data store over its box:
// a stick figure above the label, or the two lines that close a store
// above and below.
func (r *Renderer) figure(shape gw.Shape, s gw.Size) {
	o := r.o
	w, h := s.W, s.H
	if shape == gw.DataStore {
		o.str(`<path class="figure" d="M `)
		o.point(-w/2, -h/2)
		o.str(` H `)
		o.num(w / 2)
		o.str(` M `)
		o.point(-w/2, h/2)
		o.str(` H `)
		o.num(w / 2)
		o.str(`"/>`)
		return
	}
	fh := h - actorLabel
	if fh <= 0 {
		fh = h
	}
	top := float64(-h / 2)
	head := float64(math.Min(w, fh) * 0.22)
	neck, hip, foot := top+float64(2*head), top+float64(fh*0.62), top+fh
	o.str(`<circle class="figure head"`)
	o.numAttr("cx", 0)
	o.numAttr("cy", top+head)
	o.numAttr("r", head)
	o.str(`/><path class="figure" d="M `)
	o.point(0, neck)
	o.str(` V `)
	o.num(hip)
	o.str(` M `)
	o.point(-w/2, neck+float64(fh*0.12))
	o.str(` H `)
	o.num(w / 2)
	o.str(` M `)
	o.point(-w/2*0.8, foot)
	o.str(` L `)
	o.point(0, hip)
	o.str(` L `)
	o.point(w/2*0.8, foot)
	o.str(`"/>`)
}

// label writes a centred text element. A label wider than width (when
// width is positive) is cut with an ellipsis at the estimated character
// width, and marked cut; the full text follows it, marked full and not
// displayed, for the stylesheet to show in its place on hover, and then
// goes into a title child, which is also the tooltip.
func (r *Renderer) label(class, text string, x, y, width float64, opts Options) {
	r.labelAt(class, text, x, y, width, "middle", opts)
}

// labelAt is label anchored at the start, middle or end of the text.
func (r *Renderer) labelAt(class, text string, x, y, width float64, anchor string, opts Options) {
	r.formatted(class, "", text, gw.AttrList{}, x, y, width, anchor, opts)
}

// formatted is labelAt for the label a kind's format draws from the
// label as set and the attributes, written part by part, with nothing
// built.
func (r *Renderer) formatted(class, format, text string, attrs gw.AttrList, x, y, width float64, anchor string, opts Options) {
	o := r.o
	// The fit is converted only below the text's length, so that it is
	// within an int on every target.
	runes := gw.LabelRunes(format, text, attrs)
	fit := runes
	if n := width / opts.CharWidth; width > 0 && n < float64(fit) {
		fit = int(n)
	}
	if r.expand {
		if runes > fit {
			r.expanded(class, format, text, attrs, x, y, runes, anchor, opts)
		}
		return
	}
	if runes <= fit {
		r.textOpen(class, "", x, y, anchor)
		r.parts(format, text, attrs, runes)
		o.str(`</text>`)
		return
	}
	if opts.NoFullText {
		r.textOpen(class, "", x, y, anchor)
	} else {
		r.textOpen(class, cutMark, x, y, anchor)
	}
	r.parts(format, text, attrs, max(fit-1, 0))
	o.str("…</text>")
	if !opts.NoFullText {
		r.textOpen(class, fullMark, x, y, anchor)
		r.parts(format, text, attrs, runes)
		o.str(`</text>`)
	}
	o.str(`<title>`)
	r.parts(format, text, attrs, runes)
	o.str(`</title>`)
}

// The marks of a cut text and of the full text written after it.
const (
	cutMark  = "cut"
	fullMark = "full"
)

// textOpen opens a text element of the class and the mark, if any, at
// the point and anchor; a full text is not displayed unless the
// stylesheet displays it, so that without one only the cut text shows.
func (r *Renderer) textOpen(class, mark string, x, y float64, anchor string) {
	o := r.o
	o.str(`<text class="`)
	o.str(class)
	if mark != "" {
		o.byte(' ')
		o.str(mark)
	}
	o.byte('"')
	if mark == fullMark {
		o.str(` display="none"`)
	}
	o.numAttr("x", x)
	o.numAttr("y", y)
	o.str(` text-anchor="`)
	o.str(anchor)
	o.str(`">`)
}

// expanded writes a cut text in full for the overlay, at its place in
// the hovered vertex, over a box as wide as its estimated width, the
// width it was cut by.
func (r *Renderer) expanded(class, format, text string, attrs gw.AttrList, x, y float64, runes int, anchor string, opts Options) {
	o := r.o
	w := float64(float64(runes)*opts.CharWidth) + 4
	left := x - 2
	switch anchor {
	case "middle":
		left = x - float64(w/2)
	case anchorEnd:
		left = x + 2 - w
	}
	o.str(`<g class="expanded" transform="translate(`)
	o.point(r.centre.X, r.centre.Y)
	o.str(`)">`)
	r.rect(`<rect class="backing"`, gw.Viewport{X: left, Y: y - opts.FontSize, W: w, H: opts.FontSize + 4})
	r.textOpen(class, "", x, y, anchor)
	r.parts(format, text, attrs, runes)
	o.str(`</text></g>`)
}

// parts writes the first n characters of the label a format draws.
func (r *Renderer) parts(format, label string, attrs gw.AttrList, n int) {
	// bound: the format's parts, at most its length
	for more := true; more; {
		var before, value, after string
		before, value, after, format = gw.NextLabelPart(format, label, attrs)
		for _, s := range [3]string{before, value, after} {
			r.textRunes(s, n)
			n -= min(runeCount(s), n)
		}
		more = format != ""
	}
}

// runeCount counts the characters of a string.
func runeCount(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i]&0xC0 != 0x80 {
			n++
		}
	}
	return n
}

// textRunes writes the first n characters of a string, escaped.
func (r *Renderer) textRunes(s string, n int) {
	o := r.o
	seen := 0
	for i := 0; i < len(s); i++ {
		if s[i]&0xC0 != 0x80 {
			if seen == n {
				return
			}
			seen++
		}
		if nonchar(s, i) {
			i += 2
			continue
		}
		o.textByte(s[i])
	}
}

// ports writes a vertex's ports as circles on its sides.
func (r *Renderer) ports(d *gw.Document, vi gw.VertexIndex) {
	v := d.Vertex(vi)
	if v.Ports().Len() == 0 {
		return
	}
	o := r.o
	o.str(`<g class="ports">`)
	for i := range v.Ports().Len() {
		p := portAt(v, gw.PortIndex(i)) //nolint:gosec // bounded by the ports per vertex
		o.str(`<circle class="port" data-port="`)
		o.attr(v.Ports().At(i).Name)
		o.byte('"')
		o.numAttr("cx", p.X-v.Pos().X)
		o.numAttr("cy", p.Y-v.Pos().Y)
		o.numAttr("r", portRadius)
		o.str(`/>`)
	}
	// A vertex's own ports are named beside them; its kind's are
	// known from the kind.
	// bound: the vertex's own ports, at most the ports per vertex
	for i := d.Kinds().Vertex(v.Kind()).Ports().Len(); i < v.Ports().Len(); i++ {
		x, y, anchor, _ := portLabel(d, vi, gw.PortIndex(i)) //nolint:gosec // bounded by the ports per vertex
		o.str(`<text class="port-label"`)
		o.numAttr("x", x-v.Pos().X)
		o.numAttr("y", y-v.Pos().Y)
		o.str(` text-anchor="`)
		o.str(anchor)
		o.str(`">`)
		o.text(v.Ports().At(i).Name)
		o.str(`</text>`)
	}
	o.str(`</g>`)
}

// dataAttrs writes an element's attributes as data-* attributes.
func (r *Renderer) dataAttrs(attrs gw.AttrList) {
	o := r.o
	for i := range attrs.Len() {
		a := attrs.At(i)
		o.str(` data-`)
		o.name(a.Key)
		o.str(`="`)
		o.attr(a.Value)
		o.byte('"')
	}
}

// overlay writes the selection outlines in edit mode, and the hovered
// vertex's cut texts in full in either mode; an empty group otherwise.
func (r *Renderer) overlay(d *gw.Document, opts Options, maxW float64) {
	o := r.o
	o.str(`<g class="overlay">`)
	if opts.Mode == Edit {
		r.selection(d, opts, maxW)
		if opts.HasHandle && vertexLive(d, opts.Handle) {
			r.rect(`<rect class="handle"`, HandleBox(d, opts.Handle))
		}
		if opts.HasGhost {
			o.str(`<path class="ghost" d="M `)
			o.point(opts.Ghost[0].X, opts.Ghost[0].Y)
			o.str(` L `)
			o.point(opts.Ghost[1].X, opts.Ghost[1].Y)
			o.str(`"/>`)
		}
		if opts.HasTarget && vertexLive(d, opts.Target) {
			r.target(d, d.Vertex(opts.Target))
		}
		if opts.HasMarquee {
			r.rect(`<rect class="marquee"`, opts.Marquee)
		}
	}
	// The hovered vertex's cut texts are drawn in full over everything,
	// in either mode.
	if opts.HasHover && vertexLive(d, opts.Hover) {
		r.expand, r.centre = true, d.Vertex(opts.Hover).Pos()
		r.texts(d, opts, opts.Hover)
		r.expand = false
	}
	o.str(`</g>`)
}

// selection outlines each selected vertex and lays a halo over each
// selected edge, as wide as its stroke and 6 more.
func (r *Renderer) selection(d *gw.Document, opts Options, maxW float64) {
	if opts.Selection == nil {
		return
	}
	o := r.o
	nv, ne := opts.Selection.SelectedCount()
	nv, ne = min(nv, d.VertexSlots()), min(ne, d.EdgeSlots())
	// bound: the selected vertices, at most the document's capacity
	for i := range nv {
		v := opts.Selection.SelectedVertex(i)
		if !vertexLive(d, v) {
			continue
		}
		vx := d.Vertex(v)
		o.str(`<rect class="selection"`)
		o.numAttr("x", vx.Pos().X-float64(vx.Size().W/2)-3)
		o.numAttr("y", vx.Pos().Y-float64(vx.Size().H/2)-3)
		o.numAttr("width", vx.Size().W+6)
		o.numAttr("height", vx.Size().H+6)
		o.str(`/>`)
	}
	// bound: the selected edges, at most the document's capacity
	for i := range ne {
		e := opts.Selection.SelectedEdge(i)
		if !edgeLive(d, e) {
			continue
		}
		ed := d.Edge(e)
		kind := d.Kinds().Edge(ed.Kind())
		var lane [1]gw.Point
		w := wayOf(d, e, &lane)
		r.path(w.f, kind, w.a, w.through, w.shortened(kind))
		o.str(` class="selection"`)
		o.numAttr("stroke-width", strokeWidth(d, opts, int(e), maxW)+6)
		o.str(`/>`)
	}
}

// rect writes a rectangle, opened by start, over the box.
func (r *Renderer) rect(start string, b gw.Viewport) {
	o := r.o
	o.str(start)
	o.numAttr("x", b.X)
	o.numAttr("y", b.Y)
	o.numAttr("width", b.W)
	o.numAttr("height", b.H)
	o.str(`/>`)
}

// target rings a vertex where an edge can start or end: its shape again,
// stroked as wide as the band the hit test takes as its outline.
func (r *Renderer) target(d *gw.Document, v gw.VertexView) {
	o := r.o
	o.str(`<g class="target" transform="translate(`)
	o.point(v.Pos().X, v.Pos().Y)
	o.str(`)"`)
	o.numAttr("stroke-width", 2*band(v))
	o.byte('>')
	r.shape("ring", d.Kinds().Vertex(v.Kind()).Shape(), v.Size())
	o.str(`</g>`)
}

func vertexLive(d *gw.Document, v gw.VertexIndex) bool {
	return v >= 0 && int(v) < d.VertexSlots() && d.Vertex(v).Live()
}

func edgeLive(d *gw.Document, e gw.EdgeIndex) bool {
	return e >= 0 && int(e) < d.EdgeSlots() && d.Edge(e).Live()
}
