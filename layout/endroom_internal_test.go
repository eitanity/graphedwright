package layout

import (
	"slices"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestEndSpreads: an end's spread is half the step between the ends at
// its side for each beyond the first, the step EndGap or a share of the
// side across the flow. The ends of a loop, of a straight edge, of one
// not of the level, of one run in from a container and at a port are
// not spread and not counted; an edge to an earlier layer without a port
// leaves by the back side and arrives at the on side, with one the way
// its port goes; and past MaxEndSlots at a side none spreads.
func TestEndSpreads(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		k, err := gw.Compile(gw.Schema{
			VertexKinds: []gw.VertexKind{
				{Name: "box", Size: gw.Size{W: 100, H: 60}},
				{Name: "ported", Size: gw.Size{W: 100, H: 60}, Ports: []gw.Port{{Name: "out", Side: gw.Right}}},
			},
			EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}, {Name: "line"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		d, err := gw.New(k, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(f)
		var v [5]gw.VertexIndex
		for i, kind := range [5]gw.KindIndex{0, 0, 0, 1, 0} {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
			if err != nil {
				t.Fatal(err)
			}
			v[i] = ch.Vertices[0]
		}
		a, b, c, q, h := v[0], v[1], v[2], v[3], v[4]
		edge := func(kind gw.KindIndex, from, to gw.VertexIndex, port gw.PortIndex) int {
			ch, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: port, ToPort: gw.None})
			if err != nil {
				t.Fatal(err)
			}
			return int(ch.Edges[0])
		}
		l := New(d.Caps())
		l.down = f == gw.TopToBottom
		l.layer[a], l.layer[h], l.layer[b], l.layer[c], l.layer[q] = 0, 0, 1, 1, 1
		on := edge(0, a, b, gw.None)
		other := edge(0, a, c, gw.None)
		loop := edge(0, a, a, gw.None)
		line := edge(1, a, b, gw.None)
		ported := edge(0, q, a, 0)
		ported2 := edge(0, q, a, 0)
		off := edge(0, a, b, gw.None)
		back := edge(0, b, a, gw.None)
		held := edge(0, c, b, gw.None) // laid out from a, which holds c
		crowd := make([]int, gw.MaxEndSlots+1)
		for i := range crowd {
			crowd[i] = edge(0, h, c, gw.None)
		}
		l.edges = d.EdgeSlots()
		for i := range l.edges {
			e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // a few
			l.eOn[i], l.eFrom[i], l.eTo[i] = i != off, int32(e.From()), int32(e.To())
		}
		l.eFrom[held] = int32(a)
		l.endSpreads(d)
		// Three at a's on side and b's back side; two at a's back side.
		three, two := 15.0, 10.0 // a step of 60/4 across, and 20
		if f == gw.TopToBottom {
			three = 20 // a step of 20: 100/4 is more
		}
		want := map[int][2]float64{
			on: {three, three}, other: {three, 0}, loop: {}, line: {}, ported: {0, two}, ported2: {0, two},
			off: {}, back: {three, three}, held: {0, three}, crowd[0]: {},
		}
		for i, w := range want {
			if got := [2]float64{l.spread[2*i], l.spread[2*i+1]}; got != w {
				t.Errorf("flow %d: edge %d: spread %v, want %v", f, i, got, w)
			}
		}
	}
}

// TestEndDummy: the dummy next to an edge's source is its first from
// the source's layer, and the one next to its target its last; an edge
// to an earlier layer has its dummies from its target's.
func TestEndDummy(t *testing.T) {
	d, _ := levelDoc(t)
	l := New(d.Caps())
	l.eFrom[0], l.eTo[0], l.dFirst[0], l.dN[0] = 0, 1, 5, 3
	for _, c := range []struct {
		from, to int32
		want     [2]int32
	}{{0, 4, [2]int32{5, 7}}, {4, 0, [2]int32{7, 5}}} {
		l.layer[0], l.layer[1] = c.from, c.to
		if got := [2]int32{l.endDummy(0, 0), l.endDummy(0, 1)}; got != c.want {
			t.Errorf("layers %d to %d: %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// spreadLevel is a layouter with vertex 0 in layer 0 at y0, placed or
// not, the nodes of layer 1 in order at ys, of them 1 a placed vertex at
// placed if it is within ys, and the others dummies, each of an edge
// from vertex 0 whose end there spreads s.
func spreadLevel(t *testing.T, y0 float64, pinned bool, s float64, placed int, ys ...float64) *Layouter {
	t.Helper()
	d, _ := levelDoc(t)
	l := New(d.Caps())
	l.realN, l.layers, l.n = 2, 2, 2+len(ys)
	l.keys[0], l.keys[1], l.keys[2] = 0, 1, int32(1+len(ys)) //nolint:gosec // a few
	l.order[0], l.y[0], l.pin[0], l.layer[0] = 0, y0, pinned, 0
	for p, y := range ys {
		v := int32(2 + p) //nolint:gosec // a few
		if p == placed {
			v = 1
			l.pin[1] = true
		}
		l.order[1+p], l.pos[v], l.y[v], l.layer[v] = v, int32(p), y, 1 //nolint:gosec // a few
		if v > 1 {
			i := l.edges
			l.edges++
			l.eFrom[i], l.eTo[i], l.dFirst[i], l.dN[i], l.spread[2*i] = 0, 1, v, 1, s
		}
	}
	return l
}

// TestClearSpreads: a dummy next to an end within the spread there,
// once on the grid, moves out of it, the grid steps that clear it, the
// way it is off the middle, and pushes the nodes beyond it the gap clear
// in turn, up to a placed one; one at the middle or out of it once on
// the grid stays, as one does by the spread from a placed vertex off the
// grid.
func TestClearSpreads(t *testing.T) {
	const gap, g = 24, 8
	for _, c := range []struct {
		name   string
		y0     float64
		pinned bool
		s      float64
		placed int
		ys     []float64
		want   []float64
		moved  bool
	}{
		{"before the middle", 0, false, 30, -1, []float64{-30, -16}, []float64{-56, -32}, true},
		{"after it", 0, false, 30, -1, []float64{16, 32, 200}, []float64{32, 56, 200}, true},
		{"up to a placed one", 0, false, 30, 1, []float64{16, 40}, []float64{32, 40}, true},
		{"at the middle", 0, false, 30, -1, []float64{0}, []float64{0}, false},
		{"out once on the grid", 0, false, 30, -1, []float64{29}, []float64{29}, false},
		{"out by the grid", 0, false, 34, -1, []float64{37}, []float64{37}, false},
		{"at the spread's edge", 0, false, 32, -1, []float64{32}, []float64{40}, true},
		{"from a placed vertex", 3, true, 30, -1, []float64{32}, []float64{35}, true},
	} {
		l := spreadLevel(t, c.y0, c.pinned, c.s, c.placed, c.ys...)
		moved := l.clearSpreads(gap, g)
		got := make([]float64, 0, len(c.ys))
		for p := range c.ys {
			got = append(got, l.y[l.order[1+p]])
		}
		if moved != c.moved || !slices.Equal(got, c.want) {
			t.Errorf("%s: %v moved %v, want %v %v", c.name, got, moved, c.want, c.moved)
		}
	}
}

// TestStraightenClearOfSpreads: a long edge's dummies are not brought
// into a row that puts the one next to an end within the spread there.
func TestStraightenClearOfSpreads(t *testing.T) {
	d, _ := levelDoc(t)
	l := New(d.Caps())
	l.realN, l.layers, l.n, l.edges = 2, 4, 4, 1
	for k, v := range []int32{0, 2, 3, 1} {
		l.keys[k], l.order[k], l.layer[v], l.pos[v] = int32(k), v, int32(k), 0 //nolint:gosec // a few
	}
	l.keys[4] = 4
	l.y[2], l.y[3] = -16, -40
	l.eFrom[0], l.eTo[0], l.dFirst[0], l.dN[0], l.spread[0] = 0, 1, 2, 2, 30
	l.straighten(24, 8)
	if l.y[2] != -40 || l.y[3] != -40 {
		t.Errorf("dummies at %v and %v, want both at -40", l.y[2], l.y[3])
	}
}
