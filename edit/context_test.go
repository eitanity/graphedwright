package edit_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

func context(x, y float64) edit.Event { return edit.Event{Kind: edit.Context, Point: pt(x, y)} }

// fromKeys is a context request from the keyboard, whose point is not
// read.
var fromKeys = edit.Event{Kind: edit.Context, Point: pt(9999, 9999), Keyboard: true}

// TestContextSelects: a context request on an element not selected
// selects it alone; on one selected, the selection stays, so the host's
// action applies to all of it; on the background, nothing is selected.
// The request names what was hit and where.
func TestContextSelects(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	a, b := d.VertexByID("a"), d.VertexByID("b")
	if r := e.Context(); r.Hit.Kind != svg.Background || r.Hit.Vertex != gw.None || r.Hit.Edge != gw.None || r.Hit.Port != gw.None {
		t.Errorf("before any request: %+v", r)
	}
	send(t, e, down(420, 100, 0), up(420, 100), context(220, 100))
	if v, x := selected(e); ids(d, v) != "a" || len(x) != 0 {
		t.Errorf("on a with b selected: %q and %d edges", ids(d, v), len(x))
	}
	if r := e.Context(); r.Hit.Kind != svg.VertexHit || r.Hit.Vertex != a || r.Point != pt(220, 100) || r.Keyboard {
		t.Errorf("request on a: %+v", r)
	}
	// A marquee over s, a and b, and the edge between s and a.
	send(t, e, down(10, 10, 0), move(500, 150), up(500, 150), context(420, 100))
	if v, x := selected(e); ids(d, v) != "sab" || len(x) != 1 {
		t.Errorf("on b with three selected: %q and %d edges", ids(d, v), len(x))
	}
	if r := e.Context(); r.Hit.Vertex != b {
		t.Errorf("request on b: %+v", r)
	}
	send(t, e, context(400, 190))
	if v, x := selected(e); len(v) != 0 || len(x) != 0 {
		t.Errorf("on the background: %q and %d edges", ids(d, v), len(x))
	}
	if r := e.Context(); r.Hit.Kind != svg.Background || r.Point != pt(400, 190) {
		t.Errorf("request on the background: %+v", r)
	}
	send(t, e, context(130, 100))
	if v, x := selected(e); len(v) != 0 || len(x) != 1 || x[0] != 0 {
		t.Errorf("on the edge: %q and edges %v", ids(d, v), x)
	}
	if r := e.Context(); r.Hit.Kind != svg.EdgeHit || r.Hit.Edge != 0 {
		t.Errorf("request on the edge: %+v", r)
	}
	send(t, e, context(280, 100))
	if v, _ := selected(e); ids(d, v) != "a" {
		t.Errorf("on a's port: %q", ids(d, v))
	}
	if r := e.Context(); r.Hit.Kind != svg.PortHit || r.Hit.Vertex != a || r.Hit.Port != 1 {
		t.Errorf("request on a's out port: %+v", r)
	}
	if d.CanUndo() || e.State() != edit.Idle {
		t.Errorf("requests applied a command (%v) or left state %d", d.CanUndo(), e.State())
	}
}

// TestContextEndsGestures: a request during a gesture ends it as Escape
// does: a drag goes back, and a connection, a marquee, a placement, a
// resize and label editing are dropped, with nothing applied.
func TestContextEndsGestures(t *testing.T) {
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
				t.Fatalf("state %d before the request, want %d", e.State(), c.want)
			}
			send(t, e, context(420, 100), up(420, 100))
			if snapshot(d) != before || d.CanUndo() || e.State() != edit.Idle {
				t.Errorf("after the request and the release: state %d, undo %v\n%s\n%s", e.State(), d.CanUndo(), before, snapshot(d))
			}
			if v, _ := e.Editing(); v != gw.None {
				t.Errorf("still editing %d", v)
			}
		})
	}
}

