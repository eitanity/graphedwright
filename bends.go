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
func (d *Document) endsOf(e *Edge) ends {
	f, t := &d.Vertices[e.From], &d.Vertices[e.To]
	return ends{from: f.Pos, to: t.Pos, fromSize: f.Size, toSize: t.Size}
}

// SetBends gives an edge the points it is drawn through between its
// ends, outside the command log, as Place sets a position: for a layout
// engine and a loader. The points are copied into a slab of BendsPerEdge
// points per edge capacity, which the edges share and only ClearBends
// empties, and hold for the ends where they are now. A layout engine
// that reserved the points with ReserveBends and filled them before
// placing the ends passes the edge's own Bends back, which keeps them
// and records the ends afresh. It reports whether the edge was live,
// the points coordinates (within MaxCoord) and the slab had room; when not, the edge has no
// bends.
func (d *Document) SetBends(e EdgeIndex, pts []Point) bool {
	if !d.edgeLive(e) {
		return false
	}
	ed := &d.Edges[e]
	for _, p := range pts {
		if !point(p) {
			ed.route, ed.Bends = nil, nil
			return false
		}
	}
	if len(pts) > 0 && len(pts) <= len(ed.route) && &pts[0] == &ed.route[0] {
		// A prefix of the edge's own reservation: the rest goes unused
		// until the slab is cleared.
		ed.route = ed.route[:len(pts)]
		ed.Bends, ed.routed = ed.route, d.endsOf(ed)
		return true
	}
	b := d.ReserveBends(e, len(pts))
	copy(b, pts)
	return b != nil || len(pts) == 0
}

// ReserveBends gives a live edge n bend points from the slab, at zero,
// for the caller to fill with points within MaxCoord; they hold for the ends
// where they are now. It is nil, and the edge left without bends, when
// the edge is not live, n is zero or the slab has no room.
func (d *Document) ReserveBends(e EdgeIndex, n int) []Point {
	if !d.edgeLive(e) {
		return nil
	}
	ed := &d.Edges[e]
	ed.route, ed.Bends = nil, nil
	if n <= 0 || n > len(d.bendSlab)-d.bendN {
		return nil
	}
	b := d.bendSlab[d.bendN : d.bendN+n : d.bendN+n]
	clear(b)
	d.bendN += n
	ed.route, ed.Bends, ed.routed = b, b, d.endsOf(ed)
	return b
}

// ClearBends drops every edge's route and empties the slab.
func (d *Document) ClearBends() {
	for i := range d.Edges {
		e := &d.Edges[i]
		if e.Live && len(e.Bends) > 0 {
			d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		}
		e.route, e.Bends = nil, nil
	}
	d.bendN = 0
}

// carry moves the routes of the edges with both ends within container c
// by the offset, as c and what it holds move by it.
func (d *Document) carry(c VertexIndex, dx, dy float64) {
	if !d.isContainer(c) || (dx == 0 && dy == 0) {
		return
	}
	for i := range d.Edges {
		e := &d.Edges[i]
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
	e := &d.Edges[i]
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
