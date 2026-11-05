package edit_test

import (
	"errors"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// TestHoldEndsGestures: a hold ends the gesture under way as a mode
// change does: a drag goes back, reported as a change, and a connection,
// a marquee, a placement, a resize and label editing are dropped, with
// nothing applied, then or at the release that follows.
func TestHoldEndsGestures(t *testing.T) {
	for _, c := range []struct {
		name  string
		start []edit.Event
		want  edit.State
	}{
		{"drag", []edit.Event{down(220, 100, 0), move(240, 120), move(263, 141)}, edit.Dragging},
		{"connection", []edit.Event{down(280, 100, 0), move(300, 120), move(420, 100)}, edit.Connecting},
		{"marquee", []edit.Event{down(10, 10, 0), move(500, 150)}, edit.Selecting},
		{"placement", []edit.Event{{Kind: edit.PaletteStart, Item: kNode}, move(300, 30)}, edit.Placing},
		{"resize", []edit.Event{down(220, 100, 0), up(220, 100), down(280, 118, 0), move(330, 152)}, edit.Resizing},
		{"label", []edit.Event{down(220, 100, 0), up(220, 100), key("Enter", 0)}, edit.EditingText},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, e := fixture(t, gw.Caps{})
			before := snapshot(d)
			send(t, e, c.start...)
			if e.State() != c.want {
				t.Fatalf("state %d before the hold, want %d", e.State(), c.want)
			}
			ch := e.Hold()
			if c.name == "drag" && (len(ch.Vertices) != 1 || ch.Vertices[0] != d.VertexByID("a")) {
				t.Errorf("the drag's return reported as %+v", ch)
			}
			if !e.Held() || e.State() != edit.Idle {
				t.Errorf("held %v, state %d", e.Held(), e.State())
			}
			send(t, e, up(420, 100))
			if snapshot(d) != before || d.CanUndo() {
				t.Errorf("after the hold and the release, undo %v\n%s\n%s", d.CanUndo(), before, snapshot(d))
			}
			if v, _ := e.Editing(); v != gw.None {
				t.Errorf("still editing %d", v)
			}
		})
	}
}

// TestHeldRefuses: while held, a press, a double-click, a palette item
// and the keys that edit are refused with ErrHeld and change nothing.
func TestHeldRefuses(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, down(220, 100, 0), up(220, 100))
	e.Hold()
	before := snapshot(d)
	for _, ev := range []edit.Event{
		down(420, 100, 0), down(280, 100, 0), down(10, 10, 0), down(220, 100, edit.Shift),
		{Kind: edit.DoubleClick, Point: pt(220, 100)},
		{Kind: edit.PaletteStart, Item: kNode}, {Kind: edit.PaletteStart, Item: eOutcome, IsEdge: true},
		key("Delete", 0), key("ArrowLeft", 0), key("Enter", 0), key("F2", 0), key("q", 0), key("z", edit.Ctrl), key("y", edit.Ctrl),
	} {
		if _, err := e.Handle(ev); !errors.Is(err, edit.ErrHeld) {
			t.Errorf("%+v: %v, want ErrHeld", ev, err)
		}
		if e.State() != edit.Idle {
			t.Errorf("%+v: state %d", ev, e.State())
		}
	}
	if v, _ := selected(e); ids(d, v) != "a" || snapshot(d) != before || d.CanUndo() || e.EdgeKind() != gw.None {
		t.Errorf("selected %q, edge kind %d, or the document changed", ids(d, v), e.EdgeKind())
	}
}

// TestHeldLooks: while held, the user can still look: hover, a wheel
// scroll and zoom, a middle-button pan, Tab and a context request.
func TestHeldLooks(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	e.Hold()
	send(t, e, move(420, 100))
	if h := e.Hover(); h.Kind != svg.VertexHit || h.Vertex != d.VertexByID("b") {
		t.Errorf("hover %+v", h)
	}
	view := d.View()
	send(t, e, edit.Event{Kind: edit.Wheel, Delta: 40})
	if d.View().Y != view.Y+40 {
		t.Errorf("wheel: %+v from %+v", d.View(), view)
	}
	view = d.View()
	send(t, e, edit.Event{Kind: edit.Wheel, Delta: 100, Modifiers: edit.Ctrl}, key("=", edit.Ctrl))
	if d.View().W == view.W {
		t.Errorf("zoom: %+v", d.View())
	}
	view = d.View()
	send(t, e, edit.Event{Kind: edit.PointerDown, Point: pt(400, 100), Button: 1}, move(430, 120))
	if e.State() != edit.Panning || d.View().X == view.X {
		t.Errorf("middle-button pan: state %d, view %+v", e.State(), d.View())
	}
	send(t, e, up(430, 120), key("Tab", 0))
	if v, _ := selected(e); ids(d, v) != "s" {
		t.Errorf("tab: %q", ids(d, v))
	}
	send(t, e, context(420, 100))
	if v, _ := selected(e); ids(d, v) != "b" {
		t.Errorf("context: %q", ids(d, v))
	}
	if d.CanUndo() {
		t.Error("looking applied a command")
	}
}

// TestHoldRelease: a release leaves the mode as it was, in both modes,
// and a mode change while held keeps the hold; holding twice and
// releasing twice do nothing more. Released, a drag moves again.
func TestHoldRelease(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	e.Hold()
	e.Hold()
	e.SetMode(edit.View)
	if !e.Held() || e.Mode() != edit.View {
		t.Fatalf("view while held: held %v, mode %d", e.Held(), e.Mode())
	}
	e.Release()
	if e.Held() || e.Mode() != edit.View {
		t.Errorf("released in view: held %v, mode %d", e.Held(), e.Mode())
	}
	e.SetMode(edit.Edit)
	e.Hold()
	e.Release()
	e.Release()
	if e.Held() || e.Mode() != edit.Edit {
		t.Errorf("released in edit: held %v, mode %d", e.Held(), e.Mode())
	}
	send(t, e, down(220, 100, 0), move(240, 120), up(240, 120))
	if d.Vertex(d.VertexByID("a")).Pos() != pt(240, 120) || !d.CanUndo() {
		t.Errorf("a drag after the release: %v", d.Vertex(d.VertexByID("a")).Pos())
	}
}

// TestHeldInView: a held editor in view mode refuses with ErrHeld, which
// names the hold, not the mode.
func TestHeldInView(t *testing.T) {
	_, e := fixture(t, gw.Caps{})
	e.SetMode(edit.View)
	if _, err := e.Handle(key("Delete", 0)); !errors.Is(err, edit.ErrView) {
		t.Errorf("not held: %v", err)
	}
	e.Hold()
	if _, err := e.Handle(key("Delete", 0)); !errors.Is(err, edit.ErrHeld) {
		t.Errorf("held: %v", err)
	}
}

// TestAllocHold: holding and releasing allocate nothing.
func TestAllocHold(t *testing.T) {
	_, e := fixture(t, gw.Caps{})
	if n := testing.AllocsPerRun(50, func() { e.Hold(); e.Release() }); n > 0 {
		t.Errorf("%g allocations", n)
	}
}
