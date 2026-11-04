package graphedwright_test

import (
	"errors"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestLock: a locked document is read-only through every method that
// changes it: Apply refuses with ErrLocked, the others report that they
// did nothing, and neither the content, the bends nor the undo log
// changes. Each method is given something to do, so that an unguarded
// one would show: a redo pending, a container left unfitted and a bend
// set, a viewport and a flow to change. Unlocked, each does it. A group
// neither opens nor closes while locked.
func TestLock(t *testing.T) {
	d := containerDoc(t)
	a := at(t, d, cBox, "a", 0, 0)
	b := at(t, d, cBox, "b", 200, 0)
	z := at(t, d, cZone, "z", 500, 500)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	e := addEdge(t, d, 0, a, b, gw.None, gw.None)
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "before"})
	d.Undo() // something to redo
	if !d.Place(a, gw.Point{X: -300, Y: -300}, true) || !d.SetBends(e, []gw.Point{{X: 100, Y: 50}, {X: 150, Y: 50}}) {
		t.Fatal("setting up")
	}
	unfitted := d.Vertices[z].Pos
	before := snapshot(d)
	canUndo, canRedo := d.CanUndo(), d.CanRedo()

	d.Lock()
	if !d.Locked() {
		t.Fatal("not locked")
	}
	if _, err := d.Apply(gw.Command{Op: gw.Relabel, Vertex: a, Text: "locked"}); !errors.Is(err, gw.ErrLocked) {
		t.Errorf("Apply: %v, want ErrLocked", err)
	}
	if _, ok := d.Undo(); ok {
		t.Error("Undo undid while locked")
	}
	if _, ok := d.Redo(); ok {
		t.Error("Redo redid while locked")
	}
	if d.Place(a, gw.Point{X: 999, Y: 999}, true) {
		t.Error("Place placed while locked")
	}
	if d.SetBends(e, []gw.Point{{X: 1, Y: 1}}) {
		t.Error("SetBends set while locked")
	}
	if d.SetBends(e, d.Edges[e].Bends[:1]) { // a prefix of its own, which reserves nothing
		t.Error("SetBends cut the edge's own bends while locked")
	}
	if d.ReserveBends(e, 2) != nil {
		t.Error("ReserveBends reserved while locked")
	}
	if d.SetView(gw.Viewport{W: 10, H: 10}) || d.View() != (gw.Viewport{}) {
		t.Errorf("SetView set while locked: %v", d.View())
	}
	if d.SetFlow(gw.TopToBottom) || d.Flow() != gw.LeftToRight {
		t.Errorf("SetFlow set while locked: %v", d.Flow())
	}
	if vm, em := d.Compact(); vm != nil || em != nil {
		t.Error("Compact renumbered while locked")
	}
	d.ClearUndo()
	d.Fit()
	d.ClearBends()
	if got := snapshot(d); got != before {
		t.Errorf("locked document changed:\n%s\nwant\n%s", got, before)
	}
	if got := d.Edges[e].Bends; len(got) != 2 || got[0] != (gw.Point{X: 100, Y: 50}) {
		t.Errorf("bends while locked: %v", got)
	}
	if d.CanUndo() != canUndo || d.CanRedo() != canRedo {
		t.Error("the undo log changed while locked")
	}

	d.Unlock()
	if d.Locked() {
		t.Fatal("still locked")
	}
	if _, ok := d.Redo(); !ok || d.Vertices[a].Label != "before" {
		t.Error("Redo after Unlock")
	}
	d.Fit()
	if d.Vertices[z].Pos == unfitted {
		t.Error("Fit after Unlock: the container did not move to its vertex")
	}
	d.ClearBends()
	if len(d.Edges[e].Bends) != 0 {
		t.Error("ClearBends after Unlock")
	}
	if !d.Place(a, gw.Point{X: 40, Y: 40}, true) {
		t.Error("Place after Unlock")
	}
	if !d.SetView(gw.Viewport{W: 10, H: 10}) || d.View() != (gw.Viewport{W: 10, H: 10}) {
		t.Errorf("SetView after Unlock: %v", d.View())
	}
	if !d.SetFlow(gw.TopToBottom) || d.Flow() != gw.TopToBottom {
		t.Errorf("SetFlow after Unlock: %v", d.Flow())
	}

	// A Group while locked opens nothing: two commands after it are two
	// steps.
	d.Lock()
	d.Group()
	d.Unlock()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "one"})
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "two"})
	d.Undo()
	if got := d.Vertices[a].Label; got != "one" {
		t.Errorf("a group opened while locked: undid to %q, want %q", got, "one")
	}
	// An EndGroup while locked closes nothing: a group begun before the
	// lock is still one step after it.
	d.Group()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "three"})
	d.Lock()
	if _, ok := d.Discard(); ok || d.Vertices[a].Label != "three" {
		t.Error("Discard took back while locked")
	}
	d.EndGroup()
	d.Unlock()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "four"})
	d.EndGroup()
	d.Undo()
	if got := d.Vertices[a].Label; got != "one" {
		t.Errorf("a group closed while locked: undid to %q, want %q", got, "one")
	}
}
