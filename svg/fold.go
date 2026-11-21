package svg

import gw "code.eitanity.com/graphedwright"

// A container folded into one vertex is drawn at its own size, and what
// it holds is not drawn (gw.Document.Hidden). An edge with an end hidden
// in a fold is drawn to the fold, at its body rather than any port; one
// with both ends in the same fold is not drawn; and the edges a fold
// brings to the same two drawn ends, the same way, are drawn as the
// first of them, with their number.

// shown is an edge as it is drawn: its ends the vertices they are drawn
// as, and no port at an end hidden in a fold.
type shown struct {
	gw.EdgeView
	from, to gw.VertexIndex
	fp, tp   gw.PortIndex
}

// From is the vertex the edge is drawn from.
func (s shown) From() gw.VertexIndex { return s.from }

// To is the vertex the edge is drawn to.
func (s shown) To() gw.VertexIndex { return s.to }

// FromPort is the port the edge is drawn from, or None.
func (s shown) FromPort() gw.PortIndex { return s.fp }

// ToPort is the port the edge is drawn to, or None.
func (s shown) ToPort() gw.PortIndex { return s.tp }

// shownEdge is edge i as it is drawn.
func shownEdge(d *gw.Document, i gw.EdgeIndex) shown {
	e := d.Edge(i)
	s := shown{EdgeView: e, from: e.From(), to: e.To(), fp: e.FromPort(), tp: e.ToPort()}
	if f := d.DrawnAs(s.from); f != s.from {
		s.from, s.fp = f, gw.None
	}
	if t := d.DrawnAs(s.to); t != s.to {
		s.to, s.tp = t, gw.None
	}
	return s
}

// folded reports whether an edge has an end hidden in a fold.
func (s shown) folded() bool { return s.from != s.EdgeView.From() || s.to != s.EdgeView.To() }

// EdgeDrawn reports whether edge e is drawn: live, not within one fold,
// and not one of those a fold brings to the same two drawn ends but the
// first of them (Merged).
func EdgeDrawn(d *gw.Document, e gw.EdgeIndex) bool {
	if e < 0 || int(e) >= d.EdgeSlots() || !d.Edge(e).Live() || d.EdgeHidden(e) {
		return false
	}
	first, _ := Merged(d, e)
	return first == e
}

// Merged is the first of the live edges drawn as edge e is, from and to
// the same two drawn vertices, at least one end hidden in a fold, and
// how many they are; e itself and 1 for an edge drawn on its own. It
// walks the edges at an end drawn as itself, or every edge when both
// ends are hidden.
func Merged(d *gw.Document, e gw.EdgeIndex) (gw.EdgeIndex, int) {
	if e < 0 || int(e) >= d.EdgeSlots() {
		return e, 1
	}
	s := shownEdge(d, e)
	if !s.Live() || !s.folded() {
		return e, 1
	}
	first, n := e, 0
	switch {
	case s.from == s.EdgeView.From():
		// bound: the out list visits each edge of the vertex once
		for o := d.FirstOut(s.from); o != gw.None; o = d.NextOut(o) {
			first, n = sameMerged(d, s, o, first, n)
		}
	case s.to == s.EdgeView.To():
		// bound: the in list visits each edge of the vertex once
		for o := d.FirstIn(s.to); o != gw.None; o = d.NextIn(o) {
			first, n = sameMerged(d, s, o, first, n)
		}
	default:
		for o := range d.EdgeSlots() {
			first, n = sameMerged(d, s, gw.EdgeIndex(o), first, n) //nolint:gosec // bounded by the capacity
		}
	}
	return first, n
}

// sameMerged counts edge o in n, and takes it as first when it is the
// lower, when it is drawn as one with the shown edge s. It is a
// function, not a closure, which TinyGo would make on the heap.
func sameMerged(d *gw.Document, s shown, o, first gw.EdgeIndex, n int) (gw.EdgeIndex, int) {
	if x := shownEdge(d, o); x.Live() && x.folded() && x.from == s.from && x.to == s.to {
		return min(first, o), n + 1
	}
	return first, n
}
