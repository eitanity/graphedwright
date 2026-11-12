package svg

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// labelRE finds a label written as one line, "ab", or as two, "ab" and
// "cd", and where.
var labelRE = regexp.MustCompile(`<text class="label" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="[a-z]+"[^>]*>(ab|<tspan x="([-0-9.]+)" y="([-0-9.]+)">ab</tspan><tspan x="([-0-9.]+)" y="([-0-9.]+)">cd</tspan>)</text>`)

// drawnAt is where the label "ab", or "ab" and "cd", is drawn in the
// output: the text's point, and each line's.
func drawnAt(t *testing.T, out string) (at gw.Point, lines []gw.Point) {
	t.Helper()
	m := labelRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no label in %s", out)
	}
	num := func(s string) float64 {
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	at = gw.Point{X: num(m[1]), Y: num(m[2])}
	if m[4] != "" {
		lines = []gw.Point{{X: num(m[4]), Y: num(m[5])}, {X: num(m[6]), Y: num(m[7])}}
	}
	return at, lines
}

// TestLabelLinesDrawn: a label of two lines is drawn a line to a tspan,
// gw.LabelLine apart, where its one line would be: a vertex's centred
// on it, a container's title down from it, an edge's above its stroke
// up from it, beside it centred on it, and turned beside it out from
// it, and under a provided shape's box down from it; and its box is a
// line taller, or turned wider, the same way.
func TestLabelLinesDrawn(t *testing.T) {
	const L = gw.LabelLine
	type drawn struct {
		at    gw.Point
		lines []gw.Point
		box   gw.Viewport
	}
	for _, tc := range []struct {
		name string
		draw func(text string) drawn
		// want is where the two lines and the box are, from the one
		// line's point and box.
		want func(one drawn) drawn
	}{
		{"vertex", func(text string) drawn {
			d, _, _ := zoneDoc(t, gw.Point{})
			m := gw.VertexIndex(1)
			relabel(t, d, m, gw.None, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			return drawn{at, lines, VertexLabelBox(d, m)}
		}, func(one drawn) drawn {
			b := one.box
			b.Y, b.H = b.Y-L/2, b.H+L
			return drawn{gw.Point{X: one.at.X, Y: one.at.Y - L/2}, []gw.Point{{X: one.at.X, Y: one.at.Y - L/2}, {X: one.at.X, Y: one.at.Y + L/2}}, b}
		}},
		{"container title", func(text string) drawn {
			d, z, _ := zoneDoc(t, gw.Point{})
			relabel(t, d, z, gw.None, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			// From the container's top, which its taller box moves up.
			top := d.Vertex(z).Pos().Y - d.Vertex(z).Size().H/2
			at.Y += d.Vertex(z).Pos().Y - top
			for i := range lines {
				lines[i].Y += d.Vertex(z).Pos().Y - top
			}
			b := titleBox(d, d.Vertex(z))
			b.Y -= top
			return drawn{at, lines, b}
		}, func(one drawn) drawn {
			b := one.box
			b.H += L
			return drawn{one.at, []gw.Point{one.at, {X: one.at.X, Y: one.at.Y + L}}, b}
		}},
		{"under a provided shape", func(text string) drawn {
			below := hexShape()
			below.Name, below.Label = "below", gw.LabelBelow
			d := shapedDoc(t, []gw.ShapeDef{below}, nil, gw.Point{X: 200, Y: 100})
			relabel(t, d, 0, gw.None, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			return drawn{at, lines, VertexLabelBox(d, 0)}
		}, func(one drawn) drawn {
			b := one.box
			b.H += L
			return drawn{one.at, []gw.Point{one.at, {X: one.at.X, Y: one.at.Y + L}}, b}
		}},
		{"edge above", func(text string) drawn {
			d, _, e := zoneDoc(t, gw.Point{})
			relabel(t, d, gw.None, e, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			return drawn{at, lines, EdgeLabelBox(d, e)}
		}, func(one drawn) drawn {
			b := one.box
			b.Y, b.H = b.Y-L, b.H+L
			return drawn{gw.Point{X: one.at.X, Y: one.at.Y - L}, []gw.Point{{X: one.at.X, Y: one.at.Y - L}, one.at}, b}
		}},
		{"edge beside", func(text string) drawn {
			d, _, e := zoneDoc(t, gw.Point{X: 300, Y: -200})
			d.SetFlow(gw.TopToBottom)
			relabel(t, d, gw.None, e, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			return drawn{at, lines, EdgeLabelBox(d, e)}
		}, func(one drawn) drawn {
			b := one.box
			b.Y, b.H = b.Y-L/2, b.H+L
			return drawn{gw.Point{X: one.at.X, Y: one.at.Y - L/2}, []gw.Point{{X: one.at.X, Y: one.at.Y - L/2}, {X: one.at.X, Y: one.at.Y + L/2}}, b}
		}},
		{"edge turned on the left", func(text string) drawn {
			d, e := turnedDoc(t)
			relabel(t, d, gw.None, e, text)
			at, lines := drawnAt(t, render(t, d, Options{}))
			return drawn{at, lines, EdgeLabelBox(d, e)}
		}, func(one drawn) drawn {
			// Turned, a line further down as it reads is further right,
			// so the first is the one further out.
			b := one.box
			b.X, b.W = b.X-L, b.W+L
			at := gw.Point{X: one.at.X - L, Y: one.at.Y}
			return drawn{at, []gw.Point{at, {X: at.X, Y: at.Y + L}}, b}
		}},
	} {
		one, two := tc.draw("ab"), tc.draw("ab\ncd")
		if one.lines != nil {
			t.Fatalf("%s: one line written in tspans", tc.name)
		}
		want := tc.want(one)
		if two.at != want.at || len(two.lines) != 2 || two.lines[0] != want.lines[0] || two.lines[1] != want.lines[1] {
			t.Errorf("%s: two lines at %v %v, want %v %v", tc.name, two.at, two.lines, want.at, want.lines)
		}
		if two.box != want.box {
			t.Errorf("%s: box %v, want %v", tc.name, two.box, want.box)
		}
	}
}

// TestLabelLinesBreaks: a carriage return and line feed together, and a
// carriage return alone, break a line as a line feed does.
func TestLabelLinesBreaks(t *testing.T) {
	draw := func(text string) string {
		d, _, e := zoneDoc(t, gw.Point{})
		relabel(t, d, gw.None, e, text)
		return render(t, d, Options{})
	}
	want := draw("ab\ncd")
	for _, text := range []string{"ab\r\ncd", "ab\rcd"} {
		if got := draw(text); got != want {
			t.Errorf("%q drawn otherwise than a line feed", text)
		}
	}
}

// TestLabelLinesCut: a label too wide for its vertex has each line too
// long cut, the others whole, with the full text after it in lines and
// its title in lines.
func TestLabelLinesCut(t *testing.T) {
	d, _, _ := zoneDoc(t, gw.Point{})
	relabel(t, d, gw.VertexIndex(1), gw.None, "ab\r\nabcdefgh") // the box, 40 wide, holds 4 characters within its margins
	want := `<text class="label cut" x="0" y="-3" text-anchor="middle"><tspan x="0" y="-3">ab</tspan><tspan x="0" y="11">abc…</tspan></text>` +
		`<text class="label full" display="none" x="0" y="-3" text-anchor="middle"><tspan x="0" y="-3">ab</tspan><tspan x="0" y="11">abcdefgh</tspan></text>` +
		"<title>ab\nabcdefgh</title>"
	if out := render(t, d, Options{}); !strings.Contains(out, want) {
		t.Errorf("cut label not drawn as\n%s\nin\n%s", want, out)
	}
}

// TestActorLabelLines: an actor's label of two lines goes up from its
// foot, and its figure stands a line shorter above them.
func TestActorLabelLines(t *testing.T) {
	foot := regexp.MustCompile(`<path class="figure" d="[^"]* L ([-0-9.]+) ([-0-9.]+)"/>`)
	label := regexp.MustCompile(`<text class="label" x="0" y="([-0-9.]+)"`)
	draw := func(text string) (figure, first float64) {
		d, err := makeDoc(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 64}}}}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Text: text}); err != nil {
			t.Fatal(err)
		}
		out := render(t, d, Options{})
		m, l := foot.FindStringSubmatch(out), label.FindStringSubmatch(out)
		if m == nil || l == nil {
			t.Fatalf("no figure or label in %s", out)
		}
		figure, _ = strconv.ParseFloat(m[2], 64)
		first, _ = strconv.ParseFloat(l[1], 64)
		return figure, first
	}
	f1, l1 := draw("ab")
	f2, l2 := draw("ab\ncd")
	if f2 != f1-gw.LabelLine || l2 != l1-gw.LabelLine {
		t.Errorf("two lines: figure's foot %v, first line %v; one line: %v, %v", f2, l2, f1, l1)
	}
}

