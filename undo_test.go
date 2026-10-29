package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestUndoRedoEveryOp: for every undoable op, undo returns the document
// to the state before the command and redo to the state after, and the
// changes name the elements touched.
func TestUndoRedoEveryOp(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	s, a, b, c := chain(t, d)
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: "n"})
	for _, tc := range []struct {
		name string
		cmd  gw.Command
	}{
		{"add vertex", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "z", Text: "z"}},
		{"delete vertex with edges", gw.Command{Op: gw.DeleteVertex, Vertex: b}},
		{"add edge", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: c, FromPort: 1, ToPort: gw.None, Text: "x", Weight: 0.3}},
		{"delete edge", gw.Command{Op: gw.DeleteEdge, Edge: 0}},
		{"move", gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 40, Y: 48}}},
		{"resize", gw.Command{Op: gw.Resize, Vertex: s, Size: gw.Size{W: 1, H: 2}}},
		{"relabel vertex", gw.Command{Op: gw.Relabel, Vertex: a, Text: "A"}},
		{"relabel edge", gw.Command{Op: gw.Relabel, Vertex: gw.None, Edge: 1, Text: "E"}},
		{"set attr", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "p", Value: "0.25"}},
		{"add attr", gw.Command{Op: gw.SetAttr, Vertex: b, Key: "note", Value: "new"}},
		{"remove attr from the middle", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "mode", Value: ""}},
		{"edge attr", gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: 1, Key: "p", Value: "0.7"}},
		{"set ports", gw.Command{Op: gw.SetPorts, Vertex: a}},
	} {
		before := snapshot(d)
		ch := apply(t, d, tc.cmd)
		after := snapshot(d)
		if tc.name != "set ports" && after == before {
			t.Errorf("%s: no change", tc.name)
		}
		if len(ch.Vertices)+len(ch.Edges) == 0 {
			t.Errorf("%s: change names nothing", tc.name)
		}
		if _, ok := d.Undo(); !ok || snapshot(d) != before {
			t.Errorf("%s: undo\n%s\n%s", tc.name, before, snapshot(d))
		}
		if _, ok := d.Redo(); !ok || snapshot(d) != after {
			t.Errorf("%s: redo\n%s\n%s", tc.name, after, snapshot(d))
		}
		if _, ok := d.Undo(); !ok || snapshot(d) != before {
			t.Errorf("%s: second undo", tc.name)
		}
	}
}

// TestUndoLog: a new command clears what could be redone, the ring drops
// the oldest entries, a layout keeps it, Compact clears it, and an empty
// log says so.
func TestUndoLog(t *testing.T) {
	d := newDoc(t, gw.Caps{UndoDepth: 3})
	if _, ok := d.Undo(); ok || d.CanUndo() || d.CanRedo() {
		t.Error("empty log undoes")
	}
	a := addVertex(t, d, kNode, "a")
	for i := 1; i <= 5; i++ {
		apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: float64(i)}})
	}
	n := 0
	for d.CanUndo() {
		d.Undo()
		n++
	}
	if n != 3 || d.Vertices[a].Pos.X != 2 {
		t.Errorf("undid %d, x %g; want 3 and 2", n, d.Vertices[a].Pos.X)
	}
	d.Redo()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 9}})
	if d.CanRedo() {
		t.Error("redo survives a new command")
	}
	d.Undo()
	if d.Vertices[a].Pos.X != 3 {
		t.Errorf("x %g after undo of the new command, want 3", d.Vertices[a].Pos.X)
	}
	apply(t, d, gw.Command{Op: gw.Layout})
	if !d.CanUndo() {
		t.Error("a layout is not in the log")
	}
	d.Undo()
	if !d.CanUndo() {
		t.Error("the log stops at a layout")
	}
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 1}})
	apply(t, d, gw.Command{Op: gw.Compact})
	if d.CanUndo() {
		t.Error("compact keeps the log")
	}
	d0 := newDoc(t, gw.Caps{UndoDepth: 0})
	if d0.Caps().UndoDepth != 256 {
		t.Errorf("zero depth takes the default: %d", d0.Caps().UndoDepth)
	}
}

// TestUndoLayout: undoing a layout puts every vertex back where it was,
// placed as it was; redo gives the laid-out positions back.
func TestUndoLayout(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	s, a, b, c := chain(t, d)
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 50, Y: 60}})
	before := snapshot(d)
	ch := apply(t, d, gw.Command{Op: gw.Layout, Text: "all"})
	if !ch.Layout || len(ch.Vertices) != 4 || d.Vertices[a].Placed {
		t.Errorf("layout change %+v placed %v", ch, d.Vertices[a].Placed)
	}
	for i, v := range []gw.VertexIndex{s, a, b, c} {
		d.Place(v, gw.Point{X: float64(i) * 160, Y: 100}, false)
	}
	after := snapshot(d)
	if _, ok := d.Undo(); !ok || snapshot(d) != before {
		t.Errorf("undo of a layout:\n%s\n%s", before, snapshot(d))
	}
	if _, ok := d.Redo(); !ok || snapshot(d) != after {
		t.Errorf("redo of a layout:\n%s\n%s", after, snapshot(d))
	}
}

