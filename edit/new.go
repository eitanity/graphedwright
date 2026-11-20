package edit

import (
	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit/gesture"
)

// New makes an editor for the document, allocating its working arrays
// from the document's capacities. The grid is the nudge and snap unit,
// 8 when zero, or when it is not a positive size (gw.MaxCoord at most):
// a grid of NaN or infinity would make every position it snaps one the
// document refuses.
func New(d *gw.Document, mode Mode, grid float64) *Editor {
	caps := d.Caps()
	e := &Editor{
		d: d, g: gesture.New(),
		selV: make([]gw.VertexIndex, 0, caps.Vertices), selE: make([]gw.EdgeIndex, 0, caps.Edges),
		orig: make([]gw.Point, caps.Vertices), dragV: make([]gw.VertexIndex, 0, caps.Vertices), dropTo: make([]gw.VertexIndex, caps.Vertices),
		changeV: make([]gw.VertexIndex, 0, caps.Vertices), changeE: make([]gw.EdgeIndex, 0, caps.Edges),
		inV: make([]bool, caps.Vertices), inE: make([]bool, caps.Edges), atV: make([]gw.EdgeIndex, 0, caps.Edges),
	}
	e.renew(d, mode, grid)
	return e
}

// Reset makes the editor the one New would make for the document, the
// mode and the grid, in the arrays it already has, so that a host that
// shows one document after another allocates nothing for each: the
// selection, the gesture under way, the hover, the edge kind chosen, a
// hold and the cut texts' expansion all go back to New's. A gesture
// under way ends where it is, a drag's vertices not returned, since the
// document may have been read again meanwhile. It is refused,
// and the editor left as it was, for a nil document or one whose vertex
// or edge capacity is not the document's it edits.
func (e *Editor) Reset(d *gw.Document, mode Mode, grid float64) bool {
	if d == nil {
		return false
	}
	if c, was := d.Caps(), e.d.Caps(); c.Vertices != was.Vertices || c.Edges != was.Edges {
		return false
	}
	e.renew(d, mode, grid)
	return true
}

// renew sets every field as New makes it, keeping the arrays. The
// machine is taken back by its own events, a release and a mode change,
// which end whatever is under way; the drag's list is emptied first, so
// that the drag ended returns nothing, in either document.
func (e *Editor) renew(d *gw.Document, mode Mode, grid float64) {
	if !(grid > 0) || grid > gw.MaxCoord {
		grid = 8
	}
	e.dragV = e.dragV[:0]
	e.d = d
	e.Release()
	e.SetMode(mode)
	e.grid, e.elemW, e.elemH = grid, 0, 0
	e.selV, e.selE = e.selV[:0], e.selE[:0]
	e.hover, e.noExpand = background(), false
	e.down, e.last, e.panned = gw.Point{}, gw.Point{}, false
	clear(e.orig)
	clear(e.dropTo)
	e.connFrom, e.connPort, e.connKind, e.edgeKind, e.connAt = gw.None, gw.None, gw.None, gw.None, gw.Point{}
	e.sizeV, e.corner, e.item, e.typed = gw.None, gw.Point{}, 0, ""
	e.editV, e.editE, e.ctx = gw.None, gw.None, ContextRequest{Hit: background()}
	e.changeV, e.changeE, e.atV, e.layout = e.changeV[:0], e.changeE[:0], e.atV[:0], false
	clear(e.inV)
	clear(e.inE)
}
