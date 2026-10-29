package edit

import (
	"math"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/graphedwright/svg"
)

// pointerDown starts a pan, a connection, a drag or a marquee, by what
// is under the pointer; the gesture machine decides which.
func (e *Editor) pointerDown(ev Event) error {
	p := e.toDoc(ev.Point)
	hit := svg.HitTest(e.d, p)
	v, one := e.onlyVertex()
	ge := gesture.Event{
		Kind:   gesture.EvPress,
		Hit:    hitOf(hit),
		Pan:    ev.Button == 1 || ev.Modifiers&Alt != 0,
		Shift:  ev.Modifiers&Shift != 0,
		Handle: one && onHandle(svg.HandleBox(e.d, v), p),
	}
	return e.fire(ge, input{p: p, hit: hit, ev: ev})
}

// pointerMove follows the pointer: the hover, a pan, a drag or the
// overlay's ghost and marquee.
func (e *Editor) pointerMove(ev Event) error {
	err := e.fire(gesture.Event{Kind: gesture.EvMove}, input{p: e.toDoc(ev.Point), ev: ev})
	// A pan moved the viewport under the pointer, so the point in
	// document units is read again against the new viewport.
	e.last = e.toDoc(ev.Point)
	e.hover = svg.HitTest(e.d, e.last)
	return err
}

// pan moves the viewport by the pointer's move to p.
func (e *Editor) pan(p gw.Point) {
	e.d.View.X -= p.X - e.last.X
	e.d.View.Y -= p.Y - e.last.Y
	e.layout = true
}

// moveDrag moves the dragged vertices by the pointer's move to p.
func (e *Editor) moveDrag(p gw.Point) {
	for _, v := range e.dragV {
		pos := e.d.Vertices[v].Pos
		e.d.Place(v, gw.Point{X: pos.X + p.X - e.last.X, Y: pos.Y + p.Y - e.last.Y}, true)
		e.touch(v)
		e.changeE = e.d.EdgesAt(v, e.changeE)
	}
}

// touch records a vertex as changed.
func (e *Editor) touch(v gw.VertexIndex) {
	if len(e.changeV) < cap(e.changeV) {
		e.changeV = append(e.changeV, v)
	}
}

// pointerUp finishes what the pointer started.
func (e *Editor) pointerUp(ev Event) error {
	p := e.toDoc(ev.Point)
	e.last = p
	return e.fire(gesture.Event{Kind: gesture.EvRelease}, input{p: p, ev: ev})
}

// beginConnect remembers where an edge starts: a port, or an outline,
// from the point pressed, in the edge kind chosen from the palette.
func (e *Editor) beginConnect(p gw.Point, h svg.Hit) {
	e.connFrom, e.connPort, e.connKind = h.Vertex, gw.None, e.edgeKind
	if h.Kind == svg.PortHit {
		e.connPort = h.Port
		return
	}
	e.connAt = p
}

// startResize remembers the vertex being resized and its top left
// corner, which the resize keeps.
func (e *Editor) startResize() {
	v, ok := e.onlyVertex()
	if !ok {
		return
	}
	x := &e.d.Vertices[v]
	e.sizeV = v
	e.corner = gw.Point{X: x.Pos.X - float64(x.Size.W/2), Y: x.Pos.Y - float64(x.Size.H/2)}
}

// onHandle reports whether a point is on a resize handle, or within
// two units of it.
func onHandle(b gw.Viewport, p gw.Point) bool {
	return p.X >= b.X-2 && p.X <= b.X+b.W+2 && p.Y >= b.Y-2 && p.Y <= b.Y+b.H+2
}

// sizeBox is the box the vertex being resized would have: from its top
// left corner to the pointer, each side a whole number of grid units
// and at least one.
func (e *Editor) sizeBox() gw.Viewport {
	w := math.Max(e.grid, math.Round((e.last.X-e.corner.X)/e.grid)*e.grid)
	h := math.Max(e.grid, math.Round((e.last.Y-e.corner.Y)/e.grid)*e.grid)
	return gw.Viewport{X: e.corner.X, Y: e.corner.Y, W: w, H: h}
}

// endResize gives the vertex the size of the box the pointer drew,
// keeping its top left corner. A Resize keeps the centre, so a Move
// takes the vertex back to its corner, except a container that holds
// something: the fit puts its corner where what it holds does, and a
// Move would carry what it holds.
func (e *Editor) endResize() error {
	v := e.sizeV
	e.sizeV = gw.None
	if v == gw.None || !e.d.Vertices[v].Live {
		return nil
	}
	b := e.sizeBox()
	if (gw.Size{W: b.W, H: b.H}) == e.d.Vertices[v].Size {
		return nil
	}
	if err := e.apply(gw.Command{Op: gw.Resize, Vertex: v, Size: gw.Size{W: b.W, H: b.H}}); err != nil {
		return err
	}
	if e.holds(v) {
		return nil
	}
	return e.apply(gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: b.X + float64(b.W/2), Y: b.Y + float64(b.H/2)}})
}

