package svg

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// TestPortLabelsClear: laid out in either flow, with straight, curved
// and orthogonal edges at them, the label of each port of a vertex's
// own lies outside its vertex and clear of the edges at the port, and
// inside the drawing's bounds.
func TestPortLabelsClear(t *testing.T) {
	for _, route := range []gw.Route{gw.Straight, gw.Curve, gw.Orthogonal} {
		for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
			d, err := gw.New(gw.Schema{
				VertexKinds: []gw.VertexKind{
					{Name: "part", Shape: gw.Rect, Size: gw.Size{W: 100, H: 40}},
					{Name: "system", Size: gw.Size{W: 160, H: 100}, Container: true},
				},
				EdgeKinds: []gw.EdgeKind{{Name: "connect", Route: route, Arrow: true}},
			}, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			d.SetFlow(flow)
			add := func(kind gw.KindIndex, id string) gw.VertexIndex {
				return apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: id}).Vertices[0]
			}
			engine, tank, shaft, ecu, frame := add(0, "engine"), add(0, "tank"), add(0, "shaft"), add(0, "ecu"), add(0, "frame")
			sys, axle := add(1, "drive"), add(0, "axle")
			apply(t, d, gw.Command{Op: gw.SetParent, Vertex: add(0, "gearbox"), To: sys})
			for _, p := range []gw.Port{
				{Name: "fuel", Side: gw.Left}, {Name: "air", Side: gw.Left}, {Name: "torque", Side: gw.Right},
				{Name: "control", Side: gw.Top}, {Name: "mount", Side: gw.Bottom},
			} {
				apply(t, d, gw.Command{Op: gw.AddPort, Vertex: engine, Port: p})
			}
			apply(t, d, gw.Command{Op: gw.AddPort, Vertex: sys, Port: gw.Port{Name: "power", Side: gw.Left}})
			apply(t, d, gw.Command{Op: gw.AddPort, Vertex: sys, Port: gw.Port{Name: "wheels", Side: gw.Right}})
			for _, c := range []gw.Command{
				{From: tank, To: engine, ToPort: 0}, {From: ecu, To: engine, ToPort: 3},
				{From: engine, To: shaft, FromPort: 2, ToPort: gw.None}, {From: engine, To: frame, FromPort: 4, ToPort: gw.None},
				{From: shaft, To: sys, ToPort: 0}, {From: sys, To: axle, FromPort: 1, ToPort: gw.None},
			} {
				c.Op = gw.AddEdge
				if c.From != engine && c.From != sys {
					c.FromPort = gw.None
				}
				apply(t, d, c)
			}
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			view := bounds(d)
			for _, v := range []gw.VertexIndex{engine, sys} {
				x := &d.Vertices[v]
				for p := range x.Ports {
					_, _, _, box := portLabel(d, v, gw.PortIndex(p)) //nolint:gosec // a handful
					name := x.Ports[p].Name
					if overlaps(box, gw.Viewport{X: x.Pos.X - x.Size.W/2, Y: x.Pos.Y - x.Size.H/2, W: x.Size.W, H: x.Size.H}) {
						t.Errorf("route %d flow %d: %s's label %v overlaps %s at %v %v", route, flow, name, box, x.ID, x.Pos, x.Size)
					}
					if box.X < view.X || box.Y < view.Y || box.X+box.W > view.X+view.W || box.Y+box.H > view.Y+view.H {
						t.Errorf("route %d flow %d: %s's label %v outside the bounds %v", route, flow, name, box, view)
					}
					for e := range d.Edges {
						ed := &d.Edges[e]
						at := (ed.From == v && int(ed.FromPort) == p) || (ed.To == v && int(ed.ToPort) == p)
						if at && crossesBox(d, gw.EdgeIndex(e), box) { //nolint:gosec // a handful
							t.Errorf("route %d flow %d: edge %s crosses %s's label %v", route, flow, ed.ID, name, box)
						}
					}
				}
			}
		}
	}
}

// crossesBox reports whether an edge's path, sampled finely, enters the
// box, as drawn: its stroke a pixel either side of its line.
func crossesBox(d *gw.Document, i gw.EdgeIndex, box gw.Viewport) bool {
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	f := wy.f
	route := d.Schema.EdgeKinds[d.Edges[i].Kind].Route
	grown := gw.Viewport{X: box.X - 1, Y: box.Y - 1, W: box.W + 2, H: box.H + 2}
	for k := range len(wy.through) + 1 {
		u, w := leg(wy.a, wy.through, wy.b, k)
		u, w = f.t(u), f.t(w)
		r := legRoute(route, len(wy.through) > 0, u, w)
		for s := 0; s <= 400; s++ {
			p := f.t(f.at(r, u, w, float64(s)/400))
			if p.X > grown.X && p.X < grown.X+grown.W && p.Y > grown.Y && p.Y < grown.Y+grown.H {
				return true
			}
		}
	}
	return false
}
