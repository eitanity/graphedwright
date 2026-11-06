package svg

import (
	"math/rand/v2"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// TestBentRoutesClear: on random DAGs laid out in layers, with edges of
// every route, the path drawn between an edge's first and last bends,
// sampled finely, passes through no vertex: the layers it crosses are
// passed by their runs and the legs between them keep to the gaps; in
// either flow, alternating by seed.
func TestBentRoutesClear(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 120, H: 36}},
			{Name: "diamond", Shape: gw.Diamond, Size: gw.Size{W: 80, H: 48}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "line"}, {Name: "curve", Route: gw.Curve}, {Name: "elbow", Route: gw.Orthogonal}},
		Acyclic:   true,
	}
	for seed := uint64(1); seed <= 16; seed++ {
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(gw.Flow(seed % 2)) //nolint:gosec // 0 or 1
		r := rand.New(rand.NewPCG(seed, 3))
		const n = 40
		for range n {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: gw.KindIndex(r.IntN(2))}); err != nil { //nolint:gosec // below 2
				t.Fatal(err)
			}
		}
		for range 70 {
			a, b := r.IntN(n), r.IntN(n)
			if a == b {
				continue
			}
			_, _ = d.Apply(gw.Command{Op: gw.AddEdge, Kind: gw.KindIndex(r.IntN(3)), From: gw.VertexIndex(min(a, b)), To: gw.VertexIndex(max(a, b)), FromPort: gw.None, ToPort: gw.None}) //nolint:gosec // below n
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
		bent := 0
		for i := range d.EdgeSlots() {
			e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
			if !e.Live() || e.Bends().Len() == 0 {
				continue
			}
			bent++
			route := d.Kinds().Edge(e.Kind()).Route()
			a, b := ends(d, gw.EdgeIndex(i), e.Bends()) //nolint:gosec // bounded by the capacity
			f := frameOf(d, e)
			for k := 1; k < e.Bends().Len(); k++ {
				p, q := leg(a, e.Bends(), b, k)
				p, q = f.t(p), f.t(q)
				lr := legRoute(route, true, p, q)
				for j := 0; j <= 32; j++ {
					x := f.t(f.at(lr, p, q, float64(j)/32))
					for v := range d.VertexSlots() {
						u := d.Vertex(gw.VertexIndex(v))                                                         //nolint:gosec // bounded by the capacity
						if gw.VertexIndex(v) != e.From() && gw.VertexIndex(v) != e.To() && insideBy(u, x, 0.5) { //nolint:gosec // bounded
							t.Errorf("seed %d: edge %s leg %d (%v to %v) passes through %s at %v", seed, e.ID(), k, p, q, u.ID(), x)
						}
					}
				}
			}
		}
		if bent < 10 {
			t.Errorf("seed %d: only %d edges bent", seed, bent)
		}
	}
}

// TestBackwardRoutesOneWay: an edge back from the end of a chain to its
// start is laid out through bends, and its route, sampled finely from
// end to end, never turns back along the flow: no leg hooks past a bend.
// For a curve and an elbow, in either flow.
func TestBackwardRoutesOneWay(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 120, H: 36}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}, {Name: "elbow", Route: gw.Orthogonal}},
	}
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for kind := range gw.KindIndex(2) {
			d, err := makeDoc(s, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			d.SetFlow(flow)
			const n = 5
			for range n {
				if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
					t.Fatal(err)
				}
			}
			for v := range gw.VertexIndex(n - 1) {
				if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: v, To: v + 1, FromPort: gw.None, ToPort: gw.None}); err != nil {
					t.Fatal(err)
				}
			}
			c, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: n - 1, To: 0, FromPort: gw.None, ToPort: gw.None})
			if err != nil {
				t.Fatal(err)
			}
			back := c.Edges[0]
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
				t.Fatal(err)
			}
			e := d.Edge(back)
			if e.Bends().Len() == 0 || !backward(d, e) {
				t.Fatalf("flow %d kind %d: the edge back has bends %v, backward %t", flow, kind, e.Bends(), backward(d, e))
			}
			a, b := ends(d, back, e.Bends())
			f := frameOf(d, e)
			route := d.Kinds().Edge(kind).Route()
			prev := f.t(a).X
			const samples = 256
			for j := 1; j <= samples; j++ {
				x := f.t(along(f, route, a, e.Bends(), b, float64(j)/samples)).X
				if x > prev+1e-9 {
					t.Fatalf("flow %d kind %d: the route turns back at %d/%d, %g after %g", flow, kind, j, samples, x, prev)
				}
				prev = x
			}
		}
	}
}

