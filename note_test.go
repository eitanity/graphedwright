package graphedwright_test

import (
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// The kinds of the note schema, by index.
const (
	nBox gw.KindIndex = iota
	nRoot
	nLeaf
	nZone
	nNote
)

// The edge kinds of the note schema, by index.
const (
	nCurve gw.KindIndex = iota
	nAnchor
)

// noteSchema: boxes, a root and a leaf kind, a zone that holds boxes
// only, and notes; curves between any kinds, labelled, and anchors from
// a note to any kind, curved too.
func noteSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}},
			{Name: "root", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Root: true},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Leaf: true},
			{Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 100, H: 80}, Container: true, Contains: []gw.KindIndex{nBox}},
			{Name: "note", Shape: gw.Rect, Size: gw.Size{W: 80, H: 40}, Note: true},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "curve", Route: gw.Curve, Arrow: true},
			{Name: "anchor", Route: gw.Curve, From: []gw.KindIndex{nNote}, Dashed: true},
		},
		Connected: true,
	}
}

func noteDoc(t *testing.T) *gw.Document {
	t.Helper()
	d, err := makeDoc(noteSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func addAt(t *testing.T, d *gw.Document, kind gw.KindIndex, x, y float64) gw.VertexIndex {
	t.Helper()
	return apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, Pos: gw.Point{X: x, Y: y}}).Vertices[0]
}

func join(t *testing.T, d *gw.Document, kind gw.KindIndex, from, to gw.VertexIndex, label string) gw.EdgeIndex {
	t.Helper()
	return apply(t, d, gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: gw.None, ToPort: gw.None, Text: label}).Edges[0]
}

// TestNoteKind: a note kind may not be a container; the view reports a
// note; and a container holds a note though its Contains does not list
// it, but not another kind it does not list.
func TestNoteKind(t *testing.T) {
	s := noteSchema()
	s.VertexKinds[nNote].Container = true
	if _, err := gw.Compile(s); err == nil || !strings.Contains(err.Error(), "is a note and a container") {
		t.Errorf("a note container compiled: %v", err)
	}
	d := noteDoc(t)
	k := d.Kinds()
	if !k.Vertex(nNote).Note() || k.Vertex(nBox).Note() {
		t.Error("Note misreports the kinds")
	}
	for _, c := range []struct {
		c, kind gw.KindIndex
		want    bool
	}{{nZone, nNote, true}, {nZone, nBox, true}, {nZone, nLeaf, false}, {nBox, nNote, false}} {
		if got := k.Holds(c.c, c.kind); got != c.want {
			t.Errorf("Holds(%d, %d) = %v, want %v", c.c, c.kind, got, c.want)
		}
	}
	z := addAt(t, d, nZone, 0, 0)
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: addAt(t, d, nNote, 0, 0), To: z})
	refuse(t, d, gw.Command{Op: gw.SetParent, Vertex: addAt(t, d, nLeaf, 0, 0), To: z}, "cannot hold kind")
}

