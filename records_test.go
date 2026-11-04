package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// recordsDoc is a document whose vertex and edge set every field read
// through the views to a value other than its zero: a container z
// holding a, which has a kind's port and its own, an attribute and a
// size of its own, and an edge from a to b with a port at each end, a
// label, an attribute, a weight and two bends.
func recordsDoc(t *testing.T) (d *gw.Document, z, a, b gw.VertexIndex, e gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 120, H: 40}, Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right, Max: 2}}},
			{Name: "zone", Size: gw.Size{W: 300, H: 200}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "plain"}, {Name: "link", Weighted: true}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	z = apply(t, d, gw.Command{Op: gw.AddVertex, Kind: 1, ID: "z", Text: "zone", Pos: gw.Point{X: 500, Y: 500}}).Vertices[0]
	a = apply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "a", Text: "alpha", Pos: gw.Point{X: 10, Y: 20}}).Vertices[0]
	b = apply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "b", Text: "beta", Pos: gw.Point{X: 400, Y: 20}}).Vertices[0]
	apply(t, d, gw.Command{Op: gw.AddPort, Vertex: a, Port: gw.Port{Name: "aux", Side: gw.Bottom, Max: 1}})
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "k", Value: "v"})
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 160, H: 48}})
	e = apply(t, d, gw.Command{Op: gw.AddEdge, Kind: 1, ID: "e", From: a, To: b, FromPort: 1, ToPort: 1, Text: "go", Weight: 2.5}).Edges[0]
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: e, Key: "note", Value: "n"})
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: e, Key: "more", Value: "m"})
	if !d.SetBends(e, []gw.Point{{X: 100, Y: 80}, {X: 300, Y: 80}}) || !d.Place(a, gw.Point{X: 10, Y: 20}, true) {
		t.Fatal("bends and placing")
	}
	apply(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: z})
	return d, z, a, b, e
}

// TestRecordsRead: every read of a vertex and an edge is the document's.
func TestRecordsRead(t *testing.T) {
	d, z, a, b, e := recordsDoc(t)
	if d.VertexSlots() != 3 || d.EdgeSlots() != 1 {
		t.Errorf("slots: %d %d", d.VertexSlots(), d.EdgeSlots())
	}
	v := d.Vertex(a)
	if !v.Valid() || v.ID() != "a" || v.Kind() != 0 || v.Label() != "alpha" || !v.Placed() || !v.Live() || v.Parent() != z {
		t.Errorf("vertex fields: %v %v %d %q %v %v %d", v.Valid(), v.ID(), v.Kind(), v.Label(), v.Placed(), v.Live(), v.Parent())
	}
	if v.Size().W != 160 || v.Min().W != 160 {
		t.Errorf("vertex size %v, min %v", v.Size(), v.Min())
	}
	if p := v.Pos(); p.X == 0 && p.Y == 0 {
		t.Errorf("vertex position %v", p)
	}
	if ps := v.Ports(); ps.Len() != 3 || ps.At(0).Name != "in" || ps.At(1) != (gw.Port{Name: "out", Side: gw.Right, Max: 2}) || ps.At(2).Name != "aux" {
		t.Errorf("vertex ports: %d", ps.Len())
	}
	if own := v.Ports().Rest(2); own.Len() != 1 || own.At(0).Name != "aux" {
		t.Errorf("the vertex's own ports: %d", own.Len())
	}
	if as := v.Attrs(); as.Len() != 1 || as.At(0) != (gw.Attr{Key: "k", Value: "v"}) || as.Value("k") != "v" || as.Value("no") != "" {
		t.Errorf("vertex attributes: %d", as.Len())
	}
	if d.Vertex(b).Parent() != gw.None || d.Vertex(z).Kind() != 1 {
		t.Error("b has no container; z is a zone")
	}
	x := d.Edge(e)
	if !x.Valid() || x.ID() != "e" || x.Kind() != 1 || x.From() != a || x.To() != b || x.FromPort() != 1 || x.ToPort() != 1 ||
		x.Label() != "go" || x.Weight() != 2.5 || !x.Live() {
		t.Errorf("edge fields: %v %v %d %d %d %d %d %q %v %v", x.Valid(), x.ID(), x.Kind(), x.From(), x.To(), x.FromPort(), x.ToPort(), x.Label(), x.Weight(), x.Live())
	}
	if as := x.Attrs(); as.Len() != 2 || as.At(0).Key != "note" || as.At(1) != (gw.Attr{Key: "more", Value: "m"}) || as.Value("note") != "n" {
		t.Errorf("edge attributes: %d", as.Len())
	}
	if bs := x.Bends(); bs.Len() != 2 || bs.At(0) != (gw.Point{X: 100, Y: 80}) || bs.At(1) != (gw.Point{X: 300, Y: 80}) {
		t.Errorf("edge bends: %d", bs.Len())
	}
	// A tombstone is read as one.
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: e})
	if x := d.Edge(e); !x.Valid() || x.Live() || x.ID() != "e" {
		t.Errorf("a deleted edge: valid %v, live %v, %q", x.Valid(), x.Live(), x.ID())
	}
}