// relabel sets vertex v's label, or edge e's.
func relabel(t *testing.T, d *gw.Document, v gw.VertexIndex, e gw.EdgeIndex, text string) {
	t.Helper()
	if _, err := d.Apply(gw.Command{Op: gw.Relabel, Vertex: v, Edge: e, Text: text}); err != nil {
		t.Fatal(err)
	}
}

// TestLabelLinesBoxes: the box of a vertex's label of two lines is a
// line taller, the way its lines go: down from a container's title and
// a compartment kind's, up from an actor's foot, down under a provided
// shape's box, and either way about a provided shape's middle.
func TestLabelLinesBoxes(t *testing.T) {
	const L = gw.LabelLine
	actor := func() (*gw.Document, gw.VertexIndex) {
		d, err := makeDoc(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 64}}}}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
			t.Fatal(err)
		}
		return d, 0
	}
	class := func() (*gw.Document, gw.VertexIndex) {
		d, err := makeDoc(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "class", Size: gw.Size{W: 100, H: 40}, Attrs: []gw.AttrSpec{{Key: "fields"}}, Compartments: []string{"fields"}}}}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: 0, Key: "fields", Value: "id"}); err != nil {
			t.Fatal(err)
		}
		return d, 0
	}
	provided := func(i gw.VertexIndex) func() (*gw.Document, gw.VertexIndex) {
		return func() (*gw.Document, gw.VertexIndex) {
			below := hexShape()
			below.Name, below.Label = "below", gw.LabelBelow
			return shapedDoc(t, []gw.ShapeDef{tabbedShape(), below}, nil, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 100}), i
		}
	}
	for _, tc := range []struct {
		name string
		doc  func() (*gw.Document, gw.VertexIndex)
		// up is how much higher the box's top stands, from the vertex's.
		up float64
	}{
		{"container", func() (*gw.Document, gw.VertexIndex) { d, z, _ := zoneDoc(t, gw.Point{}); return d, z }, 0},
		{"compartments", class, 0},
		{"actor", actor, L},
		{"provided below", provided(1), 0},
		{"provided inside", provided(0), L / 2},
	} {
		var top [2]float64
		var box [2]gw.Viewport
		for k, text := range []string{"ab", "ab\ncd"} {
			d, v := tc.doc()
			relabel(t, d, v, gw.None, text)
			box[k] = VertexLabelBox(d, v)
			top[k] = d.Vertex(v).Pos().Y - d.Vertex(v).Size().H/2
		}
		if got, want := box[1].Y-top[1], box[0].Y-top[0]-tc.up; got != want || box[1].H != box[0].H+L || box[1].X != box[0].X || box[1].W != box[0].W {
			t.Errorf("%s: two lines' box %v, %v below the top; one line's %v, %v", tc.name, box[1], got, box[0], box[0].Y-top[0])
		}
	}
}

