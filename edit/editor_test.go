package edit_test

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

const (
	kSource gw.KindIndex = iota
	kNode
	kLeaf
	kGroup
)

const (
	eTo gw.KindIndex = iota
	eOutcome
)

// schema: sources (roots), nodes with an in port and an out port that
// takes two edges, a required number p and a choice mode, leaves; "to" from a source to a node, "outcome"
// from a node to a node or a leaf, with a text note; groups, which hold anything. Acyclic.
func schema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 96, H: 24}, Root: true},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36},
				Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right, Max: 2}},
				Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number, Required: true, Default: "0.5"}, {Key: "mode", Kind: gw.Choice, Choices: []string{"fast", "slow"}}}},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 100, H: 30}, Leaf: true},
			{Name: "group", Size: gw.Size{W: 100, H: 60}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "to", From: []gw.KindIndex{kSource}, To: []gw.KindIndex{kNode}, Arrow: true},
			{Name: "outcome", From: []gw.KindIndex{kNode}, To: []gw.KindIndex{kNode, kLeaf}, Route: gw.Curve, Attrs: []gw.AttrSpec{{Key: "note", Kind: gw.Text}}},
		},
		Acyclic: true,
	}
}

// Positions of the fixture: s at (60,100), a at (220,100), b at
// (420,100), c at (620,100); a's in port at (160,100), out at (280,100).
func fixture(t *testing.T, caps gw.Caps) (*gw.Document, *edit.Editor) {
	t.Helper()
	d, err := gw.New(schema(), caps)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		kind gw.KindIndex
		id   string
		x    float64
	}{{kSource, "s", 60}, {kNode, "a", 220}, {kNode, "b", 420}, {kLeaf, "c", 620}} {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: v.kind, ID: gw.ID(v.id), Text: v.id})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: gw.Point{X: v.x, Y: 100}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: eTo, From: 0, To: 1, FromPort: gw.None, ToPort: 0}); err != nil {
		t.Fatal(err)
	}
	d.View = gw.Viewport{X: 0, Y: 0, W: 800, H: 200}
	// The fixture's own commands are not part of any session's log.
	d.ClearUndo()
	return d, edit.New(d, edit.Edit, 8)
}

func pt(x, y float64) gw.Point { return gw.Point{X: x, Y: y} }

func down(x, y float64, m edit.Modifiers) edit.Event {
	return edit.Event{Kind: edit.PointerDown, Point: pt(x, y), Modifiers: m}
}

func move(x, y float64) edit.Event { return edit.Event{Kind: edit.PointerMove, Point: pt(x, y)} }

func up(x, y float64) edit.Event { return edit.Event{Kind: edit.PointerUp, Point: pt(x, y)} }

func key(k string, m edit.Modifiers) edit.Event {
	return edit.Event{Kind: edit.Key, Key: k, Modifiers: m}
}

// send feeds events that must all succeed.
func send(t *testing.T, e *edit.Editor, evs ...edit.Event) {
	t.Helper()
	for _, ev := range evs {
		if _, err := e.Handle(ev); err != nil {
			t.Fatalf("%+v: %v", ev, err)
		}
	}
}

func ids(d *gw.Document, vs []gw.VertexIndex) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(string(d.Vertices[v].ID))
	}
	return b.String()
}

func snapshot(d *gw.Document) string {
	var b strings.Builder
	for i := range d.Vertices {
		if v := &d.Vertices[i]; v.Live {
			b.WriteString(string(v.ID) + "@" + strconv.FormatFloat(v.Pos.X, 'g', -1, 64) + "," + strconv.FormatFloat(v.Pos.Y, 'g', -1, 64) + " " + strconv.Quote(v.Label) + " ")
		}
	}
	for i := range d.Edges {
		if x := &d.Edges[i]; x.Live {
			b.WriteString(string(x.ID) + ":" + strconv.Itoa(int(x.From)) + ">" + strconv.Itoa(int(x.To)) + " ")
		}
	}
	return b.String()
}

