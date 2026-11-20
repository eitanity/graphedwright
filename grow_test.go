package graphedwright_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// history builds a document with a log worth keeping: the build's
// steps, two layouts with what they placed, enough relabels to go round
// the log, a vertex's ports put back, a host change, a group, and a
// step undone.
func history(t *testing.T, d *gw.Document, s *recordStore) {
	t.Helper()
	if err := d.SetStore(s); err != nil {
		t.Fatal(err)
	}
	build(t, d)
	b := d.VertexByID("b")
	for i := range 2 {
		apply(t, d, gw.Command{Op: gw.Layout})
		for v := range d.VertexSlots() {
			d.Place(gw.VertexIndex(v), gw.Point{X: float64(100*i + 10*v), Y: 5}, true) //nolint:gosec // bounded by the capacity
		}
	}
	for i := range 3 {
		apply(t, d, gw.Command{Op: gw.Relabel, Vertex: b, Text: "b" + strconv.Itoa(i)})
	}
	apply(t, d, gw.Command{Op: gw.SetPorts, Vertex: b})
	apply(t, d, gw.Command{Op: gw.HostChange, Vertex: b, Edge: gw.None, Token: s.set("b", []dist{{Name: "pert", Params: []float64{1, 2, 3}}})})
	d.Group()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: 0, Pos: gw.Point{X: 7, Y: 8}})
	apply(t, d, gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 9, Y: 10}})
	d.EndGroup()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: 0, Text: "last"})
	d.Undo()
}

// shown is a document's state and its store's records.
func shown(d *gw.Document, s *recordStore) string {
	return state(d) + fmt.Sprint(" store ", s.byID["b"])
}

// TestGrowKeepsUndo: a document grown holds what the one it grew from
// held, and undoes and redoes every step it could, the layouts' among
// them, as that one would; it then takes what the old capacities could
// not. The one it grew from is left empty, its store told of nothing.
func TestGrowKeepsUndo(t *testing.T) {
	caps := gw.Caps{Vertices: 16, Edges: 16, UndoDepth: 16, UndoLayouts: 2, BendsPerEdge: 1}
	sa, sb := newRecordStore(t), newRecordStore(t)
	a, b := newDoc(t, caps), newDoc(t, caps)
	history(t, a, sa)
	history(t, b, sb)
	for _, d := range []*gw.Document{a, b} {
		d.SetBends(0, []gw.Point{{X: 3, Y: 3}})
	}
	big := gw.Caps{Vertices: 64, Edges: 128, PortsPerVertex: 12, AttrsPerElement: 20, LabelBytes: 300, UndoDepth: 32, UndoLayouts: 4, BendsPerEdge: 2}
	g, err := b.Grow(big)
	if err != nil {
		t.Fatal(err)
	}
	if g.Caps() != big {
		t.Fatalf("grown to %+v", g.Caps())
	}
	if s := state(b); s != state(newDoc(t, caps)) {
		t.Fatalf("the document grown from shows\n%s", s)
	}
	if n := sb.ledger.pending(); n != 1 {
		t.Fatalf("growing told the store of its token: %d pending", n)
	}
	step := func(what string) {
		t.Helper()
		if got, want := shown(g, sb), shown(a, sa); got != want {
			t.Fatalf("%s, grown:\n%s\nkept:\n%s", what, got, want)
		}
	}
	step("grown")
	_, ra := a.Redo()
	_, rg := g.Redo()
	if !ra || !rg {
		t.Fatalf("redo: kept %v, grown %v", ra, rg)
	}
	step("redone")
	undone := 0
	// bound: the log's depth
	for a.CanUndo() || g.CanUndo() {
		a.Undo()
		g.Undo()
		undone++
		step("undone " + strconv.Itoa(undone))
	}
	if undone < 4 {
		t.Fatalf("only %d steps undone", undone)
	}
	// bound: the log's depth
	for a.CanRedo() || g.CanRedo() {
		a.Redo()
		g.Redo()
		step("redone again")
	}
	// A group after the one kept is a step of its own, and the IDs
	// minted go on from the same count.
	both := func(c gw.Command) {
		t.Helper()
		apply(t, a, c)
		apply(t, g, c)
	}
	a.Undo()
	g.Undo()
	a.Group()
	g.Group()
	both(gw.Command{Op: gw.AddVertex, Kind: kLeaf})
	both(gw.Command{Op: gw.Relabel, Vertex: 0, Text: "grouped"})
	a.EndGroup()
	g.EndGroup()
	a.Undo()
	g.Undo()
	step("a new group undone")
	both(gw.Command{Op: gw.AddVertex, Kind: kLeaf})
	step("an ID minted")
	// The kinds' counts go on too: a source more is refused at the same
	// count.
	for _, d := range []*gw.Document{a, g} {
		_, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kSource})
		for i := 0; err == nil && i < caps.Vertices; i++ {
			_, err = d.Apply(gw.Command{Op: gw.AddVertex, Kind: kSource})
		}
	}
	step("sources added to the kind's most")
	for i := range caps.Vertices {
		addVertex(t, g, kLeaf, "more"+strconv.Itoa(i))
	}
	if !g.SetBends(0, []gw.Point{{X: 1, Y: 1}, {X: 2, Y: 2}}) {
		t.Error("the grown bends had no room")
	}
	g.ClearUndo()
	if n := sb.ledger.pending(); n != 0 {
		t.Errorf("the grown log's clearing left %d tokens", n)
	}
	sb.ledger.settled(t)
}

// TestGrowRefusals: below the document's capacities, while locked, and
// for capacities New refuses, Grow is refused and the document left as
// it was.
func TestGrowRefusals(t *testing.T) {
	caps := gw.Caps{Vertices: 32, Edges: 64, UndoDepth: 16}
	d := newDoc(t, caps)
	build(t, d)
	before := state(d)
	for _, c := range []gw.Caps{
		{Vertices: 31, Edges: 64, UndoDepth: 16},
		{Vertices: 32, Edges: 64, UndoDepth: 8},
		{Vertices: 32, Edges: 64, UndoDepth: 16, PortsPerVertex: 1},
		{Vertices: gw.MaxCap + 1, Edges: 64, UndoDepth: 16},
	} {
		if _, err := d.Grow(c); err == nil {
			t.Errorf("grown to %+v", c)
		}
	}
	d.Lock()
	if _, err := d.Grow(gw.Caps{}); !errors.Is(err, gw.ErrLocked) {
		t.Errorf("locked: %v", err)
	}
	d.Unlock()
	if s := state(d); s != before {
		t.Fatalf("a refused grow changed the document:\n%s", s)
	}
	g, err := d.Grow(gw.Caps{})
	if err != nil || g.Caps() != gw.DefaultCaps() {
		t.Fatalf("zero caps: %v", err)
	}
}
