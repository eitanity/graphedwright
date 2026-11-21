package graphedwright

// Direction is which way Around follows the edges.
type Direction uint8

// The ways Around follows the edges.
const (
	Both         Direction = iota // forward and backward
	Successors                    // from each vertex to the ends of its outgoing edges
	Predecessors                  // from each vertex to the starts of its incoming edges
)

// Around marks in in the live vertices within depth edges of those
// given, the given among them, following the live edges the way given,
// breadth first, and returns how many it marked. in has a slot per
// vertex (VertexSlots), and is cleared first; a shorter one, or a depth
// below 0, marks nothing. Folds are not followed: a hidden vertex is
// marked as any other, and what is drawn is the renderer's. It
// allocates nothing.
func (d *Document) Around(from []VertexIndex, depth int, way Direction, in []bool) int {
	if len(in) < len(d.vertices) || depth < 0 {
		return 0
	}
	clear(in)
	q := d.queue[:0]
	for _, v := range from {
		if d.vertexLive(v) && !in[v] {
			in[v] = true
			q = append(q, int32(v))
		}
	}
	// bound: each live vertex is queued once, so the queue holds at most
	// the vertex count; each round is one step further out
	for head, level := 0, 0; level < depth && head < len(q); level++ {
		end := len(q)
		// bound: the vertices queued by the round before
		for ; head < end; head++ {
			q = d.reach(VertexIndex(q[head]), way, in, q)
		}
	}
	return len(q)
}

// reach marks and queues the vertices one live edge from v, the way
// given, that are not marked yet.
func (d *Document) reach(v VertexIndex, way Direction, in []bool, q []int32) []int32 {
	if way != Predecessors {
		// bound: the out list visits each edge of v once
		for e := d.outHead[v]; e != None; e = d.nextOut[e] {
			if x := &d.edges[e]; x.Live && !in[x.To] {
				in[x.To] = true
				q = append(q, int32(x.To))
			}
		}
	}
	if way != Successors {
		// bound: the in list visits each edge of v once
		for e := d.inHead[v]; e != None; e = d.nextIn[e] {
			if x := &d.edges[e]; x.Live && !in[x.From] {
				in[x.From] = true
				q = append(q, int32(x.From))
			}
		}
	}
	return q
}