// TestSelect: click, shift-click, background, marquee and Tab.
func TestSelect(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, down(220, 100, 0), up(220, 100))
	if v, _ := e.Selected(); ids(d, v) != "a" {
		t.Errorf("click: %q", ids(d, v))
	}
	send(t, e, down(420, 100, edit.Shift), up(420, 100))
	if v, _ := e.Selected(); ids(d, v) != "ab" {
		t.Errorf("shift-click adds: %q", ids(d, v))
	}
	send(t, e, down(220, 100, edit.Shift), up(220, 100))
	if v, _ := e.Selected(); ids(d, v) != "b" {
		t.Errorf("shift-click removes: %q", ids(d, v))
	}
	send(t, e, down(400, 190, 0), up(400, 190))
	if v, _ := e.Selected(); len(v) != 0 {
		t.Errorf("background click keeps %q", ids(d, v))
	}
	send(t, e, down(10, 10, 0), move(300, 150), up(300, 150))
	if v, x := e.Selected(); ids(d, v) != "sa" || len(x) != 1 {
		t.Errorf("marquee: %q and %d edges", ids(d, v), len(x))
	}
	send(t, e, down(100, 160, 0), up(100, 160))
	if v, _ := e.Selected(); len(v) != 0 {
		t.Errorf("empty marquee keeps %q", ids(d, v))
	}
	send(t, e, key("Tab", 0))
	send(t, e, key("Tab", 0))
	if v, _ := e.Selected(); ids(d, v) != "a" {
		t.Errorf("tab twice: %q", ids(d, v))
	}
	send(t, e, down(130, 100, 0), up(130, 100))
	if _, x := e.Selected(); len(x) != 1 {
		t.Errorf("edge click: %d", len(x))
	}
	send(t, e, move(130, 100))
	if e.Hover().Kind != svg.EdgeHit {
		t.Errorf("hover %+v", e.Hover())
	}
}

// TestDrag: the selection follows the pointer, lands on the grid as
// placed, is one undo step per vertex, and goes back on Escape.
func TestDrag(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	before := snapshot(d)
	send(t, e, down(220, 100, 0), move(230, 110), move(243, 121))
	if e.State() != edit.Dragging || d.Vertices[1].Pos != pt(243, 121) {
		t.Errorf("mid-drag: state %d pos %v", e.State(), d.Vertices[1].Pos)
	}
	send(t, e, up(243, 121))
	if d.Vertices[1].Pos != pt(240, 120) || !d.Vertices[1].Placed || e.State() != edit.Idle {
		t.Errorf("after drag: %v placed %v state %d", d.Vertices[1].Pos, d.Vertices[1].Placed, e.State())
	}
	if _, ok := d.Undo(); !ok || snapshot(d) != before {
		t.Errorf("undo of the drag:\n%s\n%s", before, snapshot(d))
	}
	d.Redo()
	send(t, e, down(420, 100, edit.Shift), up(420, 100))
	send(t, e, down(240, 120, 0), move(260, 140), key("Escape", 0))
	if d.Vertices[1].Pos != pt(240, 120) || d.Vertices[2].Pos != pt(420, 100) || e.State() != edit.Idle {
		t.Errorf("escape: %v %v", d.Vertices[1].Pos, d.Vertices[2].Pos)
	}
	send(t, e, down(240, 120, 0), up(240, 120))
	if d.CanRedo() {
		t.Error("a click logged a move")
	}
}

