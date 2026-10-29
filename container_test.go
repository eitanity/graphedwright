package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// The kinds of the container schema.
const (
	cBox gw.KindIndex = iota
	cZone
	cSystem
)

// containerDoc: boxes, zones that hold anything, and systems that hold
// only boxes; edges of any kind between any vertices.
func containerDoc(t *testing.T) *gw.Document {
	t.Helper()
	d, err := gw.New(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 40, H: 20}},
			{Name: "zone", Shape: gw.RoundRect, Size: gw.Size{W: 100, H: 60}, Container: true},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Contains: []gw.KindIndex{cBox}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "any"}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// at adds a vertex of the kind and moves it to (x, y).
func at(t *testing.T, d *gw.Document, kind gw.KindIndex, id string, x, y float64) gw.VertexIndex {
	t.Helper()
	v := addVertex(t, d, kind, id)
	apply(t, d, gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: x, Y: y}})
	return v
}

// TestContainerFit: a container that holds vertices is the box around
// them with the padding and the title's room; an empty one keeps its
// own; a nested container is fitted first; the change names the
// container that moved, once.
func TestContainerFit(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 100, 40)
	z := at(t, d, cZone, "z", 500, 500)
	if d.Vertices[z].Size != (gw.Size{W: 100, H: 60}) {
		t.Errorf("empty zone: %v", d.Vertices[z].Size)
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	ch := apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: z})
	// a [-20,-10,20,10] and b [80,30,120,50]: the box [-20,-10,120,50].
	if v := d.Vertices[z]; v.Pos != (gw.Point{X: 50, Y: 10}) || v.Size != (gw.Size{W: 172, H: 112}) {
		t.Errorf("fitted zone: %v %v", v.Pos, v.Size)
	}
	seen := map[gw.VertexIndex]int{}
	for _, v := range ch.Vertices {
		seen[v]++
	}
	if seen[z] != 1 || seen[b] != 1 || !ch.Layout {
		t.Errorf("change %+v", ch)
	}
	apply(t, d, gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 200, Y: 40}})
	if d.Vertices[z].Size.W != 272 {
		t.Errorf("zone after its box moved: %v", d.Vertices[z].Size)
	}
	// Nested: an outer zone around z.
	o := at(t, d, cZone, "o", 900, 900)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: o})
	zv, ov := d.Vertices[z], d.Vertices[o]
	if ov.Size.W != zv.Size.W+2*gw.ContainerPad || ov.Size.H != zv.Size.H+2*gw.ContainerPad+gw.ContainerTitle || d.Depth(a) != 2 || !d.Within(a, o) || d.Within(o, a) {
		t.Errorf("outer %v %v around %v %v, depth %d", ov.Pos, ov.Size, zv.Pos, zv.Size, d.Depth(a))
	}
}

// TestSetParentRefusals: only into a live container that may hold the
// kind, never into itself or what it holds, and nothing changes when a
// command is refused.
func TestSetParentRefusals(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	z := at(t, d, cZone, "z", 0, 0)
	inner := at(t, d, cZone, "inner", 0, 0)
	s := at(t, d, cSystem, "s", 0, 0)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: inner, To: z})
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: a}, `vertex "a" of kind "box" is not a container`)
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: z}, `vertex "z" cannot go inside itself or what it holds`)
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: inner}, `vertex "z" cannot go inside itself or what it holds`)
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: s}, `container kind "system" cannot hold kind "zone"`)
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: 99}, "vertex 99 is not live")
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: 99, To: z}, "vertex 99 is not live")
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: s})
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: a, To: s}); err != nil || d.Vertices[a].Parent != s {
		t.Errorf("into the container it is in: %v", err)
	}
	if d.Vertices[s].Size != (gw.Size{W: 40 + 2*gw.ContainerPad, H: 20 + 2*gw.ContainerPad + gw.ContainerTitle}) {
		t.Errorf("fitted around a: %v", d.Vertices[s].Size)
	}
	// Emptied, the system is its kind's size again, its top left corner
	// (-36,-46) where the fit had it.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: gw.None})
	if v := d.Vertices[s]; d.Vertices[a].Parent != gw.None || v.Size != (gw.Size{W: 100, H: 60}) || v.Pos != (gw.Point{X: 14, Y: -16}) {
		t.Errorf("out again: parent %d, system %v %v", d.Vertices[a].Parent, v.Pos, v.Size)
	}
}

