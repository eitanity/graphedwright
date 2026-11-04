package graphedwright

// Compact drops the tombstones and renumbers the live vertices and
// edges, keeping their order. It returns, for each old vertex and edge
// index, the new index or None, read by the old index; the maps read
// the document's scratch and are valid until the next Compact. The undo
// log is cleared, since it names indices. Locked, it does nothing and
// returns empty maps.
func (d *Document) Compact() (vmap VertexList, emap EdgeList) {
	if d.locked {
		return VertexList{}, EdgeList{}
	}
	d.resetChange()
	d.compact()
	return VertexList{d.vmap[:d.vmapN]}, EdgeList{d.emap[:d.emapN]}
}

// compact is Compact without the maps returned.
func (d *Document) compact() {
	d.vmapN, d.emapN = len(d.vertices), len(d.edges)
	n := 0
	for i := range d.vertices {
		if !d.vertices[i].Live {
			d.vmap[i] = None
			continue
		}
		d.vmap[i] = VertexIndex(n) //nolint:gosec // bounded by the capacity
		if n != i {
			d.moveVertex(i, n)
		}
		n++
	}
	d.vertices = d.vertices[:n]
	for i := range d.vertices {
		if p := d.vertices[i].Parent; p != None {
			d.vertices[i].Parent = d.vmap[p]
		}
	}
	m := 0
	for i := range d.edges {
		if !d.edges[i].Live {
			d.emap[i] = None
			continue
		}
		d.emap[i] = EdgeIndex(m) //nolint:gosec // bounded by the capacity
		if m != i {
			d.moveEdge(i, m)
		}
		e := &d.edges[m]
		e.From, e.To = d.vmap[e.From], d.vmap[e.To]
		m++
	}
	d.edges = d.edges[:m]
	d.relink()
	d.log.clear()
	for i := range d.vertices {
		d.touchVertex(VertexIndex(i)) //nolint:gosec // bounded by the capacity
	}
	for i := range d.edges {
		d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	}
}

// moveVertex copies vertex slot i to slot n, its ports and attributes
// moving within their slabs.
func (d *Document) moveVertex(i, n int) {
	v := d.vertices[i]
	ports := d.portSlab[n*d.caps.PortsPerVertex : n*d.caps.PortsPerVertex+len(v.Ports)]
	copy(ports, v.Ports)
	attrs := d.vattrSlab[n*d.caps.AttrsPerElement : n*d.caps.AttrsPerElement+len(v.Attrs)]
	copy(attrs, v.Attrs)
	v.Ports, v.Attrs = ports, attrs
	d.vertices[n] = v
}

// moveEdge copies edge slot i to slot m, its attributes moving within
// their slab.
func (d *Document) moveEdge(i, m int) {
	e := d.edges[i]
	attrs := d.eattrSlab[m*d.caps.AttrsPerElement : m*d.caps.AttrsPerElement+len(e.Attrs)]
	copy(attrs, e.Attrs)
	e.Attrs = attrs
	d.edges[m] = e
}

// relink rebuilds the adjacency lists from the edges in order.
func (d *Document) relink() {
	for i := range d.vertices {
		d.outHead[i], d.inHead[i] = None, None
	}
	for i := len(d.edges) - 1; i >= 0; i-- {
		e := d.edges[i]
		d.nextOut[i], d.outHead[e.From] = d.outHead[e.From], int32(i) //nolint:gosec // bounded by the capacity
		d.nextIn[i], d.inHead[e.To] = d.inHead[e.To], int32(i)        //nolint:gosec // bounded by the capacity
	}
}
