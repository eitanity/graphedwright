package edit_test

import (
	"errors"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// grouped is the fixture with group g holding a and b, b→c, and a→c.
func grouped(t *testing.T) (*gw.Document, *edit.Editor, gw.VertexIndex) {
	t.Helper()
	d, e := fixture(t, gw.Caps{Vertices: 16, Edges: 16})
	g := add(t, d, gw.Command{Op: gw.AddVertex, Kind: kGroup, ID: "g", Text: "g"}).Vertices[0]
	for _, c := range []gw.Command{
		{Op: gw.SetParent, Vertex: 1, To: g},
		{Op: gw.SetParent, Vertex: 2, To: g},
		{Op: gw.AddEdge, Kind: eOutcome, From: 2, To: 3, FromPort: 1, ToPort: gw.None},
		{Op: gw.AddEdge, Kind: eOutcome, From: 1, To: 3, FromPort: 1, ToPort: gw.None},
	} {
		add(t, d, c)
	}
	d.ClearUndo()
	return d, e, g
}

// add applies a command that must succeed.
func add(t *testing.T, d *gw.Document, c gw.Command) gw.Change {
	t.Helper()
	ch, err := d.Apply(c)
	if err != nil {
		t.Fatalf("%+v: %v", c, err)
	}
	return ch
}

// TestReveal: a vertex in folds within folds is unfolded, as one step
// for Undo, selected alone and centred, in either mode; one not live,
// or during a gesture, is refused.
func TestReveal(t *testing.T) {
	d, e, g := grouped(t)
	inner := add(t, d, gw.Command{Op: gw.AddVertex, Kind: kGroup, ID: "inner"}).Vertices[0]
	add(t, d, gw.Command{Op: gw.SetParent, Vertex: inner, To: g})
	add(t, d, gw.Command{Op: gw.SetParent, Vertex: 1, To: inner})
	add(t, d, gw.Command{Op: gw.Fold, Vertex: inner})
	add(t, d, gw.Command{Op: gw.Fold, Vertex: g})
	e.SetMode(edit.View)
	view := d.View()
	ch, err := e.Reveal(1)
	if err != nil || d.Hidden(1) || d.Vertex(g).Folded() || d.Vertex(inner).Folded() || !ch.Layout {
		t.Fatalf("revealed: %v, hidden %v", err, d.Hidden(1))
	}
	if sv, se := selected(e); len(sv) != 1 || sv[0] != 1 || len(se) != 0 {
		t.Errorf("selected %v %v", sv, se)
	}
	v, a := d.View(), d.Vertex(1).Pos()
	if v.W != view.W || v.H != view.H || v.X+v.W/2 != a.X || v.Y+v.H/2 != a.Y {
		t.Errorf("view %+v, a at %v", v, a)
	}
	d.Undo()
	if !d.Vertex(g).Folded() || !d.Vertex(inner).Folded() {
		t.Error("one undo left a fold open")
	}
	// A vertex wider than the view is zoomed out to.
	d.SetView(gw.Viewport{W: 50, H: 50})
	if _, err := e.Reveal(3); err != nil || d.View().W < d.Vertex(3).Size().W {
		t.Errorf("wide: %v %+v", err, d.View())
	}
	if _, err := e.Reveal(99); err == nil {
		t.Error("a vertex out of range revealed")
	}
	e.SetMode(edit.Edit)
	send(t, e, down(620, 100, 0), move(640, 120))
	if _, err := e.Reveal(1); err == nil {
		t.Error("revealed during a drag")
	}
}

// TestFocus: the selection's neighbourhood, the way given; a selected
// fold's through what it holds; a vertex added after is in it; the
// overlay is given it; hidden, what is out of it is not hit; Escape on
// an empty selection ends it.
func TestFocus(t *testing.T) {
	d, e, g := grouped(t)
	if e.Focus(1, gw.Both) {
		t.Error("a focus with nothing selected")
	}
	send(t, e, down(60, 100, 0), up(60, 100))
	if !e.Focus(1, gw.Successors) || !e.InFocus(0) || !e.InFocus(1) || e.InFocus(2) || e.InFocus(3) {
		t.Fatalf("s's successors: %v %v %v %v", e.InFocus(0), e.InFocus(1), e.InFocus(2), e.InFocus(3))
	}
	n := add(t, d, gw.Command{Op: gw.AddVertex, Kind: kLeaf}).Vertices[0]
	var opts svg.Options
	e.Overlay(&opts)
	if !e.InFocus(n) || len(opts.Focus) != d.VertexSlots() || !opts.Focus[n] || opts.Focus[2] || opts.FocusHide {
		t.Errorf("overlay focus %v", opts.Focus)
	}
	e.SetFocusHide(true)
	send(t, e, down(420, 100, 0), up(420, 100))
	if sv, _ := selected(e); len(sv) != 0 {
		t.Errorf("b, out of a hiding focus, selected: %v", sv)
	}
	e.SetFocusHide(false)
	// A fold: its members' neighbours.
	add(t, d, gw.Command{Op: gw.Fold, Vertex: g})
	send(t, e, down(d.Vertex(g).Pos().X, d.Vertex(g).Pos().Y, 0), up(d.Vertex(g).Pos().X, d.Vertex(g).Pos().Y))
	if !e.Focus(1, gw.Both) || !e.InFocus(g) || !e.InFocus(0) || !e.InFocus(3) {
		t.Errorf("around fold g: %v %v %v", e.InFocus(g), e.InFocus(0), e.InFocus(3))
	}
	send(t, e, key("Escape", 0))
	if !e.Focused() {
		t.Error("Escape on a selection ended the focus")
	}
	send(t, e, down(700, 190, 0), up(700, 190), key("Escape", 0))
	if e.Focused() || !e.InFocus(2) {
		t.Error("Escape on nothing kept the focus")
	}
}

// TestFoldKeys: Ctrl+[ folds the one selected container and Ctrl+]
// unfolds it, each one step; a double-click on a fold unfolds it; a fold
// drops what it hides from the selection; refused in view mode and while
// held.
func TestFoldKeys(t *testing.T) {
	d, e, g := grouped(t)
	gp := d.Vertex(g).Pos()
	send(t, e, down(420, 100, 0), up(420, 100), down(gp.X, d.Vertex(g).Pos().Y-d.Vertex(g).Size().H/2+4, edit.Shift), up(gp.X, d.Vertex(g).Pos().Y-d.Vertex(g).Size().H/2+4))
	if sv, _ := selected(e); len(sv) != 2 {
		t.Fatalf("b and g: %v", sv)
	}
	send(t, e, down(700, 190, 0), up(700, 190), down(gp.X, d.Vertex(g).Pos().Y-d.Vertex(g).Size().H/2+4, 0), up(gp.X, d.Vertex(g).Pos().Y-d.Vertex(g).Size().H/2+4))
	send(t, e, key("[", edit.Ctrl))
	if !d.Vertex(g).Folded() {
		t.Fatal("Ctrl+[ left g open")
	}
	send(t, e, key("z", edit.Ctrl))
	if d.Vertex(g).Folded() {
		t.Error("the fold's undo")
	}
	send(t, e, key("[", edit.Ctrl), key("]", edit.Ctrl))
	if d.Vertex(g).Folded() {
		t.Error("Ctrl+] left g folded")
	}
	// The selection loses what a fold hides.
	send(t, e, down(420, 100, 0), up(420, 100))
	add(t, d, gw.Command{Op: gw.Fold, Vertex: g})
	e.Settle()
	if sv, _ := selected(e); len(sv) != 0 {
		t.Errorf("a hidden vertex stayed selected: %v", sv)
	}
	p := d.Vertex(g).Pos()
	send(t, e, edit.Event{Kind: edit.DoubleClick, Point: p})
	if sv, _ := selected(e); d.Vertex(g).Folded() || len(sv) != 1 || sv[0] != g || e.State() != edit.Idle {
		t.Errorf("double-click on the fold: folded %v, selected %v, %v", d.Vertex(g).Folded(), sv, e.State())
	}
	e.Hold()
	if _, err := e.Handle(key("[", edit.Ctrl)); !errors.Is(err, edit.ErrHeld) || d.Vertex(g).Folded() {
		t.Errorf("held: %v", err)
	}
	e.Release()
	e.SetMode(edit.View)
	if _, err := e.Handle(key("[", edit.Ctrl)); !errors.Is(err, edit.ErrView) {
		t.Errorf("view mode: %v", err)
	}
}

// TestMergedSelected: selecting the one edge drawn for those a fold
// brings to the same ends selects them all.
func TestMergedSelected(t *testing.T) {
	d, e, g := grouped(t)
	add(t, d, gw.Command{Op: gw.Fold, Vertex: g})
	gv, cv := d.Vertex(g), d.Vertex(3)
	mid := gw.Point{X: (gv.Pos().X + gv.Size().W/2 + cv.Pos().X - cv.Size().W/2) / 2, Y: (gv.Pos().Y + cv.Pos().Y) / 2}
	if h := svg.HitTest(d, mid); h.Kind != svg.EdgeHit {
		t.Fatalf("no edge at %v: %+v", mid, h)
	}
	send(t, e, down(mid.X, mid.Y, 0), up(mid.X, mid.Y))
	if _, se := selected(e); len(se) != 2 {
		t.Errorf("selected %v", se)
	}
}
