package graphedwright

// VertexSlots is the number of vertex indices in use, tombstones
// (Live false) included, until Compact: the indices run from 0 to it.
func (d *Document) VertexSlots() int { return len(d.vertices) }

// EdgeSlots is the number of edge indices in use, tombstones included.
func (d *Document) EdgeSlots() int { return len(d.edges) }

// Vertex reads the vertex at the index, live or a tombstone; out of
// range, every read of it is the zero value, or None for an index.
func (d *Document) Vertex(v VertexIndex) VertexView {
	if v < 0 || int(v) >= len(d.vertices) {
		return VertexView{}
	}
	return VertexView{&d.vertices[v]}
}

// Edge reads the edge at the index, live or a tombstone; out of range,
// every read of it is the zero value, or None for an index.
func (d *Document) Edge(e EdgeIndex) EdgeView {
	if e < 0 || int(e) >= len(d.edges) {
		return EdgeView{}
	}
	return EdgeView{&d.edges[e]}
}

// VertexView reads one vertex of a document (see Vertex). It reads the
// document as it is at each call.
type VertexView struct{ x *Vertex }

// Valid reports whether the view names a vertex index in use.
func (v VertexView) Valid() bool { return v.x != nil }

// ID is the vertex's ID.
func (v VertexView) ID() ID {
	if v.x == nil {
		return ""
	}
	return v.x.ID
}

// Kind is the vertex's kind, an index into the document's Kinds.
func (v VertexView) Kind() KindIndex {
	if v.x == nil {
		return None
	}
	return v.x.Kind
}

// Label is the label as set.
func (v VertexView) Label() string {
	if v.x == nil {
		return ""
	}
	return v.x.Label
}

// Pos is the vertex's centre.
func (v VertexView) Pos() Point {
	if v.x == nil {
		return Point{}
	}
	return v.x.Pos
}

// Size is the vertex's size as drawn.
func (v VertexView) Size() Size {
	if v.x == nil {
		return Size{}
	}
	return v.x.Size
}

// Min is a container's own size, or the least of a vertex with
// compartments; zero for its kind's size.
func (v VertexView) Min() Size {
	if v.x == nil {
		return Size{}
	}
	return v.x.Min
}

// Ports are the vertex's ports, its kind's first.
func (v VertexView) Ports() PortList {
	if v.x == nil {
		return PortList{}
	}
	return PortList{v.x.Ports}
}

// Attrs are the attributes the vertex carries.
func (v VertexView) Attrs() AttrList {
	if v.x == nil {
		return AttrList{}
	}
	return AttrList{v.x.Attrs}
}

// Placed reports whether a layout or a drag has given it a position.
func (v VertexView) Placed() bool { return v.x != nil && v.x.Placed }

// Live reports whether the vertex is in the document, not a tombstone.
func (v VertexView) Live() bool { return v.x != nil && v.x.Live }

// Parent is the container the vertex is in, or None.
func (v VertexView) Parent() VertexIndex {
	if v.x == nil {
		return None
	}
	return v.x.Parent
}

// EdgeView reads one edge of a document (see Edge). It reads the
// document as it is at each call.
type EdgeView struct{ x *Edge }

// Valid reports whether the view names an edge index in use.
func (e EdgeView) Valid() bool { return e.x != nil }

// ID is the edge's ID.
func (e EdgeView) ID() ID {
	if e.x == nil {
		return ""
	}
	return e.x.ID
}

// Kind is the edge's kind, an index into the document's Kinds.
func (e EdgeView) Kind() KindIndex {
	if e.x == nil {
		return None
	}
	return e.x.Kind
}

// From is the vertex the edge leaves.
func (e EdgeView) From() VertexIndex {
	if e.x == nil {
		return None
	}
	return e.x.From
}

// To is the vertex the edge enters.
func (e EdgeView) To() VertexIndex {
	if e.x == nil {
		return None
	}
	return e.x.To
}

// FromPort is the port of From it leaves by, or None.
func (e EdgeView) FromPort() PortIndex {
	if e.x == nil {
		return None
	}
	return e.x.FromPort
}

// ToPort is the port of To it enters by, or None.
func (e EdgeView) ToPort() PortIndex {
	if e.x == nil {
		return None
	}
	return e.x.ToPort
}

// Label is the label as set.
func (e EdgeView) Label() string {
	if e.x == nil {
		return ""
	}
	return e.x.Label
}

// Attrs are the attributes the edge carries.
func (e EdgeView) Attrs() AttrList {
	if e.x == nil {
		return AttrList{}
	}
	return AttrList{e.x.Attrs}
}

// Weight drives the stroke width when the kind says so; 0 for none.
func (e EdgeView) Weight() float64 {
	if e.x == nil {
		return 0
	}
	return e.x.Weight
}

// Live reports whether the edge is in the document, not a tombstone.
func (e EdgeView) Live() bool { return e.x != nil && e.x.Live }

// Bends are the points the edge is drawn through between its ends.
func (e EdgeView) Bends() PointList {
	if e.x == nil {
		return PointList{}
	}
	return PointList{e.x.Bends}
}
