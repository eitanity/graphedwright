package svg

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// placeKinds are placeDoc's kinds past the first: a container, and
// curved and orthogonal edges.
const (
	placeZone  gw.KindIndex = 1
	placeCurve gw.KindIndex = 1
	placeWire  gw.KindIndex = 2
)

// placeDoc is a straight edge a to b, 300 long, labelled "label", and
// a straight edge across its middle from above to below for each x
// given, and with lanes true a second edge a to b, also labelled.
func placeDoc(t *testing.T, lanes bool, caps gw.Caps, xs ...float64) (*gw.Document, gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 40, H: 20}}, {Name: "zone", Size: gw.Size{W: 80, H: 60}, Container: true}},
		EdgeKinds:   []gw.EdgeKind{{Name: "line"}, {Name: "curve", Route: gw.Curve}, {Name: "wire", Route: gw.Orthogonal}},
	}
	d, err := makeDoc(s, caps)
	if err != nil {
		t.Fatal(err)
	}
	add := func(x, y float64) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: x, Y: y}})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	link := func(a, b gw.VertexIndex, text string) gw.EdgeIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: a, To: b, FromPort: gw.None, ToPort: gw.None, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Edges[0]
	}
	a, b := add(0, 0), add(300, 0)
	e := link(a, b, "label")
	if lanes {
		link(a, b, "label")
	}
	for _, x := range xs {
		link(add(x, -200), add(x, 200), "")
	}
	return d, e
}

// placeVertexAt adds a box at p.
func placeVertexAt(t *testing.T, d *gw.Document, p gw.Point) gw.VertexIndex {
	t.Helper()
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: p})
	if err != nil {
		t.Fatal(err)
	}
	return ch.Vertices[0]
}

// placeEdge adds boxes at p and q and an edge of the kind from one to
// the other, labelled text.
func placeEdge(t *testing.T, d *gw.Document, p, q gw.Point, kind gw.KindIndex, text string) gw.EdgeIndex {
	t.Helper()
	ch, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: placeVertexAt(t, d, p), To: placeVertexAt(t, d, q), FromPort: gw.None, ToPort: gw.None, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	return ch.Edges[0]
}

// parsed is the number a test read from a drawing, NaN for none.
func parsed(s string) float64 {
	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return x
}

// crossed counts the edges e's label, as drawn, crosses.
func crossed(d *gw.Document, e gw.EdgeIndex) int {
	b, side, turned := edgeLabelBox(d, e)
	box := drawnBox(d, e, b, side, turned)
	n := 0
	for i := range d.EdgeSlots() {
		o := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		var lane [1]gw.Point
		w := wayOf(d, o, &lane)
		if o != e && d.Edge(o).Live() && w.crosses(d.Kinds().Edge(d.Edge(o).Kind()).Route(), box) {
			n++
		}
	}
	return n
}

