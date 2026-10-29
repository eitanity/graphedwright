package svg_test

import (
	"math"

	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// TestHitTest: a point names the port, the vertex, the edge or the
// background it was drawn on, for every shape and route, the port
// winning over its vertex and the later vertex over the earlier; on a
// vertex without ports, the band along its shape's outline.
func TestHitTest(t *testing.T) {
	d, _ := shapes(t)
	for _, tc := range []struct {
		name string
		p    gw.Point
		want svg.Hit
	}{
		{"background", gw.Point{X: 900, Y: 300}, svg.Hit{Kind: svg.Background, Vertex: gw.None, Port: gw.None, Edge: gw.None}},
		{"pill source", gw.Point{X: 60, Y: 60}, svg.Hit{Kind: svg.VertexHit, Vertex: 0, Port: gw.None, Edge: gw.None}},
		{"node corner", gw.Point{X: 240 + 59, Y: 60 - 17}, svg.Hit{Kind: svg.VertexHit, Vertex: 1, Port: gw.None, Edge: gw.None}},
		{"port in", gw.Point{X: 180, Y: 61}, svg.Hit{Kind: svg.PortHit, Vertex: 1, Port: 0, Edge: gw.None}},
		{"port out, second on the right", gw.Point{X: 300, Y: 66}, svg.Hit{Kind: svg.PortHit, Vertex: 1, Port: 2, Edge: gw.None}},
		{"port top", gw.Point{X: 240, Y: 42}, svg.Hit{Kind: svg.PortHit, Vertex: 1, Port: 3, Edge: gw.None}},
		{"diamond", gw.Point{X: 240, Y: 180}, svg.Hit{Kind: svg.VertexHit, Vertex: 2, Port: gw.None, Edge: gw.None}},
		{"circle", gw.Point{X: 440, Y: 60}, svg.Hit{Kind: svg.VertexHit, Vertex: 4, Port: gw.None, Edge: gw.None}},
		{"straight edge", gw.Point{X: 150, Y: 61}, svg.Hit{Kind: svg.EdgeHit, Vertex: gw.None, Port: gw.None, Edge: 0}},
		{"orthogonal elbow", gw.Point{X: 220, Y: 120}, svg.Hit{Kind: svg.EdgeHit, Vertex: gw.None, Port: gw.None, Edge: 1}},
		{"curve", gw.Point{X: 340, Y: 180}, svg.Hit{Kind: svg.EdgeHit, Vertex: gw.None, Port: gw.None, Edge: 2}},
		{"near but not on", gw.Point{X: 150, Y: 75}, svg.Hit{Kind: svg.Background, Vertex: gw.None, Port: gw.None, Edge: gw.None}},
		{"pill outline, outside", gw.Point{X: 60 + 48 + 5, Y: 60}, svg.Hit{Kind: svg.OutlineHit, Vertex: 0, Port: gw.None, Edge: gw.None}},
		{"pill outline, inside", gw.Point{X: 60 - 48 + 5, Y: 60}, svg.Hit{Kind: svg.OutlineHit, Vertex: 0, Port: gw.None, Edge: gw.None}},
		{"pill, past the band", gw.Point{X: 60 + 48 - 7, Y: 60}, svg.Hit{Kind: svg.VertexHit, Vertex: 0, Port: gw.None, Edge: gw.None}},
		{"diamond's slanted side", gw.Point{X: 240 + 20, Y: 180 + 12}, svg.Hit{Kind: svg.OutlineHit, Vertex: 2, Port: gw.None, Edge: gw.None}},
		{"diamond's box corner, off its outline", gw.Point{X: 240 + 38, Y: 180 + 22}, svg.Hit{Kind: svg.VertexHit, Vertex: 2, Port: gw.None, Edge: gw.None}},
		{"circle outline", gw.Point{X: 440 + 20 + 4, Y: 60}, svg.Hit{Kind: svg.OutlineHit, Vertex: 4, Port: gw.None, Edge: gw.None}},
		{"circle outline on the diagonal", gw.Point{X: 440 + 14.14, Y: 60 - 14.14}, svg.Hit{Kind: svg.OutlineHit, Vertex: 4, Port: gw.None, Edge: gw.None}},
		{"node's border, between ports", gw.Point{X: 240 - 30, Y: 60 + 18}, svg.Hit{Kind: svg.VertexHit, Vertex: 1, Port: gw.None, Edge: gw.None}},
	} {
		if got := svg.HitTest(d, tc.p); got != tc.want {
			t.Errorf("%s at %v: got %+v, want %+v", tc.name, tc.p, got, tc.want)
		}
	}
	// In a container: what it holds and the edges first, the container
	// where nothing else is, the innermost of nested ones.
	n, _ := notation(t)
	shop := n.VertexByID("shop")
	for _, tc := range []struct {
		p    gw.Point
		kind svg.HitKind
		v    gw.VertexIndex
	}{{gw.Point{X: 280, Y: 80}, svg.VertexHit, n.VertexByID("place order")}, {gw.Point{X: 370, Y: 40}, svg.VertexHit, shop}, {gw.Point{X: 600, Y: 300}, svg.Background, gw.None}} {
		if got := svg.HitTest(n, tc.p); got.Kind != tc.kind || got.Vertex != tc.v {
			t.Errorf("in a container at %v: %+v", tc.p, got)
		}
	}
	// Where a vertex would go: the system takes a use case, not an
	// actor, and never itself.
	if c := svg.ContainerAt(n, gw.Point{X: 370, Y: 40}, n.VertexByID("pay")); c != shop {
		t.Errorf("a use case dropped in the system: %d", c)
	}
	if c := svg.ContainerAt(n, gw.Point{X: 370, Y: 40}, n.VertexByID("user")); c != gw.None {
		t.Errorf("an actor dropped in the system: %d", c)
	}
	if c := svg.ContainerAt(n, gw.Point{X: 370, Y: 40}, shop); c != gw.None {
		t.Errorf("the system dropped in itself: %d", c)
	}
	// A later vertex drawn over an earlier one wins; a dead one is skipped.
	over := vertex(t, d, kLeaf, "over", 60, 60)
	if got := svg.HitTest(d, gw.Point{X: 60, Y: 60}); got.Vertex != over {
		t.Errorf("overlap: %+v", got)
	}
	must(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: over})
	if got := svg.HitTest(d, gw.Point{X: 60, Y: 60}); got.Vertex != 0 {
		t.Errorf("after delete: %+v", got)
	}
}