// TestContextFromKeys: a request from the keyboard is on the
// selection, at the centre of what it holds, and leaves it as it is;
// with nothing selected it is on the background at the view's centre.
func TestContextFromKeys(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	a := d.VertexByID("a")
	// a's box runs from x 160 to 280, b's from 360 to 480.
	send(t, e, down(220, 100, 0), up(220, 100), down(420, 100, edit.Shift), up(420, 100), fromKeys)
	if r := e.Context(); r.Hit.Kind != svg.VertexHit || r.Hit.Vertex != a || r.Point != pt(320, 100) || !r.Keyboard {
		t.Errorf("on a and b: %+v", r)
	}
	if v, _ := selected(e); ids(d, v) != "ab" {
		t.Errorf("selection after: %q", ids(d, v))
	}
	// The edge from s at (60,100) to a at (220,100).
	send(t, e, down(130, 100, 0), up(130, 100), fromKeys)
	if r := e.Context(); r.Hit.Kind != svg.EdgeHit || r.Hit.Edge != 0 || r.Point != pt(140, 100) {
		t.Errorf("on the edge: %+v", r)
	}
	// b made 40 by 80, from x 400 to 440 and y 60 to 140, with a.
	send(t, e, down(420, 100, 0), up(420, 100))
	if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: d.VertexByID("b"), Size: gw.Size{W: 40, H: 80}}); err != nil {
		t.Fatal(err)
	}
	send(t, e, down(220, 100, edit.Shift), up(220, 100), fromKeys)
	if r := e.Context(); r.Point != pt(300, 100) {
		t.Errorf("on b, 40 by 80, and a: %+v", r)
	}
	send(t, e, down(420, 100, edit.Shift), up(420, 100), fromKeys)
	if r := e.Context(); r.Hit.Vertex != a || r.Point != pt(220, 100) {
		t.Errorf("on a alone: %+v", r)
	}
	send(t, e, down(700, 190, 0), up(700, 190), fromKeys)
	if r := e.Context(); r.Hit.Kind != svg.Background || r.Point != pt(400, 100) {
		t.Errorf("on nothing: %+v", r)
	}
}

// TestContextInView: in view mode a request selects as a click does
// and applies nothing.
func TestContextInView(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	e.SetMode(edit.View)
	before := snapshot(d)
	for _, ev := range []edit.Event{context(220, 100), context(420, 100)} {
		if _, err := e.Handle(ev); err != nil {
			t.Fatalf("%+v: %v", ev, err)
		}
	}
	if v, _ := selected(e); ids(d, v) != "b" {
		t.Errorf("selected %q", ids(d, v))
	}
	send(t, e, context(400, 190))
	if v, _ := selected(e); len(v) != 0 || snapshot(d) != before || d.CanUndo() {
		t.Errorf("selected %q, or a command applied", ids(d, v))
	}
}

// TestSecondaryPress: a press of the secondary button starts nothing:
// a press, a move and a release with it change nothing and select
// nothing, on a vertex, a port and the background.
func TestSecondaryPress(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, down(420, 100, 0), up(420, 100))
	before := snapshot(d)
	for _, at := range []gw.Point{pt(220, 100), pt(280, 100), pt(400, 190)} {
		for _, ev := range []edit.Event{
			{Kind: edit.PointerDown, Point: at, Button: 2},
			move(at.X+60, at.Y+30),
			{Kind: edit.PointerUp, Point: pt(at.X+60, at.Y+30), Button: 2},
		} {
			ch, err := e.Handle(ev)
			if err != nil || len(ch.Vertices) != 0 || len(ch.Edges) != 0 || ch.Layout {
				t.Errorf("%+v: change %+v, %v", ev, ch, err)
			}
			if e.State() != edit.Idle {
				t.Errorf("%+v: state %d", ev, e.State())
			}
		}
	}
	if v, _ := selected(e); ids(d, v) != "b" || snapshot(d) != before || d.CanUndo() {
		t.Errorf("selected %q after, or the document changed", ids(d, v))
	}
}

// TestAllocContext: a request allocates nothing.
func TestAllocContext(t *testing.T) {
	_, e := fixture(t, gw.Caps{})
	if n := testing.AllocsPerRun(50, func() { _, _ = e.Handle(context(220, 100)); _, _ = e.Handle(fromKeys) }); n > 0 {
		t.Errorf("%g allocations", n)
	}
}
