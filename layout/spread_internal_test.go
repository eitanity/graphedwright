package layout

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestDrawnSpread: the layout takes a curved or orthogonal edge as the
// renderer draws it, its ends off the middles of their sides where
// others are drawn to them too, into a vertex and out of it (gw.Document.EndOffset), in either flow;
// and a detour between the same two vertices leaves its ends off them by
// its lane's offset only when straight, the others' ends being apart.
func TestDrawnSpread(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		k, err := gw.Compile(gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 60, H: 60}}},
			EdgeKinds:   []gw.EdgeKind{{Name: "wire", Route: gw.Orthogonal}},
		})
		if err != nil {
			t.Fatal(err)
		}
		d, err := gw.New(k, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(f)
		var vs [5]gw.VertexIndex
		for i, p := range [5][2]float64{{0, -50}, {0, 50}, {200, 0}, {400, 0}, {400, 100}} {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex})
			if err != nil {
				t.Fatal(err)
			}
			at := gw.Point{X: p[0], Y: p[1]}
			if f == gw.TopToBottom {
				at = gw.Point{X: p[1], Y: p[0]}
			}
			if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: at}); err != nil {
				t.Fatal(err)
			}
			vs[i] = ch.Vertices[0]
		}
		var es [4]gw.EdgeIndex
		for i, ft := range [4][2]int{{0, 2}, {1, 2}, {2, 3}, {2, 4}} {
			ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[ft[0]], To: vs[ft[1]], FromPort: gw.None, ToPort: gw.None})
			if err != nil {
				t.Fatal(err)
			}
			es[i] = ch.Edges[0]
		}
		l := New(d.Caps())
		l.down = f == gw.TopToBottom
		_, cy := l.frameOf(d.Vertex(vs[2]).Pos())
		for i, want := range [4]float64{-10, 10, -10, 10} {
			w := l.drawnOf(d, es[i])
			got, at := w.b.Y, d.EndOffset(es[i], true)
			if i >= 2 {
				got, at = w.a.Y, d.EndOffset(es[i], false)
			}
			if got != cy+want || got-cy != at {
				t.Errorf("flow %d: edge %d ends at %v across, want %v", f, i, got, cy+want)
			}
		}
	}
	for _, c := range []struct {
		route gw.Route
		want  float64
	}{{gw.Straight, -8}, {gw.Curve, 0}, {gw.Orthogonal, 0}} {
		if got := laneOff(c.route, 0, 2, 16); got != c.want {
			t.Errorf("route %d: the first of two detours off by %v, want %v", c.route, got, c.want)
		}
	}
}