// TestConnect: a drag from a port to a port adds an edge of the first
// kind that allows it; a refusal names the rule and adds nothing; a
// release elsewhere drops the edge.
func TestConnect(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, down(280, 100, 0), move(300, 120))
	if from, to, ok := e.Ghost(); !ok || from != pt(280, 100) || to != pt(300, 120) {
		t.Errorf("ghost %v %v %v", from, to, ok)
	}
	send(t, e, up(360, 100))
	if d.LiveEdges() != 2 || d.Edges[1].Kind != eOutcome || d.Edges[1].From != 1 || d.Edges[1].To != 2 || d.Edges[1].ToPort != 0 {
		t.Errorf("connect: %d edges, %+v", d.LiveEdges(), d.Edges[1])
	}
	_, err := e.Handle(down(480, 100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(160, 100)); err == nil || !strings.Contains(err.Error(), "edge would make a cycle: b → a → b") {
		t.Errorf("cycle: %v", err)
	}
	send(t, e, down(480, 100, 0), up(620, 100))
	if d.LiveEdges() != 3 || d.Edges[2].ToPort != gw.None {
		t.Errorf("into a leaf without ports: %d edges %+v", d.LiveEdges(), d.Edges[2])
	}
	send(t, e, down(480, 100, 0), up(700, 190))
	if d.LiveEdges() != 3 || e.State() != edit.Idle {
		t.Errorf("drop on background: %d", d.LiveEdges())
	}
	if _, err := e.Handle(down(160, 100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(60, 100)); !errors.Is(err, edit.ErrNoEdgeKind) {
		t.Errorf("into a root, which no kind joins: %v", err)
	}
	send(t, e, down(620, 100, 0), up(620, 100))
	if v, _ := e.Selected(); ids(d, v) != "c" {
		t.Errorf("a vertex without ports selects: %q", ids(d, v))
	}
}

// TestConnectFromOutline: on a vertex without ports, a drag from the
// band along its outline starts an edge and a drag from inside moves
// it; a click on the band selects; the overlay rings the vertex where
// an edge can start, and while connecting only a vertex the edge can
// join; in view mode the band only selects.
func TestConnectFromOutline(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	// s is a 96×24 pill at (60,100): its outline is at x 108.
	send(t, e, move(110, 100))
	if v, ok := e.Target(); !ok || v != 0 {
		t.Errorf("hover on the outline: target %d %v", v, ok)
	}
	send(t, e, move(70, 100))
	if _, ok := e.Target(); ok {
		t.Error("hover inside: a target")
	}
	send(t, e, down(105, 100, 0), move(300, 100))
	if from, _, ok := e.Ghost(); !ok || from != pt(105, 100) {
		t.Errorf("ghost from %v %v", from, ok)
	}
	if _, ok := e.Target(); ok {
		t.Error("over the background: a target")
	}
	send(t, e, move(360, 100))
	if v, ok := e.Target(); !ok || v != 2 {
		t.Errorf("over b's in port: target %d %v", v, ok)
	}
	send(t, e, move(570, 100))
	if _, ok := e.Target(); ok {
		t.Error("over the leaf, which no edge from a source joins: a target")
	}
	send(t, e, up(360, 100))
	if d.LiveEdges() != 2 || d.Edges[1].Kind != eTo || d.Edges[1].From != 0 || d.Edges[1].FromPort != gw.None || d.Edges[1].To != 2 || d.Edges[1].ToPort != 0 {
		t.Errorf("from the outline: %d edges, %+v", d.LiveEdges(), d.Edges[1])
	}
	// From a port onto the leaf's outline.
	send(t, e, down(280, 100, 0), up(572, 100))
	if d.LiveEdges() != 3 || d.Edges[2].To != 3 || d.Edges[2].ToPort != gw.None {
		t.Errorf("onto the outline: %d edges, %+v", d.LiveEdges(), d.Edges[2])
	}
	if _, err := e.Handle(down(105, 100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(620, 100)); !errors.Is(err, edit.ErrNoEdgeKind) {
		t.Errorf("source to leaf: %v", err)
	}
	send(t, e, down(105, 100, 0), up(106, 101))
	if v, _ := e.Selected(); ids(d, v) != "s" || d.LiveEdges() != 3 || e.State() != edit.Idle {
		t.Errorf("click on the outline: selected %q, %d edges", ids(d, v), d.LiveEdges())
	}
	send(t, e, down(60, 100, 0), move(60, 140), up(60, 140))
	if d.Vertices[0].Pos.Y == 100 {
		t.Error("a drag from inside did not move the vertex")
	}
	e.SetMode(edit.View)
	send(t, e, down(570, 100, 0), up(570, 100))
	if v, _ := e.Selected(); ids(d, v) != "c" || e.State() != edit.Idle {
		t.Errorf("view mode on the outline: selected %q, state %d", ids(d, v), e.State())
	}
	if _, ok := e.Target(); ok {
		t.Error("view mode: a target")
	}
}

// TestContainers: a vertex dragged into a group's box goes in, and
// dragged out comes out, judged by the group as it stood during the
// drag; dragging the group carries what it holds, and one undo puts it
// all back; a palette drop in the group goes in; deleting the group
// deletes what it holds.
func TestContainers(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	// A group around the leaf c at (620,100): its box is x 554 to 686,
	// y 49 to 131.
	z := d.VertexByID("c")
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kGroup, ID: "z", Text: "z"})
	if err != nil {
		t.Fatal(err)
	}
	g := ch.Vertices[0]
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: z, To: g}); err != nil {
		t.Fatal(err)
	}
	if v := d.Vertices[g]; v.Pos != pt(620, 90) || v.Size != (gw.Size{W: 132, H: 82}) {
		t.Fatalf("group %v %v", v.Pos, v.Size)
	}
	b := d.VertexByID("b")
	// b drags from (420,100) to (600,56): into the group, near its top,
	// which grows to take it only once it is in.
	send(t, e, down(420, 100, 0), move(500, 80), move(600, 56), up(600, 56))
	if d.Vertices[b].Parent != g || d.Vertices[b].Pos != pt(600, 56) || d.Vertices[g].Size.H <= 82 {
		t.Errorf("into the group: parent %d at %v, group %v", d.Vertices[b].Parent, d.Vertices[b].Pos, d.Vertices[g].Size)
	}
	send(t, e, down(600, 56, 0), move(420, 200), up(420, 200))
	if d.Vertices[b].Parent != gw.None || d.Vertices[g].Size != (gw.Size{W: 132, H: 82}) {
		t.Errorf("out of the group: parent %d, group %v", d.Vertices[b].Parent, d.Vertices[g].Size)
	}
	// The group by its empty corner, carrying c.
	before := snapshot(d)
	send(t, e, down(560, 55, 0), move(620, 95), move(660, 95), up(660, 95))
	// The group snaps to the grid and c moves with it, by the same offset.
	if d.Vertices[z].Pos != pt(720, 138) || d.Vertices[g].Pos != pt(720, 128) || d.Vertices[z].Parent != g {
		t.Errorf("dragging the group: c %v, group %v", d.Vertices[z].Pos, d.Vertices[g].Pos)
	}
	send(t, e, key("z", edit.Ctrl))
	if snapshot(d) != before {
		t.Errorf("undo of the group's drag:\n%s\nwant\n%s", snapshot(d), before)
	}
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: kNode}, up(600, 60))
	if v := d.VertexByID("v1"); v == gw.None || d.Vertices[v].Parent != g {
		t.Errorf("palette drop in the group: %d", v)
	}
	// The group has grown up and left round v1; (530,120) is in it,
	// clear of c and v1.
	send(t, e, down(530, 120, 0), up(530, 120), key("Delete", 0))
	if d.LiveVertices() != 3 || d.VertexByID("c") != gw.None {
		t.Errorf("after deleting the group: %d vertices", d.LiveVertices())
	}
}