// TestContainerMoveDeleteUndo: moving a container moves what it holds,
// and undo and redo move it all back and forth; deleting a container
// deletes what it holds and the edges at any of it, and undo brings
// back exactly those, not an edge deleted before; undoing SetParent
// takes the vertex out again; Compact keeps the parents by their new
// indices.
func TestContainerMoveDeleteUndo(t *testing.T) {
	d := containerDoc(t)
	x := at(t, d, cBox, "x", -300, 0)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 100, 40)
	z := at(t, d, cZone, "z", 0, 0)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: z})
	gone := addEdge(t, d, 0, x, b, gw.None, gw.None)
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: gone})
	addEdge(t, d, 0, x, a, gw.None, gw.None)
	addEdge(t, d, 0, a, b, gw.None, gw.None)
	before := snapshot(d)
	zp := d.Vertices[z].Pos
	apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: zp.X + 30, Y: zp.Y - 10}})
	if d.Vertices[a].Pos != (gw.Point{X: 30, Y: -10}) || d.Vertices[b].Pos != (gw.Point{X: 130, Y: 30}) || d.Vertices[z].Pos != (gw.Point{X: zp.X + 30, Y: zp.Y - 10}) {
		t.Errorf("moved zone: a %v b %v z %v", d.Vertices[a].Pos, d.Vertices[b].Pos, d.Vertices[z].Pos)
	}
	moved := snapshot(d)
	d.Undo()
	if snapshot(d) != before {
		t.Errorf("undo of the move:\n%s\nwant\n%s", snapshot(d), before)
	}
	d.Redo()
	if snapshot(d) != moved {
		t.Errorf("redo of the move:\n%s\nwant\n%s", snapshot(d), moved)
	}
	d.Undo()
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: z})
	if d.LiveVertices() != 1 || d.LiveEdges() != 0 {
		t.Errorf("after deleting the zone: %d vertices, %d edges", d.LiveVertices(), d.LiveEdges())
	}
	d.Undo()
	if snapshot(d) != before {
		t.Errorf("undo of the delete:\n%s\nwant\n%s", snapshot(d), before)
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: gw.None})
	d.Undo()
	if snapshot(d) != before {
		t.Errorf("undo of taking b out:\n%s\nwant\n%s", snapshot(d), before)
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: x})
	apply(t, d, gw.Command{Op: gw.Compact})
	z2 := d.VertexByID("z")
	if a2, b2 := d.VertexByID("a"), d.VertexByID("b"); z2 != 2 || d.Vertices[a2].Parent != z2 || d.Vertices[b2].Parent != z2 {
		t.Errorf("after compact: z %d, parents %d %d", z2, d.Vertices[a2].Parent, d.Vertices[b2].Parent)
	}
}

// TestAllocContainer: putting a vertex in a container, moving the
// container, and undoing and redoing both allocate nothing.
func TestAllocContainer(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	z := at(t, d, cZone, "z", 0, 0)
	x := 0.0
	if n := testing.AllocsPerRun(50, func() {
		x++
		apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
		apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: x, Y: 1}})
		d.Undo()
		d.Undo()
		d.Redo()
		apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: gw.None})
	}); n > 0 {
		t.Errorf("%g allocations per run", n)
	}
}

