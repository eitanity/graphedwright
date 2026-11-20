package graphedwright

// errResetKinds refuses kinds with more vertex kinds than the document
// was made for, which it has no room to count.
var errResetKinds = refusal("the kinds have more vertex kinds than the document was made with")

// Reset empties the document, as New made it, for the kinds given, and
// keeps every array, so that a document is read again into the same
// room: it allocates nothing. The capacities stay, and so does the
// host's store, whose tokens the undo log held are forgotten. It is
// refused while the document is locked, from inside host code, for nil
// kinds, kinds the capacities cannot hold, and kinds with more vertex
// kinds than the document was made with; the document is then left as
// it was.
func (d *Document) Reset(kinds *Kinds) error {
	switch {
	case d.host.calling:
		return ErrReentered
	case d.locked:
		return ErrLocked
	case kinds == nil:
		return errNoKinds
	case len(kinds.vertex) > cap(d.kindCount):
		return errResetKinds
	}
	if err := kinds.fits(d.caps); err != nil {
		return err
	}
	d.clearLog()
	// The arrays keep their room, emptied, so that they hold no text of
	// what was there.
	clear(d.vertices)
	clear(d.edges)
	d.vertices, d.edges = d.vertices[:0], d.edges[:0]
	clear(d.portSlab)
	clear(d.vattrSlab)
	clear(d.eattrSlab)
	d.bendN = 0
	d.kinds = kinds
	d.kindCount = d.kindCount[:len(kinds.vertex)]
	clear(d.kindCount)
	d.view, d.flow = Viewport{}, LeftToRight
	d.nextVertexID, d.nextEdgeID = 0, 0
	d.vmapN, d.emapN = 0, 0
	d.resetChange()
	return nil
}
