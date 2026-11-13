package layout_test

import (
	"math"
	"os"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// notedMachine is the state machine example with a note added in the
// container in, or at the top with "", anchored to the vertex target.
func notedMachine(t *testing.T, flow gw.Flow, in, target string) (*gw.Document, gw.VertexIndex, gw.VertexIndex) {
	t.Helper()
	data, err := os.ReadFile("../web/examples/state-machine.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(flow)
	n := addNote(t, d, in, "a note")
	tv := d.VertexByID(gw.ID(target))
	anchor(t, d, n, tv)
	return d, n, tv
}

func addNote(t *testing.T, d *gw.Document, in, text string) gw.VertexIndex {
	t.Helper()
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: d.Kinds().VertexNamed("note"), Text: text})
	if err != nil {
		t.Fatal(err)
	}
	n := ch.Vertices[0]
	if in != "" {
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: n, To: d.VertexByID(gw.ID(in))}); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

func anchor(t *testing.T, d *gw.Document, n, to gw.VertexIndex) {
	t.Helper()
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: d.Kinds().EdgeNamed("anchor"), From: n, To: to, FromPort: gw.None, ToPort: gw.None}); err != nil {
		t.Fatal(err)
	}
}

// beside checks that note n stands beside vertex tv across the flow, no
// further than a gap from it and not on it, level with it along the
// flow; within its container's box; and clear of every other vertex but
// those holding it, as the line between it and tv is.
func beside(t *testing.T, name string, d *gw.Document, n, tv gw.VertexIndex) {
	t.Helper()
	a, b := d.Vertex(n), d.Vertex(tv)
	gapX := math.Abs(a.Pos().X-b.Pos().X) - (a.Size().W+b.Size().W)/2
	gapY := math.Abs(a.Pos().Y-b.Pos().Y) - (a.Size().H+b.Size().H)/2
	across, along := gapY, gapX
	if d.Flow() == gw.TopToBottom {
		across, along = gapX, gapY
	}
	if across <= 0 || across > 24 || along >= 0 {
		t.Errorf("%s: the note stands %v across and %v along from its target, want beside it within the gap of 24", name, across, along)
	}
	if p := a.Parent(); p != gw.None {
		c := d.Vertex(p)
		if math.Abs(a.Pos().X-c.Pos().X)+a.Size().W/2 > c.Size().W/2 || math.Abs(a.Pos().Y-c.Pos().Y)+a.Size().H/2 > c.Size().H/2 {
			t.Errorf("%s: the note at %v is outside its container at %v, %v", name, a.Pos(), c.Pos(), c.Size())
		}
	}
	for i := range d.VertexSlots() {
		v := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if !d.Vertex(v).Live() || v == n || v == tv || d.Within(n, v) || d.Within(tv, v) {
			continue
		}
		u := d.Vertex(v)
		if math.Abs(a.Pos().X-u.Pos().X) < (a.Size().W+u.Size().W)/2 && math.Abs(a.Pos().Y-u.Pos().Y) < (a.Size().H+u.Size().H)/2 {
			t.Errorf("%s: the note overlaps %q", name, u.ID())
		}
		if crosses(a.Pos(), b.Pos(), u) {
			t.Errorf("%s: the anchor crosses %q", name, u.ID())
		}
	}
}