// TestPalette: a vertex kind drops at the snapped point, placed and
// selected; an edge kind is chosen by a press on it, without a drop,
// every edge drawn after takes it, from a port or an outline alike, and
// a second press unchooses it.
func TestPalette(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: kNode}, move(300, 30), up(301, 29))
	v := d.VertexByID("v1")
	if v == gw.None || d.Vertices[v].Pos != pt(304, 32) || !d.Vertices[v].Placed {
		t.Fatalf("placed vertex %d %+v", v, d.Vertices)
	}
	if sel, _ := e.Selected(); ids(d, sel) != "v1" {
		t.Errorf("selected %q", ids(d, sel))
	}
	if e.EdgeKind() != gw.None {
		t.Fatalf("an edge kind chosen before any: %d", e.EdgeKind())
	}
	// A press on the kind and its release anywhere, as a click on the
	// palette button gives.
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: eOutcome, IsEdge: true}, up(700, 190))
	if e.State() != edit.Idle || e.EdgeKind() != eOutcome || d.LiveEdges() != 1 {
		t.Fatalf("choosing outcome: state %d kind %d edges %d", e.State(), e.EdgeKind(), d.LiveEdges())
	}
	send(t, e, down(280, 100, 0), up(244, 32))
	if d.LiveEdges() != 2 || d.Edges[1].Kind != eOutcome || d.Edges[1].To != v {
		t.Errorf("edge from a port with outcome chosen: %+v", d.Edges[1])
	}
	send(t, e, down(280, 100, 0), up(360, 100))
	if d.LiveEdges() != 3 || d.Edges[2].Kind != eOutcome {
		t.Errorf("a second edge with outcome chosen: %+v", d.Edges[2])
	}
	// From the outline of a vertex without ports, the chosen kind too.
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: eTo, IsEdge: true}, down(108, 100, 0), up(360, 100))
	if d.LiveEdges() != 4 || d.Edges[3].Kind != eTo || d.Edges[3].From != 0 || d.Edges[3].To != 2 {
		t.Errorf("edge from an outline with to chosen: %d edges %+v", d.LiveEdges(), d.Edges[3])
	}
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: eTo, IsEdge: true})
	if e.EdgeKind() != gw.None {
		t.Errorf("a second press left %d chosen", e.EdgeKind())
	}
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: 99, IsEdge: true})
	if e.EdgeKind() != gw.None {
		t.Errorf("a kind out of range was chosen: %d", e.EdgeKind())
	}
	// A drop at the edge of the view keeps the vertex inside it.
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: kLeaf}, up(795, 3))
	if last := d.Vertices[len(d.Vertices)-1]; last.Pos != pt(750, 15) {
		t.Errorf("drop at the edge: %v", last.Pos)
	}
}

// TestGrid: a grid that is not a positive size, NaN and infinity
// included, is the default of 8; one that is moves by itself.
func TestGrid(t *testing.T) {
	for _, tc := range []struct{ grid, want float64 }{{math.NaN(), 228}, {math.Inf(1), 228}, {-4, 228}, {2e9, 228}, {20, 240}} {
		d, _ := fixture(t, gw.Caps{})
		e := edit.New(d, edit.Edit, tc.grid)
		send(t, e, down(220, 100, 0), up(220, 100), key("ArrowRight", 0))
		if d.Vertices[1].Pos.X != tc.want {
			t.Errorf("grid %v: nudged to %v, want %v", tc.grid, d.Vertices[1].Pos.X, tc.want)
		}
	}
}