// TestUndoAcrossLayouts: undo walks back through several layouts and the
// edits between them to where the document started, redo walks forward
// again, and a layout past UndoLayouts drops the oldest layout and what
// came before it.
func TestUndoAcrossLayouts(t *testing.T) {
	d := newDoc(t, gw.Caps{UndoLayouts: 2})
	s, a, b, c := chain(t, d)
	d.ClearUndo()
	start := snapshot(d)
	states := make([]string, 0, 2)
	for round := range 2 {
		apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: float64(10 * round), Y: 60}})
		apply(t, d, gw.Command{Op: gw.Layout, Text: "all"})
		for i, v := range []gw.VertexIndex{s, a, b, c} {
			d.Place(v, gw.Point{X: float64(i*160 + round), Y: 100}, false)
		}
		states = append(states, snapshot(d))
	}
	n := 0
	for d.CanUndo() {
		d.Undo()
		n++
	}
	if got := snapshot(d); got != start {
		t.Errorf("undid %d back to:\n%s\nwant\n%s", n, got, start)
	}
	for d.CanRedo() {
		d.Redo()
	}
	if got := snapshot(d); got != states[1] {
		t.Errorf("redo to the end:\n%s\nwant\n%s", got, states[1])
	}
	apply(t, d, gw.Command{Op: gw.Layout, Text: "all"})
	for d.CanUndo() {
		d.Undo()
	}
	if got := snapshot(d); got != states[0] {
		t.Errorf("a third layout of two kept undoes back to:\n%s\nwant the first layout's\n%s", got, states[0])
	}
}

// TestUndoRevivesOnlyItsOwn: undoing a vertex's deletion brings back the
// edges that died with it and not an edge deleted earlier.
func TestUndoRevivesOnlyItsOwn(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, b, _ := chain(t, d)
	e := addEdge(t, d, eOutcome, a, b, 1, 0)
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: e})
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: b})
	if d.LiveEdges() != 1 {
		t.Fatalf("live edges %d", d.LiveEdges())
	}
	d.Undo()
	if d.LiveEdges() != 3 || d.Edges[e].Live {
		t.Errorf("after undo: live edges %d, earlier deleted edge live %v", d.LiveEdges(), d.Edges[e].Live)
	}
}

// TestLayoutFlow: a Layout command sets the flow its Value names, keeps
// it when Value is empty, refuses one it does not know, and its undo
// and redo put the flow back as they put the positions back.
func TestLayoutFlow(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	chain(t, d)
	if d.Flow != gw.LeftToRight {
		t.Fatalf("a new document flows %v", d.Flow)
	}
	refuse(t, d, gw.Command{Op: gw.Layout, Value: "sideways"}, `unknown flow "sideways"`)
	apply(t, d, gw.Command{Op: gw.Layout, Value: "top-to-bottom"})
	if d.Flow != gw.TopToBottom {
		t.Errorf("after the layout: %v", d.Flow)
	}
	apply(t, d, gw.Command{Op: gw.Layout})
	if d.Flow != gw.TopToBottom {
		t.Errorf("a layout without a flow changed it: %v", d.Flow)
	}
	d.Undo()
	d.Undo()
	if d.Flow != gw.LeftToRight {
		t.Errorf("both layouts undone: %v", d.Flow)
	}
	d.Redo()
	if d.Flow != gw.TopToBottom {
		t.Errorf("the first layout redone: %v", d.Flow)
	}
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		if g, ok := gw.FlowNamed(f.String()); !ok || g != f {
			t.Errorf("flow %v does not read back by its name %q", f, f.String())
		}
	}
	if gw.Flow(9).String() != "flow 9" {
		t.Errorf("an unknown flow is named %q", gw.Flow(9).String())
	}
}

// TestUndoGroup: the commands between Group and EndGroup undo and redo as
// one step, a group within it stays part of it, a command after it is a
// step of its own, and a group whose oldest entries fell off a full log
// undoes as far as it is kept.
func TestUndoGroup(t *testing.T) {
	d := newDoc(t, gw.Caps{UndoDepth: 4})
	_, a, b, c := chain(t, d)
	d.ClearUndo()
	start := snapshot(d)
	d.Group()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 8, Y: 8}})
	d.Group()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 16, Y: 8}})
	d.EndGroup()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: c, Text: "C"})
	d.EndGroup()
	grouped := snapshot(d)
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "A"})
	if _, ok := d.Undo(); !ok || snapshot(d) != grouped {
		t.Fatal("a command after the group is not its own step")
	}
	if _, ok := d.Undo(); !ok || snapshot(d) != start {
		t.Fatal("one undo does not reverse the whole group")
	}
	if d.CanUndo() {
		t.Error("something left to undo after the group")
	}
	if _, ok := d.Redo(); !ok || snapshot(d) != grouped {
		t.Fatal("one redo does not apply the whole group again")
	}
	// Five moves in one group on a log of four: the first falls off.
	d.ClearUndo()
	d.Group()
	for i := range 5 {
		apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: float64(8 * (i + 2)), Y: 8}})
	}
	d.EndGroup()
	if _, ok := d.Undo(); !ok || d.CanUndo() {
		t.Errorf("a group cut by a full log: undone %t, more left %t", ok, d.CanUndo())
	}
	if got := d.Vertices[a].Pos; got != (gw.Point{X: 16, Y: 8}) {
		t.Errorf("after undoing the kept part a is at %v, want where the second move started", got)
	}
}
