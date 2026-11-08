package graphedwright

import "math"

// The ends of the curved and orthogonal edges drawn to one side of a
// vertex are spread along it, so that no two arrive at one point: each
// edge's last leg comes in level, and two at one point would lie on each
// other there. A straight edge is drawn toward the vertex's centre and
// an edge with a port at the vertex ends at the port, so neither shares
// a side. The edges at a side are ordered across the flow by where they
// head from it (heading), which keeps their last legs from crossing:
// the first bend of the route that a layout places without regard to
// where the ends fall, or, for a route without one, where its other end
// stands; and by index where two head level, as the lanes between the
// same two vertices are (Parallel). So a layout can bring the bends next
// to an end level with it, once the routes are found, and leave the
// order as it was.

// EndGap is the most room across the flow between the ends of two edges
// drawn to one side of a vertex.
const EndGap = 20

// MaxEndSlots is the most edges whose ends are spread along one side of
// a vertex; with more, they all end at its middle.
const MaxEndSlots = 16

// endSide is the side of vertex v that edge e is drawn to: 1 for the
// side facing on along the flow, -1 for the side facing back, and 0 when
// the edge is not one whose ends are spread: dead, a loop, straight, or
// with a port at v. An edge leaves its source's on side and arrives at
// its target's back side, the other way round when it runs backward, to
// a vertex earlier in the flow, which an edge with a port never does.
func (d *Document) endSide(e EdgeIndex, v VertexIndex) int {
	ed := &d.edges[e]
	if !ed.Live || ed.From == ed.To || d.kinds.edge[ed.Kind].Route == Straight {
		return 0
	}
	if ed.From == v && ed.FromPort != None || ed.To == v && ed.ToPort != None {
		return 0
	}
	side := 1
	if ed.To == v {
		side = -1
	}
	if ed.FromPort == None && ed.ToPort == None && d.along(ed.To) < d.along(ed.From) {
		side = -side
	}
	return side
}

// along and across are where a vertex stands along the flow and across
// it.
func (d *Document) along(v VertexIndex) float64 {
	if d.flow == TopToBottom {
		return d.vertices[v].Pos.Y
	}
	return d.vertices[v].Pos.X
}

func (d *Document) across(v VertexIndex) float64 {
	if d.flow == TopToBottom {
		return d.vertices[v].Pos.X
	}
	return d.vertices[v].Pos.Y
}

// EndSlot is where edge e's end at its target, or with target false at
// its source, falls among the ends spread along that side of the vertex:
// k of them come before it across the flow, of n in all, e included. An
// end that is not spread, or one of more than MaxEndSlots, is alone: 0
// of 1.
func (d *Document) EndSlot(e EdgeIndex, target bool) (k, n int) {
	if e < 0 || int(e) >= len(d.edges) {
		return 0, 1
	}
	v := d.edges[e].From
	if target {
		v = d.edges[e].To
	}
	side := d.endSide(e, v)
	if side == 0 {
		return 0, 1
	}
	n = d.sideEnds(v, side)
	if n > MaxEndSlots {
		return 0, 1
	}
	key := d.heading(e, target)
	o, in := d.outHead[v], false
	if o == None {
		o, in = d.inHead[v], true
	}
	// bound: the out list and then the in list each visit each edge of v once
	for o != None {
		// Those heading lower come before e, and those heading level of a
		// lower index; a loop, in both lists, has no side.
		if i := EdgeIndex(o); d.endSide(i, v) == side {
			if at := d.heading(i, in); at < key || at == key && i < e {
				k++
			}
		}
		if in {
			o = d.nextIn[o]
		} else if o = d.nextOut[o]; o == None {
			o, in = d.inHead[v], true
		}
	}
	return k, n
}

