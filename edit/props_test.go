package edit_test

import (
	"errors"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// TestLabelEditing: a double-click on a vertex or an edge, on its path
// or its label, edits its label; Enter, F2 or a printable character
// with one element selected does too, the character kept for the input
// to start with; the text done relabels, as one command; nothing starts
// on the background, with two selected, with Ctrl held or in view mode.
func TestLabelEditing(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	dbl := func(x, y float64) edit.Event { return edit.Event{Kind: edit.DoubleClick, Point: pt(x, y)} }
	editing := func(name string, v gw.VertexIndex, x gw.EdgeIndex, typed string) {
		t.Helper()
		gv, ge := e.Editing()
		if e.State() != edit.EditingText || gv != v || ge != x || e.Typed() != typed {
			t.Errorf("%s: state %d editing %d %d typed %q", name, e.State(), gv, ge, e.Typed())
		}
		send(t, e, edit.Event{Kind: edit.TextCancel})
	}
	send(t, e, dbl(220, 100))
	editing("double-click on a", 1, gw.None, "")
	send(t, e, dbl(134, 100))
	editing("double-click on the edge from s to a", gw.None, 0, "")
	send(t, e, dbl(220, 100), edit.Event{Kind: edit.TextDone, Text: "renamed"})
	if d.Vertices[1].Label != "renamed" || e.State() != edit.Idle {
		t.Errorf("text done: label %q state %d", d.Vertices[1].Label, e.State())
	}
	send(t, e, key("z", edit.Ctrl))
	if d.Vertices[1].Label != "a" {
		t.Errorf("undo of the relabel: %q", d.Vertices[1].Label)
	}

	// The label of an edge is a place to double-click it.
	send(t, e, down(280, 100, 0), up(360, 100))
	o := gw.EdgeIndex(len(d.Edges) - 1) //nolint:gosec // bounded by the capacity
	if _, err := d.Apply(gw.Command{Op: gw.Relabel, Vertex: gw.None, Edge: o, Text: "a long label here"}); err != nil {
		t.Fatal(err)
	}
	box := svg.EdgeLabelBox(d, o)
	send(t, e, dbl(box.X+box.W/2+20, box.Y+box.H/2))
	editing("double-click on an edge's label", gw.None, o, "")

	send(t, e, down(220, 100, 0), up(220, 100), key("F2", 0))
	editing("F2", 1, gw.None, "")
	send(t, e, down(220, 100, 0), up(220, 100), key("Enter", 0))
	editing("Enter", 1, gw.None, "")
	send(t, e, down(220, 100, 0), up(220, 100), key("é", 0))
	editing("typing é", 1, gw.None, "é")

	for _, k := range []edit.Event{key("x", edit.Ctrl), key("Shift", 0), key("ArrowLeft", 0), key("\x01", 0)} {
		send(t, e, down(220, 100, 0), up(220, 100), k)
		if e.State() == edit.EditingText {
			t.Errorf("%q started editing", k.Key)
			send(t, e, edit.Event{Kind: edit.TextCancel})
		}
	}
	send(t, e, dbl(700, 190))
	if e.State() != edit.Idle {
		t.Errorf("double-click on the background: state %d", e.State())
	}
	send(t, e, down(220, 100, 0), up(220, 100), down(420, 100, edit.Shift), up(420, 100), key("x", 0))
	if e.State() != edit.Idle {
		t.Errorf("typing with two selected: state %d", e.State())
	}
	e.SetMode(edit.View)
	if _, err := e.Handle(dbl(220, 100)); !errors.Is(err, edit.ErrView) {
		t.Errorf("double-click in view mode: %v", err)
	}
}

// TestProperties: the one selected vertex or edge is described with its
// kind's attributes in order, their schema and values, then those the
// schema does not name; SetLabel and SetAttr change it as commands, the
// schema's refusal coming back; with no one element selected there is
// nothing to show or set.
func TestProperties(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	if _, ok := e.Properties(); ok {
		t.Error("properties with nothing selected")
	}
	if _, err := e.SetLabel("x"); !errors.Is(err, edit.ErrNotOne) {
		t.Errorf("SetLabel with nothing selected: %v", err)
	}
	if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: 1, Key: "extra", Value: "kept"}); err != nil {
		t.Fatal(err)
	}
	send(t, e, down(220, 100, 0), up(220, 100))
	p, ok := e.Properties()
	if !ok || p.Vertex != 1 || p.Edge != gw.None || p.ID != "a" || p.Kind != "node" || p.Label != "a" || len(p.Attrs) != 3 {
		t.Fatalf("properties of a: %+v", p)
	}
	if a := p.Attrs[0]; a.Key != "p" || a.Value != "0.5" || a.Kind != gw.Number || !a.Required || !a.Known {
		t.Errorf("p: %+v", a)
	}
	if a := p.Attrs[1]; a.Key != "mode" || a.Value != "" || a.Kind != gw.Choice || strings.Join(a.Choices, ",") != "fast,slow" || a.Required {
		t.Errorf("mode: %+v", a)
	}
	if a := p.Attrs[2]; a.Key != "extra" || a.Value != "kept" || a.Kind != gw.Text || a.Known {
		t.Errorf("extra: %+v", a)
	}
	ch, err := e.SetAttr("mode", "slow")
	if err != nil || len(ch.Vertices) != 1 || ch.Vertices[0] != 1 {
		t.Fatalf("SetAttr: %v %+v", err, ch)
	}
	if _, err := e.SetAttr("p", "lots"); err == nil || !strings.Contains(err.Error(), "must be a number") {
		t.Errorf("a word for a number: %v", err)
	}
	if _, err := e.SetLabel("alpha"); err != nil {
		t.Fatal(err)
	}
	if p, _ := e.Properties(); p.Label != "alpha" || p.Attrs[1].Value != "slow" {
		t.Errorf("after the edits: %+v", p)
	}
	send(t, e, key("z", edit.Ctrl), key("z", edit.Ctrl))
	if p, _ := e.Properties(); p.Label != "a" || p.Attrs[1].Value != "" {
		t.Errorf("both undone: %+v", p)
	}

	send(t, e, down(134, 100, 0), up(134, 100))
	if p, ok := e.Properties(); !ok || p.Edge != 0 || p.Kind != "to" || p.From != "s" || p.To != "a" || len(p.Attrs) != 0 {
		t.Errorf("properties of the edge: %+v", p)
	}
	e.SetMode(edit.View)
	if _, err := e.SetLabel("x"); !errors.Is(err, edit.ErrView) {
		t.Errorf("SetLabel in view mode: %v", err)
	}
}

