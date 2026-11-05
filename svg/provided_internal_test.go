package svg

import (
	"bytes"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// tabbedShape is a package's: a tab 40 by 12 in the left and top
// margins, above a body that fills the rest.
func tabbedShape() gw.ShapeDef {
	return gw.ShapeDef{
		Name: "tabbed", Box: gw.Size{W: 100, H: 60}, Margins: [4]float64{40, 12, 0, 0},
		Paths: []gw.ShapePath{{D: "M 0 12 H 100 V 60 H 0 Z"}, {D: "M 0 0 H 40 V 12 H 0 Z", Fill: true}},
	}
}

// hexShape is a hexagon with its outline the same polygon.
func hexShape() gw.ShapeDef {
	poly := []gw.Point{{X: 20, Y: 0}, {X: 100, Y: 0}, {X: 120, Y: 30}, {X: 100, Y: 60}, {X: 20, Y: 60}, {X: 0, Y: 30}}
	return gw.ShapeDef{
		Name: "hex", Box: gw.Size{W: 120, H: 60}, Outline: gw.OutlinePolygon, Polygon: poly,
		Paths: []gw.ShapePath{{D: "M 20 0 L 100 0 L 120 30 L 100 60 L 20 60 L 0 30 Z"}},
	}
}

// shapedDoc is a document of a schema with a kind for each shape, of
// the shape's box size, and a straight edge kind; its vertices are
// added at the points given, in the kinds' order and round again.
func shapedDoc(t testing.TB, defs []gw.ShapeDef, ports [][]gw.Port, at ...gw.Point) *gw.Document {
	t.Helper()
	s := gw.Schema{EdgeKinds: []gw.EdgeKind{{Name: "line"}}, Shapes: defs}
	for i, def := range defs {
		k := gw.VertexKind{Name: def.Name, Shape: gw.Provided + gw.Shape(i), Size: def.Box} //nolint:gosec // a handful of shapes
		if i < len(ports) {
			k.Ports = ports[i]
		}
		s.VertexKinds = append(s.VertexKinds, k)
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range at {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: gw.KindIndex(i % len(defs)), Pos: p, Text: "v" + strconv.Itoa(i)}); err != nil { //nolint:gosec // a handful of kinds
			t.Fatal(err)
		}
	}
	return d
}

func render(t testing.TB, d *gw.Document, opts Options) string {
	t.Helper()
	var b bytes.Buffer
	if err := New().Render(d, opts, &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// n is a number as the renderer writes it, to two places.
func n(x float64) string { return strconv.FormatFloat(math.Round(x*100)/100, 'f', -1, 64) }

// TestNineSlice: a tabbed rectangle drawn at three widths and two
// heights keeps its tab 40 by 12 at the top left, with the body filling
// the rest; narrower than its margins, the tab shrinks to the width.
func TestNineSlice(t *testing.T) {
	d := shapedDoc(t, []gw.ShapeDef{tabbedShape()}, nil, gw.Point{X: 200, Y: 100})
	for _, c := range []struct{ w, h, tab float64 }{{100, 60, 40}, {160, 60, 40}, {240, 60, 40}, {100, 90, 40}, {160, 90, 40}, {240, 90, 40}, {30, 60, 30}} {
		if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: 0, Size: gw.Size{W: c.w, H: c.h}}); err != nil {
			t.Fatal(err)
		}
		svg := render(t, d, Options{})
		l, top, r, b := -c.w/2, -c.h/2, c.w/2, c.h/2
		tab := `<path class="mark" d="M ` + n(l) + " " + n(top) + " H " + n(l+c.tab) + " V " + n(top+12) + " H " + n(l) + ` Z"/>`
		body := `<g class="shape shape-tabbed"><path class="line" d="M ` + n(l) + " " + n(top+12) + " H " + n(r) + " V " + n(b) + " H " + n(l) + ` Z"/>`
		if !strings.Contains(svg, tab) || !strings.Contains(svg, body) {
			t.Errorf("%g by %g: want\n%s\n%s\nin\n%s", c.w, c.h, body, tab, svg)
		}
	}
}

