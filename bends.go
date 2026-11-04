package graphedwright

// An edge's bends are derived from the positions: a layout routes a long
// edge around the layers it crosses, and the route holds for the ends
// where they were then. An edge keeps the route it was given until the
// next ClearBends, and its Bends are that route while both ends are
// where and the size they were when it was routed, and none otherwise.
// So moving an end straightens the edge, and an undo that brings the end
// back, or a redone layout, brings the bends back with it; nothing about
// bends is in the undo log. Moving a container carries the routes of the
// edges within it.

// ends is the position and size of an edge's two ends.
type ends struct {
	from, to         Point
	fromSize, toSize Size
}

// endsOf is where an edge's ends are now.
func (d *Document) endsOf(e *edge) ends {
	f, t := &d.vertices[e.From], &d.vertices[e.To]
	return ends{from: f.Pos, to: t.Pos, fromSize: f.Size, toSize: t.Size}
}

// SetBends gives an edge the points it is drawn through between its
// ends, outside the command log, as Place sets a position: for a layout
// engine and a loader. The points are copied into a slab of BendsPerEdge
// points per edge capacity, which the edges share and only ClearBends
// empties, and hold for the ends where they are now. A layout engine
// that builds a route in place before placing the ends uses
// ReserveBends and its writer's Keep instead. It reports whether the edge was live,
// the points coordinates (within MaxCoord) and the slab had room; when not, the edge has no
// bends.
func (d *Document) SetBends(e EdgeIndex, pts []Point) bool {
	if d.locked || !d.edgeLive(e) {
		return false
	}
	ed := &d.edges[e]
	for _, p := range pts {
		if !point(p) {
			d.dropRoute(ed)
			return false
		}
	}
	b := d.reserve(e, len(pts))
	copy(b, pts)
	return b != nil || len(pts) == 0
}

// ReserveBends gives a live edge n bend points from the slab, at zero,
// which hold for the ends where they are now, and returns the writer
// that fills them. The writer is not Valid, and the edge left without
// bends, when the document is locked, the edge is not live, n is zero
// or the slab has no room.
func (d *Document) ReserveBends(e EdgeIndex, n int) BendWriter {
	if d.reserve(e, n) == nil {
		return BendWriter{}
	}
	return BendWriter{d: d, e: e, gen: d.edges[e].routeGen}
}

// reserve is ReserveBends, returning the points reserved, or nil.
func (d *Document) reserve(e EdgeIndex, n int) []Point {
	if d.locked || !d.edgeLive(e) {
		return nil
	}
	ed := &d.edges[e]
	d.dropRoute(ed)
	if n <= 0 || n > len(d.bendSlab)-d.bendN {
		return nil
	}
	b := d.bendSlab[d.bendN : d.bendN+n : d.bendN+n]
	clear(b)
	d.bendN += n
	d.bendGen++
	ed.route, ed.Bends, ed.routed, ed.routeGen = b, b, d.endsOf(ed), d.bendGen
	return b
}

// dropRoute leaves an edge without bends, and any writer of its route
// stale.
func (d *Document) dropRoute(ed *edge) { ed.route, ed.Bends, ed.routeGen = nil, nil, 0 }

// BendWriter fills the bends ReserveBends reserved for one edge: a
// layout engine's way to build a route in place, point by point, before
// placing the ends. Every write is checked: while the document is
// locked, out of range, or once the reservation is no longer the
// edge's own (after ClearBends, a new reservation, or Compact), a write
// does nothing and reports false, and a read is the zero value. A point
// that is not a coordinate drops the edge's route, as SetBends does.
type BendWriter struct {
	d   *Document
	e   EdgeIndex
	gen uint64
}

// route is the reservation, when it is still the edge's own.
func (w BendWriter) route() (*edge, bool) {
	if w.d == nil || int(w.e) >= len(w.d.edges) { // a writer's edge was live once: after Compact, it may be past the end
		return nil, false
	}
	ed := &w.d.edges[w.e]
	return ed, ed.routeGen == w.gen
}

// Valid reports whether the reservation is still the edge's own.
func (w BendWriter) Valid() bool {
	_, ok := w.route()
	return ok
}

// Len is the number of points reserved, or kept.
func (w BendWriter) Len() int {
	if ed, ok := w.route(); ok {
		return len(ed.route)
	}
	return 0
}

// At is the ith point; out of range, or stale, the zero Point.
func (w BendWriter) At(i int) Point {
	ed, ok := w.route()
	if !ok || i < 0 || i >= len(ed.route) {
		return Point{}
	}
	return ed.route[i]
}

// Set sets the ith point.
func (w BendWriter) Set(i int, p Point) bool {
	ed, ok := w.route()
	if !ok || w.d.locked || i < 0 || i >= len(ed.route) {
		return false
	}
	if !point(p) {
		w.d.dropRoute(ed)
		return false
	}
	ed.route[i] = p
	return true
}

// Move copies n points from position from to position to, as the
// built-in copy does, the two ranges overlapping or not.
func (w BendWriter) Move(to, from, n int) bool {
	ed, ok := w.route()
	if !ok || w.d.locked || n < 0 || to < 0 || from < 0 || to+n > len(ed.route) || from+n > len(ed.route) {
		return false
	}
	copy(ed.route[to:to+n], ed.route[from:from+n])
	return true
}

// Keep makes the first n points the edge's route, for its ends where
// they are now; the rest go unused until the slab is cleared.
func (w BendWriter) Keep(n int) bool {
	ed, ok := w.route()
	if !ok || w.d.locked || n < 0 || n > len(ed.route) {
		return false
	}
	ed.route = ed.route[:n]
	ed.Bends, ed.routed = ed.route, w.d.endsOf(ed)
	return true
}

// ClearBends drops every edge's route and empties the slab.
func (d *Document) ClearBends() {
	if d.locked {
		return
	}
	for i := range d.edges {
		e := &d.edges[i]
		if e.Live && len(e.Bends) > 0 {
			d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		}
		d.dropRoute(e)
	}
	d.bendN = 0
}

// carry moves the routes of the edges with both ends within container c
// by the offset, as c and what it holds move by it.
func (d *Document) carry(c VertexIndex, dx, dy float64) {
	if !d.isContainer(c) || (dx == 0 && dy == 0) {
		return
	}
	for i := range d.edges {
		e := &d.edges[i]
		if len(e.route) == 0 || !d.Within(e.From, c) || !d.Within(e.To, c) {
			continue
		}
		for j := range e.route {
			e.route[j].X += dx
			e.route[j].Y += dy
		}
		e.routed.from.X += dx
		e.routed.from.Y += dy
		e.routed.to.X += dx
		e.routed.to.Y += dy
	}
}

// rebend shows or hides the route of every edge, live or not, at each
// vertex the change names, by whether its ends are where they were when
// it was routed, and names the live edges whose drawing changed.
func (d *Document) rebend() {
	for _, v := range d.changeV {
		// bound: the out list visits each edge of v once
		for e := d.outHead[v]; e != None; e = d.nextOut[e] {
			d.rebendEdge(EdgeIndex(e))
		}
		// bound: the in list visits each edge of v once
		for e := d.inHead[v]; e != None; e = d.nextIn[e] {
			d.rebendEdge(EdgeIndex(e))
		}
	}
}

func (d *Document) rebendEdge(i EdgeIndex) {
	e := &d.edges[i]
	var b []Point
	if len(e.route) > 0 && e.routed == d.endsOf(e) {
		b = e.route
	}
	if (b == nil) != (e.Bends == nil) {
		e.Bends = b
		if e.Live {
			d.touchEdge(i)
		}
	}
}