// TestAnchors: an edge at a note is an anchor, drawn straight whatever
// its kind's route; a note's target is the other end of its live anchor
// of the lowest index that joins no other note, either way round; and a
// root and a leaf take an anchor, but not another edge.
func TestAnchors(t *testing.T) {
	d := noteDoc(t)
	a, b := addAt(t, d, nBox, 0, 0), addAt(t, d, nBox, 200, 0)
	n, m := addAt(t, d, nNote, 0, 100), addAt(t, d, nNote, 200, 100)
	r, l := addAt(t, d, nRoot, 400, 0), addAt(t, d, nLeaf, 600, 0)
	plain := join(t, d, nCurve, a, b, "")
	if d.Anchor(plain) || d.Route(plain) != gw.Curve {
		t.Error("a curve between boxes is an anchor, or not curved")
	}
	if d.NoteTarget(n) != gw.None || d.NoteTarget(a) != gw.None {
		t.Error("a note without an anchor, or a box, has a target")
	}
	toNote := join(t, d, nAnchor, n, m, "")
	first := join(t, d, nAnchor, n, b, "")
	join(t, d, nAnchor, m, n, "")
	gone := join(t, d, nCurve, b, n, "")
	into := join(t, d, nCurve, a, n, "")
	join(t, d, nAnchor, n, r, "") // a root takes it
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: gone})
	if !d.Anchor(toNote) || !d.Anchor(first) || !d.Anchor(into) || d.Route(first) != gw.Straight || d.Route(into) != gw.Straight {
		t.Error("an edge at a note is not an anchor drawn straight")
	}
	if got := d.NoteTarget(n); got != b {
		t.Errorf("target %d, want b (%d): the first anchor joins another note, and the last out comes after", got, b)
	}
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: first})
	if got := d.NoteTarget(n); got != a {
		t.Errorf("target %d once the first is gone, want a (%d): of those in, the first is from a note and the next deleted", got, a)
	}
	if d.Anchor(gw.EdgeIndex(-1)) || d.Route(gw.EdgeIndex(-1)) != gw.Straight || d.NoteTarget(gw.VertexIndex(-1)) != gw.None {
		t.Error("an index out of range is misread")
	}
	join(t, d, nCurve, l, m, "") // a leaf takes one
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: nCurve, From: a, To: r, FromPort: gw.None, ToPort: gw.None}, "takes no incoming edge")
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: nCurve, From: l, To: a, FromPort: gw.None, ToPort: gw.None}, "takes no outgoing edge")
}

// TestNotesReachable: no root reaching a note is no warning, while a box
// no root reaches is.
func TestNotesReachable(t *testing.T) {
	d := noteDoc(t)
	r := addAt(t, d, nRoot, 0, 0)
	b := addAt(t, d, nBox, 200, 0)
	join(t, d, nCurve, r, b, "")
	addAt(t, d, nNote, 0, 100)
	lone := addAt(t, d, nBox, 400, 0)
	got := d.Unreachable(make([]gw.VertexIndex, 0, 8))
	if len(got) != 1 || got[0] != lone {
		t.Errorf("unreachable %v, want only %d", got, lone)
	}
}

// TestAnchorsTakeNoRoom: an anchor takes no slot among the ends at a
// side, gives its container's side no room for its label, and does not
// make the container stand out by the band room, though curved and
// labelled as the edges that do are.
func TestAnchorsTakeNoRoom(t *testing.T) {
	d := noteDoc(t)
	tv := addAt(t, d, nBox, 300, 0)
	s0, s1 := addAt(t, d, nBox, 0, -50), addAt(t, d, nBox, 0, 50)
	n := addAt(t, d, nNote, 0, 0)
	e := join(t, d, nCurve, s0, tv, "")
	join(t, d, nCurve, s1, tv, "")
	join(t, d, nAnchor, n, tv, "")
	if k, all := d.EndSlot(e, true); k != 0 || all != 2 {
		t.Errorf("slot %d of %d, want 0 of 2: the anchor takes none", k, all)
	}
	// A note in a zone, anchored to the zone and labelled, after a
	// member along the flow, and anchored to a box outside.
	z := addAt(t, d, nZone, 1000, 0)
	a, in := addAt(t, d, nBox, 1000, 0), addAt(t, d, nNote, 1300, 0)
	for _, v := range [2]gw.VertexIndex{a, in} {
		apply(t, d, gw.Command{Op: gw.SetParent, Vertex: v, To: z})
	}
	size := d.Vertex(z).Size()
	join(t, d, nAnchor, in, z, "a long label for the side")
	if back, on := d.EnclosedRoom(z); back != 0 || on != 0 {
		t.Errorf("room %v, %v for an anchor, want none", back, on)
	}
	join(t, d, nAnchor, in, s0, "")
	if got := d.Vertex(z).Size(); got != size {
		t.Errorf("the zone is %v with anchors, %v without", got, size)
	}
}