// TestArcScale: an arc's radii scale as the slices its end point is in:
// across the middle, a circle with no margins becomes the vertex's
// ellipse; within the margins, a corner's arc keeps its size.
func TestArcScale(t *testing.T) {
	ring := gw.ShapeDef{Name: "ring", Box: gw.Size{W: 100, H: 100}, Paths: []gw.ShapePath{{D: "M 0 50 A 50 50 0 0 1 100 50 A 50 50 0 1 0 0 50 Z"}}}
	corner := gw.ShapeDef{Name: "corner", Box: gw.Size{W: 100, H: 100}, Margins: [4]float64{20, 20, 20, 20}, Paths: []gw.ShapePath{{D: "M 0 20 A 20 20 0 0 1 20 0 C 30 10 70 10 80 0 Q 90 50 100 100"}}}
	d := shapedDoc(t, []gw.ShapeDef{ring, corner}, nil, gw.Point{X: 200, Y: 100}, gw.Point{X: 600, Y: 300})
	for v, size := range []gw.Size{{W: 200, H: 80}, {W: 300, H: 300}} {
		if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: gw.VertexIndex(v), Size: size}); err != nil { //nolint:gosec // two vertices
			t.Fatal(err)
		}
	}
	svg := render(t, d, Options{})
	for _, want := range []string{
		`d="M -100 0 A 100 40 0 0 1 100 0 A 100 40 0 1 0 -100 0 Z"`,
		// The middle of 300 less 40 of margins over the box's 60.
		`d="M -150 -130 A 20 20 0 0 1 -130 -150 C ` + n(-150+20+10*260.0/60) + ` -140 ` + n(-150+20+50*260.0/60) + ` -140 130 -150 Q 140 ` + n(-130+30*260.0/60) + ` 150 150"`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("want %s in\n%s", want, svg)
		}
	}
}

// TestPolygonOutline: for a polygon outline, laid out in both flows, a
// straight edge ends on the outline; a point just inside and just
// outside the outline hits its band, the inside hits the vertex, and a
// point in the box beyond the band hits the background.
func TestPolygonOutline(t *testing.T) {
	for _, flow := range []string{"left-to-right", "top-to-bottom"} {
		d := shapedDoc(t, []gw.ShapeDef{hexShape()}, nil, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 240})
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: flow}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		a, b := ends(d, d.Edge(0), gw.PointList{})
		for i, p := range []gw.Point{a, b} {
			v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // two vertices
			if dist := vertexOutlineDistance(d, v, p); math.Abs(dist) > 1e-9 || p == v.Pos() {
				t.Errorf("%s: end %d at %v is %g from the outline", flow, i, p, dist)
			}
		}
		c := d.Vertex(0).Pos()
		// The hexagon's left point is 60 left of its centre, its top
		// left corner 40 left and 30 up.
		for _, h := range []struct {
			name string
			p    gw.Point
			want HitKind
		}{
			{"just inside the left point", gw.Point{X: c.X - 57, Y: c.Y}, OutlineHit},
			{"just outside it", gw.Point{X: c.X - 62, Y: c.Y}, OutlineHit},
			{"inside", c, VertexHit},
			{"the box's corner, beyond the band", gw.Point{X: c.X - 58, Y: c.Y - 28}, Background},
			{"beyond the box", gw.Point{X: c.X - 70, Y: c.Y}, Background},
		} {
			if got := HitTest(d, h.p); got.Kind != h.want || (h.want != Background && got.Vertex != 0) {
				t.Errorf("%s: %s hits %+v", flow, h.name, got)
			}
		}
	}
}