// TestPlaceLabels: a label crossed by an edge at its middle is moved
// along its own edge to the nearest place clear of it, toward the target
// first, on its own side of its line and then the other, and drawn
// there, as drawn the length of its text; one clear at its middle
// stays there; where no
// place is clear it goes to the first that crosses fewest, crossing
// lines rather than covering a vertex, and never nearer an end than
// placeEnds. A deleted vertex, a container, the box an unlabelled edge
// would give a label and, top to bottom, a curve's own route are not
// counted. A lane's label is not moved, nor one on an orthogonal edge's
// own last leg, nor one crossed only by a deleted edge, and an edge
// without a label is given no place. A document with PlaceEdges edges
// is placed; one with more, or locked, is left as it is.
func TestPlaceLabels(t *testing.T) {
	d, e := placeDoc(t, false, gw.Caps{}, 150)
	if crossed(d, e) != 1 {
		t.Fatalf("at its middle the label crosses %d edges, want 1", crossed(d, e))
	}
	PlaceLabels(d)
	// From x 20 to 280, the label 43 wide as drawn: below its line at
	// the middle and at 0.575 it still meets the line at 150, at 0.6 it
	// is clear, as it is at 0.4 toward the source.
	if at := d.Edge(e).LabelAt(); math.Abs(at-0.6) > 1e-9 || crossed(d, e) != 0 {
		t.Errorf("placed at %v, crossing %d edges; want 0.6, crossing none", at, crossed(d, e))
	}
	var buf bytes.Buffer
	if err := Render(d, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	box := EdgeLabelBox(d, e)
	if want := `x="` + n(box.X+box.W/2) + `"`; !strings.Contains(buf.String(), `<text class="label" `+want) {
		t.Errorf("the label is not drawn at its place, %s:\n%s", want, buf.String())
	}

	// A line along the label's side, level just above its line: the
	// middle on the other side is clear, and the label is drawn there.
	d, e = placeDoc(t, false, gw.Caps{})
	placeEdge(t, d, gw.Point{X: 60, Y: -9}, gw.Point{X: 240, Y: -9}, 0, "")
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != -0.5 || crossed(d, e) != 0 {
		t.Errorf("a line along its side: placed at %v, crossing %d; want -0.5, the other side", at, crossed(d, e))
	}
	buf.Reset()
	if err := Render(d, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	if m := textRE.FindStringSubmatch(buf.String()); m == nil || !(parsed(m[3]) > 0) {
		t.Errorf("the label not drawn below its line:\n%s", buf.String())
	}

	// Another edge's short label, its text clear of this one's though
	// the wider box an input takes would meet it: it stays.
	d, e = placeDoc(t, false, gw.Caps{})
	placeEdge(t, d, gw.Point{X: 155, Y: 14}, gw.Point{X: 235, Y: 14}, 0, "x")
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("a short label beside it: moved to %v", at)
	}

	d, e = placeDoc(t, false, gw.Caps{}, 20)
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("a clear label moved to %v", at)
	}

	// A vertex under the label at its middle, and another edge's label
	// over it from below, its own line clear of it.
	for _, c := range []struct {
		name  string
		build func(*gw.Document)
	}{
		{"a vertex", func(d *gw.Document) {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: 150, Y: -20}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"a label", func(d *gw.Document) {
			var vs [2]gw.VertexIndex
			for k, x := range [2]float64{130, 210} {
				ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: x, Y: 14}})
				if err != nil {
					t.Fatal(err)
				}
				vs[k] = ch.Vertices[0]
			}
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: vs[0], To: vs[1], FromPort: gw.None, ToPort: gw.None, Text: "below"}); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		d, e = placeDoc(t, false, gw.Caps{})
		c.build(d)
		PlaceLabels(d)
		if at := d.Edge(e).LabelAt(); at == 0 {
			t.Errorf("%s under the label: it stayed at the middle", c.name)
		}
	}

	// Nowhere clear: on the target's side a vertex, past it and on the
	// source's side lines. The label crosses lines rather than cover the
	// vertex, which a place past the middle would cross alone.
	d, e = placeDoc(t, false, gw.Caps{}, 10, 40, 70, 100, 130, 145, 155, 255, 285)
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: 200, Y: -10}})
	if err != nil {
		t.Fatal(err)
	}
	PlaceLabels(d)
	if at, v := d.Edge(e).LabelAt(), vertexBox(d.Vertex(ch.Vertices[0])); at == 0 || boxesMeet(EdgeLabelBox(d, e), v) {
		t.Errorf("a vertex or lines: placed at %v, box %+v, the vertex %+v", at, EdgeLabelBox(d, e), v)
	}

	xs := []float64{145, 150, 155}
	for x := 30.0; x <= 270; x += 20 {
		xs = append(xs, x)
	}
	d, e = placeDoc(t, false, gw.Caps{}, xs...)
	before := crossed(d, e)
	PlaceLabels(d)
	// The first of the places that cross fewest: at 0.625 the label, 43
	// wide, meets the lines at 170 and 190, as it meets two anywhere on
	// the lines 20 apart and only covers a vertex past them.
	if after := crossed(d, e); math.Abs(d.Edge(e).LabelAt()-0.625) > 1e-9 || after == 0 || after >= before {
		t.Errorf("nowhere clear: placed at %v crossing %d, at the middle %d; want 0.625", d.Edge(e).LabelAt(), after, before)
	}

	d, e = placeDoc(t, true, gw.Caps{}, 150)
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("a lane's label moved to %v", at)
	}

	// What is not counted: a deleted vertex or a container under the
	// label, the box an unlabelled edge would give a label at its
	// middle, its line clear, and, top to bottom, a curve's own route
	// through its label beside it.
	for _, c := range []struct {
		name  string
		build func(*gw.Document) gw.EdgeIndex
	}{
		{"a deleted vertex", func(d *gw.Document) gw.EdgeIndex {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: 150, Y: -20}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Apply(gw.Command{Op: gw.DeleteVertex, Vertex: ch.Vertices[0]}); err != nil {
				t.Fatal(err)
			}
			return 0
		}},
		{"a container", func(d *gw.Document) gw.EdgeIndex {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: placeZone, Pos: gw.Point{X: 150, Y: -20}}); err != nil {
				t.Fatal(err)
			}
			return 0
		}},
		{"an unlabelled edge's middle", func(d *gw.Document) gw.EdgeIndex {
			placeEdge(t, d, gw.Point{X: -200, Y: 10}, gw.Point{X: 500, Y: 10}, 0, "")
			return 0
		}},
		{"its own curve", func(d *gw.Document) gw.EdgeIndex {
			d.SetFlow(gw.TopToBottom)
			return placeEdge(t, d, gw.Point{X: 0, Y: 300}, gw.Point{X: 300, Y: 600}, placeCurve, "curved")
		}},
	} {
		d, e = placeDoc(t, false, gw.Caps{})
		if o := c.build(d); o != 0 {
			e = o
		}
		if crossings(d, e, EdgeLabelBox(d, e)) != 0 {
			t.Errorf("%s: counted", c.name)
		}
		PlaceLabels(d)
		if at := d.Edge(e).LabelAt(); at != 0 {
			t.Errorf("%s: the label moved to %v", c.name, at)
		}
	}

	// An orthogonal edge whose turn another shares has its label on its
	// own last leg, which it keeps though a line crosses it there.
	d, _ = placeDoc(t, false, gw.Caps{})
	w := placeEdge(t, d, gw.Point{X: 0, Y: 300}, gw.Point{X: 300, Y: 400}, placeWire, "wired")
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: placeWire, From: d.Edge(w).From(), To: placeVertexAt(t, d, gw.Point{X: 300, Y: 500}), FromPort: gw.None, ToPort: gw.None}); err != nil {
		t.Fatal(err)
	}
	placeEdge(t, d, gw.Point{X: 215, Y: 250}, gw.Point{X: 215, Y: 700}, 0, "")
	PlaceLabels(d)
	if at := d.Edge(w).LabelAt(); at != 0 || crossings(d, w, EdgeLabelBox(d, w)) == 0 {
		t.Errorf("a label on its own last leg, crossed: moved to %v", at)
	}

	// An edge deleted across the label is not counted, and an edge
	// without a label, a vertex at its middle, is given no place.
	d, e = placeDoc(t, false, gw.Caps{}, 150, 400)
	if _, err := d.Apply(gw.Command{Op: gw.DeleteEdge, Edge: e + 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: gw.Point{X: 400, Y: 0}}); err != nil {
		t.Fatal(err)
	}
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("an edge deleted across the label: moved to %v", at)
	}
	if at := d.Edge(e + 2).LabelAt(); at != 0 {
		t.Errorf("an edge without a label given a place, %v", at)
	}

	// A long edge crossed every 20 from 60 to 880, from x 20 to 980:
	// clear only past 0.9 of the way, where a label is not put.
	lines := make([]float64, 0, 42)
	for x := 60.0; x <= 880; x += 20 {
		lines = append(lines, x)
	}
	d, e = placeDoc(t, false, gw.Caps{}, lines...)
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: d.Edge(e).To(), Pos: gw.Point{X: 1000, Y: 0}}); err != nil {
		t.Fatal(err)
	}
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at == 0 || at > 1-placeEnds+1e-9 {
		t.Errorf("clear only near the target: placed at %v", at)
	}

	// The edge crossed, the one across it, and lines far off to make up
	// PlaceEdges edges, and one more.
	far := []float64{150}
	for x := 1000.0; len(far) < PlaceEdges-1; x += 10 {
		far = append(far, x)
	}
	d, e = placeDoc(t, false, gw.Caps{}, far...)
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at == 0 {
		t.Errorf("with %d edges: not moved", d.EdgeSlots())
	}
	d, e = placeDoc(t, false, gw.Caps{}, append(far, 5000)...)
	PlaceLabels(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("with %d edges: moved to %v", d.EdgeSlots(), at)
	}
	d.Lock()
	PlaceLabels(d)
	d.Unlock()
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("locked: moved to %v", at)
	}
}

