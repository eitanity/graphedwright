package graphedwright_test

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// folded builds zone o holding zone z holding boxes a and b, box c in o
// beside z, box x outside, and edges x→a, a→b, b→c and c→x.
func folded(t *testing.T) (d *gw.Document, o, z, a, b, c, x gw.VertexIndex) {
	t.Helper()
	d = containerDoc(t)
	a = at(t, d, cBox, "a", 0, 0)
	b = at(t, d, cBox, "b", 100, 40)
	c = at(t, d, cBox, "c", 100, 200)
	x = at(t, d, cBox, "x", 600, 0)
	z = at(t, d, cZone, "z", 500, 500)
	o = at(t, d, cZone, "o", 900, 900)
	for _, p := range [][2]gw.VertexIndex{{a, z}, {b, z}, {z, o}, {c, o}} {
		apply(t, d, gw.Command{Op: gw.SetParent, Vertex: p[0], To: p[1]})
	}
	for _, e := range [][2]gw.VertexIndex{{x, a}, {a, b}, {b, c}, {c, x}} {
		addEdge(t, d, 0, e[0], e[1], gw.None, gw.None)
	}
	return d, o, z, a, b, c, x
}

// TestFold: a folded container is its own size with its top left corner
// kept, what it holds hidden and drawn as the outermost fold; an edge
// within one fold is hidden; a Move carries the hidden; Unfold fits it
// again; each undoes and redoes; and the refusals leave it as it was.
func TestFold(t *testing.T) {
	d, o, z, a, b, c, x := folded(t)
	open := d.Vertex(z)
	corner := gw.Point{X: open.Pos().X - open.Size().W/2, Y: open.Pos().Y - open.Size().H/2}
	before := snapshot(d)
	ch := apply(t, d, gw.Command{Op: gw.Fold, Vertex: z})
	if !ch.Layout || !slices.Contains(ch.Vertices, a) || !slices.Contains(ch.Edges, 0) {
		t.Errorf("fold's change %+v", ch)
	}
	v := d.Vertex(z)
	if !v.Folded() || v.Size() != (gw.Size{W: 100, H: 60}) || v.Pos().X-v.Size().W/2 != corner.X || v.Pos().Y-v.Size().H/2 != corner.Y {
		t.Errorf("folded z at %v size %v, its corner was %v", v.Pos(), v.Size(), corner)
	}
	if !d.Hidden(a) || !d.Hidden(b) || d.Hidden(z) || d.Hidden(c) || d.DrawnAs(a) != z || d.DrawnAs(x) != x {
		t.Errorf("hidden a %v b %v z %v c %v, a drawn as %d", d.Hidden(a), d.Hidden(b), d.Hidden(z), d.Hidden(c), d.DrawnAs(a))
	}
	if !d.EdgeHidden(1) || d.EdgeHidden(0) || d.EdgeHidden(2) || d.Members(z) != 2 || d.Members(o) != 4 || d.Members(a) != 0 {
		t.Errorf("edges hidden %v %v %v, members %d %d", d.EdgeHidden(0), d.EdgeHidden(1), d.EdgeHidden(2), d.Members(z), d.Members(o))
	}
	// Folded within a fold: drawn as the outermost.
	apply(t, d, gw.Command{Op: gw.Fold, Vertex: o})
	if d.DrawnAs(a) != o || d.DrawnAs(z) != o || !d.EdgeHidden(2) || d.EdgeHidden(3) {
		t.Errorf("in o folded: a drawn as %d, z as %d", d.DrawnAs(a), d.DrawnAs(z))
	}
	d.Undo()
	pa := d.Vertex(a).Pos()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: v.Pos().X + 10, Y: v.Pos().Y}})
	if d.Vertex(a).Pos().X != pa.X+10 {
		t.Errorf("a hidden in z moved from %v to %v", pa, d.Vertex(a).Pos())
	}
	d.Undo()
	for _, bad := range []gw.Command{
		{Op: gw.Fold, Vertex: z},
		{Op: gw.Fold, Vertex: a},
		{Op: gw.Unfold, Vertex: o},
		{Op: gw.Fold, Vertex: gw.None},
		{Op: gw.Unfold, Vertex: 99},
	} {
		was := snapshot(d)
		if _, err := d.Apply(bad); err == nil || snapshot(d) != was {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	apply(t, d, gw.Command{Op: gw.Unfold, Vertex: z})
	if snapshot(d) != before {
		t.Errorf("unfolded:\n%s\nwant\n%s", snapshot(d), before)
	}
	d.Undo()
	if !d.Vertex(z).Folded() || !d.Hidden(a) {
		t.Error("the unfold's undo left z unfolded")
	}
	d.Undo()
	if snapshot(d) != before {
		t.Errorf("the fold's undo:\n%s\nwant\n%s", snapshot(d), before)
	}
	d.Redo()
	if !d.Vertex(z).Folded() || d.Vertex(z).Size() != v.Size() {
		t.Error("the fold's redo")
	}
	// SetParent into a folded container hides the vertex.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: x, To: z})
	if !d.Hidden(x) || !d.EdgeHidden(0) {
		t.Errorf("x put in folded z: hidden %v", d.Hidden(x))
	}
	for _, i := range []gw.VertexIndex{gw.None, 99} {
		if d.Hidden(i) || d.DrawnAs(i) != gw.None || d.Members(i) != 0 {
			t.Errorf("out of range %d", i)
		}
	}
	if d.EdgeHidden(gw.None) || d.EdgeHidden(99) {
		t.Error("an edge out of range hidden")
	}
}

// TestFrameless: a frameless kind must be a container.
func TestFrameless(t *testing.T) {
	s := gw.Schema{VertexKinds: []gw.VertexKind{{Name: "group", Size: gw.Size{W: 10, H: 10}, Frameless: true}}}
	if _, err := gw.Compile(s); err == nil {
		t.Error("a frameless kind that is not a container compiled")
	}
	s.VertexKinds[0].Container = true
	k, err := gw.Compile(s)
	if err != nil || !k.Vertex(0).Frameless() || k.Vertex(9).Frameless() {
		t.Fatalf("frameless container: %v", err)
	}
}

// TestFind: the next match after the one given, round to the start, by
// label or ID with the case folded, hidden ones too; none for empty
// text or no match; and the count.
func TestFind(t *testing.T) {
	d, _, z, a, b, _, x := folded(t)
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "Pumpe ÄRGER"})
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: b, Text: "the pump"})
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: x, Text: "KK"}) // K and the Kelvin sign
	apply(t, d, gw.Command{Op: gw.Fold, Vertex: z})
	for _, c := range []struct {
		text  string
		after gw.VertexIndex
		want  gw.VertexIndex
	}{
		{"pump", gw.None, a},
		{"PUMP", a, b},
		{"pump", b, a},
		{"ärger", gw.None, a},
		{"kk", gw.None, x},
		{"x", gw.None, x},
		{"Z", gw.None, z},
		{"none", gw.None, gw.None},
		{"", gw.None, gw.None},
		{"pump", 99, a},
	} {
		if got := d.Find(c.text, c.after); got != c.want {
			t.Errorf("Find(%q, %d) = %d, want %d", c.text, c.after, got, c.want)
		}
	}
	if n := d.FindCount("PUMP"); n != 2 || d.FindCount("") != 0 {
		t.Errorf("count %d", n)
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: b})
	if got := d.Find("pump", a); got != a {
		t.Errorf("a deleted vertex found: %d", got)
	}
	if got := newDoc(t, gw.Caps{}).Find("a", gw.None); got != gw.None {
		t.Errorf("found in an empty document: %d", got)
	}
}

