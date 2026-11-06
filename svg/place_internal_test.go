package svg

import (
	"bytes"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// placeDoc is a straight edge a to b, 300 long, labelled "label", and
// a straight edge across its middle from above to below for each x
// given, and with lanes true a second edge a to b, also labelled.
func placeDoc(t *testing.T, lanes bool, caps gw.Caps, xs ...float64) (*gw.Document, gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 40, H: 20}}}, EdgeKinds: []gw.EdgeKind{{Name: "line"}}}
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

// crossed counts the edges e's label box, as drawn, crosses.
func crossed(d *gw.Document, e gw.EdgeIndex) int {
	box := EdgeLabelBox(d, e)
	n := 0
	for i := range d.EdgeSlots() {
		o := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if o != e && d.Edge(o).Live() && routeCrosses(d, o, box) {
			n++
		}
	}
	return n
}

// TestLabelPlacer: a label crossed by an edge at its middle is moved
// along its own edge to the nearest place clear of it, toward the target
// first, and drawn there; one clear at its middle stays there; where no
// place is clear it goes where it crosses fewest; a lane's label is not
// moved; and a document with more edges than the placer was made for,
// or locked, is left as it is.
func TestLabelPlacer(t *testing.T) {
	d, e := placeDoc(t, false, gw.Caps{}, 150)
	if crossed(d, e) != 1 {
		t.Fatalf("at its middle the label crosses %d edges, want 1", crossed(d, e))
	}
	NewLabelPlacer(d.Caps().Edges).Place(d)
	if at := d.Edge(e).LabelAt(); at <= 0.5 || crossed(d, e) != 0 {
		t.Errorf("placed at %v, crossing %d edges; want past the middle, crossing none", at, crossed(d, e))
	}
	var buf bytes.Buffer
	if err := Render(d, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	box := EdgeLabelBox(d, e)
	if want := `x="` + n(box.X+box.W/2) + `"`; !strings.Contains(buf.String(), `<text class="label" `+want) {
		t.Errorf("the label is not drawn at its place, %s:\n%s", want, buf.String())
	}

	d, e = placeDoc(t, false, gw.Caps{}, 20)
	NewLabelPlacer(d.Caps().Edges).Place(d)
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
			for k, x := range [2]float64{160, 240} {
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
		NewLabelPlacer(d.Caps().Edges).Place(d)
		if at := d.Edge(e).LabelAt(); at == 0 {
			t.Errorf("%s under the label: it stayed at the middle", c.name)
		}
	}

	xs := []float64{145, 150, 155}
	for x := 30.0; x <= 270; x += 20 {
		xs = append(xs, x)
	}
	d, e = placeDoc(t, false, gw.Caps{}, xs...)
	before := crossed(d, e)
	NewLabelPlacer(d.Caps().Edges).Place(d)
	if after := crossed(d, e); d.Edge(e).LabelAt() == 0 || after == 0 || after >= before {
		t.Errorf("nowhere clear: placed at %v crossing %d, at the middle %d", d.Edge(e).LabelAt(), after, before)
	}

	d, e = placeDoc(t, true, gw.Caps{}, 150)
	NewLabelPlacer(d.Caps().Edges).Place(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("a lane's label moved to %v", at)
	}

	d, e = placeDoc(t, false, gw.Caps{}, 150)
	NewLabelPlacer(1).Place(d)
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("past the placer's capacity: moved to %v", at)
	}
	d.Lock()
	NewLabelPlacer(d.Caps().Edges).Place(d)
	d.Unlock()
	if at := d.Edge(e).LabelAt(); at != 0 {
		t.Errorf("locked: moved to %v", at)
	}
}

// TestAllocPlacer: placing labels allocates nothing.
func TestAllocPlacer(t *testing.T) {
	d, _ := placeDoc(t, false, gw.Caps{}, 150)
	p := NewLabelPlacer(d.Caps().Edges)
	if a := testing.AllocsPerRun(20, func() { p.Place(d) }); a > 0 {
		t.Errorf("%g allocations per run", a)
	}
}
