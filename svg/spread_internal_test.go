package svg

import (
	"math"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// spreadDoc is a target of the shape at (300, 0), 100 by 60 turned to
// the flow, and n curved edges into it from sources spread across the
// flow, in flow f.
func spreadDoc(t *testing.T, f gw.Flow, shape gw.Shape, n int) *gw.Document {
	t.Helper()
	size := gw.Size{W: 100, H: 60}
	if f == gw.TopToBottom {
		size = gw.Size{W: 60, H: 100}
	}
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "target", Shape: shape, Size: size}, {Name: "source", Size: gw.Size{W: 40, H: 20}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(f)
	place := func(kind gw.KindIndex, along, across float64) gw.VertexIndex {
		p := gw.Point{X: along, Y: across}
		if f == gw.TopToBottom {
			p = gw.Point{X: across, Y: along}
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: p}); err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	tv := place(0, 300, 0)
	for k := range n {
		v := place(1, 0, float64(100*k-50*(n-1)))
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: v, To: tv, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// TestSpreadEnds: two curves into one side of a vertex end apart along
// it, by their offsets, on the shape's outline: on the box's side for a
// rectangle, moved in to the slanting side of a diamond and the curve of
// an ellipse; one alone ends at the middle; in either flow. Each curve's
// label goes on the side away from the other's end, and a lone curve's
// on the usual side.
func TestSpreadEnds(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		ft := frame{down: f == gw.TopToBottom}
		for _, shape := range []gw.Shape{gw.Rect, gw.Diamond, gw.Circle} {
			d := spreadDoc(t, f, shape, 2)
			tv := d.Vertex(0)
			for i := range gw.EdgeIndex(2) {
				_, b := ends(d, i, gw.PointList{})
				q := ft.t(b)
				if want := float64(20*i - 10); q.Y != want {
					t.Errorf("flow %d, shape %d, edge %d: ends %v across, want %v", f, shape, i, q.Y, want)
				}
				if dist := outlineDistance(shape, tv, b); math.Abs(dist) > 1e-6 {
					t.Errorf("flow %d, shape %d, edge %d: ends at %v, %v off the outline", f, shape, i, b, dist)
				}
				if shape == gw.Rect && q.X != 250 {
					t.Errorf("flow %d: edge %d ends at %v along, off the box's side", f, i, q.X)
				}
				var lane [1]gw.Point
				w := wayOf(d, i, &lane)
				want := 1.0 // the first, before the middle: above left to right, left top to bottom
				if f == gw.TopToBottom {
					want = -1
				}
				if i == 1 {
					want = -want
				}
				if side := w.side(); side != want {
					t.Errorf("flow %d, shape %d, edge %d: texts on side %v, want %v", f, shape, i, side, want)
				}
			}
			alone := spreadDoc(t, f, shape, 1)
			_, b := ends(alone, 0, gw.PointList{})
			if q := ft.t(b); q.Y != 0 || q.X != 250 {
				t.Errorf("flow %d, shape %d: a lone curve ends at %v, not the middle", f, shape, b)
			}
			var lane [1]gw.Point
			if w := wayOf(alone, 0, &lane); w.side() != 1 {
				t.Errorf("flow %d, shape %d: a lone curve's texts on side %v", f, shape, w.side())
			}
		}
	}
}

// TestSpreadAwayFromSource: an edge alone at its target but spread at
// its source, one of two curves out of one vertex to two targets across
// the flow from each other, puts its texts on the side away from the
// other's end there; in either flow.
func TestSpreadAwayFromSource(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		s := gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 60, H: 60}}},
			EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
		}
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(f)
		var vs [3]gw.VertexIndex
		for i, p := range [3][2]float64{{0, 0}, {200, -100}, {200, 100}} {
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
		for i := range 2 {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[0], To: vs[i+1], FromPort: gw.None, ToPort: gw.None}); err != nil {
				t.Fatal(err)
			}
		}
		for i := range gw.EdgeIndex(2) {
			want := 1.0
			if f == gw.TopToBottom {
				want = -1
			}
			if i == 1 {
				want = -want
			}
			var lane [1]gw.Point
			if w := wayOf(d, i, &lane); w.side() != want {
				t.Errorf("flow %d, edge %d: texts on side %v, want %v", f, i, w.side(), want)
			}
		}
	}
}
