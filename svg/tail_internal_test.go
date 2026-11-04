package svg

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

var pathStartRE = regexp.MustCompile(`<g class="edge [^"]*" id="([^"]+)"[^>]*><path d="M ([-0-9.]+) ([-0-9.]+)`)

// TestTails: laid out in either flow, an edge with a tail starts its
// stroke the tail's length from where it leaves its source, which is on
// the source's outline, so the tail spans the gap; and a point on the
// tail, past the outline's band, hits the edge.
func TestTails(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "block", Shape: gw.Rect, Size: gw.Size{W: 100, H: 36}}},
		EdgeKinds: []gw.EdgeKind{
			{Name: "composition", Tail: gw.FilledDiamond},
			{Name: "aggregation", Route: gw.Curve, Tail: gw.HollowDiamond, Arrow: true},
			{Name: "containment", Route: gw.Orthogonal, Tail: gw.CircledPlus},
		},
	}
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(flow)
		var vs [6]gw.VertexIndex
		for i := range vs {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "v" + strconv.Itoa(i)})
			if err != nil {
				t.Fatal(err)
			}
			vs[i] = ch.Vertices[0]
		}
		for k := range 3 {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: gw.KindIndex(k), From: vs[2*k], To: vs[2*k+1], FromPort: gw.None, ToPort: gw.None}); err != nil { //nolint:gosec // below 3
				t.Fatal(err)
			}
		}
		// A straight edge at an angle, its tail clipped to the outline.
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: vs[0], To: vs[5], FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Render(d, Options{}, &buf); err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, m := range pathStartRE.FindAllStringSubmatch(buf.String(), -1) {
			e := d.EdgeByID(gw.ID(m[1]))
			ed := &d.Edges[e]
			kind := d.Kinds().Edge(ed.Kind)
			x, _ := strconv.ParseFloat(m[2], 64)
			y, _ := strconv.ParseFloat(m[3], 64)
			var lane [1]gw.Point
			w := wayOf(d, e, &lane)
			from := &d.Vertices[ed.From]
			if od := outlineDistance(d.Kinds().Vertex(from.Kind).Shape(), from, w.a); math.Abs(od) > 0.5 {
				t.Errorf("flow %d: %s leaves %g off its source's outline", flow, ed.ID, od)
			}
			tl := tailLength(kind.Tail())
			if got := math.Hypot(x-w.a.X, y-w.a.Y); math.Abs(got-tl) > 0.5 {
				t.Errorf("flow %d: %s's stroke starts %g from its source, want the tail's %g", flow, ed.ID, got, tl)
			}
			dir := gw.Point{X: (x - w.a.X) / tl, Y: (y - w.a.Y) / tl}
			p := gw.Point{X: w.a.X + dir.X*(tl-1), Y: w.a.Y + dir.Y*(tl-1)}
			// Within the outline's band, the vertex's outline is hit
			// first, as for any edge.
			if h := HitTest(d, p); math.Abs(outlineDistance(gw.Rect, from, p)) > band(from) && (h.Kind != EdgeHit || h.Edge != e) {
				t.Errorf("flow %d: %v on %s's tail hits %+v", flow, p, ed.ID, h)
			}
			n++
		}
		if n != 4 {
			t.Errorf("flow %d: checked %d edges, want 4", flow, n)
		}
	}
}
