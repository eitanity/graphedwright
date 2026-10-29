package graphedwright_test

import (
	"math"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestSetBends: bends are copied into the slab for a live edge; a dead
// edge, a point that is not finite, or more points than the slab has
// room for are refused and leave the edge without bends; ClearBends
// empties the slab for the next layout.
// TestBendsInRange: a bend past MaxCoord is refused, and leaves the
// edge without bends.
func TestBendsInRange(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 400, 0)
	e := addEdge(t, d, 0, a, b, gw.None, gw.None)
	if !d.SetBends(e, []gw.Point{{X: 100, Y: gw.MaxCoord}}) || d.SetBends(e, []gw.Point{{X: 100, Y: 2e9}}) || d.Edges[e].Bends != nil {
		t.Errorf("bends %v", d.Edges[e].Bends)
	}
}

func TestSetBends(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 400, 0)
	e := addEdge(t, d, 0, a, b, gw.None, gw.None)
	pts := []gw.Point{{X: 100, Y: 80}, {X: 200, Y: 80}}
	if !d.SetBends(e, pts) {
		t.Fatal("refused")
	}
	pts[0].X = -1
	if got := d.Edges[e].Bends; len(got) != 2 || got[0] != (gw.Point{X: 100, Y: 80}) {
		t.Errorf("bends %v, not a copy", got)
	}
	if d.SetBends(e, []gw.Point{{X: math.NaN()}}) || d.Edges[e].Bends != nil {
		t.Errorf("a NaN was taken: %v", d.Edges[e].Bends)
	}
	if d.SetBends(gw.EdgeIndex(7), pts) {
		t.Error("an edge out of range took bends")
	}
	if !d.SetBends(e, nil) || d.Edges[e].Bends != nil {
		t.Error("no bends refused")
	}

	small, err := gw.New(d.Schema, gw.Caps{Edges: 2, BendsPerEdge: 2})
	if err != nil {
		t.Fatal(err)
	}
	x := apply(t, small, gw.Command{Op: gw.AddVertex, Kind: cBox}).Vertices[0]
	y := apply(t, small, gw.Command{Op: gw.AddVertex, Kind: cBox}).Vertices[0]
	f := apply(t, small, gw.Command{Op: gw.AddEdge, From: x, To: y, FromPort: gw.None, ToPort: gw.None}).Edges[0]
	if !small.SetBends(f, make([]gw.Point, 3)) || small.SetBends(f, make([]gw.Point, 2)) {
		t.Error("the slab of four points took five")
	}
	if small.Edges[f].Bends != nil {
		t.Error("a refused edge kept its bends")
	}
	small.ClearBends()
	if !small.SetBends(f, make([]gw.Point, 4)) {
		t.Error("ClearBends did not empty the slab")
	}
	small.ClearBends()
	if small.Edges[f].Bends != nil {
		t.Error("ClearBends left the bends")
	}
}

// TestBendsHold: an edge is drawn through its bends while its ends are
// where and the size they were when it was routed: moving or resizing
// an end hides them, an undo that puts it back shows them again, moving
// a vertex at neither end leaves them; moving a container carries the
// bends of the edges within it and hides those of the edges that leave
// it; the change names the edges whose drawing changed.
func TestBendsHold(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 400, 0)
	c := at(t, d, cBox, "c", 800, 0)
	z := at(t, d, cZone, "z", 500, 500)
	ab := addEdge(t, d, 0, a, b, gw.None, gw.None)
	bc := addEdge(t, d, 0, b, c, gw.None, gw.None)
	set := func() {
		t.Helper()
		d.ClearBends()
		for _, e := range []gw.EdgeIndex{ab, bc} {
			if !d.SetBends(e, []gw.Point{{X: 10, Y: 80}}) {
				t.Fatal("refused")
			}
		}
	}
	bent := func(name string, e gw.EdgeIndex, want bool) {
		t.Helper()
		if got := len(d.Edges[e].Bends) > 0; got != want {
			t.Errorf("%s: edge %d bent %v, want %v", name, e, got, want)
		}
	}

	set()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: c, Pos: gw.Point{X: 800, Y: 0}})
	bent("move in place", bc, true)
	ch := apply(t, d, gw.Command{Op: gw.Move, Vertex: c, Pos: gw.Point{X: 808, Y: 0}})
	bent("move c", bc, false)
	bent("move c", ab, true)
	if len(ch.Edges) != 1 || ch.Edges[0] != bc {
		t.Errorf("move c: change names edges %v", ch.Edges)
	}
	d.Undo()
	bent("undo the move", bc, true)
	d.Redo()
	bent("redo the move", bc, false)
	d.Undo()
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 60, H: 20}})
	bent("resize a", ab, false)
	bent("resize a", bc, true)
	d.Undo()
	bent("undo the resize", ab, true)
	apply(t, d, gw.Command{Op: gw.Layout})
	bent("a layout command, before the layout moves anything", ab, true)

	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: z})
	set()
	at0 := d.Vertices[z].Pos
	apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: at0.X + 16, Y: at0.Y + 8}})
	if got := d.Edges[ab].Bends; len(got) != 1 || got[0] != (gw.Point{X: 26, Y: 88}) {
		t.Errorf("an edge within the moved zone: bends %v, want carried to (26, 88)", got)
	}
	bent("the edge leaving the zone", bc, false)
	d.Undo()
	if got := d.Edges[ab].Bends; len(got) != 1 || got[0] != (gw.Point{X: 10, Y: 80}) {
		t.Errorf("the zone's move undone: bends %v, want carried back to (10, 80)", got)
	}
	bent("the zone's move undone, the edge leaving it", bc, true)

	d.ClearBends()
	bent("ClearBends", ab, false)
	apply(t, d, gw.Command{Op: gw.Move, Vertex: c, Pos: gw.Point{X: 808, Y: 0}})
	d.Undo()
	bent("ClearBends, then a move undone", bc, false)
}

// TestBendsKept: a deleted edge keeps its bends for its undo, and
// Compact moves them with the edge.
func TestBendsKept(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 400, 0)
	first := addEdge(t, d, 0, a, b, gw.None, gw.None)
	e := addEdge(t, d, 0, a, b, gw.None, gw.None)
	d.SetBends(e, []gw.Point{{X: 1, Y: 2}})
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: e})
	d.Undo()
	if len(d.Edges[e].Bends) != 1 {
		t.Errorf("undone delete: bends %v", d.Edges[e].Bends)
	}
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: first})
	_, emap := d.Compact()
	if got := d.Edges[emap[e]].Bends; len(got) != 1 || got[0] != (gw.Point{X: 1, Y: 2}) {
		t.Errorf("after Compact: bends %v", got)
	}
}

// TestAllocBends: setting, carrying and clearing bends allocates
// nothing.
func TestAllocBends(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 400, 0)
	z := at(t, d, cZone, "z", 500, 500)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: z})
	e := addEdge(t, d, 0, a, b, gw.None, gw.None)
	pts := []gw.Point{{X: 1, Y: 2}, {X: 3, Y: 4}}
	x := 0.0
	if n := testing.AllocsPerRun(50, func() {
		d.ClearBends()
		d.SetBends(e, pts)
		x += 8
		apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: x, Y: 0}})
	}); n > 0 {
		t.Errorf("%g allocations per run", n)
	}
}
