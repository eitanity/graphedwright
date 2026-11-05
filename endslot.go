package graphedwright

// The ends of the curved and orthogonal edges drawn to one side of a
// vertex are spread along it, so that no two arrive at one point: each
// edge's last leg comes in level, and two at one point would lie on each
// other there. A straight edge is drawn toward the vertex's centre and
// an edge with a port at the vertex ends at the port, so neither shares
// a side. The edges at a side are ordered across the flow by where their
// other ends stand, which keeps their last legs from crossing, and by
// index where two stand level, as the lanes between the same two
// vertices are (Parallel).

// EndGap is the most room across the flow between the ends of two edges
// drawn to one side of a vertex.
const EndGap = 20

// MaxEndSlots is the most edges whose ends are spread along one side of
// a vertex; with more, they all end at its middle.
const MaxEndSlots = 8

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
	ed := &d.edges[e]
	v, far := ed.From, ed.To
	if target {
		v, far = ed.To, ed.From
	}
	side := d.endSide(e, v)
	if side == 0 {
		return 0, 1
	}
	key := d.across(far)
	// bound: the out list visits each edge of v once
	for o := d.outHead[v]; o != None; o = d.nextOut[o] {
		if d.endSide(EdgeIndex(o), v) == side {
			n++
			if d.before(EdgeIndex(o), d.edges[o].To, e, key) {
				k++
			}
		}
	}
	// bound: the in list visits each edge of v once
	for o := d.inHead[v]; o != None; o = d.nextIn[o] {
		if d.endSide(EdgeIndex(o), v) == side { // a loop, in both lists, has none
			n++
			if d.before(EdgeIndex(o), d.edges[o].From, e, key) {
				k++
			}
		}
	}
	if n > MaxEndSlots {
		return 0, 1
	}
	return k, n
}

// before reports whether edge o, whose other end is far, comes before
// edge e, whose other end stands at key across the flow.
func (d *Document) before(o EdgeIndex, far VertexIndex, e EdgeIndex, key float64) bool {
	at := d.across(far)
	return at < key || at == key && o < e
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
	side := d.vertices[v].Size.H
	if d.flow == TopToBottom {
		side = d.vertices[v].Size.W
	}
	step := min(EndGap, side/float64(n+1))
	return float64(float64(k)-float64(float64(n-1)/2)) * step
}
