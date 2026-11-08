package layout

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// levelDoc is, left to right, a box a with an orthogonal edge to b
// ahead of it and one to c ahead and below, so that the two ends at a's
// side spread 10 either side of its middle.
func levelDoc(t *testing.T) (d *gw.Document, wire gw.EdgeIndex) {
	t.Helper()
	k, err := gw.Compile(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 60, H: 60}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "wire", Route: gw.Orthogonal}},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err = gw.New(k, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	var vs [3]gw.VertexIndex
	for i, p := range [3]gw.Point{{X: 0, Y: 0}, {X: 400, Y: 0}, {X: 400, Y: 100}} {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: p}); err != nil {
			t.Fatal(err)
		}
		vs[i] = ch.Vertices[0]
	}
	var es [2]gw.EdgeIndex
	for i, to := range [2]gw.VertexIndex{vs[1], vs[2]} {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[0], To: to, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		es[i] = ch.Edges[0]
	}
	return d, es[0]
}

// TestDetourLevels: a detour's first and last corners are level with
// its ends, and so is the next corner where the leg from an end runs
// level; once the routes are found, they are brought level with where
// the ends are drawn. A detour of fewer than four corners keeps those
// between to neither end, so that its middle stays where it was found.
func TestDetourLevels(t *testing.T) {
	for _, c := range []struct {
		path, want []gw.Point
	}{
		{
			path: []gw.Point{{X: 40, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 50}, {X: 370, Y: 50}},
			want: []gw.Point{{X: 40, Y: -10}, {X: 100, Y: -10}, {X: 100, Y: 0}, {X: 370, Y: 0}},
		},
		{
			path: []gw.Point{{X: 40, Y: 5}, {X: 100, Y: 20}, {X: 200, Y: 30}, {X: 370, Y: 40}},
			want: []gw.Point{{X: 40, Y: -10}, {X: 100, Y: 20}, {X: 200, Y: 30}, {X: 370, Y: 0}},
		},
		{
			path: []gw.Point{{X: 40, Y: 0}, {X: 100, Y: 0}, {X: 370, Y: 50}},
			want: []gw.Point{{X: 40, Y: -10}, {X: 100, Y: 0}, {X: 370, Y: 0}},
		},
	} {
		d, wire := levelDoc(t)
		l := New(d.Caps())
		copy(l.dt.path[:], c.path)
		l.setDetour(d, wire, l.drawnOf(d, wire), len(c.path))
		l.levelEnds(d)
		b := d.Edge(wire).Bends()
		if b.Len() != len(c.want) {
			t.Fatalf("%v: %d bends, want %d", c.path, b.Len(), len(c.want))
		}
		for k, p := range c.want {
			if b.At(k) != p {
				t.Errorf("%v: bend %d at %v, want %v", c.path, k, b.At(k), p)
			}
		}
		if l.route[wire].Valid() {
			t.Errorf("%v: the writer is kept after the ends are levelled", c.path)
		}
	}
}

// TestSetLevel: the bends recorded level with an end are one or two;
// none are for a count out of range.
func TestSetLevel(t *testing.T) {
	d, wire := levelDoc(t)
	l := New(d.Caps())
	for _, c := range []struct {
		e       gw.EdgeIndex
		end, at int
		n       int
		want    int32
	}{
		{e: wire, end: 1, at: 3, n: 2, want: 14},
		{e: wire, end: 0, at: 0, n: 1, want: 1},
		{e: wire, end: 0, at: 1, n: 0},
		{e: wire, end: 0, at: 1, n: 3},
	} {
		clear(l.lvl)
		l.setLevel(c.e, c.end, c.at, c.n)
		if got := l.lvl[2*int(c.e)+c.end]; got != c.want {
			t.Errorf("edge %d, end %d, %d from %d: recorded %d, want %d", c.e, c.end, c.n, c.at, got, c.want)
		}
	}
}

// TestInsideRunsInnermost: the run in to a member is taken to arrive
// level with its end only when it is the run inside the member's own
// container. Into m, inside c1 inside c2, past b before c1 in c2, the
// run along the gap inside c2 is found; inside c1, where m stands first
// and the leg to it is clear, there is none, and the route's last
// points, the run's along the gap to c1, are level with c1's middle.
func TestInsideRunsInnermost(t *testing.T) {
	k, err := gw.Compile(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 60, H: 60}},
			{Name: "zone", Size: gw.Size{W: 60, H: 60}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := gw.New(k, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex, at gw.Point) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: at}); err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	x, c2, b, c1, m := add(0, gw.Point{X: -300, Y: 0}), add(1, gw.Point{}), add(0, gw.Point{X: 0, Y: 0}), add(1, gw.Point{}), add(0, gw.Point{X: 200, Y: 0})
	for _, p := range [3][2]gw.VertexIndex{{b, c2}, {c1, c2}, {m, c1}} {
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: p[0], To: p[1]}); err != nil {
			t.Fatal(err)
		}
	}
	d.Fit()
	ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: x, To: m, FromPort: gw.None, ToPort: gw.None})
	if err != nil {
		t.Fatal(err)
	}
	l := New(d.Caps())
	var buf [insidePts]gw.Point
	n, atEnd := l.insideRuns(d, ch.Edges[0], m, d.Vertex(x).Pos(), false, buf[:])
	if n != gapPts || atEnd {
		t.Errorf("%d points, the last level with the end %v; want %d along the gap inside %v, and not", n, atEnd, gapPts, d.Vertex(c2).Pos())
	}
}

// TestRunClearsLevels: what a layouter recorded of the bends level with
// the ends in a run before is not taken for this run's: laid out with
// every edge's first two bends at either end recorded level, a long
// edge, whose bends are its dummies', keeps them as a new layouter
// places them.
func TestRunClearsLevels(t *testing.T) {
	var got [2]*gw.Document
	long := gw.EdgeIndex(-1)
	for k := range got {
		kinds, err := gw.Compile(gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 60, H: 60}}},
			EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
		})
		if err != nil {
			t.Fatal(err)
		}
		d, err := gw.New(kinds, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		var vs [4]gw.VertexIndex
		for i := range vs {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex})
			if err != nil {
				t.Fatal(err)
			}
			vs[i] = ch.Vertices[0]
		}
		for _, ft := range [5][2]int{{0, 1}, {1, 2}, {0, 2}, {0, 3}, {3, 2}} {
			ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[ft[0]], To: vs[ft[1]], FromPort: gw.None, ToPort: gw.None})
			if err != nil {
				t.Fatal(err)
			}
			if ft == [2]int{0, 2} {
				long = ch.Edges[0]
			}
		}
		l := New(d.Caps())
		if k == 0 {
			for i := range l.lvl {
				l.lvl[i] = 4*0 + levelBends
			}
		}
		if _, err := l.Run(d, Options{All: true}); err != nil {
			t.Fatal(err)
		}
		got[k] = d
	}
	for i := range got[0].EdgeSlots() {
		a, b := got[0].Edge(gw.EdgeIndex(i)).Bends(), got[1].Edge(gw.EdgeIndex(i)).Bends() //nolint:gosec // bounded by the capacity
		same := a.Len() == b.Len()
		for k := 0; same && k < a.Len(); k++ {
			same = a.At(k) == b.At(k)
		}
		if !same || a.Len() == 0 && gw.EdgeIndex(i) == long { //nolint:gosec // bounded by the capacity
			t.Errorf("edge %d laid out through %v, by a new layouter through %v", i, a, b)
		}
	}
}
