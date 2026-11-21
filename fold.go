package graphedwright

// A folded container is drawn as one vertex, its own size, or its
// kind's when it has none, and what it holds is hidden: not drawn, hit,
// laid out or selected, though it keeps its place, its size and its
// container, and a Move of the folded container carries it, so that an
// unfold finds it where it belongs. An edge with a hidden end is drawn
// to the outermost folded container of that end (DrawnAs), and one with
// both ends in the same fold is not drawn. Fold and Unfold are commands,
// so they undo; Apply takes commands on what is hidden, as a host may
// edit its model through any view.

// fold folds or unfolds a container, as the command's Op says.
func (d *Document) fold(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	if !d.isContainer(c.Vertex) {
		return refusal("vertex " + d.vertexName(c.Vertex) + " of kind " + d.kindName(d.vertices[c.Vertex].Kind) + " is not a container")
	}
	folding := c.Op == Fold
	if d.vertices[c.Vertex].Folded == folding {
		if folding {
			return refusal("container " + d.vertexName(c.Vertex) + " is folded already")
		}
		return refusal("container " + d.vertexName(c.Vertex) + " is not folded")
	}
	d.seq++
	d.record(entry{cmd: c, seq: d.seq, vertex: c.Vertex})
	d.setFolded(c.Vertex, folding)
	return nil
}

// setFolded folds or unfolds a container and names it, what it holds
// and their edges as changed.
func (d *Document) setFolded(c VertexIndex, folded bool) {
	d.vertices[c].Folded = folded
	for i := range d.vertices {
		u := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if d.vertices[i].Live && d.Within(u, c) {
			d.touchVertex(u)
			d.touchEdgesAt(u)
		}
	}
}

// DrawnAs is the vertex a vertex is drawn as: itself, or, when it is
// hidden, the outermost folded container it is in; None for an index
// out of range.
func (d *Document) DrawnAs(v VertexIndex) VertexIndex {
	if v < 0 || int(v) >= len(d.vertices) {
		return None
	}
	drawn := v
	// bound: the parent chain is acyclic, so at most the vertex count
	for p, n := d.vertices[v].Parent, 0; p != None && n < len(d.vertices); p, n = d.vertices[p].Parent, n+1 {
		if d.vertices[p].Folded {
			drawn = p
		}
	}
	return drawn
}

// Hidden reports whether a vertex is in a folded container, at any
// depth; false for an index out of range.
func (d *Document) Hidden(v VertexIndex) bool {
	return v >= 0 && int(v) < len(d.vertices) && d.DrawnAs(v) != v
}

// EdgeHidden reports whether an edge is not drawn: both its ends drawn
// as the same folded container. An edge between a container and what it
// holds is hidden while the container is folded.
func (d *Document) EdgeHidden(e EdgeIndex) bool {
	if e < 0 || int(e) >= len(d.edges) {
		return false
	}
	x := &d.edges[e]
	from, to := d.DrawnAs(x.From), d.DrawnAs(x.To)
	return from == to && (from != x.From || to != x.To)
}

// Members is how many live vertices a container holds, at any depth; 0
// for a vertex that holds none or an index out of range.
func (d *Document) Members(c VertexIndex) int {
	if !d.vertexLive(c) || !d.isContainer(c) {
		return 0
	}
	n := 0
	for i := range d.vertices {
		u := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if u != c && d.vertices[i].Live && d.Within(u, c) {
			n++
		}
	}
	return n
}