// TestAttrEditing: on a kind that names an EditAttr, the label input
// edits that attribute: it starts with the attribute's value, its text
// done sets the attribute and leaves the label, as one command undone
// in one step; a value the schema refuses comes back as the error and
// changes nothing; cancelling changes nothing. A vertex kind's EditAttr
// works as an edge kind's does.
func TestAttrEditing(t *testing.T) {
	s := schema()
	s.VertexKinds[kNode].EditAttr = "p"
	s.EdgeKinds[1].Attrs = []gw.AttrSpec{{Key: "share", Kind: gw.Number, Bounded: true, Min: 0, Max: 1}}
	s.EdgeKinds[1].LabelFormat, s.EdgeKinds[1].EditAttr = "{share = |share}", "share"
	d, err := gw.New(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []float64{220, 420} {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "n"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: pt(x, 100)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 1, From: 0, To: 1, FromPort: 1, ToPort: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: 0, Key: "share", Value: "0.5"}); err != nil {
		t.Fatal(err)
	}
	d.View = gw.Viewport{X: 0, Y: 0, W: 800, H: 200}
	d.ClearUndo()
	e := edit.New(d, edit.Edit, 8)
	share := func() string { return gw.AttrValue(d.Edges[0].Attrs, "share") }
	dbl := edit.Event{Kind: edit.DoubleClick, Point: pt(320, 100)}

	if e.EditText() != "" {
		t.Errorf("edit text while idle: %q", e.EditText())
	}
	send(t, e, dbl)
	if v, x := e.Editing(); e.State() != edit.EditingText || v != gw.None || x != 0 || e.EditText() != "0.5" {
		t.Fatalf("double-click on the edge: state %d editing %d %d text %q", e.State(), v, x, e.EditText())
	}
	send(t, e, edit.Event{Kind: edit.TextDone, Text: "0.7"})
	if share() != "0.7" || d.Edges[0].Label != "" || e.State() != edit.Idle {
		t.Errorf("text done: share %q label %q state %d", share(), d.Edges[0].Label, e.State())
	}
	send(t, e, key("z", edit.Ctrl))
	if share() != "0.5" || d.CanUndo() {
		t.Errorf("one undo: share %q, more to undo %v", share(), d.CanUndo())
	}

	send(t, e, dbl)
	if _, err := e.Handle(edit.Event{Kind: edit.TextDone, Text: "1.5"}); err == nil || !strings.Contains(err.Error(), "must be a number from 0 to 1") {
		t.Errorf("an out-of-bounds share: %v", err)
	}
	if share() != "0.5" || d.CanUndo() || e.State() != edit.Idle {
		t.Errorf("after the refusal: share %q, something to undo %v, state %d", share(), d.CanUndo(), e.State())
	}
	send(t, e, dbl, edit.Event{Kind: edit.TextCancel})
	if share() != "0.5" || d.CanUndo() {
		t.Errorf("cancel: share %q, something to undo %v", share(), d.CanUndo())
	}

	// A key typed begins the editing with it, the attribute's value
	// still what the input would hold without it.
	send(t, e, key("0", 0))
	if e.State() != edit.EditingText || e.Typed() != "0" || e.EditText() != "0.5" {
		t.Errorf("typing 0 on the selected edge: state %d typed %q text %q", e.State(), e.Typed(), e.EditText())
	}
	send(t, e, edit.Event{Kind: edit.TextCancel})

	send(t, e, edit.Event{Kind: edit.DoubleClick, Point: pt(220, 100)})
	if e.EditText() != "0.5" {
		t.Errorf("the vertex's text: %q, want its p", e.EditText())
	}
	send(t, e, edit.Event{Kind: edit.TextDone, Text: "0.9"})
	if p := gw.AttrValue(d.Vertices[0].Attrs, "p"); p != "0.9" || d.Vertices[0].Label != "n" {
		t.Errorf("the vertex's text done: p %q label %q", p, d.Vertices[0].Label)
	}
}