// TestContainerOwnSize: a resized container is its own size when it
// holds nothing and at least that when it holds something, grown right
// and down from the corner the fit gives it; taking out what it holds,
// by SetParent, delete or undo, brings it back to its own size, with
// its top left corner kept; undo of the resize brings back the size
// and the own size before it.
func TestContainerOwnSize(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	z := at(t, d, cZone, "z", 500, 500)
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: z, Size: gw.Size{W: 200, H: 100}})
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 200, H: 100}) || v.Min != v.Size || v.Pos != (gw.Point{X: 500, Y: 500}) {
		t.Errorf("resized empty zone: %v %v own %v", v.Pos, v.Size, v.Min)
	}
	// a [-20,-10,20,10] fits to the corner (-36,-46); the zone is
	// 200 by 100 from there.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 200, H: 100}) || v.Pos != (gw.Point{X: 64, Y: 4}) {
		t.Errorf("zone holding a: %v %v", v.Pos, v.Size)
	}
	// a moved far right and down is held; the zone grows past its own
	// size to fit it.
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 300, Y: 100}})
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 200, H: 100}) || v.Pos != (gw.Point{X: 364, Y: 104}) {
		t.Errorf("zone around a moved: %v %v", v.Pos, v.Size)
	}
	d.Undo()
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: z, Size: gw.Size{W: 50, H: 50}})
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 72, H: 72}) || v.Min != (gw.Size{W: 50, H: 50}) {
		t.Errorf("resized below what it holds: %v own %v", v.Size, v.Min)
	}
	d.Undo()
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 200, H: 100}) || v.Min != v.Size || v.Pos != (gw.Point{X: 64, Y: 4}) {
		t.Errorf("undo of the resize: %v %v own %v", v.Pos, v.Size, v.Min)
	}
	for _, out := range []gw.Command{{Op: gw.SetParent, Vertex: a, To: gw.None}, {Op: gw.DeleteVertex, Vertex: a}} {
		apply(t, d, out)
		if v := d.Vertices[z]; v.Size != (gw.Size{W: 200, H: 100}) || v.Pos != (gw.Point{X: 64, Y: 4}) {
			t.Errorf("after %v: zone %v %v", out.Op, v.Pos, v.Size)
		}
		d.Undo()
	}
	// Never resized, an emptied zone is its kind's size, 100 by 60.
	b := at(t, d, cBox, "b", 0, 0)
	y := at(t, d, cZone, "y", 0, 0)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: y})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: gw.None})
	if v := d.Vertices[y]; v.Size != (gw.Size{W: 100, H: 60}) || v.Pos != (gw.Point{X: 14, Y: -16}) || v.Min != (gw.Size{}) {
		t.Errorf("emptied zone: %v %v own %v", v.Pos, v.Size, v.Min)
	}
}

// TestContainerOrder: the containers are listed for drawing each before
// what it holds and the next beside it after all it holds, whatever
// their indices, and the order follows SetParent, delete, undo and
// Compact.
func TestContainerOrder(t *testing.T) {
	d := containerDoc(t)
	inner := at(t, d, cZone, "inner", 0, 0)
	other := at(t, d, cZone, "other", 0, 0)
	outer := at(t, d, cZone, "outer", 0, 0)
	at(t, d, cBox, "box", 0, 0)
	mid := at(t, d, cZone, "mid", 0, 0)
	order := func() string {
		s := ""
		for _, c := range d.Containers() {
			s += string(d.Vertices[c].ID) + " "
		}
		return s
	}
	if got := order(); got != "inner other outer mid " {
		t.Errorf("flat: %s", got)
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: mid, To: outer})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: inner, To: mid})
	if got := order(); got != "other outer mid inner " {
		t.Errorf("nested: %s", got)
	}
	// mid taken out of outer, holding inner: outer, then mid with inner.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: mid, To: gw.None})
	if got := order(); got != "other outer mid inner " {
		t.Errorf("mid out: %s", got)
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: other, To: mid})
	if got := order(); got != "outer mid inner other " {
		t.Errorf("other in mid: %s", got)
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: mid})
	if got := order(); got != "outer " {
		t.Errorf("mid deleted: %s", got)
	}
	d.Undo()
	d.Undo()
	if got := order(); got != "other outer mid inner " {
		t.Errorf("undone: %s", got)
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: other})
	apply(t, d, gw.Command{Op: gw.Compact})
	if got := order(); got != "outer mid inner " || d.Containers()[0] != d.VertexByID("outer") {
		t.Errorf("compacted: %s", got)
	}
}