// TestKeys: Delete, the arrows, undo and redo, Enter with the text,
// and a label over the limit refused.
func TestKeys(t *testing.T) {
	d, e := fixture(t, gw.Caps{LabelBytes: 8})
	send(t, e, down(220, 100, 0), up(220, 100), key("ArrowRight", 0), key("ArrowDown", 0))
	if d.Vertices[1].Pos != pt(228, 108) {
		t.Errorf("nudge: %v", d.Vertices[1].Pos)
	}
	send(t, e, key("z", edit.Ctrl), key("z", edit.Ctrl))
	if d.Vertices[1].Pos != pt(220, 100) {
		t.Errorf("undo: %v", d.Vertices[1].Pos)
	}
	send(t, e, key("y", edit.Ctrl), key("Z", edit.Ctrl|edit.Shift))
	if d.Vertices[1].Pos != pt(228, 108) {
		t.Errorf("redo: %v", d.Vertices[1].Pos)
	}
	send(t, e, key("Enter", 0))
	if v, _ := e.Editing(); e.State() != edit.EditingText || v != 1 {
		t.Fatalf("editing: state %d vertex %d", e.State(), v)
	}
	if _, err := e.Handle(edit.Event{Kind: edit.TextDone, Text: "far too long"}); err == nil || !strings.Contains(err.Error(), "label of 12 bytes") {
		t.Errorf("long label: %v", err)
	}
	send(t, e, key("Enter", 0), edit.Event{Kind: edit.TextDone, Text: "A"})
	if d.Vertices[1].Label != "A" || e.State() != edit.Idle {
		t.Errorf("relabel: %q", d.Vertices[1].Label)
	}
	send(t, e, key("Enter", 0), edit.Event{Kind: edit.TextCancel})
	if d.Vertices[1].Label != "A" {
		t.Error("cancel relabelled")
	}
	send(t, e, down(130, 105, 0), up(130, 105), key("Enter", 0), edit.Event{Kind: edit.TextDone, Text: "e"})
	if d.Edges[0].Label != "e" {
		t.Errorf("edge relabel: %q", d.Edges[0].Label)
	}
	send(t, e, down(10, 10, 0), move(500, 190), up(500, 190), key("Delete", 0))
	if d.LiveVertices() != 1 || d.LiveEdges() != 0 {
		t.Errorf("delete: %d vertices %d edges", d.LiveVertices(), d.LiveEdges())
	}
	if v, x := e.Selected(); len(v)+len(x) != 0 {
		t.Error("selection survives delete")
	}
	send(t, e, key("z", edit.Ctrl))
	if d.LiveVertices() != 4 || d.LiveEdges() != 1 {
		t.Errorf("one undo of a delete of three vertices and an edge: %d vertices %d edges, want 4 and 1", d.LiveVertices(), d.LiveEdges())
	}
	if _, err := e.Handle(edit.Event{Kind: edit.TextDone, Text: "x"}); err != nil {
		t.Errorf("text outside editing: %v", err)
	}
}

// TestViewMode: selection, hover, pan and zoom work; every command is
// refused with ErrView and nothing changes.
func TestViewMode(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	e.SetMode(edit.View)
	before := snapshot(d)
	send(t, e, down(220, 100, 0), move(300, 150), up(300, 150))
	if v, _ := e.Selected(); ids(d, v) != "a" || snapshot(d) != before {
		t.Errorf("view drag: %q\n%s", ids(d, v), snapshot(d))
	}
	send(t, e, down(280, 100, 0), up(360, 100))
	if d.LiveEdges() != 1 {
		t.Error("view mode connected")
	}
	for _, ev := range []edit.Event{key("Delete", 0), key("ArrowLeft", 0), {Kind: edit.PaletteStart, Item: kNode}} {
		if _, err := e.Handle(ev); !errors.Is(err, edit.ErrView) {
			t.Errorf("%+v: %v", ev, err)
		}
	}
	send(t, e, edit.Event{Kind: edit.PointerDown, Point: pt(100, 100), Button: 1}, move(110, 120), up(110, 120))
	if d.View.X != -10 || d.View.Y != -20 {
		t.Errorf("pan: %+v", d.View)
	}
	send(t, e, edit.Event{Kind: edit.Wheel, Point: pt(0, 0), Delta: 100, Modifiers: edit.Ctrl})
	if d.View.W < 879 || d.View.W > 881 {
		t.Errorf("zoom: %+v", d.View)
	}
	send(t, e, edit.Event{Kind: edit.Wheel, Delta: 30, DeltaX: -5})
	if d.View.X != -16 || d.View.Y != 8 {
		t.Errorf("wheel pan: %+v", d.View)
	}
	if snapshot(d) != before {
		t.Error("view mode changed the document")
	}
	e.SetMode(edit.Edit)
	send(t, e, edit.Event{Kind: edit.Resize, Point: pt(400, 100)})
	// The view is {-16 8 880 220} in a 400 by 100 element: a's centre
	// (220, 100) is at client (107, 42).
	send(t, e, down(107, 42, 0), up(107, 42))
	if v, _ := e.Selected(); ids(d, v) != "a" {
		t.Errorf("client coordinates through the viewport: %q", ids(d, v))
	}
}

