package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestAllocApply: once a document is made, applying, undoing and redoing
// commands allocates nothing. IDs are supplied, since minting one makes
// a string.
func TestAllocApply(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, b, c := chain(t, d)
	run := func(name string, f func()) {
		t.Helper()
		if n := testing.AllocsPerRun(50, f); n > 0 {
			t.Errorf("%s: %g allocations per run", name, n)
		}
	}
	i := 0
	run("move", func() { apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 1, Y: 2}}) })
	run("resize", func() { apply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 10, H: 10}}) })
	run("set attr", func() { apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "p", Value: "0.75"}) })
	run("undo redo", func() { d.Undo(); d.Redo() })
	ids := make([]gw.ID, 64)
	for i := range ids {
		ids[i] = gw.ID("x" + string(rune('A'+i/26)) + string(rune('a'+i%26)))
	}
	run("add and delete edge", func() {
		i++
		ch := apply(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: c, FromPort: 1, ToPort: gw.None, ID: ids[i%64]})
		apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: ch.Edges[0]})
	})
	run("delete and undo vertex", func() {
		apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: b})
		d.Undo()
	})
	ids2 := make([]gw.ID, 64)
	for i := range ids2 {
		ids2[i] = gw.ID("y" + string(rune('A'+i/26)) + string(rune('a'+i%26)))
	}
	i = 0
	run("add and delete vertex", func() {
		i++
		v := addVertex(t, d, kNode, string(ids2[i%64]))
		apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: v})
	})
}