// holds reports whether a live vertex is in v.
func (e *Editor) holds(v gw.VertexIndex) bool {
	for i := range e.d.Vertices {
		if e.d.Vertices[i].Live && e.d.Vertices[i].Parent == v {
			return true
		}
	}
	return false
}

// startDrag remembers where the dragged vertices were: the selected
// ones and what the selected containers hold.
func (e *Editor) startDrag() {
	e.dragV = e.dragV[:0]
	for i := range e.d.Vertices {
		v := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if e.d.Vertices[i].Live && e.dragged(v) {
			e.dragV = append(e.dragV, v)
			e.orig[v] = e.d.Vertices[i].Pos
		}
	}
}

// dragged reports whether a vertex is selected or in a selected one.
func (e *Editor) dragged(v gw.VertexIndex) bool {
	for _, s := range e.selV {
		if e.d.Within(v, s) {
			return true
		}
	}
	return false
}

// root reports whether a selected vertex is in no other selected one,
// so that its own Move carries it and what it holds.
func (e *Editor) root(s gw.VertexIndex) bool {
	for _, o := range e.selV {
		if o != s && e.d.Within(s, o) {
			return false
		}
	}
	return true
}

// restoreDrag puts the dragged vertices back where the drag began.
func (e *Editor) restoreDrag() {
	for _, v := range e.dragV {
		e.d.Place(v, e.orig[v], e.d.Vertices[v].Placed)
		e.touch(v)
	}
}

// endDrag commits a drag: the vertices go back to their origin and a
// Move command takes each selected one that no other selected one holds
// to its snapped position, carrying what it holds, in one undo group, so
// the whole drag is one step to undo. Where a vertex lands
// decides its container, judged by the containers as they stood while
// it was dragged, before any is fitted to where it lands: dropped in a
// container's box it goes in, dropped outside it comes out.
func (e *Editor) endDrag() error {
	e.d.Group() // the moves and new containers, one step to undo
	defer e.d.EndGroup()
	for _, v := range e.selV {
		if e.root(v) {
			e.dropTo[v] = svg.ContainerAt(e.d, e.d.Vertices[v].Pos, v)
		}
	}
	dx, dy := e.last.X-e.down.X, e.last.Y-e.down.Y
	e.restoreDrag()
	for _, v := range e.selV {
		if !e.root(v) {
			continue
		}
		to := e.snap(gw.Point{X: e.orig[v].X + dx, Y: e.orig[v].Y + dy})
		if err := e.apply(gw.Command{Op: gw.Move, Vertex: v, Pos: to}); err != nil {
			return err
		}
		if c := e.dropTo[v]; c != e.d.Vertices[v].Parent {
			if err := e.apply(gw.Command{Op: gw.SetParent, Vertex: v, To: c}); err != nil {
				return err
			}
		}
	}
	return nil
}

// endConnect adds the edge if the pointer is over a port, or a vertex
// without ports, the schema allows; else the edge is dropped and the
// refusal returned. A connection begun at an outline and released on
// its own vertex was a click: it selects the vertex.
func (e *Editor) endConnect(p gw.Point, shift bool) error {
	from, port, kind := e.connFrom, e.connPort, e.connKind
	e.connFrom, e.connPort, e.connKind = gw.None, gw.None, gw.None
	hit := svg.HitTest(e.d, p)
	toPort := gw.PortIndex(gw.None)
	if port == gw.None && hit.Vertex == from {
		e.selectVertex(from, shift)
		return nil
	}
	switch hit.Kind {
	case svg.PortHit:
		toPort = hit.Port
	case svg.OutlineHit:
	case svg.VertexHit:
		if len(e.d.Vertices[hit.Vertex].Ports) > 0 {
			return nil
		}
	case svg.EdgeHit, svg.Background:
		return nil
	}
	if kind == gw.None {
		kind = e.edgeKindFor(from, hit.Vertex)
		if kind == gw.None {
			return ErrNoEdgeKind
		}
	}
	return e.apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: hit.Vertex, FromPort: port, ToPort: toPort})
}

