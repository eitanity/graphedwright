package svg

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// noted adds to an example a note anchored to the last vertex that is
// not a container, in that vertex's container, and, where there is one,
// a note anchored to the first container, in its container; and returns
// the notes.
func noted(t *testing.T, d *gw.Document) []gw.VertexIndex {
	t.Helper()
	last, first := gw.VertexIndex(gw.None), gw.VertexIndex(gw.None)
	for i := range d.VertexSlots() {
		v := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if !d.Vertex(v).Live() {
			continue
		}
		if !d.Kinds().Vertex(d.Vertex(v).Kind()).Container() {
			last = v
		} else if first == gw.None {
			first = v
		}
	}
	var notes []gw.VertexIndex
	for _, tv := range [2]gw.VertexIndex{last, first} {
		if tv == gw.None {
			continue
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: d.Kinds().VertexNamed("note"), Text: "a note\non two lines"})
		if err != nil {
			t.Fatal(err)
		}
		n := ch.Vertices[0]
		if p := d.Vertex(tv).Parent(); p != gw.None {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: n, To: p}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: d.Kinds().EdgeNamed("anchor"), From: n, To: tv, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
		notes = append(notes, n)
	}
	return notes
}

// TestNotedExamplesClear: the demo's examples, each with a note added
// beside a vertex and another beside a container, laid out afresh in
// either flow, keep their texts and edges apart as they do without
// them; no edge runs through a note; and no note overlaps a vertex but
// a container that holds it.
func TestNotedExamplesClear(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, name := range exampleNames {
			d := readExample(t, name)
			d.SetFlow(flow)
			notes := noted(t, d)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			PlaceLabels(d)
			label := name + " " + strconv.Itoa(int(flow))
			footLabelsClear(t, label, d)
			r := &reports{t: t}
			edgeTextsClear(r, label, d)
			knownReports(t, label+" noted", r.errs, knownTexts[label])
			edgesApart(t, label, d, append(knownAlong[label], knownNotedAlong[label]...)...)
			edgesOffSides(t, label, d)
			for _, n := range notes {
				notesClear(t, label, d, n)
			}
		}
	}
}

// knownNotedAlong are the pairs of edges that run along each other in
// the examples with notes added, past those without (knownAlong). Left
// to right, the note beside d widens d's layer, so the gap before it is
// longer, and the curve from b and the elbow from c, which come in level
// with each other to d's side, run along each other further across it.
// In the state machine, TextDone and TextCancel, from EditingText back
// to Idle in Edit mode, are routed below Edit mode's box, which holds
// what is in it and not the routes between them; the note beside Edit
// mode stands below it, in their way, and the detour round it takes
// both in lanes along the top.
var knownNotedAlong = map[string][][2]gw.ID{
	"diamond 0":       {{"e4", "e5"}, {"e5", "e4"}},
	"state-machine 0": {{"t14", "t15"}, {"t15", "t14"}},
	"state-machine 1": {{"t14", "t15"}, {"t15", "t14"}},
}

// notesClear checks that no edge but note n's own anchors runs through
// it, and that it overlaps no vertex but a container holding it.
func notesClear(t *testing.T, name string, d *gw.Document, n gw.VertexIndex) {
	t.Helper()
	box := vertexBox(d.Vertex(n))
	for i := range d.EdgeSlots() {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if x := d.Edge(e); !x.Live() || x.From() == n || x.To() == n {
			continue
		}
		for _, p := range drawnPoints(d, e) {
			if inBox2(box, p) {
				t.Errorf("%s: edge %s runs through the note at %v", name, d.Edge(e).ID(), p)
				break
			}
		}
	}
	for i := range d.VertexSlots() {
		v := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if v != n && d.Vertex(v).Live() && !d.Within(n, v) && boxesMeet(box, vertexBox(d.Vertex(v))) {
			t.Errorf("%s: the note overlaps vertex %s", name, d.Vertex(v).ID())
		}
	}
}

// TestNoteDrawn: a note's anchor is drawn as one straight line, its
// kind dashed and without an arrow, though its kind were curved; and the
// note's label of two lines is drawn as two, inside its folded box.
func TestNoteDrawn(t *testing.T) {
	d := readExample(t, "state-machine")
	notes := noted(t, d)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Render(d, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	n := d.Vertex(notes[0])
	at := strings.Index(out, `id="`+string(n.ID())+`"`)
	if at < 0 {
		t.Fatalf("no note %s in\n%s", n.ID(), out)
	}
	_, g, _ := strings.Cut(out[at:], "<text")
	g, _, _ = strings.Cut(g, "</text>")
	if strings.Count(g, "<tspan") != 2 || !strings.Contains(g, ">a note</tspan>") || !strings.Contains(g, ">on two lines</tspan>") {
		t.Errorf("the note's label is not drawn as two lines in\n%s", g)
	}
	for i := range d.EdgeSlots() {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if x := d.Edge(e); !x.Live() || x.From() != notes[0] {
			continue
		}
		_, g, _ := strings.Cut(out, `id="`+string(d.Edge(e).ID())+`"`)
		g, _, _ = strings.Cut(g, "</g>")
		_, path, _ := strings.Cut(g, `<path d="`)
		path, _, _ = strings.Cut(path, `"`)
		if strings.ContainsAny(path, "CQHV") || strings.Count(path, " L ") != 1 || strings.Contains(g, "marker-end") {
			t.Errorf("the anchor is drawn %q, want one straight line without an arrow", path)
		}
	}
}

// TestCurvedAnchorStraight: an anchor of a curved kind is drawn as one
// straight line between the boxes, with no lane or turn.
func TestCurvedAnchorStraight(t *testing.T) {
	d, err := doc.Decode([]byte(`{"schema": "basic", "vertices": [
		{"id": "a", "kind": "box", "x": 0, "y": 0}, {"id": "n", "kind": "note", "x": 300, "y": 120}],
		"edges": [{"id": "c", "kind": "curve", "from": "n", "to": "a"}]}`), doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Render(d, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	_, g, _ := strings.Cut(buf.String(), `id="c"`)
	_, path, _ := strings.Cut(g, `<path d="`)
	path, _, _ = strings.Cut(path, `"`)
	if strings.ContainsAny(path, "CQHV") || strings.Count(path, " L ") != 1 {
		t.Fatalf("the curved anchor is drawn %q, want one straight line", path)
	}
	// Its ends lie on the line between the centres, at (300, 120) and
	// (0, 0), where it leaves each box.
	var x0, y0, x1, y1 float64
	if _, err := fmt.Sscanf(path, "M %g %g L %g %g", &x0, &y0, &x1, &y1); err != nil {
		t.Fatal(err)
	}
	for _, p := range [2][2]float64{{x0, y0}, {x1, y1}} {
		if math.Abs(p[1]-p[0]*120/300) > 0.5 {
			t.Errorf("the anchor's end %v is off the line between the centres, in %q", p, path)
		}
	}
}

// TestNoteDroppedIn: a note dropped on a container goes in it, though
// the container's kind lists only the kinds of a state machine.
func TestNoteDroppedIn(t *testing.T) {
	d := readExample(t, "state-machine")
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: d.Kinds().VertexNamed("note")})
	if err != nil {
		t.Fatal(err)
	}
	at := d.Vertex(d.VertexByID("selecting")).Pos()
	at.Y += 30 // below Selecting, in Pointer gesture
	if got := ContainerAt(d, at, ch.Vertices[0]); got != d.VertexByID("gesture") {
		t.Errorf("a note dropped in Pointer gesture goes in %d", got)
	}
}
