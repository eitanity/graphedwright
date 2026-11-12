package layout_test

import (
	"math"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// entryDoc is an outside vertex o and a zone holding p and q, p before
// q by an edge of kind inner; o's edge of kind outer goes to q, in flow
// f; with beside, that many more members, also after p, stand either
// side of q in its layer. It returns the document and the edge from o.
func entryDoc(t *testing.T, f gw.Flow, outer, inner gw.Route, beside ...int) (*gw.Document, gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}},
			{Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "outer", Route: outer}, {Name: "inner", Route: inner}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex, text string) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	o, z, p := add(0, "o"), add(1, "z"), add(0, "p")
	members := []gw.VertexIndex{p}
	n := 0
	if len(beside) > 0 {
		n = beside[0]
	}
	for k := range n + 1 {
		if k == n/2 {
			members = append(members, add(0, "q"))
		}
		if k < n {
			members = append(members, add(0, "r"+strconv.Itoa(k)))
		}
	}
	var q gw.VertexIndex
	for _, v := range members {
		if d.Vertex(v).Label() == "q" {
			q = v
		}
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: z}); err != nil {
			t.Fatal(err)
		}
		if v == p {
			continue
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 1, From: p, To: v, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	ch, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: o, To: q, FromPort: gw.None, ToPort: gw.None})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(f)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	return d, ch.Edges[0]
}

// frameOf is p in the layout's frame: along the flow, then across it.
func frameOf(f gw.Flow, p gw.Point) (along, across float64) {
	if f == gw.TopToBottom {
		return p.Y, p.X
	}
	return p.X, p.Y
}

// TestEntryBeside: a straight edge into a member of a zone, which a
// member of an earlier layer stands before, comes in from the side,
// across the flow at the member's middle, not along the gap before its
// layer where the edge from the earlier member and its label are; in
// either flow.
func TestEntryBeside(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, e := entryDoc(t, f, gw.Straight, gw.Straight)
		q := d.Vertex(d.Edge(e).To())
		bends := d.Edge(e).Bends()
		if bends.Len() == 0 {
			t.Fatalf("flow %d: no bends", f)
		}
		la, lc := frameOf(f, bends.At(bends.Len()-1))
		qa, qc := frameOf(f, q.Pos())
		_, half := frameOf(f, gw.Point{X: q.Size().W / 2, Y: q.Size().H / 2})
		if la != qa || math.Abs(lc-qc) <= half {
			t.Errorf("flow %d: the last bend %v is not beside q at %v", f, bends.At(bends.Len()-1), q.Pos())
		}
	}
}

// TestEntryOffSide: an orthogonal or curved edge into a member of a
// zone, which must arrive on the side facing the flow, comes in level
// with its end there, which is off the middle where the edge from the
// member before arrives too (gw.Document.EndOffset), and along the band
// on that side, so that it does not cross that edge; in either flow.
func TestEntryOffSide(t *testing.T) {
	for _, c := range [...]struct {
		f     gw.Flow
		route gw.Route
	}{{gw.LeftToRight, gw.Orthogonal}, {gw.TopToBottom, gw.Orthogonal}, {gw.LeftToRight, gw.Curve}, {gw.TopToBottom, gw.Curve}} {
		f := c.f
		d, e := entryDoc(t, f, c.route, gw.Orthogonal)
		q := d.Vertex(d.Edge(e).To())
		bends := d.Edge(e).Bends()
		if bends.Len() < 3 {
			t.Fatalf("flow %d: %d bends", f, bends.Len())
		}
		_, lc := frameOf(f, bends.At(bends.Len()-1))
		_, pc := frameOf(f, bends.At(bends.Len()-2))
		_, bc := frameOf(f, bends.At(bends.Len()-3))
		_, qc := frameOf(f, q.Pos())
		off := d.EndOffset(e, true)
		if off == 0 || lc-qc != off || pc != lc || (lc-qc)*(bc-qc) <= 0 {
			t.Errorf("flow %d: the last two bends %v, %v arrive %v off q's middle %v, want %v towards the band at %v", f, bends.At(bends.Len()-2), bends.At(bends.Len()-1), lc-qc, q.Pos(), off, bc)
		}
	}
}