// TestRefusalsThroughTheEditor: the capacities, the port limit and the
// schema's rules reach the host as the document's own errors.
func TestRefusalsThroughTheEditor(t *testing.T) {
	d, e := fixture(t, gw.Caps{Vertices: 5, Edges: 3})
	send(t, e, edit.Event{Kind: edit.PaletteStart, Item: kLeaf}, up(300, 30))
	if _, err := e.Handle(edit.Event{Kind: edit.PaletteStart, Item: kLeaf}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(300, 60)); err == nil || !strings.Contains(err.Error(), "vertex limit 5 reached") {
		t.Errorf("vertex limit: %v", err)
	}
	send(t, e, down(280, 100, 0), up(620, 100), down(280, 100, 0), up(300, 30))
	if _, err := e.Handle(down(280, 100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(360, 100)); err == nil || !strings.Contains(err.Error(), `port "out" of vertex "a" is full (2 of 2)`) {
		t.Errorf("port full: %v", err)
	}
	if _, err := e.Handle(down(480, 100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(620, 100)); err == nil || !strings.Contains(err.Error(), "edge limit 3 reached") {
		t.Errorf("edge limit: %v", err)
	}
	if _, err := e.Handle(down(300, 30, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(edit.Event{Kind: edit.PaletteStart, Item: eTo, IsEdge: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(down(280, 100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Handle(up(360, 100)); err == nil || !strings.Contains(err.Error(), `edge kind "to" cannot join "node" to "node"`) {
		t.Errorf("kind not allowed: %v", err)
	}
	if d.LiveEdges() != 3 {
		t.Errorf("refusals changed the document: %d edges", d.LiveEdges())
	}
}

// TestReplay: a recorded session replays to the same document from a
// fresh start, and undoing everything returns to the start.
func TestReplay(t *testing.T) {
	session := []edit.Event{
		{Kind: edit.PaletteStart, Item: kNode}, up(300, 30),
		down(280, 100, 0), up(244, 32),
		down(304, 32, 0), move(330, 60), up(330, 60),
		key("ArrowLeft", 0), key("Enter", 0), {Kind: edit.TextDone, Text: "new"},
		down(420, 100, 0), up(420, 100), key("Delete", 0),
	}
	var out [2]string
	var docs [2]*gw.Document
	for i := range out {
		d, e := fixture(t, gw.Caps{})
		send(t, e, session...)
		out[i], docs[i] = snapshot(d), d
	}
	if out[0] != out[1] {
		t.Errorf("replay differs:\n%s\n%s", out[0], out[1])
	}
	start := snapshot(func() *gw.Document { d, _ := fixture(t, gw.Caps{}); return d }())
	for docs[0].CanUndo() {
		docs[0].Undo()
	}
	if snapshot(docs[0]) != start {
		t.Errorf("undo all:\n%s\n%s", start, snapshot(docs[0]))
	}
}

// TestAllocMove: a pointer move allocates nothing, hovering or
// dragging; a connection's ghost neither.
func TestAllocMove(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	if n := testing.AllocsPerRun(50, func() { _, _ = e.Handle(move(300, 120)) }); n > 0 {
		t.Errorf("hover: %g allocations", n)
	}
	send(t, e, down(220, 100, 0))
	x := 221.0
	if n := testing.AllocsPerRun(50, func() { x++; _, _ = e.Handle(move(x, 100)) }); n > 0 {
		t.Errorf("drag: %g allocations", n)
	}
	send(t, e, key("Escape", 0), down(280, 100, 0))
	if n := testing.AllocsPerRun(50, func() { _, _ = e.Handle(move(300, 120)) }); n > 0 {
		t.Errorf("connecting: %g allocations", n)
	}
	send(t, e, key("Escape", 0))
	_ = d
}

// TestOverlay: the editor fills the renderer's options, and the ghost
// and the marquee appear in the drawing.
func TestOverlay(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	send(t, e, down(280, 100, 0), move(300, 120))
	var opts svg.Options
	e.Overlay(&opts)
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<path class="ghost" d="M 280 100 L 300 120"/>`) || opts.Mode != svg.Edit {
		t.Errorf("ghost missing:\n%s", buf.String())
	}
	send(t, e, key("Escape", 0), down(10, 10, 0), move(50, 40))
	e.Overlay(&opts)
	buf.Reset()
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<rect class="marquee" x="10" y="10" width="40" height="30"/>`) {
		t.Errorf("marquee missing:\n%s", buf.String())
	}
	send(t, e, edit.Event{Kind: edit.Blur})
	if e.State() != edit.Idle {
		t.Errorf("blur: state %d", e.State())
	}
	if _, _, ok := e.Ghost(); ok {
		t.Error("ghost after blur")
	}
	if e.Document() != d || e.Mode() != edit.Edit {
		t.Error("accessors")
	}
}

// TestResize: the one selected vertex has a handle on its bottom right
// corner; dragging it resizes the vertex to whole grid units, keeping
// its top left corner, as a Resize and a Move; a container that holds
// something is resized alone, its corner kept by the fit, and is its
// own size from then; the handle is not there in view mode or with two
// selected, and a click on it changes nothing.
func TestResize(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	a := d.VertexByID("a")
	send(t, e, down(220, 100, 0), up(220, 100))
	if v, ok := e.ResizeHandle(); !ok || v != a {
		t.Fatalf("handle: %d %v", v, ok)
	}
	var opts svg.Options
	e.Overlay(&opts)
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	// a is 120 by 36 at (220,100): its corner is (280,118).
	if !strings.Contains(buf.String(), `<rect class="handle" x="276" y="114" width="8" height="8"/>`) {
		t.Errorf("handle not drawn:\n%s", buf.String())
	}
	before := snapshot(d)
	send(t, e, down(281, 119, 0), up(281, 119))
	if snapshot(d) != before || e.State() != edit.Idle {
		t.Errorf("a click on the handle changed the document")
	}
	// From the corner (160,82) to (330,152): 170 by 70, to the grid 168
	// by 72.
	send(t, e, down(280, 118, 0), move(300, 130), move(330, 152))
	if e.State() != edit.Resizing {
		t.Fatalf("state %d", e.State())
	}
	if box, ok := e.Marquee(); !ok || box != (gw.Viewport{X: 160, Y: 82, W: 168, H: 72}) {
		t.Errorf("resize box %v %v", box, ok)
	}
	send(t, e, up(330, 152))
	if v := d.Vertices[a]; v.Size != (gw.Size{W: 168, H: 72}) || v.Pos != pt(244, 118) {
		t.Errorf("resized a: %v %v", v.Pos, v.Size)
	}
	send(t, e, key("z", edit.Ctrl), key("z", edit.Ctrl))
	if snapshot(d) != before {
		t.Errorf("undo of the resize:\n%s\nwant\n%s", snapshot(d), before)
	}
	// Escape drops a resize under way.
	send(t, e, down(280, 118, 0), move(400, 200), key("Escape", 0), up(400, 200))
	if snapshot(d) != before {
		t.Errorf("resize after Escape changed the document")
	}
	// A group around c, fitted to (620,90) 132 by 82: its corner is
	// (554,49), its handle (686,131).
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kGroup, ID: "z", Text: "z"})
	if err != nil {
		t.Fatal(err)
	}
	g, c := ch.Vertices[0], d.VertexByID("c")
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: c, To: g}); err != nil {
		t.Fatal(err)
	}
	send(t, e, down(560, 55, 0), up(560, 55), down(686, 131, 0), move(800, 200), up(800, 200))
	if v := d.Vertices[g]; v.Size != (gw.Size{W: 248, H: 152}) || v.Pos != pt(678, 125) || d.Vertices[c].Pos != pt(620, 100) {
		t.Errorf("resized group: %v %v, c %v", v.Pos, v.Size, d.Vertices[c].Pos)
	}
	// Taking c out leaves the group its own size, corner kept.
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: c, To: gw.None}); err != nil {
		t.Fatal(err)
	}
	if v := d.Vertices[g]; v.Size != (gw.Size{W: 248, H: 152}) || v.Pos != pt(678, 125) {
		t.Errorf("emptied group: %v %v", v.Pos, v.Size)
	}
	send(t, e, down(220, 100, edit.Shift), up(220, 100))
	if _, ok := e.ResizeHandle(); ok {
		t.Error("handle with two selected")
	}
	send(t, e, down(220, 100, 0), up(220, 100))
	e.SetMode(edit.View)
	if _, ok := e.ResizeHandle(); ok {
		t.Error("handle in view mode")
	}
}

// TestZoom: Ctrl with = or + zooms in about the view's centre, with -
// out, and with 0 fits the drawing, in either mode; Zoom and Fit do the
// same for a host's buttons; a zoom past the bounds is ignored.
func TestZoom(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	e.SetMode(edit.View)
	send(t, e, key("=", edit.Ctrl))
	if d.View != (gw.Viewport{X: 80, Y: 20, W: 640, H: 160}) {
		t.Errorf("zoom in: %+v", d.View)
	}
	send(t, e, key("-", edit.Ctrl))
	if d.View != (gw.Viewport{X: 0, Y: 0, W: 800, H: 200}) {
		t.Errorf("zoom out: %+v", d.View)
	}
	send(t, e, key("0", edit.Ctrl))
	if d.View != svg.Bounds(d) {
		t.Errorf("fit: %+v, bounds %+v", d.View, svg.Bounds(d))
	}
	e.SetMode(edit.Edit)
	d.View = gw.Viewport{W: 800, H: 200}
	if ch := e.Zoom(2); !ch.Layout || d.View != (gw.Viewport{X: 200, Y: 50, W: 400, H: 100}) {
		t.Errorf("Zoom(2): %+v %+v", ch, d.View)
	}
	if ch := e.Zoom(1000); ch.Layout || d.View.W != 400 {
		t.Errorf("Zoom past the least view: %+v %+v", ch, d.View)
	}
	if ch := e.Zoom(0); ch.Layout {
		t.Errorf("Zoom(0): %+v", ch)
	}
	if ch := e.Fit(); !ch.Layout || d.View != svg.Bounds(d) {
		t.Errorf("Fit: %+v %+v", ch, d.View)
	}
	before := snapshot(d)
	send(t, e, key("+", edit.Ctrl), key("_", edit.Ctrl))
	if snapshot(d) != before || e.State() != edit.Idle {
		t.Error("zoom keys changed the document or began editing")
	}
}

// TestGestureKeepsItsOwn: during a drag, keys other than Escape and a
// second press do nothing, so the drag's release or cancel still finds
// the document as it left it; a release does not end label editing.
// merilo found each of these leaving a change outside the undo log
// (internal/verify).
func TestGestureKeepsItsOwn(t *testing.T) {
	for _, k := range []edit.Event{key("Delete", 0), key("ArrowLeft", 0), key("z", edit.Ctrl), key("Enter", 0), key("Tab", 0), down(300, 160, 0), {Kind: edit.PointerDown, Point: pt(260, 140), Button: 1}} {
		d, e := fixture(t, gw.Caps{})
		start := snapshot(d)
		send(t, e, down(220, 100, 0), move(260, 140), k)
		if e.State() != edit.Dragging {
			t.Errorf("%+v: state %d during the drag, want Dragging", k, e.State())
		}
		send(t, e, up(260, 140))
		for d.CanUndo() {
			d.Undo()
		}
		if got := snapshot(d); got != start {
			t.Errorf("%+v: undo all after the drag:\n%s\nwant\n%s", k, got, start)
		}
	}
	_, e := fixture(t, gw.Caps{})
	send(t, e, down(220, 100, 0), up(220, 100), key("Enter", 0), up(220, 100))
	if e.State() != edit.EditingText {
		t.Errorf("a release ended label editing: state %d", e.State())
	}
}

// TestNudgeUndoesInOneStep: an arrow key with two vertices selected moves
// both, and one Ctrl-Z puts both back.
func TestNudgeUndoesInOneStep(t *testing.T) {
	d, e := fixture(t, gw.Caps{})
	start := []gw.Point{d.Vertices[1].Pos, d.Vertices[2].Pos}
	send(t, e, down(220, 100, 0), up(220, 100), down(420, 100, edit.Shift), up(420, 100), key("ArrowLeft", 0))
	if d.Vertices[1].Pos == start[0] || d.Vertices[2].Pos == start[1] {
		t.Fatalf("the nudge moved a to %v and b to %v", d.Vertices[1].Pos, d.Vertices[2].Pos)
	}
	send(t, e, key("z", edit.Ctrl))
	if d.Vertices[1].Pos != start[0] || d.Vertices[2].Pos != start[1] {
		t.Errorf("one undo left a at %v and b at %v, want %v", d.Vertices[1].Pos, d.Vertices[2].Pos, start)
	}
	if d.CanUndo() {
		t.Error("a second step is left to undo")
	}
}
