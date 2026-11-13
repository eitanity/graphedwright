package graphedwright

// Notes explain other vertices: a note kind's vertices (VertexKind.Note)
// are joined by anchors, the edges at them, to what they explain. An
// anchor is drawn straight between the two boxes, takes no slot among
// the ends at a side and no room at a container's side, and is left out
// of the layout, which puts each note beside its target (NoteTarget).

// Anchor reports whether edge e joins a note.
func (d *Document) Anchor(e EdgeIndex) bool {
	if e < 0 || int(e) >= len(d.edges) {
		return false
	}
	x := &d.edges[e]
	return d.isNote(x.From) || d.isNote(x.To)
}

// isNote reports whether vertex v's kind is a note kind.
func (d *Document) isNote(v VertexIndex) bool {
	return v >= 0 && int(v) < len(d.vertices) && d.kinds.vertex[d.vertices[v].Kind].Note
}

// Route is how edge e is drawn between its ends: its kind's route, or
// straight for an anchor.
func (d *Document) Route(e EdgeIndex) Route {
	if e < 0 || int(e) >= len(d.edges) || d.Anchor(e) {
		return Straight
	}
	return d.kinds.edge[d.edges[e].Kind].Route
}

// NoteTarget is the vertex that note v explains: the other end of the
// live anchor of the lowest index at v that joins it to a vertex that is
// not a note. It is None for a vertex that is not a note, and for a note
// without such an anchor.
func (d *Document) NoteTarget(v VertexIndex) VertexIndex {
	if !d.isNote(v) || !d.vertices[v].Live {
		return None
	}
	best, at := VertexIndex(None), int32(None)
	// bound: the out list visits each edge of v once
	for o := d.outHead[v]; o != None; o = d.nextOut[o] {
		if x := &d.edges[o]; x.Live && (at == None || o < at) && !d.isNote(x.To) {
			best, at = x.To, o
		}
	}
	// bound: the in list visits each edge of v once
	for o := d.inHead[v]; o != None; o = d.nextIn[o] {
		if x := &d.edges[o]; x.Live && (at == None || o < at) && !d.isNote(x.From) {
			best, at = x.From, o
		}
	}
	return best
}
