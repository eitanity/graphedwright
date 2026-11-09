package layout_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// gapDoc is a zone holding p and, a layer on, q, by an edge with label;
// an outside vertex o before the zone with an edge to q, and r after it,
// which the zone has an edge to, with an edge back to p; in flow f. With
// below, the zone is 400 across the flow, and o and r are then placed 400
// past it that way and the rest laid out again. It returns the document
// and the edges from o and from r.
func gapDoc(t *testing.T, f gw.Flow, label string, below ...bool) (d *gw.Document, fromO, fromR gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}},
			{Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "line", Route: gw.Orthogonal}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	edge := func(from, to gw.VertexIndex, text string) gw.EdgeIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: from, To: to, FromPort: gw.None, ToPort: gw.None, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Edges[0]
	}
	o, z, p, q, r := add(0), add(1), add(0), add(0), add(0)
	for _, v := range [2]gw.VertexIndex{p, q} {
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: z}); err != nil {
			t.Fatal(err)
		}
	}
	edge(p, q, label)
	fromO = edge(o, q, "")
	edge(z, r, "")
	fromR = edge(r, p, "")
	d.SetFlow(f)
	if len(below) > 0 {
		size := gw.Size{W: 60, H: 400}
		if f == gw.TopToBottom {
			size = gw.Size{W: 400, H: 60}
		}
		if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: z, Size: size}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	if len(below) > 0 {
		for _, v := range [2]gw.VertexIndex{o, r} {
			a, c := frameOf(f, d.Vertex(v).Pos())
			at := gw.Point{X: a, Y: c + 400}
			if f == gw.TopToBottom {
				at = gw.Point{X: c + 400, Y: a}
			}
			if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: v, Pos: at}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	return d, fromO, fromR
}

// box is vertex v's extent in the frame: along the flow, then across.
func box(f gw.Flow, v gw.VertexView) (a0, a1, c0, c1 float64) {
	a, c := frameOf(f, v.Pos())
	ha, hc := frameOf(f, gw.Point{X: v.Size().W / 2, Y: v.Size().H / 2})
	return a - ha, a + ha, c - hc, c + hc
}

// alongGap is where along the flow the run of edge e along a gap goes,
// the bend before its last, and where across the flow its band is, the
// run's first bend, four from the end.
func alongGap(t *testing.T, f gw.Flow, d *gw.Document, e gw.EdgeIndex) (gx, band float64) {
	t.Helper()
	b := d.Edge(e).Bends()
	if b.Len() < 4 {
		t.Fatalf("flow %d: edge %d has %d bends, want a run along the gap", f, e, b.Len())
	}
	gx, _ = frameOf(f, b.At(b.Len()-2))
	_, band = frameOf(f, b.At(b.Len()-4))
	return gx, band
}

// TestGapRoom: a gap in a zone that a labelled edge crosses is as wide
// as the label and gapRunRoom either side, 16. Left to right, the band
// above the zone's members has the zone's title above it, room for two:
// a run along the gap from before the zone keeps half the room beside
// the layer of the member it goes to, and one from after the zone half
// the room beside the other side's, along the band 8 further out. Top to
// bottom the padding has room for one band, and the runs keep five
// eighths of the way from their members' layers across the gap, off
// each other, as they do with no label in either flow. A zone larger
// than what it holds has room for two bands below it too, in either
// flow, which runs from below take.
func TestGapRoom(t *testing.T) {
	const label = "a label" // 7 runes: 7*7+16 = 65 wide
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, text := range []string{label, ""} {
			d, fromO, fromR := gapDoc(t, f, text)
			_, p1, _, _ := box(f, d.Vertex(d.Edge(fromR).To()))
			q0, _, _, _ := box(f, d.Vertex(d.Edge(fromO).To()))
			gxO, bandO := alongGap(t, f, d, fromO)
			gxR, bandR := alongGap(t, f, d, fromR)
			if text != "" && f == gw.LeftToRight {
				if q0-p1 < 65+2*16 {
					t.Errorf("flow %d: the gap is %g, want at least %d", f, q0-p1, 65+2*16)
				}
				if gxO != q0-8 || gxR != p1+8 || bandR != bandO-8 {
					t.Errorf("flow %d: the runs go along the gap at %g and %g, from bands %g and %g, want %g and %g, from %g and %g", f, gxO, gxR, bandO, bandR, q0-8, p1+8, bandO, bandO-8)
				}
				continue
			}
			if gxO != p1*5/8+q0*3/8 || gxR != q0*5/8+p1*3/8 || bandO != bandR {
				t.Errorf("flow %d, label %q: the runs go along the gap at %g and %g, from bands %g and %g, want %g and %g on one band", f, text, gxO, gxR, bandO, bandR, p1*5/8+q0*3/8, q0*5/8+p1*3/8)
			}
		}
		d, fromO, fromR := gapDoc(t, f, label, true)
		_, p1, _, _ := box(f, d.Vertex(d.Edge(fromR).To()))
		q0, _, _, c1 := box(f, d.Vertex(d.Edge(fromO).To()))
		gxO, bandO := alongGap(t, f, d, fromO)
		gxR, bandR := alongGap(t, f, d, fromR)
		if gxO != q0-8 || gxR != p1+8 || bandO != c1+8 || bandR != bandO+8 {
			t.Errorf("flow %d, below: the runs go along the gap at %g and %g, from bands %g and %g, want %g and %g, from %g and %g", f, gxO, gxR, bandO, bandR, q0-8, p1+8, c1+8, c1+16)
		}
	}
}
