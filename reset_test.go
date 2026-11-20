package graphedwright_test

import (
	"errors"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// build makes a document's content the same way each time: a chain,
// minted IDs, an attribute, a port, bends, a view, a flow, and a step
// to undo.
func build(t *testing.T, d *gw.Document) {
	t.Helper()
	src, a, b, _ := chain(t, d)
	leaf := apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kLeaf}).Vertices[0]
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: "kept"})
	apply(t, d, gw.Command{Op: gw.AddPort, Vertex: b, Port: gw.Port{Name: "extra", Side: gw.Bottom}})
	e := addEdge(t, d, eAny, src, leaf, gw.None, gw.None)
	d.SetBends(e, []gw.Point{{X: 1, Y: 2}, {X: 3, Y: 4}})
	d.SetView(gw.Viewport{X: 1, Y: 2, W: 300, H: 200})
	d.SetFlow(gw.TopToBottom)
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 40, Y: 50}})
}

// state is what a document shows: its content, the bends, the view, the
// flow, and what undo can reach.
func state(d *gw.Document) string {
	s := snapshot(d)
	for i := range d.EdgeSlots() {
		bends := d.Edge(gw.EdgeIndex(i)).Bends() //nolint:gosec // bounded by the capacity
		for j := range bends.Len() {
			s += " b" + ftoa(bends.At(j).X) + "," + ftoa(bends.At(j).Y)
		}
	}
	v := d.View()
	s += " view " + ftoa(v.X) + "," + ftoa(v.Y) + " " + ftoa(v.W) + "x" + ftoa(v.H) + " " + d.Flow().String()
	return s + " undo " + strconv.FormatBool(d.CanUndo()) + " redo " + strconv.FormatBool(d.CanRedo())
}

// TestResetIsNew: a document that held another, reset, is the document
// New makes: built the same way, it shows the same, mints the same IDs
// and undoes the same; and the reset allocates nothing.
func TestResetIsNew(t *testing.T) {
	caps := gw.Caps{Vertices: 32, Edges: 64, BendsPerEdge: 1}
	fresh := newDoc(t, caps)
	build(t, fresh)
	used := newDoc(t, caps)
	// Two sources more, so that the build's reaches the kind's most, and
	// the bend room used up, so that a reset must give both back.
	for _, id := range []string{"x", "y"} {
		addVertex(t, used, kSource, id)
	}
	build(t, used)
	for range caps.Edges {
		used.SetBends(0, []gw.Point{{X: 9, Y: 9}})
	}
	apply(t, used, gw.Command{Op: gw.Relabel, Vertex: 0, Text: "other"})
	used.Undo()
	if err := used.Reset(used.Kinds()); err != nil {
		t.Fatal(err)
	}
	if s := state(used); s != state(newDoc(t, caps)) {
		t.Fatalf("reset shows\n%s", s)
	}
	build(t, used)
	if got, want := state(used), state(fresh); got != want {
		t.Fatalf("built after a reset:\n%s\nnew:\n%s", got, want)
	}
	for range 4 {
		fresh.Undo()
		used.Undo()
	}
	if got, want := state(used), state(fresh); got != want {
		t.Fatalf("undone after a reset:\n%s\nnew:\n%s", got, want)
	}
	if n := testing.AllocsPerRun(20, func() {
		if err := used.Reset(used.Kinds()); err != nil {
			t.Fatal(err)
		}
	}); n > 0 {
		t.Errorf("Reset: %g allocations", n)
	}
}

// TestResetRefusals: while locked, for nil kinds and for kinds with
// more vertex kinds than the document was made with, Reset is refused
// and the document left as it was; kinds with fewer are taken.
func TestResetRefusals(t *testing.T) {
	d := newDoc(t, gw.Caps{Vertices: 32, Edges: 64})
	build(t, d)
	before := state(d)
	d.Lock()
	if err := d.Reset(d.Kinds()); !errors.Is(err, gw.ErrLocked) {
		t.Errorf("locked: %v", err)
	}
	d.Unlock()
	if err := d.Reset(nil); err == nil {
		t.Error("nil kinds taken")
	}
	more := testSchema()
	more.VertexKinds = append(more.VertexKinds, gw.VertexKind{Name: "extra", Shape: gw.Rect, Size: gw.Size{W: 10, H: 10}})
	k, err := gw.Compile(more)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reset(k); err == nil {
		t.Error("kinds with more vertex kinds taken")
	}
	if s := state(d); s != before {
		t.Fatalf("a refused reset changed the document:\n%s", s)
	}
	fewer, err := gw.Compile(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "one", Shape: gw.Rect, Size: gw.Size{W: 10, H: 10}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reset(fewer); err != nil || d.Kinds() != fewer || d.LiveVertices() != 0 {
		t.Fatalf("fewer kinds: %v", err)
	}
	addVertex(t, d, 0, "x")
}
