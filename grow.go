package graphedwright

// errGrowSmaller refuses capacities below the document's: what it holds
// might not fit them.
var errGrowSmaller = refusal("grow: every capacity must be at least the document's")

// Grow makes a document of the capacities given that holds all this one
// holds: its vertices and edges, tombstones included, at the same
// indices and with the same IDs; their positions, bends and the view;
// the undo log, what can be undone and redone, with the layouts it can
// reach back across; and the host's store. So a host whose document has
// filled goes on in a larger one and the user keeps their undo.
//
// Every capacity must be at least this document's, and a zero one takes
// its default, as for New. Grow allocates the new document, as New does.
// This document is left empty, as Reset leaves it, without the store or
// the log, whose host changes the new document holds now; the store is
// told of none. Grow is refused while the document is locked, from
// inside host code, and for capacities New refuses or below the
// document's; this document is then left as it was.
func (d *Document) Grow(caps Caps) (*Document, error) {
	switch {
	case d.host.calling:
		return nil, ErrReentered
	case d.locked:
		return nil, ErrLocked
	}
	caps = caps.filled()
	if !caps.holds(d.caps) {
		return nil, errGrowSmaller
	}
	n, err := New(d.kinds, caps)
	if err != nil {
		return nil, err
	}
	n.view, n.flow = d.view, d.flow
	n.nextVertexID, n.nextEdgeID, n.seq = d.nextVertexID, d.nextEdgeID, d.seq
	n.bendGen, n.layouts, n.gen, n.stamp, n.resized = d.bendGen, d.layouts, d.gen, d.stamp, d.resized
	copy(n.kindCount, d.kindCount)
	n.growVertices(d)
	n.growEdges(d)
	n.growLog(d)
	n.growLayouts(d)
	n.growScratch(d)
	if d.host.store != nil {
		n.host.store = d.host.store
		n.host.ids = append(make([]ID, 0, caps.Vertices+caps.Edges), d.host.ids...)
	}
	// The new document holds the log's host changes now: this one lets
	// go of them without telling the store, and of the store.
	d.log.clear()
	d.host.store, d.host.ids = nil, nil
	_ = d.Reset(d.kinds) // discarded: the kinds are its own and it is not locked, so it cannot be refused
	return n, nil
}

// holds reports whether every capacity is at least the other's.
func (c Caps) holds(o Caps) bool {
	return c.Vertices >= o.Vertices && c.Edges >= o.Edges &&
		c.PortsPerVertex >= o.PortsPerVertex && c.AttrsPerElement >= o.AttrsPerElement &&
		c.LabelBytes >= o.LabelBytes && c.UndoDepth >= o.UndoDepth &&
		c.UndoLayouts >= o.UndoLayouts && c.BendsPerEdge >= o.BendsPerEdge
}

// growVertices copies the other document's vertices and their
// adjacency, the ports and attributes into this one's slabs.
func (d *Document) growVertices(o *Document) {
	d.vertices = d.vertices[:len(o.vertices)]
	// bound: the other document's vertices, within this one's capacity
	for i := range o.vertices {
		v := o.vertices[i]
		ports := d.portSlab[i*d.caps.PortsPerVertex : i*d.caps.PortsPerVertex+len(v.Ports)]
		copy(ports, v.Ports)
		attrs := d.vattrSlab[i*d.caps.AttrsPerElement : i*d.caps.AttrsPerElement+len(v.Attrs)]
		copy(attrs, v.Attrs)
		v.Ports, v.Attrs = ports, attrs
		d.vertices[i] = v
	}
	copy(d.outHead, o.outHead)
	copy(d.inHead, o.inHead)
}

// growEdges copies the other document's edges and their adjacency, the
// attributes into this one's slab and the routes packed into its bends.
func (d *Document) growEdges(o *Document) {
	d.edges = d.edges[:len(o.edges)]
	// bound: the other document's edges, within this one's capacity
	for i := range o.edges {
		e := o.edges[i]
		attrs := d.eattrSlab[i*d.caps.AttrsPerElement : i*d.caps.AttrsPerElement+len(e.Attrs)]
		copy(attrs, e.Attrs)
		e.Attrs = attrs
		if e.route != nil {
			r := d.bendSlab[d.bendN : d.bendN+len(e.route) : d.bendN+len(e.route)]
			copy(r, e.route)
			d.bendN += len(r)
			shown := e.Bends != nil
			e.route, e.Bends = r, nil
			if shown {
				e.Bends = r
			}
		}
		d.edges[i] = e
	}
	copy(d.nextOut, o.nextOut)
	copy(d.nextIn, o.nextIn)
}

// growLog copies the other document's undo log, its oldest entry first,
// the ports its entries hold into this one's slab.
func (d *Document) growLog(o *Document) {
	l, ol := &d.log, &o.log
	// bound: the entries kept, at most the other log's depth
	for i := 0; i < ol.n; i++ {
		e := ol.entries[(ol.start+i)%len(ol.entries)]
		if e.ports != nil {
			p := l.portsFor(i, len(e.ports))
			copy(p, e.ports)
			e.ports = p
		}
		l.entries[i] = e
	}
	l.n, l.done = ol.n, ol.done
	l.groups, l.group, l.open, l.lost = ol.groups, ol.group, ol.open, ol.lost
}

// growLayouts copies the snapshots of the layouts the other document's
// log can still reach back across: the last UndoLayouts of them.
func (d *Document) growLayouts(o *Document) {
	k := uint64(0)
	if span := uint64(o.caps.UndoLayouts); o.layouts > span { //nolint:gosec // bounded by MaxCap
		k = o.layouts - span
	}
	// bound: at most the other document's UndoLayouts
	for k++; k <= o.layouts; k++ {
		for _, after := range [...]bool{false, true} {
			pos, placed, n := o.layoutSnap(k, after)
			np, nplaced, nn := d.layoutSnap(k, after)
			*nn = *n
			copy(np, pos[:*n])
			copy(nplaced, placed[:*n])
		}
	}
}

// growScratch copies what the other document keeps between calls in its
// scratch: the marks and stamps, the containers in drawing order and
// what each holds, and what the last Compact moved.
func (d *Document) growScratch(o *Document) {
	copy(d.mark, o.mark)
	copy(d.vstamp, o.vstamp)
	copy(d.estamp, o.estamp)
	copy(d.depth, o.depth)
	copy(d.box, o.box)
	copy(d.crossing, o.crossing)
	copy(d.kid, o.kid)
	copy(d.sib, o.sib)
	d.order = append(d.order, o.order...)
	copy(d.vmap, o.vmap)
	copy(d.emap, o.emap)
	d.vmapN, d.emapN = o.vmapN, o.emapN
}