// TestHitBends: a bent edge is hit along the bends it is drawn through,
// and no longer along the straight line between its ends.
func TestHitBends(t *testing.T) {
	d, _ := bends(t)
	skip := gw.EdgeIndex(3)
	b := d.Edges[skip].Bends
	if len(b) != 4 {
		t.Fatalf("skip has bends %v", b)
	}
	run := gw.Point{X: (b[2].X + b[3].X) / 2, Y: b[2].Y}
	if h := svg.HitTest(d, run); h.Kind != svg.EdgeHit || h.Edge != skip {
		t.Errorf("on the run past the third layer %v: %+v", run, h)
	}
	from, to := d.Vertices[d.Edges[skip].From].Pos, d.Vertices[d.Edges[skip].To].Pos
	chord := gw.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	if h := svg.HitTest(d, chord); h.Kind == svg.EdgeHit && h.Edge == skip {
		t.Errorf("on the straight line between the ends %v: %+v", chord, h)
	}
}

// TestHitBendsDown: in a top-to-bottom flow a bent edge's runs are
// vertical, and hit there.
func TestHitBendsDown(t *testing.T) {
	d, _ := tree(t)
	skip := gw.EdgeIndex(3)
	b := d.Edges[skip].Bends
	if len(b) != 4 || b[2].X != b[3].X {
		t.Fatalf("skip has bends %v, not vertical runs", b)
	}
	run := gw.Point{X: b[2].X, Y: (b[2].Y + b[3].Y) / 2}
	if h := svg.HitTest(d, run); h.Kind != svg.EdgeHit || h.Edge != skip {
		t.Errorf("on the run past the third layer %v: %+v", run, h)
	}
}

// TestLabelBoxes: an edge is hit on its label's text, but not on the
// rest of the wider box an input takes; a vertex's label box lies across
// it at the label, an actor's at its foot and a container's title at its
// top, each within the vertex, and an edge's above its middle.
func TestLabelBoxes(t *testing.T) {
	d, _ := diamond(t)
	fail := gw.EdgeIndex(1)
	b := svg.EdgeLabelBox(d, fail)
	centre := gw.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
	if h := svg.HitTest(d, centre); h.Kind != svg.EdgeHit || h.Edge != fail {
		t.Errorf("on the label of %q at %v: %+v", d.Edges[fail].Label, centre, h)
	}
	if b.W < 80 {
		t.Errorf("the input's box is %v wide", b.W)
	}
	if h := svg.HitTest(d, gw.Point{X: b.X + 2, Y: centre.Y}); h.Kind == svg.EdgeHit && h.Edge == fail {
		t.Error("the edge is hit past the end of its short label")
	}
	if a := d.Vertices[1]; !(b.Y+b.H < a.Pos.Y+40) {
		t.Errorf("the label box %v is not above its edge", b)
	}

	n, _ := notation(t)
	for v := range n.Vertices {
		x := &n.Vertices[v]
		lb := svg.VertexLabelBox(n, gw.VertexIndex(v)) //nolint:gosec // bounded
		cy := lb.Y + lb.H/2
		switch x.Kind {
		case 0: // an actor: its label at the foot of its box
			if cy < x.Pos.Y+x.Size.H/4 || cy > x.Pos.Y+x.Size.H/2 {
				t.Errorf("actor %s: label box %v", x.ID, lb)
			}
		case 4: // a container: its title at the top
			if lb.Y < x.Pos.Y-x.Size.H/2 || cy > x.Pos.Y-x.Size.H/2+gw.ContainerTitle || lb.X < x.Pos.X-x.Size.W/2 {
				t.Errorf("container %s: title box %v for %v %v", x.ID, lb, x.Pos, x.Size)
			}
		default:
			if math.Abs(cy-x.Pos.Y) > 4 || math.Abs(lb.X+lb.W/2-x.Pos.X) > 0.01 {
				t.Errorf("%s: label box %v for %v", x.ID, lb, x.Pos)
			}
		}
	}
}
