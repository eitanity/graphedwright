package graphedwright

// cycleThrough reports the cycle an edge from → to would close, as an
// error naming it, or nil. The search is a depth-first walk from to
// along live out-edges with an explicit stack, each vertex visited once,
// looking for from.
func (d *Document) cycleThrough(from, to VertexIndex) error {
	if from == to {
		return refusal("edge would make a cycle: " + string(d.Vertices[from].ID) + " → " + string(d.Vertices[from].ID))
	}
	d.gen++
	if d.gen == 0 {
		clear(d.mark)
		d.gen = 1
	}
	top := 0
	d.stack[top] = int32(to) //nolint:gosec // bounded by the capacity
	top++
	d.mark[to] = d.gen
	d.parent[to] = None
	// bound: each vertex is pushed at most once
	for top > 0 {
		top--
		v := d.stack[top]
		// bound: the out list visits each edge of v once
		for e := d.outHead[v]; e != None; e = d.nextOut[e] {
			if !d.Edges[e].Live {
				continue
			}
			w := d.Edges[e].To
			if w == from {
				d.parent[w] = v
				return d.cycleError(from, to)
			}
			if d.mark[w] != d.gen {
				d.mark[w] = d.gen
				d.parent[w] = v
				d.stack[top] = int32(w) //nolint:gosec // bounded by the capacity
				top++
			}
		}
	}
	return nil
}

// maxCycleNames bounds the vertices a cycle refusal names.
const maxCycleNames = 8

// cycleError names the cycle from → to → … → from found by the walk,
// reading the parent chain back from from to to.
func (d *Document) cycleError(from, to VertexIndex) error {
	// The chain from → … → to is read backwards, so it is gathered on the
	// stack first and written out in order.
	n := 0
	// bound: the parent chain visits each vertex at most once
	for v := d.parent[from]; v != None && n < len(d.stack); v = d.parent[v] {
		d.stack[n] = v
		n++
		if VertexIndex(v) == to {
			break
		}
	}
	msg := "edge would make a cycle: " + string(d.Vertices[from].ID)
	names := 0
	for i := n - 1; i >= 0; i-- {
		if names == maxCycleNames {
			msg += " → …"
			break
		}
		msg += " → " + string(d.Vertices[d.stack[i]].ID)
		names++
	}
	return refusal(msg + " → " + string(d.Vertices[from].ID))
}

// Unreachable lists the live vertices no root-kind vertex reaches,
// which the schema's Connected flag makes a warning for the host. The
// result is written into dst, which the caller sizes; the walk is a
// breadth-first visit from every root along live out-edges, each vertex
// visited once.
func (d *Document) Unreachable(dst []VertexIndex) []VertexIndex {
	d.gen++
	if d.gen == 0 {
		clear(d.mark)
		d.gen = 1
	}
	top := 0
	for i := range d.Vertices {
		if v := &d.Vertices[i]; v.Live && d.Schema.VertexKinds[v.Kind].Root {
			d.mark[i] = d.gen
			d.stack[top] = int32(i) //nolint:gosec // bounded by the capacity
			top++
		}
	}
	// bound: each vertex is pushed at most once
	for top > 0 {
		top--
		v := d.stack[top]
		// bound: the out list visits each edge of v once
		for e := d.outHead[v]; e != None; e = d.nextOut[e] {
			if w := d.Edges[e].To; d.Edges[e].Live && d.mark[w] != d.gen {
				d.mark[w] = d.gen
				d.stack[top] = int32(w) //nolint:gosec // bounded by the capacity
				top++
			}
		}
	}
	dst = dst[:0]
	for i := range d.Vertices {
		if d.Vertices[i].Live && d.mark[i] != d.gen && len(dst) < cap(dst) {
			dst = append(dst, VertexIndex(i)) //nolint:gosec // bounded by the capacity
		}
	}
	return dst
}
