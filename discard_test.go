package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestDiscard: Discard takes back the open group's commands, and only
// those, and forgets them, so that nothing is left to redo; with no
// group open it does nothing; a group whose first command has fallen
// off the log, past the undo depth or dropped by a layout past the
// layouts kept, is not taken back in part.
func TestDiscard(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, _, _ := chain(t, d)
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "kept"})
	before := snapshot(d)
	if _, ok := d.Discard(); ok || snapshot(d) != before {
		t.Fatal("Discard with no group open took something back")
	}
	d.Group()
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: a, Text: "one"})
	apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "two"})
	ch, ok := d.Discard()
	d.EndGroup()
	if !ok || snapshot(d) != before {
		t.Fatalf("after Discard:\n%s\nwant\n%s", snapshot(d), before)
	}
	if len(ch.Vertices) == 0 {
		t.Error("Discard reported no change")
	}
	if d.CanRedo() {
		t.Error("what Discard took back can be redone")
	}
	if _, ok := d.Undo(); !ok || d.Vertex(a).Label() != "a" {
		t.Errorf("the command before the group: undid to %q", d.Vertex(a).Label())
	}

	// A group of four in a log three deep.
	s := newDoc(t, gw.Caps{UndoDepth: 3})
	v := addVertex(t, s, kNode, "v")
	s.Group()
	for _, l := range []string{"1", "2", "3", "4"} {
		apply(t, s, gw.Command{Op: gw.Relabel, Vertex: v, Text: l})
	}
	if _, ok := s.Discard(); ok || s.Vertex(v).Label() != "4" {
		t.Errorf("a group past the undo depth: Discard %v, label %q", ok, s.Vertex(v).Label())
	}
	s.EndGroup()
	// The next group, wholly in the log, is taken back.
	s.Group()
	apply(t, s, gw.Command{Op: gw.Relabel, Vertex: v, Text: "5"})
	if _, ok := s.Discard(); !ok || s.Vertex(v).Label() != "4" {
		t.Errorf("the next group: Discard %v, label %q", ok, s.Vertex(v).Label())
	}
	s.EndGroup()
	// No group open, once a group has lost its start: still nothing.
	if _, ok := s.Discard(); ok {
		t.Error("Discard with no group open reported a take-back")
	}

	// A group whose start a layout past the layouts kept has dropped.
	u := newDoc(t, gw.Caps{UndoLayouts: 1})
	w := addVertex(t, u, kNode, "w")
	u.Group()
	apply(t, u, gw.Command{Op: gw.Relabel, Vertex: w, Text: "1"})
	apply(t, u, gw.Command{Op: gw.Layout})
	apply(t, u, gw.Command{Op: gw.Relabel, Vertex: w, Text: "2"})
	apply(t, u, gw.Command{Op: gw.Layout})
	if _, ok := u.Discard(); ok || u.Vertex(w).Label() != "2" {
		t.Errorf("a group a layout dropped the start of: Discard %v, label %q", ok, u.Vertex(w).Label())
	}
	u.EndGroup()
	// And one of which a layout dropped every entry before it.
	u.Group()
	apply(t, u, gw.Command{Op: gw.Relabel, Vertex: w, Text: "3"})
	apply(t, u, gw.Command{Op: gw.Layout})
	apply(t, u, gw.Command{Op: gw.Layout})
	if _, ok := u.Discard(); ok || u.Vertex(w).Label() != "3" {
		t.Errorf("a group a layout dropped all but itself of: Discard %v, label %q", ok, u.Vertex(w).Label())
	}
	u.EndGroup()
}