// TestAllocPlaceLabels: placing labels allocates nothing.
func TestAllocPlaceLabels(t *testing.T) {
	d, _ := placeDoc(t, false, gw.Caps{}, 150)
	if a := testing.AllocsPerRun(20, func() { PlaceLabels(d) }); a > 0 {
		t.Errorf("%g allocations per run", a)
	}
}

// zoneDoc is placeDoc's kinds with a zone titled "Zone" holding a box m
// at (300, 0), and an edge labelled "label" from a box at from to m; it
// returns the document, the zone and the edge.
func zoneDoc(t *testing.T, from gw.Point) (*gw.Document, gw.VertexIndex, gw.EdgeIndex) {
	t.Helper()
	return keywordZoneDoc(t, from, "", "Zone")
}

// keywordZoneDoc is zoneDoc with the zone's kind given a keyword, and
// the zone the title.
func keywordZoneDoc(t *testing.T, from gw.Point, keyword, title string) (*gw.Document, gw.VertexIndex, gw.EdgeIndex) {
	t.Helper()
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 40, H: 20}}, {Name: "zone", Size: gw.Size{W: 80, H: 60}, Container: true, Keyword: keyword}},
		EdgeKinds:   []gw.EdgeKind{{Name: "line"}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: placeZone, Text: title})
	if err != nil {
		t.Fatal(err)
	}
	z := ch.Vertices[0]
	m := placeVertexAt(t, d, gw.Point{X: 300, Y: 0})
	if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: m, To: z}); err != nil {
		t.Fatal(err)
	}
	ch, err = d.Apply(gw.Command{Op: gw.AddEdge, From: placeVertexAt(t, d, from), To: m, FromPort: gw.None, ToPort: gw.None, Text: "label"})
	if err != nil {
		t.Fatal(err)
	}
	return d, z, ch.Edges[0]
}