// TestLabelLinesAround: what is drawn about a vertex's label of two
// lines makes room for them: its annotation is under the last line,
// its compartments start under it, and the drawing's bounds take in a
// label under a provided shape's box.
func TestLabelLinesAround(t *testing.T) {
	const L = gw.LabelLine
	num := func(s string) float64 {
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	annotation := regexp.MustCompile(`<text class="annotation" x="[-0-9.]+" y="([-0-9.]+)"`)
	rule := regexp.MustCompile(`<g class="compartment" data-key="fields"><line x1="[-0-9.]+" y1="([-0-9.]+)"`)
	viewBox := regexp.MustCompile(`viewBox="[-0-9.]+ [-0-9.]+ [-0-9.]+ ([-0-9.]+)"`)
	last := regexp.MustCompile(`<text class="label" x="0" y="([-0-9.]+)" text-anchor="middle">(?:ab|<tspan[^>]*>ab</tspan><tspan x="0" y="([-0-9.]+)">cd</tspan>)</text>`)
	var note, bounds [2]float64
	for k, text := range []string{"ab", "ab\ncd"} {
		d, _, _ := zoneDoc(t, gw.Point{})
		relabel(t, d, 1, gw.None, text)
		m := annotation.FindStringSubmatch(render(t, d, Options{VertexNotes: []Annotation{{}, {Text: "note"}}}))
		if m == nil {
			t.Fatal("no annotation")
		}
		note[k] = num(m[1])

		d, err := makeDoc(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "class", Size: gw.Size{W: 100, H: 40}, Attrs: []gw.AttrSpec{{Key: "fields"}}, Compartments: []string{"fields"}}}}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Text: text}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: 0, Key: "fields", Value: "id"}); err != nil {
			t.Fatal(err)
		}
		out := render(t, d, Options{})
		r, l := rule.FindStringSubmatch(out), last.FindStringSubmatch(out)
		if r == nil || l == nil {
			t.Fatalf("no rule or label in %s", out)
		}
		lastY := num(l[1])
		if l[2] != "" {
			lastY = num(l[2])
		}
		if got := num(r[1]) - lastY; got != 8 {
			t.Errorf("%q: the compartment's rule %v under the label's last line, want 8", text, got)
		}

		below := hexShape()
		below.Name, below.Label = "below", gw.LabelBelow
		d = shapedDoc(t, []gw.ShapeDef{below}, nil, gw.Point{X: 200, Y: 100})
		relabel(t, d, 0, gw.None, text)
		v := viewBox.FindStringSubmatch(render(t, d, Options{}))
		if v == nil {
			t.Fatal("no view box")
		}
		bounds[k] = num(v[1])
	}
	if note[1] != note[0]+L/2 {
		t.Errorf("annotation at %v under two lines, %v under one; want %v", note[1], note[0], note[0]+L/2)
	}
	if bounds[1] != bounds[0]+L {
		t.Errorf("drawing %v high with two lines under the box, %v with one", bounds[1], bounds[0])
	}
}

