package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// writerDoc is recordsDoc's document with its edge's bends cleared, the
// edge e from a to b.
func writerDoc(t *testing.T) (*gw.Document, gw.EdgeIndex) {
	t.Helper()
	d, _, _, _, e := recordsDoc(t)
	d.ClearBends()
	return d, e
}

// TestBendWriter: a reservation is filled point by point, read back,
// moved within and kept; every write out of range, of a point that is
// not a coordinate or through the zero writer is refused.
func TestBendWriter(t *testing.T) {
	d, e := writerDoc(t)
	w := d.ReserveBends(e, 4)
	if !w.Valid() || w.Len() != 4 || d.Edge(e).Bends().Len() != 4 {
		t.Fatalf("reserved: valid %v, %d points, the edge %d", w.Valid(), w.Len(), d.Edge(e).Bends().Len())
	}
	for i := range 4 {
		if !w.Set(i, gw.Point{X: float64(10 * (i + 1)), Y: 5}) {
			t.Fatalf("set %d", i)
		}
	}
	if w.At(2) != (gw.Point{X: 30, Y: 5}) || w.At(-1) != (gw.Point{}) || w.At(4) != (gw.Point{}) {
		t.Errorf("read back: %v, out of range %v %v", w.At(2), w.At(-1), w.At(4))
	}
	if w.Set(-1, gw.Point{}) || w.Set(4, gw.Point{}) {
		t.Error("set out of range")
	}
	// An overlapping move, as copy does: 10 20 30 40 to 10 10 20 30.
	if !w.Move(1, 0, 3) || w.At(0).X != 10 || w.At(1).X != 10 || w.At(2).X != 20 || w.At(3).X != 30 {
		t.Errorf("move up: %v %v %v %v", w.At(0), w.At(1), w.At(2), w.At(3))
	}
	if !w.Move(0, 2, 2) || w.At(0).X != 20 || w.At(1).X != 30 {
		t.Errorf("move down: %v %v", w.At(0), w.At(1))
	}
	for _, m := range [][3]int{{-1, 0, 1}, {0, -1, 1}, {0, 0, -1}, {3, 0, 2}, {0, 3, 2}} {
		if w.Move(m[0], m[1], m[2]) {
			t.Errorf("move %v allowed", m)
		}
	}
	if w.Keep(-1) || w.Keep(5) {
		t.Error("kept out of range")
	}
	if !w.Keep(2) || w.Len() != 2 {
		t.Fatalf("keep 2: %d", w.Len())
	}
	if bs := d.Edge(e).Bends(); bs.Len() != 2 || bs.At(0) != (gw.Point{X: 20, Y: 5}) || bs.At(1) != (gw.Point{X: 30, Y: 5}) {
		t.Errorf("the edge's bends after keep: %d", bs.Len())
	}
	if !w.Keep(0) || d.Edge(e).Bends().Len() != 0 {
		t.Error("keep none")
	}
	var zero gw.BendWriter
	if zero.Valid() || zero.Len() != 0 || zero.At(0) != (gw.Point{}) || zero.Set(0, gw.Point{}) || zero.Move(0, 0, 0) || zero.Keep(0) {
		t.Error("the zero writer")
	}
	if d.ReserveBends(e, 0).Valid() || d.ReserveBends(gw.EdgeIndex(9), 1).Valid() || d.ReserveBends(e, 1<<30).Valid() {
		t.Error("a reservation of none, of no edge or past the slab")
	}
}

// TestBendWriterBadPoint: a point that is not a coordinate drops the
// edge's route, as SetBends does, and leaves the writer stale.
func TestBendWriterBadPoint(t *testing.T) {
	d, e := writerDoc(t)
	w := d.ReserveBends(e, 2)
	if !w.Set(0, gw.Point{X: 1, Y: 1}) || w.Set(1, gw.Point{X: 2 * gw.MaxCoord}) {
		t.Fatal("a point past MaxCoord taken")
	}
	if w.Valid() || d.Edge(e).Bends().Len() != 0 || w.Set(0, gw.Point{}) || w.Keep(1) {
		t.Errorf("after a bad point: valid %v, the edge %d bends", w.Valid(), d.Edge(e).Bends().Len())
	}
}