// TestLanesApart: two edges each way between two vertices in adjacent
// layers each run in a lane of their own, so that no two share their
// middle and no two labels overlap, and each, sampled finely, runs one
// way along the flow, without a loop. For a curve and an elbow, in
// either flow.
func TestLanesApart(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 120, H: 36}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}, {Name: "elbow", Route: gw.Orthogonal}},
	}
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for kind := range gw.KindIndex(2) {
			d, err := makeDoc(s, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			d.SetFlow(flow)
			for range 2 {
				if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
					t.Fatal(err)
				}
			}
			for k, ends := range [4][2]gw.VertexIndex{{0, 1}, {1, 0}, {0, 1}, {1, 0}} {
				if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: ends[0], To: ends[1], FromPort: gw.None, ToPort: gw.None, Text: "label " + string(rune('a'+k))}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
				t.Fatal(err)
			}
			route := s.EdgeKinds[kind].Route
			for i := range gw.EdgeIndex(4) {
				var lane [1]gw.Point
				w := wayOf(d, i, &lane)
				if w.n != 4 || w.k != int(i) {
					t.Fatalf("flow %d kind %d: edge %d in lane %d of %d", flow, kind, i, w.k, w.n)
				}
				dir := 1.0
				if w.f.back {
					dir = -1
				}
				prev := w.f.t(w.a).X
				for j := 1; j <= 256; j++ {
					x := w.f.t(along(w.f, route, w.a, w.through, w.b, float64(j)/256)).X
					if dir*(x-prev) < -1e-9 {
						t.Fatalf("flow %d kind %d: edge %d turns back at %d/256", flow, kind, i, j)
					}
					prev = x
				}
				bi := EdgeLabelBox(d, i)
				for o := range i {
					var olane [1]gw.Point
					ow := wayOf(d, o, &olane)
					if w.middle(route) == ow.middle(route) {
						t.Errorf("flow %d kind %d: edges %d and %d share their middle", flow, kind, o, i)
					}
					bo := EdgeLabelBox(d, o)
					if bi.X < bo.X+bo.W && bo.X < bi.X+bi.W && bi.Y < bo.Y+bo.H && bo.Y < bi.Y+bi.H {
						t.Errorf("flow %d kind %d: the labels of edges %d and %d overlap: %v %v", flow, kind, o, i, bo, bi)
					}
				}
			}
		}
	}
}

// insideBy reports whether p is inside the vertex's box by more than m.
func insideBy(v gw.VertexView, p gw.Point, m float64) bool {
	return p.X > v.Pos().X-v.Size().W/2+m && p.X < v.Pos().X+v.Size().W/2-m && p.Y > v.Pos().Y-v.Size().H/2+m && p.Y < v.Pos().Y+v.Size().H/2-m
}

// TestAcrossLegStraight: a leg of a bent curve straight across the
// flow, which the layout gives a run along the gap before a container's
// member, is drawn as a line, every point of it on the leg's line,
// where a curve leaving and arriving level doubled back into an S; a
// curve without bends keeps its route; in either flow.
func TestAcrossLegStraight(t *testing.T) {
	for _, f := range []frame{{}, {down: true}} {
		p, q := gw.Point{X: 100, Y: 0}, gw.Point{X: 100, Y: 200}
		lr := legRoute(gw.Curve, true, p, q)
		if lr != gw.Straight {
			t.Errorf("down %v: a leg across the flow drawn as route %d", f.down, lr)
		}
		for j := 0; j <= 16; j++ {
			if x := f.at(lr, p, q, float64(j)/16); x.X != 100 {
				t.Errorf("down %v: the leg strays to %v", f.down, x)
			}
		}
		if lr := legRoute(gw.Curve, false, p, q); lr != gw.Curve {
			t.Errorf("down %v: an unbent curve drawn as route %d", f.down, lr)
		}
	}
}

// TestLabelOffUpright: a route whose middle leg, by its bends, is an
// upright, straight across the flow, which a layout gives a leg at one
// end only, puts its label on the run beside it toward the middle: the
// run through the dummy before an upright at the target, after one at
// the source; in either flow.
func TestLabelOffUpright(t *testing.T) {
	for _, f := range []frame{{}, {down: true}} {
		pts := func(ps ...gw.Point) gw.PointList {
			for i := range ps {
				ps[i] = f.t(ps[i])
			}
			return gw.PointsOf(ps)
		}
		a, b := f.t(gw.Point{X: 0, Y: 0}), f.t(gw.Point{X: 400, Y: 300})
		atTarget := pts(gw.Point{X: 100, Y: 0}, gw.Point{X: 200, Y: 0}, gw.Point{X: 240, Y: 0}, gw.Point{X: 240, Y: 300})
		if got, want := f.t(middle(f, gw.Curve, a, atTarget, b)), (gw.Point{X: 150, Y: 0}); got != want {
			t.Errorf("down %v, upright at the target: label at %v, want %v", f.down, got, want)
		}
		atSource := pts(gw.Point{X: 40, Y: 0}, gw.Point{X: 40, Y: 300}, gw.Point{X: 100, Y: 300}, gw.Point{X: 200, Y: 300})
		if got, want := f.t(middle(f, gw.Curve, a, atSource, b)), (gw.Point{X: 150, Y: 300}); got != want {
			t.Errorf("down %v, upright at the source: label at %v, want %v", f.down, got, want)
		}
	}
}
