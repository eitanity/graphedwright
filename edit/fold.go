package edit

import (
	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// A large drawing is read by finding a vertex and bringing it into
// view (Reveal), by focusing on the selection's neighbourhood (Focus),
// the rest dimmed or not drawn, and by folding containers into one
// vertex, which are commands (gw.Fold and gw.Unfold) and so undo. The
// focus and the view are not undone, as a selection, a pan or a zoom is
// not.

// Reveal makes vertex v seen: it unfolds every folded container it is
// in, one step for Undo, selects it alone, and pans the view to centre
// it, zooming out only where it is wider or higher than the view. The
// unfolds are the host's, made in either mode as the host's own commands
// are. It is refused for a vertex that is not live, and while a gesture
// is under way, which a change of the document under it would break.
func (e *Editor) Reveal(v gw.VertexIndex) (gw.Change, error) {
	e.resetChange()
	if !e.d.Vertex(v).Live() {
		return gw.Change{}, errNotLive
	}
	if !e.idle() {
		return gw.Change{}, errBusy
	}
	if e.d.Hidden(v) {
		e.d.Group()
		// bound: each unfold takes the outermost fold, a container v is
		// in, so at most its depth
		for n := 0; e.d.Hidden(v) && n <= e.d.VertexSlots(); n++ {
			ch, err := e.d.Apply(gw.Command{Op: gw.Unfold, Vertex: e.d.DrawnAs(v)})
			if err != nil {
				e.d.EndGroup()
				return gw.Change{}, err
			}
			e.record(ch)
		}
		e.d.EndGroup()
	}
	e.clearSelection()
	e.selV = push(e.selV, v)
	e.centre(v)
	return gw.Change{Vertices: e.changeV, Edges: e.changeE, Layout: e.layout}, nil
}

// centre pans the view to centre vertex v, and zooms out, about its
// centre, as far as it takes for v to fit.
func (e *Editor) centre(v gw.VertexIndex) {
	view, x := e.d.View(), e.d.Vertex(v)
	if view.W <= 0 || view.H <= 0 {
		e.fit()
		return
	}
	f := max(1, x.Size().W/view.W, x.Size().H/view.H)
	view.W, view.H = view.W*f, view.H*f
	view.X, view.Y = x.Pos().X-float64(view.W/2), x.Pos().Y-float64(view.H/2)
	e.d.SetView(view)
	e.layout = true
}

// Focus focuses on the selection's neighbourhood: the vertices within
// depth edges of it, the way given, and those a selected fold holds,
// at depth 0 the selection alone. A fold is in the focus when anything
// it holds is. The rest is drawn dimmed, or with SetFocusHide not at
// all, and then not hit either. A vertex added while the focus holds is
// in it. It reports false, and changes nothing, when nothing is
// selected or the depth is below 0.
func (e *Editor) Focus(depth int, way gw.Direction) bool {
	if len(e.selV) == 0 && len(e.selE) == 0 || depth < 0 {
		return false
	}
	// The seeds are marked as they are gathered, so that each is listed
	// once and the list stays within the vertex capacity.
	clear(e.focus)
	from := e.focusFrom[:0]
	for _, v := range e.selV {
		from = e.seed(from, v)
	}
	for _, x := range e.selE {
		from = e.seed(from, e.d.Edge(x).From())
		from = e.seed(from, e.d.Edge(x).To())
	}
	// What a selected fold holds is where its edges are drawn from.
	for i := range e.d.VertexSlots() {
		u := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if f := e.d.DrawnAs(u); f != u && e.isSelectedV(f) {
			from = e.seed(from, u)
		}
	}
	e.d.Around(from, depth, way, e.focus)
	for i := range e.d.VertexSlots() {
		if u := gw.VertexIndex(i); e.focus[i] { //nolint:gosec // bounded by the capacity
			e.focus[e.d.DrawnAs(u)] = true
		}
	}
	e.focused, e.focusN = true, e.d.VertexSlots()
	return true
}

// seed marks live vertex v in the focus and lists it in from, once.
// It is a method, not a closure, which TinyGo would make on the heap.
func (e *Editor) seed(from []gw.VertexIndex, v gw.VertexIndex) []gw.VertexIndex {
	if e.d.Vertex(v).Live() && !e.focus[v] {
		e.focus[v] = true
		from = push(from, v)
	}
	return from
}

// Unfocus ends the focus, so that everything is drawn as it is.
func (e *Editor) Unfocus() { e.focused = false }

// Focused reports whether a focus holds.
func (e *Editor) Focused() bool { return e.focused }

// InFocus reports whether vertex v is in the focus, or no focus holds.
func (e *Editor) InFocus(v gw.VertexIndex) bool {
	if !e.focused {
		return true
	}
	if v < 0 || int(v) >= e.d.VertexSlots() {
		return false
	}
	return int(v) >= e.focusN || e.focus[v]
}

// SetFocusHide draws what is out of the focus not at all, rather than
// dimmed, and keeps it from being hit.
func (e *Editor) SetFocusHide(hide bool) { e.focusHide = hide }

// focusOptions gives the overlay's options the focus: the vertices
// added since it was taken are in it.
func (e *Editor) focusOptions(opts *svg.Options) {
	opts.Focus, opts.FocusHide = nil, false
	if !e.focused {
		return
	}
	n := e.d.VertexSlots()
	// bound: the vertex slots
	for i := e.focusN; i < n; i++ {
		e.focus[i] = true
	}
	e.focusN = n
	opts.Focus, opts.FocusHide = e.focus[:n], e.focusHide
}

// hitAt is what is drawn at document point p: the hit test's, but for
// what a focus that hides leaves undrawn, which is the background.
func (e *Editor) hitAt(p gw.Point) svg.Hit {
	h := svg.HitTest(e.d, p)
	if !e.focused || !e.focusHide {
		return h
	}
	switch h.Kind {
	case svg.VertexHit, svg.OutlineHit, svg.PortHit:
		if !e.InFocus(h.Vertex) {
			return background()
		}
	case svg.EdgeHit:
		x := e.d.Edge(h.Edge)
		if !e.InFocus(e.d.DrawnAs(x.From())) || !e.InFocus(e.d.DrawnAs(x.To())) {
			return background()
		}
	case svg.Background:
	}
	return h
}

// foldKey folds the one selected container, or unfolds it, as one
// command, for Ctrl with [ or ].
func (e *Editor) foldKey(op gw.Op) error {
	if e.g.Held() {
		return ErrHeld
	}
	v, ok := e.onlyVertex()
	if !ok || !e.d.Kinds().Vertex(e.d.Vertex(v).Kind()).Container() {
		return nil
	}
	if err := e.apply(gw.Command{Op: op, Vertex: v}); err != nil {
		return err
	}
	e.dropDead()
	return nil
}

// selectMerged adds to the selection the edges drawn as one with edge
// x (svg.Merged), so that selecting the one drawn selects them all.
func (e *Editor) selectMerged(x gw.EdgeIndex) {
	if _, n := svg.Merged(e.d, x); n < 2 {
		return
	}
	for i := range e.d.EdgeSlots() {
		o := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if first, _ := svg.Merged(e.d, o); o != x && first == x && !e.isSelectedE(o) {
			e.selE = push(e.selE, o)
		}
	}
}
