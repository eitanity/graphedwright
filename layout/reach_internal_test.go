package layout

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// reachDoc is, left to right, a vertex s before a container c that
// holds a, m behind a, with a vertex b far up beside a, and q far down
// beside a, and a member far down beyond the others that is gone; with
// edges from s to m, to q and to c. The layouter is left as a level at
// the top would leave it, with c's members placed from its centre: s
// at layer 0 and c at layer 1, both at 0 across, the gap between them
// 56 deep, so that a leg going more than 224 across is given an upright.
func reachDoc(t *testing.T) (l *Layouter, d *gw.Document, vs map[string]gw.VertexIndex, es map[string]gw.EdgeIndex) {
	t.Helper()
	k, err := gw.Compile(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 60, H: 40}},
			{Name: "zone", Size: gw.Size{W: 60, H: 40}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err = gw.New(k, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	vs = map[string]gw.VertexIndex{}
	for _, name := range []string{"s", "c", "a", "m", "b", "q", "gone"} {
		kind := gw.KindIndex(0)
		if name == "c" {
			kind = 1
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		vs[name] = ch.Vertices[0]
	}
	l = New(d.Caps())
	l.realN = d.VertexSlots()
	for name, p := range map[string][2]float64{"a": {-50, 0}, "m": {50, 0}, "b": {-50, -400}, "q": {-50, 300}, "gone": {150, 2000}} {
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: vs[name], To: vs["c"]}); err != nil {
			t.Fatal(err)
		}
		u := vs[name]
		l.rx[u], l.ry[u], l.w[u], l.h[u] = p[0], p[1], 60, 40
	}
	es = map[string]gw.EdgeIndex{}
	for _, to := range []string{"m", "q", "c"} {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs["s"], To: vs[to], FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		e := ch.Edges[0]
		es[to] = e
		l.eOn[e], l.eFrom[e], l.eTo[e] = true, int32(vs["s"]), int32(vs["c"])
	}
	if _, err := d.Apply(gw.Command{Op: gw.DeleteVertex, Vertex: vs["gone"]}); err != nil {
		t.Fatal(err)
	}
	l.edges = d.EdgeSlots()
	l.layer[vs["s"]], l.layer[vs["c"]] = 0, 1
	l.lx[0], l.lw[0], l.lx[1], l.lw[1] = 0, 60, 200, 200
	return l, d, vs, es
}

// TestReach: a leg into a container goes on inside it to the end, as
// far across as the end stands from the container's middle; or, where a
// member stands between the side it comes in at and the one on the way
// to the end, to the band past all the members that the run inside
// takes round them, half the padding out, on the side nearer the far
// end. A member that is gone stands nowhere, and a leg to the container
// itself goes to its middle; a leg to the far end, outside, goes to it.
func TestReach(t *testing.T) {
	l, d, vs, es := reachDoc(t)
	band := 300 + 20 + float64(gw.ContainerPad)/2 // past q, nearer s than past b
	for _, c := range []struct {
		name string
		e    gw.EdgeIndex
		at   gw.VertexIndex
		from gw.VertexIndex
		want float64
	}{
		{"to m, behind a", es["m"], vs["c"], vs["s"], band},
		{"to q, first on its side", es["q"], vs["c"], vs["s"], 300},
		{"to the container", es["c"], vs["c"], vs["s"], 0},
		{"back to s", es["m"], vs["s"], vs["c"], 0},
	} {
		if got := l.reachY(d, int(c.e), int32(c.at), int32(c.from), 8); got != c.want {
			t.Errorf("%s: reaches %v across, want %v", c.name, got, c.want)
		}
	}
	// From the high side, beyond m, nothing stands between: the run
	// comes straight in.
	if got, ok := l.band(d, vs["c"], vs["m"], 0, false); ok {
		t.Errorf("from the high side: a band at %v, want none", got)
	}
	// Into a from the high side, m stands between: the band past q.
	if got, ok := l.band(d, vs["c"], vs["a"], 0, false); !ok || got != band {
		t.Errorf("into a from the high side: a band at %v (%v), want %v", got, ok, band)
	}
	// From far up, the band past b.
	if got, ok := l.band(d, vs["c"], vs["m"], -1000, true); !ok || got != -400-20-float64(gw.ContainerPad)/2 {
		t.Errorf("from far up: a band at %v (%v), want %v", got, ok, -400-20-float64(gw.ContainerPad)/2)
	}
}

// TestShallowLegsReach: the legs given uprights are those going more
// than shallowRatio times their gap's depth across to where they reach,
// inside the container: to m, behind a, along the band, and to q, far
// down; not the leg to the container's middle.
func TestShallowLegsReach(t *testing.T) {
	l, d, _, es := reachDoc(t)
	n := l.shallowLegs(d, 8)
	got := map[int32]bool{}
	for k := range n {
		got[l.zIdx[k]/2] = true
	}
	for name, want := range map[string]bool{"m": true, "q": true, "c": false} {
		if got[int32(es[name])] != want {
			t.Errorf("the leg to %s given an upright %v, want %v", name, got[int32(es[name])], want)
		}
	}
}
