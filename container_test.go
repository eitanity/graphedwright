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
	d, err := makeDoc(gw.Schema{
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
	if d.Vertex(z).Size() != (gw.Size{W: 100, H: 60}) {
		t.Errorf("empty zone: %v", d.Vertex(z).Size())
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	ch := apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: z})
	// a [-20,-10,20,10] and b [80,30,120,50]: the box [-20,-10,120,50].
	if v := d.Vertex(z); v.Pos() != (gw.Point{X: 50, Y: 10}) || v.Size() != (gw.Size{W: 172, H: 112}) {
		t.Errorf("fitted zone: %v %v", v.Pos(), v.Size())
	}
	seen := map[gw.VertexIndex]int{}
	for _, v := range ch.Vertices {
		seen[v]++
	}
	if seen[z] != 1 || seen[b] != 1 || !ch.Layout {
		t.Errorf("change %+v", ch)
	}
	apply(t, d, gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 200, Y: 40}})
	if d.Vertex(z).Size().W != 272 {
		t.Errorf("zone after its box moved: %v", d.Vertex(z).Size())
	}
	// Nested: an outer zone around z.
	o := at(t, d, cZone, "o", 900, 900)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: o})
	zv, ov := d.Vertex(z), d.Vertex(o)
	if ov.Size().W != zv.Size().W+2*gw.ContainerPad || ov.Size().H != zv.Size().H+2*gw.ContainerPad+gw.ContainerTitle || d.Depth(a) != 2 || !d.Within(a, o) || d.Within(o, a) {
		t.Errorf("outer %v %v around %v %v, depth %d", ov.Pos(), ov.Size(), zv.Pos(), zv.Size(), d.Depth(a))
	}
	// Out of range, as every read of the document: nothing, and no panic.
	past := gw.VertexIndex(d.VertexSlots()) //nolint:gosec // bounded by the capacity
	for _, v := range []gw.VertexIndex{gw.None, -5, past} {
		if d.Depth(v) != 0 || d.Within(v, o) || d.Within(v, v) {
			t.Errorf("vertex %d out of range: depth %d, within %v %v", v, d.Depth(v), d.Within(v, o), d.Within(v, v))
		}
	}
	if d.Within(a, past) || d.Within(a, gw.None) {
		t.Error("a vertex within a container out of range")
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
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: a, To: s}); err != nil || d.Vertex(a).Parent() != s {
		t.Errorf("into the container it is in: %v", err)
	}
	if d.Vertex(s).Size() != (gw.Size{W: 40 + 2*gw.ContainerPad, H: 20 + 2*gw.ContainerPad + gw.ContainerTitle}) {
		t.Errorf("fitted around a: %v", d.Vertex(s).Size())
	}
	// Emptied, the system is its kind's size again, its top left corner
	// (-36,-46) where the fit had it.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: gw.None})
	if v := d.Vertex(s); d.Vertex(a).Parent() != gw.None || v.Size() != (gw.Size{W: 100, H: 60}) || v.Pos() != (gw.Point{X: 14, Y: -16}) {
		t.Errorf("out again: parent %d, system %v %v", d.Vertex(a).Parent(), v.Pos(), v.Size())
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
	zp := d.Vertex(z).Pos()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: z, Pos: gw.Point{X: zp.X + 30, Y: zp.Y - 10}})
	if d.Vertex(a).Pos() != (gw.Point{X: 30, Y: -10}) || d.Vertex(b).Pos() != (gw.Point{X: 130, Y: 30}) || d.Vertex(z).Pos() != (gw.Point{X: zp.X + 30, Y: zp.Y - 10}) {
		t.Errorf("moved zone: a %v b %v z %v", d.Vertex(a).Pos(), d.Vertex(b).Pos(), d.Vertex(z).Pos())
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
	if a2, b2 := d.VertexByID("a"), d.VertexByID("b"); z2 != 2 || d.Vertex(a2).Parent() != z2 || d.Vertex(b2).Parent() != z2 {
		t.Errorf("after compact: z %d, parents %d %d", z2, d.Vertex(a2).Parent(), d.Vertex(b2).Parent())
	}
	// The Compact method too lists the containers by their new indices.
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: d.VertexByID("a")})
	d.Compact()
	if z3 := d.VertexByID("z"); d.Containers().Len() != 1 || d.Containers().At(0) != z3 || z3 == z2 {
		t.Errorf("after the Compact method: containers %v, z %d, was %d", d.Containers(), z3, z2)
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
	if v := d.Vertex(z); v.Size() != (gw.Size{W: 200, H: 100}) || v.Min() != v.Size() || v.Pos() != (gw.Point{X: 500, Y: 500}) {
		t.Errorf("resized empty zone: %v %v own %v", v.Pos(), v.Size(), v.Min())
	}
	// a [-20,-10,20,10] fits to the corner (-36,-46); the zone is
	// 200 by 100 from there.
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	if v := d.Vertex(z); v.Size() != (gw.Size{W: 200, H: 100}) || v.Pos() != (gw.Point{X: 64, Y: 4}) {
		t.Errorf("zone holding a: %v %v", v.Pos(), v.Size())
	}
	// a moved far right and down is held; the zone grows past its own
	// size to fit it.
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 300, Y: 100}})
	if v := d.Vertex(z); v.Size() != (gw.Size{W: 200, H: 100}) || v.Pos() != (gw.Point{X: 364, Y: 104}) {
		t.Errorf("zone around a moved: %v %v", v.Pos(), v.Size())
	}
	d.Undo()
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: z, Size: gw.Size{W: 50, H: 50}})
	if v := d.Vertex(z); v.Size() != (gw.Size{W: 72, H: 72}) || v.Min() != (gw.Size{W: 50, H: 50}) {
		t.Errorf("resized below what it holds: %v own %v", v.Size(), v.Min())
	}
	d.Undo()
	if v := d.Vertex(z); v.Size() != (gw.Size{W: 200, H: 100}) || v.Min() != v.Size() || v.Pos() != (gw.Point{X: 64, Y: 4}) {
		t.Errorf("undo of the resize: %v %v own %v", v.Pos(), v.Size(), v.Min())
	}
	for _, out := range []gw.Command{{Op: gw.SetParent, Vertex: a, To: gw.None}, {Op: gw.DeleteVertex, Vertex: a}} {
		apply(t, d, out)
		if v := d.Vertex(z); v.Size() != (gw.Size{W: 200, H: 100}) || v.Pos() != (gw.Point{X: 64, Y: 4}) {
			t.Errorf("after %v: zone %v %v", out.Op, v.Pos(), v.Size())
		}
		d.Undo()
	}
	// Never resized, an emptied zone is its kind's size, 100 by 60.
	b := at(t, d, cBox, "b", 0, 0)
	y := at(t, d, cZone, "y", 0, 0)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: y})
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: b, To: gw.None})
	if v := d.Vertex(y); v.Size() != (gw.Size{W: 100, H: 60}) || v.Pos() != (gw.Point{X: 14, Y: -16}) || v.Min() != (gw.Size{}) {
		t.Errorf("emptied zone: %v %v own %v", v.Pos(), v.Size(), v.Min())
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
		for i := range d.Containers().Len() {
			c := d.Containers().At(i)
			s += string(d.Vertex(c).ID()) + " "
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
	if got := order(); got != "outer mid inner " || d.Containers().At(0) != d.VertexByID("outer") {
		t.Errorf("compacted: %s", got)
	}
}

// TestEnclosedEdges: an edge between a container and a vertex it holds,
// at any depth, is Enclosing, 1 in and -1 out, and no other is. The
// container's side facing back stands out by the room of the widest
// text of the curved edges in, its label or stereotype at CharWidth a
// character and 8 either side, and the side facing on by the edges out;
// a straight or deleted edge gives none. Left to right those are its
// left and right sides, top to bottom its top and bottom. The vertex's
// end is spread at its back side for an edge in and its side facing on
// for one out, whichever way the two stand, and the container's is not
// spread, and heads straight along from its vertex; and no such edge
// runs in a lane with another between the same two. A relabel or an
// attribute its format shows refits the container.
func TestEnclosedEdges(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := makeDoc(gw.Schema{
			VertexKinds: []gw.VertexKind{
				{Name: "box", Size: gw.Size{W: 40, H: 20}},
				{Name: "zone", Size: gw.Size{W: 100, H: 60}, Container: true},
			},
			EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve, Stereotype: "a stereotype", Attrs: []gw.AttrSpec{{Key: "k"}}, LabelFormat: "{label}{|k}"}, {Name: "line"}},
		}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(flow)
		o := at(t, d, 1, "o", 0, 0)
		z := at(t, d, 1, "z", 0, 0)
		a := at(t, d, 0, "a", 0, 0)
		b := at(t, d, 0, "b", 100, 100)
		apply(t, d, gw.Command{Op: gw.SetParent, Vertex: z, To: o})
		for _, v := range [2]gw.VertexIndex{a, b} {
			apply(t, d, gw.Command{Op: gw.SetParent, Vertex: v, To: z})
		}
		bare := d.Vertex(z).Size()
		edge := func(kind gw.KindIndex, from, to gw.VertexIndex, text string) gw.EdgeIndex {
			t.Helper()
			return apply(t, d, gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: gw.None, ToPort: gw.None, Text: text}).Edges[0]
		}
		in := edge(0, z, b, "a label longer than the stereotype")
		in2 := edge(0, z, a, "")
		deep := edge(0, o, a, "x") // in from the outer zone, at depth two: the stereotype's room
		out := edge(0, a, z, "")
		out2 := edge(0, b, o, "")
		edge(0, a, b, "")
		edge(1, z, a, "a straight edge's label, the longest")
		gone := edge(0, b, z, "a deleted edge's label, the longest")
		apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: gone})
		for _, c := range []struct {
			from, to gw.VertexIndex
			want     int
		}{{z, a, 1}, {o, b, 1}, {a, z, -1}, {b, o, -1}, {a, b, 0}, {z, z, 0}, {a, a, 0}, {z, o, -1}, {o, z, 1}, {gw.None, a, 0}, {a, gw.None, 0}} {
			if got := d.Enclosing(c.from, c.to); got != c.want {
				t.Errorf("flow %d: Enclosing(%d, %d) = %d, want %d", flow, c.from, c.to, got, c.want)
			}
		}
		labelRoom := float64(len("a label longer than the stereotype"))*gw.CharWidth + 16
		stereoRoom := float64(len("a stereotype"))*gw.CharWidth + 16
		if back, on := d.EnclosedRoom(z); back != labelRoom || on != stereoRoom {
			t.Errorf("flow %d: z's room %v %v, want %v %v", flow, back, on, labelRoom, stereoRoom)
		}
		if back, on := d.EnclosedRoom(o); back != stereoRoom || on != stereoRoom {
			t.Errorf("flow %d: o's room %v %v, want %v %v", flow, back, on, stereoRoom, stereoRoom)
		}
		if back, on := d.EnclosedRoom(a); back != 0 || on != 0 {
			t.Errorf("flow %d: a box's room %v %v", flow, back, on)
		}
		if back, on := d.EnclosedRoom(gw.None); back != 0 || on != 0 {
			t.Errorf("flow %d: no vertex's room %v %v", flow, back, on)
		}
		grown := d.Vertex(z).Size()
		w, h := bare.W+labelRoom+stereoRoom, bare.H
		if flow == gw.TopToBottom {
			w, h = bare.W, bare.H+labelRoom+stereoRoom
		}
		if grown != (gw.Size{W: w, H: h}) {
			t.Errorf("flow %d: z %v, from %v, want %v %v", flow, grown, bare, w, h)
		}
		// Relabelled longer, the edge in makes z wider along the flow,
		// and undone, as it was.
		apply(t, d, gw.Command{Op: gw.Relabel, Edge: in, Vertex: gw.None, Text: "a label longer than the stereotype, and then some"})
		if l := d.Vertex(z).Size(); l.W+l.H != grown.W+grown.H+15*gw.CharWidth {
			t.Errorf("flow %d: z relabelled %v, from %v", flow, l, grown)
		}
		if _, ok := d.Undo(); !ok {
			t.Fatal("nothing to undo")
		}
		if d.Vertex(z).Size() != grown {
			t.Errorf("flow %d: z undone %v, want %v", flow, d.Vertex(z).Size(), grown)
		}
		// z to a, curved and straight, and a to z run in no lanes.
		if k, n := d.Parallel(out); k != 0 || n != 1 {
			t.Errorf("flow %d: a to z in lane %d of %d", flow, k, n)
		}
		// So does an attribute the label's format shows.
		apply(t, d, gw.Command{Op: gw.SetAttr, Edge: in, Vertex: gw.None, Key: "k", Value: "0123456789"})
		if l := d.Vertex(z).Size(); l.W+l.H != grown.W+grown.H+10*gw.CharWidth {
			t.Errorf("flow %d: z with the attribute %v, from %v", flow, l, grown)
		}
		// a stands before z's and o's middles and b after them; with an
		// edge a to b, the ends in are at the vertex's back side and those
		// out at its side facing on all the same, and the container's
		// two ends in are not spread. At b's back side, among the edges
		// from a, and from p and q outside before b and either side of it,
		// the edge in from z, heading straight along from b, is between
		// p's and q's.
		pos := func(along, across float64) (float64, float64) {
			if flow == gw.TopToBottom {
				return across, along
			}
			return along, across
		}
		px, py := pos(-300, 75)
		qx, qy := pos(-300, 125)
		edge(0, at(t, d, 0, "p", px, py), b, "")
		edge(0, at(t, d, 0, "q", qx, qy), b, "")
		if k, n := d.EndSlot(in, true); k != 2 || n != 4 {
			t.Errorf("flow %d: z to b at b: slot %d of %d, want 2 of 4", flow, k, n)
		}
		for _, c := range []struct {
			name   string
			e      gw.EdgeIndex
			target bool
			n      int
		}{
			{"z to b at b, with a, p and q to b", in, true, 4},
			{"o to a at a, with z to a", deep, true, 2},
			{"a to z at a, with a to b", out, false, 2},
			{"b to o at b, alone on", out2, false, 1},
			{"z to b at z", in, false, 1},
			{"z to a at z", in2, false, 1},
		} {
			if _, n := d.EndSlot(c.e, c.target); n != c.n {
				t.Errorf("flow %d: %s: %d at its side, want %d", flow, c.name, n, c.n)
			}
		}
	}
}
