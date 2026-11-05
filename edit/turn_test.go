package edit_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
)

// TestPlaceTurned: a turning kind dropped from the palette in a
// top-to-bottom flow is kept in the view at its turned size: an 8 by 80
// bar across the flow is 80 wide, so dropped at the view's right edge
// its centre is 40 in from it.
func TestPlaceTurned(t *testing.T) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "fork", Shape: gw.Provided, Size: gw.Size{W: 8, H: 80}}},
		Shapes:      []gw.ShapeDef{{Name: "bar", Box: gw.Size{W: 8, H: 80}, Turns: true, Paths: []gw.ShapePath{{D: "M 0 0 H 8 V 80 H 0 Z", Fill: true}}}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(gw.TopToBottom)
	d.SetView(gw.Viewport{W: 400, H: 200})
	e := edit.New(d, edit.Edit, 8)
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: 0}, move(400, 100), up(400, 100))
	if v := d.Vertex(0); v.Size() != (gw.Size{W: 80, H: 8}) || v.Pos().X != 360 {
		t.Errorf("placed at %v, %v", v.Pos(), v.Size())
	}
}