// joins reports whether an edge can join two vertices' kinds: of the
// connection's kind, or of any when it has none.
func (e *Editor) joins(from, to gw.VertexIndex) bool {
	if e.connKind == gw.None {
		return e.edgeKindFor(from, to) != gw.None
	}
	k := e.d.Schema.EdgeKinds[e.connKind]
	return kindIn(k.From, e.d.Vertices[from].Kind) && kindIn(k.To, e.d.Vertices[to].Kind)
}

// edgeKindFor is the first edge kind the schema allows between two
// vertices' kinds, or None.
func (e *Editor) edgeKindFor(from, to gw.VertexIndex) gw.KindIndex {
	fk, tk := e.d.Vertices[from].Kind, e.d.Vertices[to].Kind
	for i, k := range e.d.Schema.EdgeKinds {
		if kindIn(k.From, fk) && kindIn(k.To, tk) {
			return gw.KindIndex(i) //nolint:gosec // bounded by the schema
		}
	}
	return gw.None
}

func kindIn(list []gw.KindIndex, k gw.KindIndex) bool {
	if len(list) == 0 {
		return true
	}
	for _, x := range list {
		if x == k {
			return true
		}
	}
	return false
}

// place drops a vertex kind from the palette at the snapped point, in
// the container there that takes it.
func (e *Editor) place(p gw.Point) error {
	item := e.item
	e.item = gw.None
	at := e.inView(e.snap(p), item)
	if err := e.apply(gw.Command{Op: gw.AddVertex, Kind: item, Pos: at}); err != nil {
		return err
	}
	v := e.changeV[len(e.changeV)-1]
	e.d.Place(v, at, true)
	if c := svg.ContainerAt(e.d, at, v); c != gw.None {
		if err := e.apply(gw.Command{Op: gw.SetParent, Vertex: v, To: c}); err != nil {
			return err
		}
	}
	e.selectVertex(v, false)
	return nil
}

// inView moves a new vertex's centre so that a vertex of the kind sits
// wholly inside the viewport, when there is one.
func (e *Editor) inView(p gw.Point, kind gw.KindIndex) gw.Point {
	v := e.d.View
	size := e.d.Schema.VertexKinds[kind].Size
	if v.W <= 0 || v.H <= 0 || size.W > v.W || size.H > v.H {
		return p
	}
	p.X = math.Min(math.Max(p.X, v.X+float64(size.W/2)), v.X+v.W-float64(size.W/2))
	p.Y = math.Min(math.Max(p.Y, v.Y+float64(size.H/2)), v.Y+v.H-float64(size.H/2))
	return p
}

// zoomStep is the factor one wheel notch of 100 units zooms by.
const zoomStep = 1.1

// wheel pans the viewport by the scroll, or with Ctrl zooms it about
// the pointer.
func (e *Editor) wheel(ev Event) {
	if ev.Modifiers&Ctrl != 0 {
		e.zoom(ev)
		return
	}
	v := &e.d.View
	if v.W <= 0 || v.H <= 0 {
		return
	}
	// A scroll of the element's height moves one viewport; the editor
	// has no element size, so a wheel unit is a document unit.
	v.X += ev.DeltaX
	v.Y += ev.Delta
	e.layout = true
}

// keyZoom is the factor a zoom key, or a host's zoom button, zooms by.
const keyZoom = 1.25

// minView is the least width and height a zoom gives the viewport.
const minView = 1

// zoom scales the viewport about the pointer.
func (e *Editor) zoom(ev Event) {
	e.scale(math.Pow(zoomStep, ev.Delta/100), e.toDoc(ev.Point))
}

// zoomBy scales the viewport about its centre, in by the factor when
// it is above 1.
func (e *Editor) zoomBy(f float64) {
	v := e.d.View
	e.scale(1/f, gw.Point{X: v.X + float64(v.W/2), Y: v.Y + float64(v.H/2)})
}

// scale multiplies the viewport's size by f about the point, which
// stays where it is on the screen. A viewport narrower or lower than
// minView, or wider or higher than gw.MaxCoord, is not taken.
func (e *Editor) scale(f float64, p gw.Point) {
	v := &e.d.View
	if v.W <= 0 || v.H <= 0 || !(f > 0) || math.IsInf(f, 0) {
		return
	}
	w, h := v.W*f, v.H*f
	if w < minView || h < minView || w > gw.MaxCoord || h > gw.MaxCoord {
		return
	}
	v.X, v.Y = p.X-float64((p.X-v.X)*f), p.Y-float64((p.Y-v.Y)*f)
	v.W, v.H = w, h
	e.layout = true
}

// fit sets the viewport to the drawing's bounds, when it holds anything.
func (e *Editor) fit() {
	if e.d.LiveVertices() == 0 {
		return
	}
	e.d.View = svg.Bounds(e.d)
	e.layout = true
}