// TestNoteBeside: in either flow, a note anchored to a vertex at the
// top, to a container, and to a vertex inside a container, each in its
// target's container, stands beside it, and the layout has the layers it
// has without the note. Laid out as a vertex in the flow, it stood in
// the first layer, or the first of its container, with its anchor run
// across the layers between. Start and View mode are each the first of
// their layer, with nothing before them, and stand where they do
// without their notes, which go there; a note that did not follow its
// target as the layers were centred held the target back off its place.
// A gap under the grid keeps the note half a step off its target or
// more, as a container's box fitted round what it holds may fall half a
// step off the layout's.
func TestNoteBeside(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, c := range [][2]string{{"", "start"}, {"", "view"}, {"edit", "gesture"}, {"gesture", "dragging"}, {"edit", "idle"}} {
			for _, gap := range []float64{0, 2} {
				d, n, tv := notedMachine(t, flow, c[0], c[1])
				name := c[1] + " " + string(rune('0'+flow))
				r, err := layout.New(d.Caps()).Run(d, layout.Options{All: true, Gap: gap})
				if err != nil {
					t.Fatal(err)
				}
				if gap == 0 {
					beside(t, name, d, n, tv)
				} else if g := acrossGap(d, n, tv); g <= 0 || g >= 12 {
					t.Errorf("%s: the note stands %v from its target with a gap of 2 on a grid of 8, want from half a step to one and a half", name, g)
				}
				at := d.Vertex(tv).Pos()
				if _, err := d.Apply(gw.Command{Op: gw.DeleteVertex, Vertex: n}); err != nil {
					t.Fatal(err)
				}
				r0, err := layout.New(d.Caps()).Run(d, layout.Options{All: true, Gap: gap})
				if err != nil {
					t.Fatal(err)
				}
				if r.Layers != r0.Layers {
					t.Errorf("%s: %d layers with the note, %d without", name, r.Layers, r0.Layers)
				}
				if first := c[1] == "start" || c[1] == "view"; first && gap == 0 && d.Vertex(tv).Pos() != at {
					t.Errorf("%s: the target stands at %v with its note, %v without", name, at, d.Vertex(tv).Pos())
				}
			}
		}
	}
}

// acrossGap is the room across the flow between note n and vertex tv.
func acrossGap(d *gw.Document, n, tv gw.VertexIndex) float64 {
	a, b := d.Vertex(n), d.Vertex(tv)
	if d.Flow() == gw.TopToBottom {
		return math.Abs(a.Pos().X-b.Pos().X) - (a.Size().W+b.Size().W)/2
	}
	return math.Abs(a.Pos().Y-b.Pos().Y) - (a.Size().H+b.Size().H)/2
}

// TestNoteSide: a note stands after its target across the flow, below
// it left to right, but before it, above, where the target is the first
// of its layer with others after it.
func TestNoteSide(t *testing.T) {
	d, err := doc.Decode([]byte(`{"schema": "basic", "vertices": [
		{"id": "s", "kind": "box"}, {"id": "a", "kind": "box"}, {"id": "b", "kind": "box"},
		{"id": "na", "kind": "note"}, {"id": "nb", "kind": "note"}],
		"edges": [{"id": "e1", "kind": "arrow", "from": "s", "to": "a"}, {"id": "e2", "kind": "arrow", "from": "s", "to": "b"},
		{"id": "x1", "kind": "anchor", "from": "na", "to": "a"}, {"id": "x2", "kind": "anchor", "from": "nb", "to": "b"}]}`), doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	y := func(id gw.ID) float64 { return d.Vertex(d.VertexByID(id)).Pos().Y }
	if !(y("na") < y("a") && y("a") < y("b") && y("b") < y("nb")) {
		t.Errorf("across the flow: na %v, a %v, b %v, nb %v; want na above a, nb below b", y("na"), y("a"), y("b"), y("nb"))
	}
}

// TestNotesBothSides: two notes on one vertex stand either side of it,
// each beside it; a third goes on past the first, and a fourth past the
// second.
func TestNotesBothSides(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, n, tv := notedMachine(t, flow, "gesture", "dragging")
		m := addNote(t, d, "gesture", "another")
		anchor(t, d, m, tv)
		o := addNote(t, d, "gesture", "a third")
		anchor(t, d, o, tv)
		p := addNote(t, d, "gesture", "a fourth")
		anchor(t, d, p, tv)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		beside(t, "first", d, n, tv)
		beside(t, "second", d, m, tv)
		across := func(v gw.VertexIndex) float64 {
			if flow == gw.TopToBottom {
				return d.Vertex(v).Pos().X - d.Vertex(tv).Pos().X
			}
			return d.Vertex(v).Pos().Y - d.Vertex(tv).Pos().Y
		}
		if a, b, c, e := across(n), across(m), across(o), across(p); a*b >= 0 || a*c <= 0 || b*e <= 0 || math.Abs(c) <= math.Abs(a) || math.Abs(e) <= math.Abs(b) {
			t.Errorf("flow %d: the notes stand %v, %v, %v and %v across from their target, want either side, the third past the first and the fourth past the second", flow, a, b, c, e)
		}
	}
}

