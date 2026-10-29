package graphedwright

// Compact drops the tombstones and renumbers the live vertices and
// edges, keeping their order. It returns, for each old vertex and edge
// index, the new index or None; the maps are the document's scratch and
// are valid until the next Compact. The undo log is cleared, since it
// names indices.
func (d *Document) Compact() (vmap []VertexIndex, emap []EdgeIndex) {
	d.resetChange()
	d.compact()
	return d.vmap[:d.vmapN], d.emap[:d.emapN]
}

// compact is Compact without the maps returned.
func (d *Document) compact() {
	d.vmapN, d.emapN = len(d.Vertices), len(d.Edges)
	n := 0
	for i := range d.Vertices {
		if !d.Vertices[i].Live {
			d.vmap[i] = None
			continue
		}
		d.vmap[i] = VertexIndex(n) //nolint:gosec // bounded by the capacity
		if n != i {
			d.moveVertex(i, n)
		}
		n++
	}
	d.Vertices = d.Vertices[:n]
	for i := range d.Vertices {
		if p := d.Vertices[i].Parent; p != None {
			d.Vertices[i].Parent = d.vmap[p]
		}
	}
	m := 0
	for i := range d.Edges {
		if !d.Edges[i].Live {
			d.emap[i] = None
			continue
		}
		d.emap[i] = EdgeIndex(m) //nolint:gosec // bounded by the capacity
		if m != i {
			d.moveEdge(i, m)
		}
		e := &d.Edges[m]
		e.From, e.To = d.vmap[e.From], d.vmap[e.To]
		m++
	}
	d.Edges = d.Edges[:m]
	d.relink()
	d.log.clear()
	for i := range d.Vertices {
		d.touchVertex(VertexIndex(i)) //nolint:gosec // bounded by the capacity
	}
	for i := range d.Edges {
		d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	}
}

// moveVertex copies vertex slot i to slot n, its ports and attributes
// moving within their slabs.
func (d *Document) moveVertex(i, n int) {
	v := d.Vertices[i]
	ports := d.portSlab[n*d.caps.PortsPerVertex : n*d.caps.PortsPerVertex+len(v.Ports)]
	copy(ports, v.Ports)
	attrs := d.vattrSlab[n*d.caps.AttrsPerElement : n*d.caps.AttrsPerElement+len(v.Attrs)]
	copy(attrs, v.Attrs)
	v.Ports, v.Attrs = ports, attrs
	d.Vertices[n] = v
}

// moveEdge copies edge slot i to slot m, its attributes moving within
// their slab.
func (d *Document) moveEdge(i, m int) {
	e := d.Edges[i]
	attrs := d.eattrSlab[m*d.caps.AttrsPerElement : m*d.caps.AttrsPerElement+len(e.Attrs)]
	copy(attrs, e.Attrs)
	e.Attrs = attrs
	d.Edges[m] = e
}

// relink rebuilds the adjacency lists from the edges in order.
func (d *Document) relink() {
	for i := range d.Vertices {
		d.outHead[i], d.inHead[i] = None, None
	}
	for i := len(d.Edges) - 1; i >= 0; i-- {
		e := d.Edges[i]
		d.nextOut[i], d.outHead[e.From] = d.outHead[e.From], int32(i) //nolint:gosec // bounded by the capacity
		d.nextIn[i], d.inHead[e.To] = d.inHead[e.To], int32(i)        //nolint:gosec // bounded by the capacity
	}
}