// TestLabelLinesStereotype: an edge's stereotype is drawn clear of its
// label's lines, above them left to right and top to bottom beside the
// edge, and beyond them turned on either side.
func TestLabelLinesStereotype(t *testing.T) {
	stereoRE := regexp.MustCompile(`<text class="(label|stereotype)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="(start|middle|end)"( transform="rotate\(-90 [-0-9. ]+\)")?>((?:[^<]|</?tspan[^>]*>)*)</text>`)
	edgeDoc := func(flow gw.Flow, to gw.Point) (*gw.Document, gw.EdgeIndex) {
		d, err := makeDoc(gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 40, H: 20}}},
			EdgeKinds:   []gw.EdgeKind{{Name: "flow", Stereotype: "«st»"}},
		}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(flow)
		for _, p := range []gw.Point{{}, to} {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: p}); err != nil {
				t.Fatal(err)
			}
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		return d, ch.Edges[0]
	}
	mirrored := func() (*gw.Document, gw.EdgeIndex) {
		d, e := turnedDoc(t)
		for v := range 3 {
			p := d.Vertex(gw.VertexIndex(v)).Pos()
			if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: gw.VertexIndex(v), Pos: gw.Point{X: -p.X, Y: p.Y}}); err != nil {
				t.Fatal(err)
			}
		}
		if !d.SetBends(e, []gw.Point{{X: 10, Y: 100}, {X: 10, Y: 300}}) || !d.SetLabelAt(e, 0.02) {
			t.Fatal("no bends or label place")
		}
		return d, e
	}
	for _, tc := range []struct {
		name string
		doc  func() (*gw.Document, gw.EdgeIndex)
	}{
		{"above", func() (*gw.Document, gw.EdgeIndex) { return edgeDoc(gw.LeftToRight, gw.Point{X: 300}) }},
		{"beside", func() (*gw.Document, gw.EdgeIndex) { return edgeDoc(gw.TopToBottom, gw.Point{Y: 300}) }},
		{"turned left", func() (*gw.Document, gw.EdgeIndex) { return turnedDoc(t) }},
		{"turned right", mirrored},
	} {
		d, e := tc.doc()
		relabel(t, d, gw.None, e, "abcdef\nabcdef\nabcdef")
		out := render(t, d, Options{})
		// The label's own stereotype is the first, before the other
		// edge's.
		var boxes [2]gw.Viewport
		found, stereo := 0, false
		for _, m := range stereoRE.FindAllStringSubmatch(out, -1) {
			x, _ := strconv.ParseFloat(m[2], 64)
			y, _ := strconv.ParseFloat(m[3], 64)
			b := textBox(x, y, m[4], m[6])
			if m[5] != "" {
				b = turnBox(b, x, y)
			}
			if m[1] == "stereotype" && !stereo {
				boxes[1], stereo = b, true
			} else if strings.Contains(m[6], "abcdef") {
				boxes[0] = b
			}
			found++
		}
		if found < 2 {
			t.Fatalf("%s: no label and stereotype in %s", tc.name, out)
		}
		if a, b := boxes[0], boxes[1]; a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
			t.Errorf("%s: the stereotype %v overlaps the label's lines %v", tc.name, b, a)
		}
	}
}