// TestClipOffCentre: the line from the centre of a polygon that does
// not hold it leaves where it last crosses the outline, not where it
// enters.
func TestClipOffCentre(t *testing.T) {
	lid := gw.ShapeDef{Name: "cap", Box: gw.Size{W: 100, H: 60}, Outline: gw.OutlinePolygon,
		Polygon: []gw.Point{{X: 50, Y: 25}, {X: 0, Y: 0}, {X: 100, Y: 0}}, Paths: []gw.ShapePath{{D: "M 50 25 L 0 0 L 100 0 Z"}}}
	d := shapedDoc(t, []gw.ShapeDef{lid}, nil, gw.Point{X: 200, Y: 100})
	if got := vertexClip(d, d.Vertex(0), gw.Point{X: 200, Y: -400}); got != (gw.Point{X: 200, Y: 70}) {
		t.Errorf("up from the centre: %v, want the top edge at (200,70)", got)
	}
}

// TestPortsOnOutline: a port sits where the outline meets the line in
// from its place on the box's side: on a diamond's and an ellipse's
// sides, on a polygon's, and on the box where the line misses the
// shape; the hit test finds it there.
func TestPortsOnOutline(t *testing.T) {
	diamond := gw.ShapeDef{Name: "gate", Box: gw.Size{W: 100, H: 60}, Outline: gw.OutlineDiamond, Paths: []gw.ShapePath{{D: "M 50 0 L 100 30 L 50 60 L 0 30 Z"}}}
	oval := gw.ShapeDef{Name: "oval", Box: gw.Size{W: 100, H: 60}, Outline: gw.OutlineEllipse, Paths: []gw.ShapePath{{D: "M 0 30 A 50 30 0 0 1 100 30 A 50 30 0 0 1 0 30 Z"}}}
	slab := gw.ShapeDef{Name: "slab", Box: gw.Size{W: 100, H: 60}, Outline: gw.OutlinePolygon, Polygon: []gw.Point{{X: 20, Y: 20}, {X: 80, Y: 20}, {X: 80, Y: 40}, {X: 20, Y: 40}}, Paths: []gw.ShapePath{{D: "M 20 20 H 80 V 40 H 20 Z"}}}
	two := []gw.Port{{Name: "a", Side: gw.Left}, {Name: "b", Side: gw.Left}, {Name: "c", Side: gw.Top}}
	three := []gw.Port{{Name: "a", Side: gw.Left}, {Name: "b", Side: gw.Left}, {Name: "c", Side: gw.Left}}
	d := shapedDoc(t, []gw.ShapeDef{diamond, oval, slab}, [][]gw.Port{two, two, three}, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 100}, gw.Point{X: 600, Y: 100})
	near := func(p, q gw.Point) bool { return math.Abs(p.X-q.X) < 1e-9 && math.Abs(p.Y-q.Y) < 1e-9 }
	// The diamond 10 above its middle is 50 * (1 - 10/30) wide each side.
	ellipseX := 50 * math.Sqrt(1-(10.0*10.0)/(30*30))
	for _, c := range []struct {
		name string
		v    gw.VertexIndex
		p    gw.PortIndex
		want gw.Point
	}{
		{"diamond, left, upper", 0, 0, gw.Point{X: 200 - 50*2.0/3, Y: 90}},
		{"diamond, left, lower", 0, 1, gw.Point{X: 200 - 50*2.0/3, Y: 110}},
		{"diamond, top", 0, 2, gw.Point{X: 200, Y: 70}},
		{"ellipse, left, upper", 1, 0, gw.Point{X: 400 - ellipseX, Y: 90}},
		{"slab, left, missing", 2, 0, gw.Point{X: 550, Y: 85}},
		{"slab, left, through", 2, 1, gw.Point{X: 570, Y: 100}},
	} {
		got := PortOn(d, c.v, c.p)
		if !near(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		if h := HitTest(d, got); h.Kind != PortHit || h.Vertex != c.v || h.Port != c.p {
			t.Errorf("%s: hit at %v is %+v", c.name, got, h)
		}
	}
	if !strings.Contains(render(t, d, Options{}), `<circle class="port" data-port="a" cx="`+n(-50*2.0/3)+`" cy="-10"`) {
		t.Error("the diamond's port is not drawn on its side")
	}
}

