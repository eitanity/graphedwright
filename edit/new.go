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
	if !(grid > 0) || grid > gw.MaxCoord {
		grid = 8
	}
	caps := d.Caps()
	e := &Editor{
		d: d, g: gesture.New(), grid: grid,
		selV: make([]gw.VertexIndex, 0, caps.Vertices), selE: make([]gw.EdgeIndex, 0, caps.Edges),
		orig: make([]gw.Point, caps.Vertices), hover: background(),
		dragV: make([]gw.VertexIndex, 0, caps.Vertices), dropTo: make([]gw.VertexIndex, caps.Vertices),
		connFrom: gw.None, connPort: gw.None, connKind: gw.None, edgeKind: gw.None, editV: gw.None, editE: gw.None, sizeV: gw.None,
		changeV: make([]gw.VertexIndex, 0, caps.Vertices), changeE: make([]gw.EdgeIndex, 0, caps.Edges),
		inV: make([]bool, caps.Vertices), inE: make([]bool, caps.Edges), atV: make([]gw.EdgeIndex, 0, caps.Edges),
		ctx: ContextRequest{Hit: background()},
	}
	e.SetMode(mode)
	return e
}
