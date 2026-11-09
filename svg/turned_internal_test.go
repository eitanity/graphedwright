package svg

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestTurnedAt: top to bottom, a label at a point on a straight run
// along the flow, the legs either side at its x taken in, is centred
// there, or moved along it as little as keeps it within the run; none
// is turned off a run, off its stretch along it, on a slanting leg, on a
// run too short, on a lane, or left to right.
func TestTurnedAt(t *testing.T) {
	// Down x 0 from 0 to 200 in two legs, a slant to (50, 250), and down
	// x 50 to 400.
	bent := way{f: frame{down: true}, a: gw.Point{}, b: gw.Point{X: 50, Y: 400}, n: 1,
		through: gw.PointsOf([]gw.Point{{X: 0, Y: 100}, {X: 0, Y: 200}, {X: 50, Y: 250}})}
	lane := way{f: frame{down: true}, a: gw.Point{}, b: gw.Point{Y: 400}, k: 1, n: 3,
		through: gw.PointsOf([]gw.Point{{Y: 200}})}
	across := bent
	across.f.down = false
	for _, c := range []struct {
		name string
		w    way
		mid  gw.Point
		need float64
		want float64
		ok   bool
	}{
		{"centred", bent, gw.Point{Y: 100}, 100, 100, true},
		{"the legs before", bent, gw.Point{Y: 150}, 180, 110, true},
		{"the legs after", bent, gw.Point{Y: 50}, 180, 90, true},
		{"kept from the end", bent, gw.Point{Y: 190}, 180, 110, true},
		{"too short", bent, gw.Point{X: 50, Y: 300}, 180, 0, false},
		{"off the run", bent, gw.Point{X: 10, Y: 150}, 30, 0, false},
		{"off its stretch", bent, gw.Point{Y: 300}, 30, 0, false},
		{"slanting", bent, gw.Point{X: 0.2, Y: 210}, 30, 0, false},
		{"a lane", lane, gw.Point{Y: 200}, 30, 0, false},
		{"left to right", across, gw.Point{Y: 100}, 30, 0, false},
	} {
		if got, ok := c.w.turnedAt(c.mid, c.need); ok != c.ok || got != c.want {
			t.Errorf("%s: %v %v, want %v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// turnedDoc is, top to bottom, a: an edge labelled "ab" from a, wide,
// down a straight run to b, its end the first of two at a's side, so
// that its texts go on the left, away from the other's; and its label
// placed near its top, so that it is moved down the run.
func turnedDoc(t *testing.T) (*gw.Document, gw.EdgeIndex) {
	t.Helper()
	k, err := gw.Compile(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 100, H: 20}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "flow", Route: gw.Curve, Stereotype: "st"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := gw.New(k, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(gw.TopToBottom)
	var vs [3]gw.VertexIndex
	for i, p := range [3]gw.Point{{}, {X: -10, Y: 400}, {X: 200, Y: 400}} {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex})
		if err != nil {
			t.Fatal(err)
		}
		vs[i] = ch.Vertices[0]
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: vs[i], Pos: p}); err != nil {
			t.Fatal(err)
		}
	}
	var es [2]gw.EdgeIndex
	for i, to := range [2]gw.VertexIndex{vs[1], vs[2]} {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[0], To: to, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		es[i] = ch.Edges[0]
	}
	if _, err := d.Apply(gw.Command{Op: gw.Relabel, Vertex: gw.None, Edge: es[0], Text: "ab"}); err != nil {
		t.Fatal(err)
	}
	if !d.SetBends(es[0], []gw.Point{{X: -10, Y: 100}, {X: -10, Y: 300}}) || !d.SetLabelAt(es[0], 0.02) {
		t.Fatal("no bends or label place")
	}
	var lane [1]gw.Point
	if w := wayOf(d, es[0], &lane); w.side() != -1 || w.a.X != -10 {
		t.Fatalf("the edge's texts on side %v, from %v", w.side(), w.a)
	}
	return d, es[0]
}

// TestTurnedBox: the turned label's box stands left of the run, as
// tall as a label is wide, moved down the run to hold the label; only
// its text's own length is hit; and it is typed in level about the same
// centre.
func TestTurnedBox(t *testing.T) {
	d, e := turnedDoc(t)
	// The run from a's side at 10 to b's at 390; the label 2·7+16 long,
	// its middle 15 down it.
	b, _, turned := edgeLabelBox(d, e)
	if want := (gw.Viewport{X: -32, Y: 25 - 40, W: labelHeight, H: 80}); !turned || b != want {
		t.Errorf("box %v turned %v, want %v", b, turned, want)
	}
	if got, want := EdgeLabelInputBox(d, e), (gw.Viewport{X: -62, Y: 15, W: 80, H: labelHeight}); got != want {
		t.Errorf("input box %v, want %v", got, want)
	}
	if other := gw.EdgeIndex(1); EdgeLabelInputBox(d, other) != EdgeLabelBox(d, other) {
		t.Errorf("a level label's input box is not its box")
	}
	if !inLabel(d, e, gw.Point{X: -20, Y: 25}) || inLabel(d, e, gw.Point{X: -20, Y: 25 + 30}) {
		t.Errorf("hit beyond the text, or not on it")
	}
}

var turnedRE = regexp.MustCompile(`<text class="(label|stereotype|annotation)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="middle" transform="rotate\(-90 ([-0-9.]+) ([-0-9.]+)\)">`)

// TestTurnedTexts: the turned texts are centred where the label's box
// is, turned about where they are drawn: the label on the left of the
// run, clear of the stroke by the gap and the font's descent, the
// stereotype a line further out, and the annotation on the right, clear
// by the gap and the font's ascent.
func TestTurnedTexts(t *testing.T) {
	d, e := turnedDoc(t)
	notes := make([]Annotation, d.EdgeSlots())
	notes[e] = Annotation{Text: "n"}
	var buf bytes.Buffer
	if err := Render(d, Options{EdgeNotes: notes}, &buf); err != nil {
		t.Fatal(err)
	}
	half := strokeWidth(d, Options{DefaultWeight: defaultStroke}, int(e), 0) / 2
	want := map[string]float64{
		"label":      -10 - half - 4 - defaultFontSize/4,
		"stereotype": -10 - half - 4 - defaultFontSize/4 - defaultFontSize,
		"annotation": -10 + half + 4 + defaultFontSize*3/4,
	}
	got := map[string]bool{}
	for _, m := range turnedRE.FindAllStringSubmatch(buf.String(), -1) {
		x, _ := strconv.ParseFloat(m[2], 64)
		y, _ := strconv.ParseFloat(m[3], 64)
		if x != want[m[1]] || y != 25 || m[4] != m[2] || m[5] != m[3] {
			t.Errorf("%s at %v, %v turned about %s %s, want at %v, 25", m[1], x, y, m[4], m[5], want[m[1]])
		}
		got[m[1]] = true
	}
	if len(got) != 3 {
		t.Errorf("turned %v in\n%s", got, buf.String())
	}
}

// TestDrawnBox: the text of a label takes the length of its runes and
// some room within its box: from the line's side beside a run top to
// bottom, on either side; about its middle above a run left to right;
// and about its middle along a turned one.
func TestDrawnBox(t *testing.T) {
	d, e := turnedDoc(t)
	b := gw.Viewport{X: 100, Y: 200, W: 80, H: 20}
	turned := gw.Viewport{X: 100, Y: 200, W: 20, H: 80}
	for _, c := range []struct {
		name   string
		b      gw.Viewport
		side   float64
		turned bool
		want   gw.Viewport
	}{
		{"right", b, 1, false, gw.Viewport{X: 100, Y: 200, W: 22, H: 20}},
		{"left", b, -1, false, gw.Viewport{X: 158, Y: 200, W: 22, H: 20}},
		{"turned", turned, -1, true, gw.Viewport{X: 100, Y: 229, W: 20, H: 22}},
	} {
		if got := drawnBox(d, e, c.b, c.side, c.turned); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	d.SetFlow(gw.LeftToRight)
	if got, want := drawnBox(d, e, b, 1, false), (gw.Viewport{X: 129, Y: 200, W: 22, H: 20}); got != want {
		t.Errorf("left to right: %v, want %v", got, want)
	}
}