// TestAround: Around marks what a breadth-first search of the edges,
// written plainly, reaches within the depth, each way, on random
// graphs; refuses a short buffer or a negative depth; and allocates
// nothing.
func TestAround(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 9)) //nolint:gosec // a test's fixed sequence
	for round := range 40 {
		d := newDoc(t, gw.Caps{Vertices: 64, Edges: 256})
		n := 2 + r.IntN(30)
		for i := range n {
			addVertex(t, d, kLeaf, "v"+strconv.Itoa(i))
		}
		for range r.IntN(3 * n) {
			f, to := gw.VertexIndex(r.IntN(n)), gw.VertexIndex(r.IntN(n)) //nolint:gosec // bounded by n
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: eAny, From: f, To: to, FromPort: gw.None, ToPort: gw.None}); err != nil {
				continue
			}
		}
		if n > 3 {
			apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: 3})
		}
		from := []gw.VertexIndex{gw.VertexIndex(r.IntN(n))} //nolint:gosec // bounded by n
		in := make([]bool, d.VertexSlots())
		for _, way := range []gw.Direction{gw.Both, gw.Successors, gw.Predecessors} {
			for depth := range 4 {
				got := d.Around(from, depth, way, in)
				want := reached(d, from, depth, way)
				count := 0
				for i := range in {
					if in[i] != want[i] {
						t.Fatalf("round %d way %d depth %d: vertex %d marked %v", round, way, depth, i, in[i])
					}
					if in[i] {
						count++
					}
				}
				if got != count {
					t.Fatalf("round %d: %d marked, returned %d", round, count, got)
				}
			}
		}
	}
	d, _, _, a, _, _, _ := folded(t)
	if d.Around([]gw.VertexIndex{a}, 1, gw.Both, make([]bool, 2)) != 0 || d.Around([]gw.VertexIndex{a}, -1, gw.Both, make([]bool, 8)) != 0 {
		t.Error("a short buffer or a negative depth marked")
	}
	in := make([]bool, d.VertexSlots())
	if n := testing.AllocsPerRun(20, func() { d.Around([]gw.VertexIndex{a}, 3, gw.Both, in) }); n > 0 {
		t.Errorf("Around: %g allocations", n)
	}
}

