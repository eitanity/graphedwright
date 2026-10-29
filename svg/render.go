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
	SelectedV     []gw.VertexIndex
	SelectedE     []gw.EdgeIndex
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
	// IDPrefix goes before the IDs the renderer makes, the title's and
	// the arrowheads', so that two drawings in one page do not share
	// them; DocIDPrefix goes before each vertex's and edge's ID, so that
	// a document cannot take an ID the page uses. Both are written as
	// tokens: letters, digits, '-' and '_', anything else as '_'.
	IDPrefix    string
	DocIDPrefix string
}

// Renderer holds the output buffer; one can render any number of
// documents, one at a time, without allocating.
type Renderer struct {
	o *out
}

// Render writes the whole document.
func (r *Renderer) Render(d *gw.Document, opts Options, w io.Writer) error {
	o := r.o
	o.reset(w)
	opts = opts.filled()
	view := d.View
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
	for _, v := range d.Containers() {
		r.vertex(d, opts, v)
	}
	o.str(`</g><g class="edges">`)
	for i := range d.Edges {
		if d.Edges[i].Live {
			r.edge(d, opts, gw.EdgeIndex(i), maxW) //nolint:gosec // bounded by the capacity
		}
	}
	o.str(`</g><g class="vertices">`)
	for i := range d.Vertices {
		if d.Vertices[i].Live && !d.Schema.VertexKinds[d.Vertices[i].Kind].Container {
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
// named by the kind's index.
func (r *Renderer) defs(d *gw.Document, prefix string) {
	o := r.o
	o.str(`<defs>`)
	for i, k := range d.Schema.EdgeKinds {
		if !k.Arrow {
			continue
		}
		o.str(`<marker id="`)
		o.name(prefix)
		o.str(`arrow-`)
		o.int(i)
		o.str(`" class="arrowhead `)
		o.name(k.Name)
		switch k.Head {
		case gw.FilledHead:
			o.str(`" viewBox="0 0 10 10" refX="1" refY="5" markerWidth="10" markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/></marker>`)
		case gw.OpenHead:
			// The edge runs to the tip, so the tip is the reference.
			o.str(` open" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="10" markerHeight="10" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10"/></marker>`)
		case gw.HollowHead:
			o.str(` hollow" viewBox="-1 -1 12 12" refX="1" refY="5" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z"/></marker>`)
		}
	}
	o.str(`</defs>`)
}

// maxWeight is the largest weight among live weighted edges and their
// annotations, for the stroke scale; 1 when there is none.
func maxWeight(d *gw.Document, opts Options) float64 {
	m := 0.0
	for i := range d.Edges {
		e := &d.Edges[i]
		if !e.Live || !d.Schema.EdgeKinds[e.Kind].Weighted {
			continue
		}
		w := e.Weight
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
	e := &d.Edges[i]
	if !d.Schema.EdgeKinds[e.Kind].Weighted {
		return opts.DefaultWeight
	}
	w := e.Weight
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
	e := &d.Edges[i]
	kind := d.Schema.EdgeKinds[e.Kind]
	note := noteOf(opts.EdgeNotes, int(i))
	o.str(`<g class="edge `)
	o.name(kind.Name)
	if kind.Dashed {
		o.str(" dashed")
	}
	if note.Class != "" {
		o.byte(' ')
		o.name(note.Class)
	}
	o.str(`" id="`)
	o.name(opts.DocIDPrefix)
	o.attr(string(e.ID))
	o.str(`" data-kind="`)
	o.attr(kind.Name)
	o.str(`" data-from="`)
	o.attr(string(d.Vertices[e.From].ID))
	o.str(`" data-to="`)
	o.attr(string(d.Vertices[e.To].ID))
	o.byte('"')
	r.dataAttrs(e.Attrs)
	o.byte('>')
	var lane [1]gw.Point
	w := wayOf(d, i, &lane)
	f := w.f
	r.path(f, kind, w.a, w.through, w.shortened(kind))
	width := strokeWidth(d, opts, int(i), maxW)
	o.numAttr("stroke-width", width)
	if kind.Arrow {
		o.str(` marker-end="url(#`)
		o.name(opts.IDPrefix)
		o.str(`arrow-`)
		o.int(int(e.Kind))
		o.str(`)"`)
	}
	o.str(`/>`)
	mid := w.middle(kind.Route)
	if f.down {
		r.besideLabels(kind, e, note, mid, width, opts)
		o.str(`</g>`)
		return
	}
	// The stereotype and then the label sit above the stroke, the
	// annotation below it.
	above := mid.Y - float64(width/2) - 4
	if kind.Stereotype != "" {
		r.label("stereotype", kind.Stereotype, mid.X, above, 0, opts)
		above -= opts.FontSize
	}
	if e.Label != "" {
		r.label("label", e.Label, mid.X, above, 0, opts)
	}
	if note.Text != "" {
		r.label("annotation", note.Text, mid.X, mid.Y+float64(width/2)+opts.FontSize, 0, opts)
	}
	o.str(`</g>`)
}

// besideLabels writes an edge's texts for a top-to-bottom flow, whose
// edges mostly run down: the label to the right of the stroke at the
// middle, the stereotype above the label, the annotation to the left.
func (r *Renderer) besideLabels(kind gw.EdgeKind, e *gw.Edge, note Annotation, mid gw.Point, width float64, opts Options) {
	x, y := mid.X+float64(width/2)+4, mid.Y+opts.FontSize/3
	above := y
	if e.Label != "" {
		r.labelAt("label", e.Label, x, y, 0, "start", opts)
		above -= opts.FontSize
	}
	if kind.Stereotype != "" {
		r.labelAt("stereotype", kind.Stereotype, x, above, 0, "start", opts)
	}
	if note.Text != "" {
		r.labelAt("annotation", note.Text, mid.X-float64(width/2)-4, y, 0, "end", opts)
	}
}

// path writes the opening of an edge's path element with its route
// from a through the bends to b, leg by leg, each worked out in the
// frame and written back in the document's axes.
func (r *Renderer) path(f frame, kind gw.EdgeKind, a gw.Point, bends []gw.Point, b gw.Point) {
	o := r.o
	o.str(`<path d="M `)
	o.point(a.X, a.Y)
	across, down := ` H `, ` V `
	if f.down {
		across, down = down, across
	}
	for k := range len(bends) + 1 {
		p, q := leg(a, bends, b, k)
		p, q = f.t(p), f.t(q)
		switch legRoute(kind.Route, len(bends) > 0, p, q) {
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
// shape, the label with the full text as its title, the annotation,
// and the ports.
func (r *Renderer) vertex(d *gw.Document, opts Options, i gw.VertexIndex) {
	o := r.o
	v := &d.Vertices[i]
	kind := d.Schema.VertexKinds[v.Kind]
	note := noteOf(opts.VertexNotes, int(i))
	o.str(`<g class="vertex `)
	o.name(kind.Name)
	if kind.Container {
		o.str(" container")
	}
	if kind.Dashed {
		o.str(" dashed")
	}
	if note.Class != "" {
		o.byte(' ')
		o.name(note.Class)
	}
	if !v.Placed {
		o.str(" unplaced")
	}
	o.str(`" id="`)
	o.name(opts.DocIDPrefix)
	o.attr(string(v.ID))
	o.str(`" data-kind="`)
	o.attr(kind.Name)
	o.str(`" transform="translate(`)
	o.point(v.Pos.X, v.Pos.Y)
	o.str(`)"`)
	r.dataAttrs(v.Attrs)
	o.byte('>')
	r.shape(shapeClass, kind.Shape, v.Size)
	labelY, labelW := 4.0, v.Size.W-8
	if note.Text != "" {
		labelY = -2
	}
	if kind.Shape == gw.Actor {
		// An actor's label is at the foot of its box, under the
		// figure, as wide as it needs.
		labelY, labelW = float64(v.Size.H/2)-4, 0
	}
	if kind.Container {
		// A container's label is its title, in the room at its top.
		r.labelAt("label", v.Label, float64(-v.Size.W/2)+8, float64(-v.Size.H/2)+gw.ContainerTitle-6, v.Size.W-16, "start", opts)
	} else {
		r.label("label", v.Label, 0, labelY, labelW, opts)
	}
	if note.Text != "" {
		r.label("annotation", note.Text, 0, labelY+opts.FontSize, v.Size.W-8, opts)
	}
	r.ports(v)
	o.str(`</g>`)
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
// width, and the full text goes into a title child, which is also the
// tooltip.
func (r *Renderer) label(class, text string, x, y, width float64, opts Options) {
	r.labelAt(class, text, x, y, width, "middle", opts)
}

// labelAt is label anchored at the start, middle or end of the text.
func (r *Renderer) labelAt(class, text string, x, y, width float64, anchor string, opts Options) {
	o := r.o
	o.str(`<text class="`)
	o.str(class)
	o.byte('"')
	o.numAttr("x", x)
	o.numAttr("y", y)
	o.str(` text-anchor="`)
	o.str(anchor)
	o.str(`">`)
	// The fit is converted only below the text's length, so that it is
	// within an int on every target.
	fit := len(text)
	if n := width / opts.CharWidth; width > 0 && n < float64(fit) {
		fit = int(n)
	}
	runes := runeCount(text)
	if runes <= fit {
		o.text(text)
		o.str(`</text>`)
		return
	}
	keep := fit - 1
	if keep < 0 {
		keep = 0
	}
	r.textRunes(text, keep)
	o.str("…</text>")
	o.str(`<title>`)
	o.text(text)
	o.str(`</title>`)
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
func (r *Renderer) ports(v *gw.Vertex) {
	if len(v.Ports) == 0 {
		return
	}
	o := r.o
	o.str(`<g class="ports">`)
	for i := range v.Ports {
		p := portAt(v, gw.PortIndex(i)) //nolint:gosec // bounded by the ports per vertex
		o.str(`<circle class="port" data-port="`)
		o.attr(v.Ports[i].Name)
		o.byte('"')
		o.numAttr("cx", p.X-v.Pos.X)
		o.numAttr("cy", p.Y-v.Pos.Y)
		o.numAttr("r", portRadius)
		o.str(`/>`)
	}
	o.str(`</g>`)
}

// dataAttrs writes an element's attributes as data-* attributes.
func (r *Renderer) dataAttrs(attrs []gw.Attr) {
	o := r.o
	for _, a := range attrs {
		o.str(` data-`)
		o.name(a.Key)
		o.str(`="`)
		o.attr(a.Value)
		o.byte('"')
	}
}

// overlay writes the selection outlines in edit mode, and an empty
// group otherwise.
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
			r.target(d, &d.Vertices[opts.Target])
		}
		if opts.HasMarquee {
			r.rect(`<rect class="marquee"`, opts.Marquee)
		}
	}
	o.str(`</g>`)
}

// selection outlines each selected vertex and lays a halo over each
// selected edge, as wide as its stroke and 6 more.
func (r *Renderer) selection(d *gw.Document, opts Options, maxW float64) {
	o := r.o
	for _, v := range opts.SelectedV {
		if !vertexLive(d, v) {
			continue
		}
		vx := &d.Vertices[v]
		o.str(`<rect class="selection"`)
		o.numAttr("x", vx.Pos.X-float64(vx.Size.W/2)-3)
		o.numAttr("y", vx.Pos.Y-float64(vx.Size.H/2)-3)
		o.numAttr("width", vx.Size.W+6)
		o.numAttr("height", vx.Size.H+6)
		o.str(`/>`)
	}
	for _, e := range opts.SelectedE {
		if !edgeLive(d, e) {
			continue
		}
		ed := &d.Edges[e]
		kind := d.Schema.EdgeKinds[ed.Kind]
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
func (r *Renderer) target(d *gw.Document, v *gw.Vertex) {
	o := r.o
	o.str(`<g class="target" transform="translate(`)
	o.point(v.Pos.X, v.Pos.Y)
	o.str(`)"`)
	o.numAttr("stroke-width", 2*band(v))
	o.byte('>')
	r.shape("ring", d.Schema.VertexKinds[v.Kind].Shape, v.Size)
	o.str(`</g>`)
}

func vertexLive(d *gw.Document, v gw.VertexIndex) bool {
	return v >= 0 && int(v) < len(d.Vertices) && d.Vertices[v].Live
}

func edgeLive(d *gw.Document, e gw.EdgeIndex) bool {
	return e >= 0 && int(e) < len(d.Edges) && d.Edges[e].Live
}