// TestProvidedLabel: a label inside is centred between the margins and
// cut at that width; below, it is at the foot as an actor's; none, it is
// not drawn. The ring follows the outline.
func TestProvidedLabel(t *testing.T) {
	below, none := hexShape(), tabbedShape()
	below.Name, below.Label = "below", gw.LabelBelow
	none.Name, none.Label = "none", gw.LabelNone
	d := shapedDoc(t, []gw.ShapeDef{tabbedShape(), below, none}, nil, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 100}, gw.Point{X: 600, Y: 100})
	if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: 0, Size: gw.Size{W: 160, H: 90}}); err != nil {
		t.Fatal(err)
	}
	x, y, w, isBelow, isNone := labelSlot(d, d.Vertex(0))
	if x != 20 || y != 6 || w != 112 || isBelow || isNone {
		t.Errorf("inside: %g %g %g %v %v", x, y, w, isBelow, isNone)
	}
	if b := VertexLabelBox(d, 0); b.X != 200+20-60 || b.W != 120 {
		t.Errorf("inside's box %+v", b)
	}
	if _, _, _, isBelow, _ := labelSlot(d, d.Vertex(1)); !isBelow || !labelBelow(d, d.Vertex(1)) {
		t.Error("below")
	}
	if _, _, _, _, isNone := labelSlot(d, d.Vertex(2)); !isNone {
		t.Error("none")
	}
	svg := render(t, d, Options{Target: 1, HasTarget: true, Mode: Edit})
	for _, want := range []string{
		`<text class="label" x="20" y="10"`,
		`<text class="label" x="0" y="26"`,
		`<polygon class="ring" points="-40 -30 40 -30 60 0 40 30 -40 30 -60 0"/>`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("want %s in\n%s", want, svg)
		}
	}
	if strings.Contains(svg, ">v2<") {
		t.Errorf("the label of a shape without one is drawn:\n%s", svg)
	}
}

// TestAllocProvided: rendering and hit testing provided shapes allocate
// nothing.
func TestAllocProvided(t *testing.T) {
	d := shapedDoc(t, []gw.ShapeDef{tabbedShape(), hexShape()}, [][]gw.Port{nil, {{Name: "p", Side: gw.Top}}}, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 240})
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: 0, To: 1, FromPort: gw.None, ToPort: 0}); err != nil {
		t.Fatal(err)
	}
	r := New()
	opts := Options{Target: 1, HasTarget: true, Mode: Edit}
	if a := testing.AllocsPerRun(20, func() {
		if err := r.Render(d, opts, io.Discard); err != nil {
			t.Fatal(err)
		}
	}); a > 0 {
		t.Errorf("render: %g allocations", a)
	}
	if a := testing.AllocsPerRun(20, func() { HitTest(d, gw.Point{X: 160, Y: 100}); HitTest(d, gw.Point{X: 400, Y: 210}) }); a > 0 {
		t.Errorf("hit test: %g allocations", a)
	}
}

