package graphedwright

// OutlineEntry is one line of a drawing's outline: a live vertex, how
// many containers it is in, whether it is a folded container, and how
// many vertices it holds, at any depth.
type OutlineEntry struct {
	Vertex VertexIndex
	Depth  int
	Folded bool
	Holds  int
}

// Outline appends the live vertices to dst in drawing order, within its
// capacity: a walk of the tree the containers make, each container
// before what it holds and siblings by index, so that a host's panel is
// a list indented by Depth. Hidden vertices are listed, under their
// folded container. It allocates nothing beyond dst.
func (d *Document) Outline(dst []OutlineEntry) []OutlineEntry {
	root := d.outlineTree()
	open := d.queue[:0] // the entries whose Holds are still being counted
	// bound: each step lists a vertex, once each, and climbs past only
	// vertices already listed
	for v, depth := root, 0; v != None && len(dst) < cap(dst); {
		// The entries open at this depth or deeper hold no more.
		// bound: the entries open, at most those listed
		for len(open) > 0 && dst[open[len(open)-1]].Depth >= depth {
			k := open[len(open)-1]
			dst[k].Holds = len(dst) - int(k) - 1
			open = open[:len(open)-1]
		}
		open = append(open, int32(len(dst))) //nolint:gosec // bounded by the capacity
		dst = append(dst, OutlineEntry{Vertex: v, Depth: depth, Folded: d.vertices[v].Folded})
		if d.kid[v] != None {
			v, depth = d.kid[v], depth+1
			continue
		}
		// bound: the parent chain is acyclic, so at most the vertex count
		for n := 0; v != None && d.sib[v] == None && n <= len(d.vertices); n++ {
			v, depth = d.vertices[v].Parent, depth-1
		}
		if v != None {
			v = d.sib[v]
		}
	}
	for _, k := range open {
		dst[k].Holds = len(dst) - int(k) - 1
	}
	return dst
}

// outlineTree links every live vertex to its first child and next
// sibling, in index order, in the scratch reorder uses, and returns the
// first vertex in none; reorder builds its own afresh each time.
func (d *Document) outlineTree() VertexIndex {
	for i := range d.vertices {
		d.kid[i] = None
	}
	root := VertexIndex(None)
	// Backwards, so that each list of children is in index order.
	for i := len(d.vertices) - 1; i >= 0; i-- {
		if !d.vertices[i].Live {
			continue
		}
		if p := d.vertices[i].Parent; p == None {
			d.sib[i], root = root, VertexIndex(i) //nolint:gosec // bounded by the capacity
		} else {
			d.sib[i], d.kid[p] = d.kid[p], VertexIndex(i) //nolint:gosec // bounded by the capacity
		}
	}
	return root
}
