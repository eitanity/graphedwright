package layout_test

import (
	"math"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// wideDoc is a source a, a box b after it as long across the flow as
// the gap is deep fifteen times, and a target c after that; curves a to
// b, pull times, and b to c, and lanes of curves a to c, whose dummies
// pass beside b, so that their legs into c, and with a pulled level with
// b their legs out of a, have far to go across the flow. With zone
// true, c is in a container. In flow f, with the capacities, laid out.
func wideDoc(t *testing.T, f gw.Flow, lanes, pull int, zone bool, caps gw.Caps) (*gw.Document, []gw.EdgeIndex) {
	return wideDocWith(t, f, lanes, pull, wideOpts{zone: zone}, caps)
}

// wideOpts are wideDoc's further cases: c in a container, after another
// member m there, m to c, which the lanes' runs inside it go round
// (first); the lanes from c back to a (back); and b placed where it
// stands, the layout not run with All (placed).
type wideOpts struct{ zone, first, back, placed bool }

func wideDocWith(t *testing.T, f gw.Flow, lanes, pull int, o wideOpts, caps gw.Caps) (*gw.Document, []gw.EdgeIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 120, H: 40}},
			{Name: "wide", Size: gw.Size{W: 120, H: 600}},
			{Name: "zone", Size: gw.Size{W: 160, H: 80}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	}
	if f == gw.TopToBottom {
		s.VertexKinds[1].Size = gw.Size{W: 600, H: 40}
	}
	d, err := makeDoc(s, caps)
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(f)
	add := func(kind gw.KindIndex) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	link := func(a, b gw.VertexIndex) gw.EdgeIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: a, To: b, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Edges[0]
	}
	a, b, c := add(0), add(1), add(0)
	if o.zone || o.first {
		z := add(2)
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: c, To: z}); err != nil {
			t.Fatal(err)
		}
		if o.first {
			m := add(0)
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: m, To: z}); err != nil {
				t.Fatal(err)
			}
			link(m, c)
		}
	}
	for range pull {
		link(a, b)
	}
	link(b, c)
	es := make([]gw.EdgeIndex, 0, lanes)
	for range lanes {
		if o.back {
			es = append(es, link(c, a))
		} else {
			es = append(es, link(a, c))
		}
	}
	if o.placed {
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 400, Y: 360}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: !o.placed}); err != nil {
		t.Fatal(err)
	}
	return d, es
}