// TestBendWriterStale: once a reservation is not the edge's own, after
// ClearBends, a new reservation or SetBends for the edge, or Compact,
// its writer reads nothing and writes nothing, and another edge's
// bends are untouched.
func TestBendWriterStale(t *testing.T) {
	for _, c := range []struct {
		name  string
		stale func(d *gw.Document, e gw.EdgeIndex)
	}{
		{"ClearBends", func(d *gw.Document, _ gw.EdgeIndex) { d.ClearBends() }},
		{"a new reservation", func(d *gw.Document, e gw.EdgeIndex) { d.ReserveBends(e, 2) }},
		{"SetBends", func(d *gw.Document, e gw.EdgeIndex) { d.SetBends(e, []gw.Point{{X: 9, Y: 9}}) }},
		{"Compact", func(d *gw.Document, e gw.EdgeIndex) {
			apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: e})
			d.Compact()
		}},
	} {
		d, e := writerDoc(t)
		w := d.ReserveBends(e, 2)
		w.Set(0, gw.Point{X: 1, Y: 1})
		c.stale(d, e)
		before := d.Edge(e).Bends()
		at := before.At(0)
		if w.Valid() || w.Len() != 0 || w.At(0) != (gw.Point{}) || w.Set(0, gw.Point{X: 5, Y: 5}) || w.Move(1, 0, 1) || w.Keep(1) {
			t.Errorf("%s: the writer still works", c.name)
		}
		if d.Edge(e).Bends().At(0) != at {
			t.Errorf("%s: a stale writer changed a route", c.name)
		}
	}
}

// TestIndexLists: the containers in drawing order and Compact's maps
// read by position, None out of range.
func TestIndexLists(t *testing.T) {
	d, z, a, b, e := recordsDoc(t)
	if cs := d.Containers(); cs.Len() != 1 || cs.At(0) != z || cs.At(-1) != gw.None || cs.At(1) != gw.None {
		t.Errorf("containers: %d", cs.Len())
	}
	e2 := apply(t, d, gw.Command{Op: gw.AddEdge, Kind: 0, From: z, To: b, FromPort: gw.None, ToPort: 0}).Edges[0]
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: a})
	vm, em := d.Compact()
	if vm.Len() != 3 || vm.At(int(z)) != 0 || vm.At(int(a)) != gw.None || vm.At(int(b)) != 1 || vm.At(-1) != gw.None || vm.At(3) != gw.None {
		t.Errorf("vertex map: %d entries, %d %d %d", vm.Len(), vm.At(int(z)), vm.At(int(a)), vm.At(int(b)))
	}
	if em.Len() != 2 || em.At(int(e)) != gw.None || em.At(int(e2)) != 0 || em.At(-1) != gw.None || em.At(2) != gw.None {
		t.Errorf("edge map: %d entries, %d %d", em.Len(), em.At(int(e)), em.At(int(e2)))
	}
}

// TestBendWriterKeepEnds: Keep records the ends where they are then, as
// a layout places them after reserving, so the route holds when the
// ends are next checked, and goes when one moves after.
func TestBendWriterKeepEnds(t *testing.T) {
	d, _, a, _, e := recordsDoc(t)
	d.ClearBends()
	w := d.ReserveBends(e, 1)
	w.Set(0, gw.Point{X: 200, Y: 90})
	if !d.Place(a, gw.Point{X: 40, Y: 40}, true) || !w.Keep(1) {
		t.Fatal("placing and keeping")
	}
	// A move to where a already is checks its edges' routes against
	// their ends.
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: d.Vertex(a).Pos()})
	if d.Edge(e).Bends().Len() != 1 {
		t.Error("the route kept went when its ends were checked where they were placed")
	}
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 80, Y: 40}})
	if d.Edge(e).Bends().Len() != 0 {
		t.Error("the route stayed when an end moved")
	}
}