// TestLabelLinesExpanded: a cut label of two lines is drawn in full over
// the hovered vertex on a backing a line taller than one of one line's.
func TestLabelLinesExpanded(t *testing.T) {
	backing := regexp.MustCompile(`<g class="expanded"[^>]*><rect class="backing" x="[-0-9.]+" y="[-0-9.]+" width="[-0-9.]+" height="([-0-9.]+)"/>`)
	var h [2]float64
	for k, text := range []string{"abcdefgh", "ab\nabcdefgh"} {
		d, _, _ := zoneDoc(t, gw.Point{})
		relabel(t, d, 1, gw.None, text)
		var b strings.Builder
		if err := New().RenderOverlay(d, Options{Hover: 1, HasHover: true}, &b); err != nil {
			t.Fatal(err)
		}
		m := backing.FindStringSubmatch(b.String())
		if m == nil {
			t.Fatalf("%q: no backing in %s", text, b.String())
		}
		h[k], _ = strconv.ParseFloat(m[1], 64)
	}
	if h[1] != h[0]+gw.LabelLine {
		t.Errorf("backing %v high for two lines, %v for one", h[1], h[0])
	}
}

// TestLabelLinesBounds: the drawing takes in an edge's label of more
// lines than its margin holds, a little room off its edge.
func TestLabelLinesBounds(t *testing.T) {
	d, _, e := zoneDoc(t, gw.Point{})
	relabel(t, d, gw.None, e, "a\nb\nc\nd\ne\nf\ng")
	b, side, turned := edgeLabelBox(d, e)
	b = drawnBox(d, e, b, side, turned)
	if v := bounds(d); v.Y > b.Y-4 {
		t.Errorf("drawing's top %v, the label's %v", v.Y, b.Y)
	}
}