// TestUprights: lanes whose legs into their target have far to go
// across the flow in a shallow gap are drawn with an upright each:
// level, straight across, and level into the end, which ends where the
// end is drawn (gw.Document.EndOffset), each at its own place along the
// gap, a step apart and clear of the bands either side with room for
// the arrowhead, and nested so that no two lanes cross; the gap is
// deepened to hold four of them. Out of a source pulled level with the
// wide box the legs have far to go too, and their uprights stand back
// from the dummies' side. With the target in a container, and in either
// flow.
func TestUprights(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, c := range []struct {
			pull int
			zone bool
		}{{1, false}, {1, true}, {6, false}} {
			d, es := wideDoc(t, f, 4, c.pull, c.zone, gw.Caps{})
			name := func(e gw.EdgeIndex) string {
				return "flow " + strconv.Itoa(int(f)) + ", pull " + strconv.Itoa(c.pull) + ", zone " + strconv.FormatBool(c.zone) + ", lane " + strconv.Itoa(int(e))
			}
			along, across := func(p gw.Point) float64 { return p.X }, func(p gw.Point) float64 { return p.Y }
			if f == gw.TopToBottom {
				along, across = across, along
			}
			to := d.Vertex(d.Edge(es[0]).To())
			near := along(to.Pos()) - (to.Size().W / 2)
			if f == gw.TopToBottom {
				near = along(to.Pos()) - (to.Size().H / 2)
			}
			ups := map[float64]bool{}
			for _, e := range es {
				b := d.Edge(e).Bends()
				n := b.Len()
				if n < 4 {
					t.Fatalf("%s: %d bends", name(e), n)
				}
				p, q, prev := b.At(n-2), b.At(n-1), b.At(n-3)
				if along(p) != along(q) || math.Abs(across(p)-across(q)) < 100 {
					t.Errorf("%s: no upright into the end: %v, %v", name(e), p, q)
				}
				if want := across(d.Vertex(d.Edge(e).To()).Pos()) + d.EndOffset(e, true); across(q) != want {
					t.Errorf("%s: the upright ends at %v across, the end is drawn at %v", name(e), across(q), want)
				}
				if along(p)-along(prev) < 8 || near-along(q) < 16 {
					t.Errorf("%s: the upright at %v, %v after the band and %v before the end", name(e), along(p), along(p)-along(prev), near-along(q))
				}
				ups[along(p)] = true
				if c.pull > 1 {
					s0, s1 := b.At(0), b.At(1)
					if along(s0) != along(s1) || math.Abs(across(s0)-across(s1)) < 100 {
						t.Errorf("%s: no upright out of the source: %v, %v", name(e), s0, s1)
					}
					ups[along(s0)] = true
					if out := along(s0) - along(d.Vertex(d.Edge(e).From()).Pos()); along(b.At(2))-along(s0) >= out {
						t.Errorf("%s: the upright out of the source at %v stands nearer it than the dummy's bend at %v", name(e), along(s0), along(b.At(2)))
					}
				}
			}
			if want := len(es) * map[bool]int{false: 1, true: 2}[c.pull > 1]; len(ups) != want {
				t.Errorf("flow %d, pull %d: uprights at %v, want %d places", f, c.pull, ups, want)
			}
			for i, e := range es {
				for _, o := range es[i+1:] {
					if at, ok := crossing(d, e, o); ok {
						t.Errorf("%s crosses lane %d at %v", name(e), o, at)
					}
				}
			}
		}
	}
}

// crossing is where two edges' routes, from end to end through their
// bends, cross, if they do. Each route starts and ends on its vertex's
// side level with the bend next to it, as the ends are drawn.
func crossing(d *gw.Document, e, o gw.EdgeIndex) (gw.Point, bool) {
	route := func(i gw.EdgeIndex) []gw.Point {
		ed := d.Edge(i)
		b := ed.Bends()
		side := func(v gw.VertexIndex, next gw.Point) gw.Point {
			x := d.Vertex(v)
			if d.Flow() == gw.TopToBottom {
				return gw.Point{X: next.X, Y: x.Pos().Y + math.Copysign(x.Size().H/2, next.Y-x.Pos().Y)}
			}
			return gw.Point{X: x.Pos().X + math.Copysign(x.Size().W/2, next.X-x.Pos().X), Y: next.Y}
		}
		ps := []gw.Point{side(ed.From(), b.At(0))}
		for k := range b.Len() {
			ps = append(ps, b.At(k))
		}
		return append(ps, side(ed.To(), b.At(b.Len()-1)))
	}
	a, b := route(e), route(o)
	for i := 1; i < len(a); i++ {
		for j := 1; j < len(b); j++ {
			if p, ok := segCross(a[i-1], a[i], b[j-1], b[j]); ok {
				return p, true
			}
		}
	}
	return gw.Point{}, false
}

// segCross is where segments pq and rs cross strictly inside both.
func segCross(p, q, r, s gw.Point) (gw.Point, bool) {
	dx1, dy1, dx2, dy2 := q.X-p.X, q.Y-p.Y, s.X-r.X, s.Y-r.Y
	den := dx1*dy2 - dy1*dx2
	if den == 0 {
		return gw.Point{}, false
	}
	t := ((r.X-p.X)*dy2 - (r.Y-p.Y)*dx2) / den
	u := ((r.X-p.X)*dy1 - (r.Y-p.Y)*dx1) / den
	if t <= 1e-9 || t >= 1-1e-9 || u <= 1e-9 || u >= 1-1e-9 {
		return gw.Point{}, false
	}
	return gw.Point{X: p.X + t*dx1, Y: p.Y + t*dy1}, true
}