// TestEntryBlocked: where members of its layer stand on both sides of
// the member a straight edge goes to, the edge does not come in from the
// side through one of them, but along the gap before the layer, its last
// bend before q and within its span across the flow, and off its middle,
// where the edge from p arrives.
func TestEntryBlocked(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, e := entryDoc(t, f, gw.Straight, gw.Straight, 2)
		ed := d.Edge(e)
		q := d.Vertex(ed.To())
		if ed.Bends().Len() == 0 {
			t.Fatalf("flow %d: no bends", f)
		}
		la, lc := frameOf(f, ed.Bends().At(ed.Bends().Len()-1))
		qa, qc := frameOf(f, q.Pos())
		along, across := frameOf(f, gw.Point{X: q.Size().W / 2, Y: q.Size().H / 2})
		if la > qa-along || math.Abs(lc-qc) >= across || lc == qc {
			t.Errorf("flow %d: the last bend %v is not before q at %v", f, ed.Bends().At(ed.Bends().Len()-1), q.Pos())
		}
		pts := make([]gw.Point, 0, ed.Bends().Len()+2)
		pts = append(pts, d.Vertex(ed.From()).Pos())
		for k := range ed.Bends().Len() {
			pts = append(pts, ed.Bends().At(k))
		}
		pts = append(pts, d.Vertex(ed.To()).Pos())
		for i := range d.VertexSlots() {
			v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
			u := gw.VertexIndex(i)           //nolint:gosec // bounded by the capacity
			if u == ed.From() || u == ed.To() || d.Kinds().Vertex(v.Kind()).Container() {
				continue
			}
			for k := 1; k < len(pts); k++ {
				if crosses(pts[k-1], pts[k], v) {
					t.Errorf("flow %d: the edge to q crosses %s", f, v.Label())
				}
			}
		}
	}
}

// TestFootWidth: actors side by side in a layer stand apart by the name
// at their feet, or the keyword above their heads where that is wider,
// not by their figures alone, so that the texts keep the gap between
// them; in a top-to-bottom flow, where a layer runs across.
func TestFootWidth(t *testing.T) {
	for _, c := range []struct {
		label, keyword string
		width          float64
	}{{"a much longer name", "", 18 * 7}, {"a", "«external system»", 17 * 7}} {
		s := gw.Schema{VertexKinds: []gw.VertexKind{{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 76}, Keyword: c.keyword}}}
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: c.label}); err != nil {
				t.Fatal(err)
			}
		}
		d.SetFlow(gw.TopToBottom)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		if gap := math.Abs(d.Vertex(1).Pos().X-d.Vertex(0).Pos().X) - c.width; gap < 24 {
			t.Errorf("%q, %q: the texts are %v apart, want 24", c.label, c.keyword, gap)
		}
	}
}

// TestFootHeight: provided shapes that draw their names under their
// boxes, one above the other in a layer of a left-to-right flow, stand
// apart by the name too, its baseline 14 under the foot and 4 more for
// the descent, and keep the usual gap of 20 beyond it; without names
// they stand the gap apart, short of a name's room more. A keyword over
// the box, with no name, takes the line above as the name takes the one
// below. A name of four lines takes three lines more.
func TestFootHeight(t *testing.T) {
	for _, c := range []struct {
		label, keyword string
		room           float64
	}{{"", "", 20}, {"a", "", 20 + 18}, {"", "«k»", 20 + 18}, {"a\nb\nc\nd", "", 20 + 18 + 3*gw.LabelLine}} {
		s := gw.Schema{
			Shapes:      []gw.ShapeDef{{Name: "dot", Box: gw.Size{W: 28, H: 28}, Outline: gw.OutlineEllipse, Label: gw.LabelBelow, Paths: []gw.ShapePath{{D: "M 0 14 A 14 14 0 0 1 28 14 A 14 14 0 0 1 0 14 Z"}}}},
			VertexKinds: []gw.VertexKind{{Name: "dot", Shape: gw.Provided, Size: gw.Size{W: 32, H: 32}, Keyword: c.keyword}},
		}
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: c.label}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		if gap := math.Abs(d.Vertex(1).Pos().Y-d.Vertex(0).Pos().Y) - 32; gap < c.room || c.room == 20 && gap >= 20+18 {
			t.Errorf("%q: the boxes are %v apart, want %v", c.label, gap, c.room)
		}
	}
}

// TestExitOffSide: an orthogonal edge out of a member of a zone to a
// vertex past the zone, where an edge to a later member leaves the same
// side, leaves level with its end there, off the side's middle
// (gw.Document.EndOffset); in either flow.
func TestExitOffSide(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		s := gw.Schema{
			VertexKinds: []gw.VertexKind{
				{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}},
				{Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Container: true},
			},
			EdgeKinds: []gw.EdgeKind{{Name: "wire", Route: gw.Orthogonal}},
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
		z, q, r, o := add(1), add(0), add(0), add(0)
		for _, v := range [2]gw.VertexIndex{q, r} {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: z}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: q, To: r, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: q, To: o, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		e := ch.Edges[0]
		d.SetFlow(f)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		bends := d.Edge(e).Bends()
		if bends.Len() < 2 {
			t.Fatalf("flow %d: %d bends", f, bends.Len())
		}
		_, fc := frameOf(f, bends.At(0))
		_, sc := frameOf(f, bends.At(1))
		_, qc := frameOf(f, d.Vertex(q).Pos())
		if off := d.EndOffset(e, false); off == 0 || fc-qc != off || sc != fc {
			t.Errorf("flow %d: the first two bends %v, %v leave %v off q's middle %v, want %v", f, bends.At(0), bends.At(1), fc-qc, d.Vertex(q).Pos(), off)
		}
	}
}