// sideEnds is how many ends are spread along the side of vertex v,
// facing on along the flow for side 1 and back for -1.
func (d *Document) sideEnds(v VertexIndex, side int) int {
	n := 0
	o, in := d.outHead[v], false
	if o == None {
		o, in = d.inHead[v], true
	}
	// bound: the out list and then the in list each visit each edge of v once
	for o != None {
		if d.endSide(EdgeIndex(o), v) == side {
			n++
		}
		if in {
			o = d.nextIn[o]
		} else if o = d.nextOut[o]; o == None {
			o, in = d.inHead[v], true
		}
	}
	return n
}

// levelTol is how near a bend is taken to be to the spread of a side's
// ends: a layout puts the bends it levels with an end exactly there.
const levelTol = 1e-6

// heading is where edge e heads across the flow from its end at its
// target, or with target false at its source: the first of its bends,
// walked from that end, beyond the spread of the end's side, where a
// layout brings the bends next to the ends level with them, or within a
// container that holds the other end but not this one, where the route
// goes into it; or, where
// the walk comes first to the spread at the other end, which stands
// where the ends there fall, where the other end stands. A bend within
// both spreads is passed by: one levelled with this end is within its
// spread wherever it is levelled, and one levelled with the other end
// is among the last, so that passing it ends the walk there as
// stopping at it would.
func (d *Document) heading(e EdgeIndex, target bool) float64 {
	ed := &d.edges[e]
	v, far := ed.From, ed.To
	if target {
		v, far = ed.To, ed.From
	}
	near, farOff := d.spread(e, v), d.spread(e, far)
	b := ed.Bends
	n := len(b)
	// bound: the edge's bends, at most the slab
	for k := range n {
		p := b[k]
		if target {
			p = b[n-1-k]
		}
		at := p.Y
		if d.flow == TopToBottom {
			at = p.X
		}
		if math.Abs(at-d.across(v)) <= near+levelTol && !d.entering(v, far, p) {
			continue
		}
		if math.Abs(at-d.across(far)) <= farOff+levelTol {
			break
		}
		return at
	}
	return d.across(far)
}

// entering is whether point p is within a container that holds vertex
// far but not v: where a route from v goes into the container on its way
// to far, which a layout does not bring level with the end at v.
func (d *Document) entering(v, far VertexIndex, p Point) bool {
	c := d.vertices[far].Parent
	// bound: the parent chain is acyclic, so at most the vertex count
	for n := 0; c != None && n <= len(d.vertices); n++ {
		if d.Within(v, c) {
			return false
		}
		box := &d.vertices[c]
		if math.Abs(p.X-box.Pos.X) < box.Size.W/2 && math.Abs(p.Y-box.Pos.Y) < box.Size.H/2 {
			return true
		}
		c = box.Parent
	}
	return false
}

// spread is how far off the middle of its side the ends spread along
// the side of vertex v that edge e is drawn to may fall: none where e's
// end there is not spread, or all are at the middle.
func (d *Document) spread(e EdgeIndex, v VertexIndex) float64 {
	side := d.endSide(e, v)
	if side == 0 {
		return 0
	}
	n := d.sideEnds(v, side)
	if n > MaxEndSlots {
		return 0
	}
	return float64(float64(n-1)/2) * d.endStep(v, n)
}

// endStep is how far apart n ends spread along a side of vertex v fall:
// EndGap or, where the side is too short for that, a share of it.
func (d *Document) endStep(v VertexIndex, n int) float64 {
	side := d.vertices[v].Size.H
	if d.flow == TopToBottom {
		side = d.vertices[v].Size.W
	}
	return min(EndGap, side/float64(n+1))
}

// EndOffset is how far across the flow edge e's end at its target, or
// with target false at its source, is drawn from the middle of the side
// it is drawn to: its slot's (EndSlot) place among the n spread evenly
// about the middle, EndGap apart or, where the side is too short for
// that, a share of the side, so that every end stays on it.
func (d *Document) EndOffset(e EdgeIndex, target bool) float64 {
	k, n := d.EndSlot(e, target)
	if n < 2 {
		return 0 // alone, or e out of range
	}
	v := d.edges[e].From
	if target {
		v = d.edges[e].To
	}
	return float64(float64(k)-float64(float64(n-1)/2)) * d.endStep(v, n)
}