// TestRecordsOutOfRange: a vertex or an edge read out of range, below 0
// or at the slots, is the zero value, or None for an index, and so is
// every list read out of range.
func TestRecordsOutOfRange(t *testing.T) {
	d, _, a, _, e := recordsDoc(t)
	for _, i := range []gw.VertexIndex{-1, 3} {
		v := d.Vertex(i)
		if v.Valid() || v.ID() != "" || v.Kind() != gw.None || v.Label() != "" || v.Pos() != (gw.Point{}) || v.Size() != (gw.Size{}) ||
			v.Min() != (gw.Size{}) || v.Ports().Len() != 0 || v.Attrs().Len() != 0 || v.Placed() || v.Live() || v.Parent() != gw.None {
			t.Errorf("vertex %d read", i)
		}
	}
	for _, i := range []gw.EdgeIndex{-1, 1} {
		x := d.Edge(i)
		if x.Valid() || x.ID() != "" || x.Kind() != gw.None || x.From() != gw.None || x.To() != gw.None || x.FromPort() != gw.None ||
			x.ToPort() != gw.None || x.Label() != "" || x.Attrs().Len() != 0 || x.Weight() != 0 || x.Live() || x.Bends().Len() != 0 {
			t.Errorf("edge %d read", i)
		}
	}
	v, x := d.Vertex(a), d.Edge(e)
	for _, j := range []int{-1, 3} {
		if v.Ports().At(j) != (gw.Port{}) {
			t.Errorf("port %d read", j)
		}
	}
	for _, j := range []int{-1, 1} {
		if v.Attrs().At(j) != (gw.Attr{}) {
			t.Errorf("vertex attribute %d read", j)
		}
	}
	for _, j := range []int{-1, 2} {
		if x.Attrs().At(j) != (gw.Attr{}) {
			t.Errorf("edge attribute %d read", j)
		}
	}
	for _, j := range []int{-1, 2} {
		if x.Bends().At(j) != (gw.Point{}) {
			t.Errorf("bend %d read", j)
		}
	}
	if r := v.Ports().Rest(3); r.Len() != 0 {
		t.Errorf("rest at the end: %d", r.Len())
	}
	if r := v.Ports().Rest(9); r.Len() != 0 {
		t.Errorf("rest past the end: %d", r.Len())
	}
	if r := v.Ports().Rest(-1); r.Len() != 3 || r.At(0).Name != "in" {
		t.Errorf("rest below 0: %d", r.Len())
	}
}

// TestListsOf: a caller's own lists read the same way, out of range
// too, and the kinds' lists read out of range as the zero value or
// None.
func TestListsOf(t *testing.T) {
	ps := gw.PortsOf([]gw.Port{{Name: "p"}})
	as := gw.AttrsOf([]gw.Attr{{Key: "k", Value: "v"}})
	pts := gw.PointsOf([]gw.Point{{X: 1, Y: 2}})
	if ps.Len() != 1 || ps.At(0).Name != "p" || ps.At(1) != (gw.Port{}) || ps.At(-1) != (gw.Port{}) {
		t.Error("PortsOf")
	}
	if as.Len() != 1 || as.At(0).Value != "v" || as.Value("k") != "v" || as.At(1) != (gw.Attr{}) || as.At(-1) != (gw.Attr{}) {
		t.Error("AttrsOf")
	}
	if pts.Len() != 1 || pts.At(0) != (gw.Point{X: 1, Y: 2}) || pts.At(1) != (gw.Point{}) || pts.At(-1) != (gw.Point{}) {
		t.Error("PointsOf")
	}
	k, err := gw.Compile(kindsSchema())
	if err != nil {
		t.Fatal(err)
	}
	box, zone, link := k.Vertex(0), k.Vertex(1), k.Edge(0)
	for _, j := range []int{-1, 9} {
		if box.Compartments().At(j) != "" || zone.Contains().At(j) != gw.None || link.From().At(j) != gw.None || link.Attrs().At(j).Valid() ||
			box.Attrs().At(1).Choices().At(j) != "" {
			t.Errorf("kind list %d read", j)
		}
	}
}
