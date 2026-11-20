package edit_test

import (
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
)

// session is a user's work on the fixture: a selection, a drag, an edge
// kind chosen and a gesture left under way.
func session(t *testing.T, e *edit.Editor) {
	t.Helper()
	send(t, e, down(220, 100, 0), up(220, 100), down(420, 100, edit.Shift), up(420, 100))
	send(t, e, down(220, 100, 0), move(240, 120), move(260, 140), up(260, 140))
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: eOutcome, IsEdge: true})
	send(t, e, down(10, 10, 0), move(300, 150))
}

// seen is what an editor shows of itself.
func seen(e *edit.Editor) string {
	sv, se := e.Selected(make([]gw.VertexIndex, 0, 16), make([]gw.EdgeIndex, 0, 32))
	v, ed := e.Editing()
	box, marquee := e.Marquee()
	_, _, ghost := e.Ghost()
	return fmt("mode", int(e.Mode())) + fmt(" state", int(e.State())) + fmt(" kind", int(e.EdgeKind())) +
		fmt(" selected", len(sv)) + fmt("/", len(se)) + fmt(" editing", int(v)) + fmt("/", int(ed)) +
		" held " + strconv.FormatBool(e.Held()) + " marquee " + strconv.FormatBool(marquee) + fmt(" ", int(box.W)) +
		" ghost " + strconv.FormatBool(ghost) + " typed " + e.Typed()
}

func fmt(label string, n int) string { return label + " " + strconv.Itoa(n) }

// TestResetIsNew: an editor used on one document, reset for another of
// its capacities, is the editor New makes for it: it shows the same,
// and the same events do the same to both documents; and the reset
// allocates nothing.
func TestResetIsNew(t *testing.T) {
	_, used := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
	session(t, used)
	send(t, used, up(300, 150), down(264, 144, 0), up(264, 144))
	if sv, _ := used.Selected(make([]gw.VertexIndex, 0, 16), nil); len(sv) == 0 {
		t.Fatal("the session selected nothing")
	}
	d1, _ := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
	d2, _ := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
	if !used.Reset(d1, edit.View, 4) {
		t.Fatal("refused")
	}
	fresh := edit.New(d2, edit.View, 4)
	if a, b := seen(used), seen(fresh); a != b {
		t.Fatalf("reset:\n%s\nnew:\n%s", a, b)
	}
	if used.Document() != d1 {
		t.Fatal("the reset editor edits another document")
	}
	used.SetMode(edit.Edit)
	fresh.SetMode(edit.Edit)
	session(t, used)
	session(t, fresh)
	send(t, used, up(300, 150), key("ArrowRight", 0))
	send(t, fresh, up(300, 150), key("ArrowRight", 0))
	if a, b := seen(used)+snapshot(d1), seen(fresh)+snapshot(d2); a != b {
		t.Fatalf("after the same events, reset:\n%s\nnew:\n%s", a, b)
	}
	if n := testing.AllocsPerRun(20, func() { used.Reset(d1, edit.Edit, 8) }); n > 0 {
		t.Errorf("Reset: %g allocations", n)
	}
}

// TestResetRefused: for no document, or one of other vertex or edge
// capacities, Reset is refused and the editor left as it was.
func TestResetRefused(t *testing.T) {
	d, e := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
	session(t, e)
	before := seen(e)
	other, _ := fixture(t, gw.Caps{Vertices: 16, Edges: 33})
	if e.Reset(nil, edit.Edit, 8) || e.Reset(other, edit.Edit, 8) {
		t.Fatal("taken")
	}
	if seen(e) != before || e.Document() != d {
		t.Fatalf("changed:\n%s\n%s", seen(e), before)
	}
}

// TestResetMidDrag: an editor reset in the middle of a drag, for the
// document it edits or another, ends the drag without returning it,
// since the document may have been read again in the meantime: nothing
// is applied or left to undo, and the other document is not touched.
func TestResetMidDrag(t *testing.T) {
	for _, same := range []bool{true, false} {
		d, e := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
		other, _ := fixture(t, gw.Caps{Vertices: 16, Edges: 32})
		next := other
		if same {
			next = d
		}
		beforeOther := snapshot(other)
		send(t, e, down(220, 100, 0), move(240, 120), move(300, 160))
		if e.State() != edit.Dragging {
			t.Fatalf("state %d, not dragging", e.State())
		}
		if !e.Reset(next, edit.Edit, 8) {
			t.Fatal("refused")
		}
		send(t, e, up(300, 160))
		if e.State() != edit.Idle || d.CanUndo() || other.CanUndo() || (!same && snapshot(other) != beforeOther) {
			t.Errorf("same %v: state %d, undo %v %v, other document\n%s", same, e.State(), d.CanUndo(), other.CanUndo(), snapshot(other))
		}
	}
}