// TestPlaceLabelsContainers: the placer counts a label that lies across
// a container's side as crossing a line, one over its title as covering
// a vertex, and one inside clear of the title as nothing; and moves a
// label off a container's side.
func TestPlaceLabelsContainers(t *testing.T) {
	// From a box at 200 into the zone's at 300, the label at the middle,
	// 43 wide, lies across the zone's left side at 264, crossing nothing
	// else.
	d, z, e := zoneDoc(t, gw.Point{X: 200, Y: 0})
	zb, title := vertexBox(d.Vertex(z)), titleBox(d, d.Vertex(z))
	for _, c := range []struct {
		name string
		box  gw.Viewport
		want int
	}{
		{"inside", gw.Viewport{X: zb.X + 4, Y: zb.Y + zb.H - 14, W: 30, H: 10}, 0},
		{"across the left side", gw.Viewport{X: zb.X - 10, Y: zb.Y + zb.H - 14, W: 30, H: 10}, 1},
		{"across the bottom", gw.Viewport{X: zb.X + 4, Y: zb.Y + zb.H - 5, W: 30, H: 10}, 1},
		{"over the title", gw.Viewport{X: title.X + 2, Y: title.Y + 4, W: 10, H: 10}, placeVertex},
		{"right of the title", gw.Viewport{X: title.X + title.W + 1, Y: title.Y + 4, W: 10, H: 10}, 0},
	} {
		if n := crossings(d, e, c.box); n != c.want {
			t.Errorf("%s: %d crossings, want %d", c.name, n, c.want)
		}
	}
	// The title's text is its keyword's, nine tenths of a character
	// each and half one after, and its label's, cut at the band: a box
	// just past the label's width is under the keyword's, and one in the
	// padding right of the band is clear of a title too long for it.
	kd, kz, ke := keywordZoneDoc(t, gw.Point{X: 200, Y: 0}, "kw", "Zone")
	kt := titleBox(kd, kd.Vertex(kz))
	if n := crossings(kd, ke, gw.Viewport{X: kt.X + 4*defaultCharWidth + 2, Y: kt.Y + 4, W: 4, H: 10}); n != placeVertex {
		t.Errorf("a box in the title past its label, with a keyword: %d crossings, want %d", n, placeVertex)
	}
	ld, lz, le := keywordZoneDoc(t, gw.Point{X: 200, Y: 0}, "", "A title far longer than the zone")
	lb := vertexBox(ld.Vertex(lz))
	if n := crossings(ld, le, gw.Viewport{X: lb.X + lb.W - 7, Y: lb.Y + 4, W: 4, H: 10}); n != 0 {
		t.Errorf("a box right of the band under a long title: %d crossings, want 0", n)
	}

	if b, side, turned := edgeLabelBox(d, e); !acrossSide(drawnBox(d, e, b, side, turned), zb) {
		t.Fatalf("at its middle the label is not across the side: %+v, zone %+v", drawnBox(d, e, b, side, turned), zb)
	}
	PlaceLabels(d)
	if b, side, turned := edgeLabelBox(d, e); acrossSide(drawnBox(d, e, b, side, turned), zb) {
		t.Errorf("placed at %v, the label is across the side: %+v, zone %+v", d.Edge(e).LabelAt(), drawnBox(d, e, b, side, turned), zb)
	}
}