// TestTurned: a fork bar is drawn across the flow in both flows, its
// ports at each end of it along the flow; turned, an arc's radii swap
// and its sweep flips, a polygon outline turns with the paths, and the
// label sits between the turned margins.
func TestTurned(t *testing.T) {
	fork := gw.ShapeDef{Name: "bar", Box: gw.Size{W: 8, H: 80}, Turns: true, Paths: []gw.ShapePath{{D: "M 0 0 H 8 V 80 H 0 Z", Fill: true}}}
	hook := gw.ShapeDef{Name: "hook", Box: gw.Size{W: 20, H: 60}, Margins: [4]float64{0, 10, 0, 0}, Turns: true, Outline: gw.OutlinePolygon,
		Polygon: []gw.Point{{X: 0, Y: 0}, {X: 20, Y: 10}, {X: 20, Y: 60}, {X: 0, Y: 60}}, Paths: []gw.ShapePath{{D: "M 0 0 A 10 30 0 0 1 20 60"}}}
	ports := []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right}}
	d := shapedDoc(t, []gw.ShapeDef{fork, hook}, [][]gw.Port{ports}, gw.Point{X: 200, Y: 100}, gw.Point{X: 400, Y: 100})
	svg := render(t, d, Options{})
	if !strings.Contains(svg, `<path class="mark" d="M -4 -40 H 4 V 40 H -4 Z"/>`) || PortOn(d, 0, 0) != (gw.Point{X: 196, Y: 100}) || PortOn(d, 0, 1) != (gw.Point{X: 204, Y: 100}) {
		t.Errorf("left to right: ports %v %v\n%s", PortOn(d, 0, 0), PortOn(d, 0, 1), svg)
	}
	if _, err := d.Apply(gw.Command{Op: gw.Layout, Value: "top-to-bottom"}); err != nil {
		t.Fatal(err)
	}
	svg = render(t, d, Options{Target: 1, HasTarget: true, Mode: Edit})
	for _, want := range []string{
		`<path class="mark" d="M -40 -4 V 4 H 40 V -4 Z"/>`,
		// The hook's box turned is 60 by 20 with its 10 top margin on
		// the left: the arc's end (20,60) is (60,20), its radii 30 by 10
		// and its sweep 0.
		`<path class="line" d="M -30 -10 A 30 10 0 0 0 30 10"/>`,
		`<polygon class="ring" points="-30 -10 -20 10 30 10 30 -10"/>`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("top to bottom: want %s in\n%s", want, svg)
		}
	}
	if PortOn(d, 0, 0) != (gw.Point{X: 200, Y: 96}) || PortOn(d, 0, 1) != (gw.Point{X: 200, Y: 104}) {
		t.Errorf("top to bottom: ports %v %v", PortOn(d, 0, 0), PortOn(d, 0, 1))
	}
	if h := HitTest(d, gw.Point{X: 200, Y: 96}); h.Kind != PortHit || h.Port != 0 {
		t.Errorf("the turned port's hit: %+v", h)
	}
	// The hook's polygon turned: its edge from (0,0) to (10,20) in the
	// box, from (-30,-10) to (-20,10) on the vertex at (400,100).
	if dist := vertexOutlineDistance(d, d.Vertex(1), gw.Point{X: 375, Y: 100}); math.Abs(dist) > 1e-9 {
		t.Errorf("a point on the turned polygon is %g from it", dist)
	}
	if x, _, w, _, _ := labelSlot(d, d.Vertex(1)); x != 5 || w != 42 {
		t.Errorf("the turned label: x %g, w %g", x, w)
	}
	// A port of the hook's own on its top, turned to its left, sits on
	// the slanted side, where the line in across the flow meets it; its
	// label goes beside it as a left port's does.
	if _, err := d.Apply(gw.Command{Op: gw.AddPort, Vertex: 1, Port: gw.Port{Name: "p", Side: gw.Top}}); err != nil {
		t.Fatal(err)
	}
	// The slanted side runs from the top left corner to 10 in at the
	// bottom, across the vertex's own size, which the port grew; at its middle
	// height it is 5 in.
	want := 400 - d.Vertex(1).Size().W/2 + 5
	if got := PortOn(d, 1, 0); math.Abs(got.X-want) > 1e-9 || got.Y != 100 {
		t.Errorf("the hook's turned port: %v, want (%g,100), size %v", got, want, d.Vertex(1).Size())
	}
	if _, _, anchor, _ := portLabel(d, 1, 0); anchor != anchorEnd {
		t.Errorf("the turned port's label is anchored %q", anchor)
	}
}