// reached is a plain breadth-first search: the live vertices within
// depth live edges of from, the way given.
func reached(d *gw.Document, from []gw.VertexIndex, depth int, way gw.Direction) []bool {
	in := make([]bool, d.VertexSlots())
	var front []gw.VertexIndex
	for _, v := range from {
		if d.Vertex(v).Live() && !in[v] {
			in[v] = true
			front = append(front, v)
		}
	}
	for range depth {
		var next []gw.VertexIndex
		for _, v := range front {
			for e := range d.EdgeSlots() {
				x := d.Edge(gw.EdgeIndex(e)) //nolint:gosec // bounded by the capacity
				if !x.Live() {
					continue
				}
				if way != gw.Predecessors && x.From() == v && !in[x.To()] {
					in[x.To()] = true
					next = append(next, x.To())
				}
				if way != gw.Successors && x.To() == v && !in[x.From()] {
					in[x.From()] = true
					next = append(next, x.From())
				}
			}
		}
		front = next
	}
	return in
}

// TestOutline: every live vertex in drawing order, each container before
// what it holds, with depth, fold and how many it holds; within the
// buffer's capacity; allocating nothing.
func TestOutline(t *testing.T) {
	d, o, z, a, b, c, x := folded(t)
	apply(t, d, gw.Command{Op: gw.Fold, Vertex: z})
	got := d.Outline(make([]gw.OutlineEntry, 0, d.VertexSlots()))
	want := []gw.OutlineEntry{
		{Vertex: x, Depth: 0},
		{Vertex: o, Depth: 0, Holds: 4},
		{Vertex: c, Depth: 1},
		{Vertex: z, Depth: 1, Folded: true, Holds: 2},
		{Vertex: a, Depth: 2},
		{Vertex: b, Depth: 2},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("outline\n%+v\nwant\n%+v", got, want)
	}
	if short := d.Outline(make([]gw.OutlineEntry, 0, 2)); len(short) != 2 || short[1].Vertex != o {
		t.Errorf("into two: %+v", short)
	}
	if d.Containers().Len() != 2 {
		t.Errorf("the outline changed the containers: %d", d.Containers().Len())
	}
	buf := make([]gw.OutlineEntry, 0, d.VertexSlots())
	if n := testing.AllocsPerRun(20, func() { d.Outline(buf) }); n > 0 {
		t.Errorf("Outline: %g allocations", n)
	}
	if n := testing.AllocsPerRun(20, func() { d.Find("zz", gw.None) }); n > 0 {
		t.Errorf("Find: %g allocations", n)
	}
}