// TestUprightsNoRoom: where the bends left in the document's slab do not
// hold a lane's uprights, it keeps the bends of its dummy alone rather
// than losing its route: the slab of 11 edges with 2 bends each holds
// three lanes' six bends and one lane's two.
func TestUprightsNoRoom(t *testing.T) {
	d, es := wideDoc(t, gw.LeftToRight, 4, 6, false, gw.Caps{Edges: 11, BendsPerEdge: 2})
	n := make([]int, 0, len(es))
	for _, e := range es {
		n = append(n, d.Edge(e).Bends().Len())
	}
	if n[0] != 6 || n[1] != 6 || n[2] != 6 || n[3] != 2 {
		t.Errorf("bends per lane %v, want 6, 6, 6 and 2", n)
	}
}

// TestUprightsBackAndInside: lanes run back against the flow, from c to
// a, have their uprights at the start of their routes, out of c, level
// with c's end where it is drawn; and lanes into a container's member
// that the runs inside the container take on bring their uprights level
// with the first point of those runs rather than with the member, where
// the route would hook back. In either flow.
func TestUprightsBackAndInside(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		across := func(p gw.Point) float64 { return p.Y }
		along := func(p gw.Point) float64 { return p.X }
		if f == gw.TopToBottom {
			across, along = along, across
		}
		d, es := wideDocWith(t, f, 2, 1, wideOpts{back: true}, gw.Caps{})
		for _, e := range es {
			b := d.Edge(e).Bends()
			p, q := b.At(0), b.At(1)
			want := across(d.Vertex(d.Edge(e).From()).Pos()) + d.EndOffset(e, false)
			if along(p) != along(q) || across(p) != want || math.Abs(across(q)-across(p)) < 100 {
				t.Errorf("flow %d, lane %d back: the route starts %v, %v; want an upright from %v across", f, e, p, q, want)
			}
		}
		d, es = wideDocWith(t, f, 2, 1, wideOpts{first: true}, gw.Caps{})
		for _, e := range es {
			b := d.Edge(e).Bends()
			k := -1
			for j := 1; j < b.Len(); j++ {
				if along(b.At(j-1)) == along(b.At(j)) && math.Abs(across(b.At(j))-across(b.At(j-1))) >= 100 {
					k = j
					break
				}
			}
			if k < 0 || k+1 >= b.Len() || across(b.At(k+1)) != across(b.At(k)) {
				t.Errorf("flow %d, lane %d into the container: no upright level with the run on: %v", f, e, b)
			}
		}
	}
}

// TestLabelCarriers: edges that cross a layer side by side, each with a
// label, stand their dummies apart by the room the label takes across
// the flow, so that the labels, drawn on the runs through them, clear
// each other: top to bottom, beside the run as wide as the text, they
// lay across each other's runs when the dummies stood the gap (24)
// apart; left to right, a line above the run, the gap holds them.
func TestLabelCarriers(t *testing.T) {
	const label = "a label of twenty ch"
	for _, f := range []gw.Flow{gw.TopToBottom, gw.LeftToRight} {
		s := gw.Schema{VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 120, H: 40}}}, EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}}}
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(f)
		add := func() gw.VertexIndex {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex})
			if err != nil {
				t.Fatal(err)
			}
			return ch.Vertices[0]
		}
		m, tv := add(), add()
		es := make([]gw.EdgeIndex, 0, 3)
		for range 3 {
			sv := add()
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: sv, To: m, FromPort: gw.None, ToPort: gw.None}); err != nil {
				t.Fatal(err)
			}
			ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: sv, To: tv, FromPort: gw.None, ToPort: gw.None, Text: label})
			if err != nil {
				t.Fatal(err)
			}
			es = append(es, ch.Edges[0])
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: m, To: tv, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		for i, e := range es {
			a := svg.EdgeLabelBox(d, e)
			for _, o := range es[i+1:] {
				if b := svg.EdgeLabelBox(d, o); a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
					t.Errorf("flow %d: the labels of edges %d and %d overlap: %+v, %+v", f, e, o, a, b)
				}
			}
		}
	}
}