// TestNotePlacedOrAway: a placed note stays where it was placed, and one
// whose target is outside its container stays inside it; each is laid
// out as it is without its anchor, and the rest as they are then.
func TestNotePlacedOrAway(t *testing.T) {
	d, n, _ := notedMachine(t, gw.LeftToRight, "gesture", "idle")
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	sameWithout(t, "away", d, layout.Options{All: true})
	a, c := d.Vertex(n), d.Vertex(d.VertexByID("gesture"))
	if math.Abs(a.Pos().X-c.Pos().X)+a.Size().W/2 > c.Size().W/2 || math.Abs(a.Pos().Y-c.Pos().Y)+a.Size().H/2 > c.Size().H/2 {
		t.Errorf("the note at %v is outside its container at %v, %v", a.Pos(), c.Pos(), c.Size())
	}
	d, n, _ = notedMachine(t, gw.LeftToRight, "", "start")
	at := gw.Point{X: -400, Y: -400}
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: n, Pos: at}); err != nil {
		t.Fatal(err)
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	if got := d.Vertex(n).Pos(); got != at {
		t.Errorf("the placed note moved to %v", got)
	}
	sameWithout(t, "placed", d, layout.Options{})
	// Placed in a container, which places it from the container's
	// centre as laid out: the same document with and without the anchor,
	// each laid out once from where it stood.
	var laid [2][]gw.Point
	for k := range laid {
		d, n, _ = notedMachine(t, gw.LeftToRight, "gesture", "dragging")
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		at = d.Vertex(d.VertexByID("resizing")).Pos()
		at.Y += 120
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: n, Pos: at}); err != nil {
			t.Fatal(err)
		}
		if k == 1 {
			deleteLastEdge(t, d)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
		for i := range d.VertexSlots() {
			laid[k] = append(laid[k], d.Vertex(gw.VertexIndex(i)).Pos()) //nolint:gosec // bounded by the capacity
		}
	}
	for i := range laid[0] {
		if laid[0][i] != laid[1][i] {
			t.Errorf("placed in a container: %s stands at %v with the anchor, %v without", d.Vertex(gw.VertexIndex(i)).ID(), laid[0][i], laid[1][i]) //nolint:gosec // bounded by the capacity
		}
	}
	// A group that nothing outside joins, but the note's anchor, and
	// another box beside it: the anchor gives the group no room for a
	// run in round its first member.
	d, err := doc.Decode([]byte(`{"schema": "basic", "vertices": [
		{"id": "g", "kind": "group"}, {"id": "a", "kind": "box", "parent": "g"}, {"id": "b", "kind": "box", "parent": "g"},
		{"id": "n", "kind": "note", "parent": "g"}, {"id": "o", "kind": "box"}],
		"edges": [{"id": "e", "kind": "arrow", "from": "a", "to": "b"}, {"id": "x", "kind": "anchor", "from": "n", "to": "o"}]}`), doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	sameWithout(t, "out of a group", d, layout.Options{All: true})
}

// deleteLastEdge deletes the live edge of the highest index, which must
// be an anchor.
func deleteLastEdge(t *testing.T, d *gw.Document) {
	t.Helper()
	last := gw.EdgeIndex(-1)
	for i := range d.EdgeSlots() {
		if d.Edge(gw.EdgeIndex(i)).Live() { //nolint:gosec // bounded by the capacity
			last = gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		}
	}
	if !d.Anchor(last) {
		t.Fatal("the last edge is not an anchor")
	}
	if _, err := d.Apply(gw.Command{Op: gw.DeleteEdge, Edge: last}); err != nil {
		t.Fatal(err)
	}
}

// sameWithout checks that document d, just laid out, is laid out the
// same with its last edge, an anchor, deleted.
func sameWithout(t *testing.T, name string, d *gw.Document, opts layout.Options) {
	t.Helper()
	pos := make([]gw.Point, d.VertexSlots())
	for i := range pos {
		pos[i] = d.Vertex(gw.VertexIndex(i)).Pos() //nolint:gosec // bounded by the capacity
	}
	deleteLastEdge(t, d)
	if _, err := layout.New(d.Caps()).Run(d, opts); err != nil {
		t.Fatal(err)
	}
	for i, p := range pos {
		if v := d.Vertex(gw.VertexIndex(i)); v.Live() && v.Pos() != p { //nolint:gosec // bounded by the capacity
			t.Errorf("%s: %s stands at %v without the anchor, %v with it", name, v.ID(), v.Pos(), p)
		}
	}
}
