package layout_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// runLabel is the label the label runs are tested with: 20 runes, 156
// wide with its room.
const runLabel = "a label of 20 runes."

// runDoc lays out, in flow f, vertices of kind node, the edges, each a
// from and to by index, labelled where it is the first, of the route
// given; and returns the document.
func runDoc(t *testing.T, f gw.Flow, route gw.Route, n int, edges ...[2]int) *gw.Document {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "line", Route: route}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for range n {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
			t.Fatal(err)
		}
	}
	for k, e := range edges {
		text := ""
		if k == 0 {
			text = runLabel
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: gw.VertexIndex(e[0]), To: gw.VertexIndex(e[1]), FromPort: gw.None, ToPort: gw.None, Text: text}); err != nil { //nolint:gosec // a few
			t.Fatal(err)
		}
	}
	d.SetFlow(f)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	return d
}

// TestLabelRun: a labelled edge between adjacent layers whose ends are
// at different heights runs level out of the end at the side with fewer
// ends, the source where they have as many, at the height that end is drawn at, from a grid step off it for
// at least as long as its label, and climbs to the other end in the last
// 64 of the gap, less up to a grid step to bring the run out to the grid;
// in either flow, from its source or its target, toward a later layer or
// an earlier one. An edge that does not climb, or is straight, has no
// run.
func TestLabelRun(t *testing.T) {
	const width = 20*7 + 16
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, c := range []struct {
			name  string
			n     int
			edges [][2]int
			at    int // the vertex the run is at
		}{
			{"from the source", 4, [][2]int{{1, 3}, {0, 3}, {2, 3}}, 1},
			{"from the source, as many at each end", 5, [][2]int{{1, 3}, {0, 3}, {1, 4}}, 1},
			{"from the target", 5, [][2]int{{0, 2}, {0, 3}, {0, 4}, {1, 2}}, 2},
			{"back from the source", 5, [][2]int{{2, 0}, {0, 3}, {0, 4}, {1, 2}, {0, 2}}, 2},
		} {
			d := runDoc(t, f, gw.Orthogonal, c.n, c.edges...)
			b := d.Edge(0).Bends()
			at, other := d.Vertex(gw.VertexIndex(c.at)), d.Vertex(d.Edge(0).To()) //nolint:gosec // a few
			if c.at == int(d.Edge(0).To()) {
				other = d.Vertex(d.Edge(0).From())
			}
			ea, ec := frameOf(f, at.Pos())
			oa, _ := frameOf(f, other.Pos())
			half, _ := frameOf(f, gw.Point{X: at.Size().W / 2, Y: at.Size().H / 2})
			dir := 1.0
			if oa < ea {
				dir = -1
			}
			if b.Len() != 2 {
				t.Errorf("flow %d, %s: %d bends, want 2", f, c.name, b.Len())
				continue
			}
			a0, c0 := frameOf(f, b.At(0))
			a1, c1 := frameOf(f, b.At(1))
			if c.at == int(d.Edge(0).To()) {
				a0, a1 = a1, a0
			}
			if end := ec + d.EndOffset(0, c.at == int(d.Edge(0).To())); c0 != end || c1 != end {
				t.Errorf("flow %d, %s: the run is at %g and %g across, want %g", f, c.name, c0, c1, end)
			}
			if a0 != ea+dir*(half+8) || (a1-a0)*dir < width || (oa-dir*half-a1)*dir < 64-8 {
				t.Errorf("flow %d, %s: the run goes from %g to %g, want from %g at least %d, and 64 less a grid step short of %g", f, c.name, a0, a1, ea+dir*(half+8), width, oa-dir*half)
			}
		}
		for _, c := range []struct {
			name  string
			route gw.Route
			n     int
			edges [][2]int
		}{
			{"level", gw.Orthogonal, 2, [][2]int{{0, 1}}},
			{"straight", gw.Straight, 3, [][2]int{{0, 2}, {1, 2}}},
		} {
			if d := runDoc(t, f, c.route, c.n, c.edges...); d.Edge(0).Bends().Len() != 0 {
				t.Errorf("flow %d, %s: %d bends, want none", f, c.name, d.Edge(0).Bends().Len())
			}
		}
	}
}

// TestLabelRunPlaced: an edge with a placed end has no label run, the
// end standing where it was put rather than in its layer's band, which
// the run is measured from; with every vertex but the run's own end
// placed where it was laid out, at the source or at the target.
func TestLabelRunPlaced(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, c := range []struct {
			name  string
			edges [][2]int
		}{
			{"the source placed", [][2]int{{0, 2}, {0, 3}, {0, 4}, {1, 2}}},
			{"the target placed", [][2]int{{2, 0}, {0, 3}, {0, 4}, {1, 2}, {0, 2}}},
		} {
			d := runDoc(t, f, gw.Orthogonal, 5, c.edges...)
			if d.Edge(0).Bends().Len() != 2 {
				t.Fatalf("flow %d, %s: no run to start with", f, c.name)
			}
			for v := range gw.VertexIndex(5) {
				if v == 2 {
					continue // the run's end
				}
				if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: v, Pos: d.Vertex(v).Pos()}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
				t.Fatal(err)
			}
			if n := d.Edge(0).Bends().Len(); n != 0 {
				t.Errorf("flow %d, %s: %d bends, want none", f, c.name, n)
			}
		}
	}
}

// TestLabelRunInZone: the room a label run in a zone takes is given to
// the gap in the zone, not to the top level's: the gap from the zone to
// the vertex after it is as wide with the label as without.
func TestLabelRunInZone(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		var gaps [2]float64
		for k, text := range [2]string{runLabel, ""} {
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
			for range 5 {
				if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
					t.Fatal(err)
				}
			}
			var zx [2]gw.VertexIndex // the zone and the vertex after it
			for k, kind := range [2]gw.KindIndex{1, 0} {
				ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind})
				if err != nil {
					t.Fatal(err)
				}
				zx[k] = ch.Vertices[0]
			}
			for v := range gw.VertexIndex(5) {
				if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: zx[0]}); err != nil {
					t.Fatal(err)
				}
			}
			for j, e := range [5][2]gw.VertexIndex{{0, 2}, {0, 3}, {0, 4}, {1, 2}, {zx[0], zx[1]}} {
				label := ""
				if j == 0 {
					label = text
				}
				if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e[0], To: e[1], FromPort: gw.None, ToPort: gw.None, Text: label}); err != nil {
					t.Fatal(err)
				}
			}
			d.SetFlow(f)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			if text != "" && d.Edge(0).Bends().Len() != 2 {
				t.Errorf("flow %d: the edge in the zone has %d bends, want its run", f, d.Edge(0).Bends().Len())
			}
			_, z1, _, _ := box(f, d.Vertex(zx[0]))
			x0, _, _, _ := box(f, d.Vertex(zx[1]))
			gaps[k] = x0 - z1
		}
		if gaps[0] != gaps[1] {
			t.Errorf("flow %d: the gap after the zone is %g with the label, %g without", f, gaps[0], gaps[1])
		}
	}
}

// TestLabelRunPort: an edge whose end is at a port has no label run: the
// end is drawn at the port, not spread along its side, at the source or
// at the target; each end the run would be at, without the port.
func TestLabelRunPort(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		on, back := gw.Right, gw.Left
		if f == gw.TopToBottom {
			on, back = gw.Bottom, gw.Top
		}
		for _, c := range []struct {
			name   string
			n      int
			edges  [][2]int
			ported int // the vertex at the port, the run's end
			side   gw.Side
		}{
			{"from a port", 4, [][2]int{{1, 3}, {0, 3}, {2, 3}}, 1, on},
			{"to a port", 5, [][2]int{{0, 2}, {0, 3}, {0, 4}, {1, 2}}, 2, back},
		} {
			s := gw.Schema{
				VertexKinds: []gw.VertexKind{
					{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}},
					{Name: "ported", Shape: gw.Rect, Size: gw.Size{W: 60, H: 60}, Ports: []gw.Port{{Name: "p", Side: c.side}}},
				},
				EdgeKinds: []gw.EdgeKind{{Name: "line", Route: gw.Orthogonal}},
			}
			d, err := makeDoc(s, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			for v := range c.n {
				kind := gw.KindIndex(0)
				if v == c.ported {
					kind = 1
				}
				if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind}); err != nil {
					t.Fatal(err)
				}
			}
			for k, e := range c.edges {
				cmd := gw.Command{Op: gw.AddEdge, From: gw.VertexIndex(e[0]), To: gw.VertexIndex(e[1]), FromPort: gw.None, ToPort: gw.None} //nolint:gosec // a few
				if k == 0 {
					cmd.Text = runLabel
				}
				if e[0] == c.ported {
					cmd.FromPort = 0
				}
				if e[1] == c.ported {
					cmd.ToPort = 0
				}
				if _, err := d.Apply(cmd); err != nil {
					t.Fatal(err)
				}
			}
			d.SetFlow(f)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			_, ca := frameOf(f, d.Vertex(d.Edge(0).From()).Pos())
			_, cb := frameOf(f, d.Vertex(d.Edge(0).To()).Pos())
			if ca-cb <= 16 && cb-ca <= 16 {
				t.Fatalf("flow %d, %s: the edge does not climb", f, c.name)
			}
			if n := d.Edge(0).Bends().Len(); n != 0 {
				t.Errorf("flow %d, %s: %d bends, want none", f, c.name, n)
			}
		}
	}
}

// TestLabelRunReused: a layouter that gave an edge a label run lays out
// the next document as a new one does, where the edge of that index is
// long and climbs, with no room for a run.
func TestLabelRunReused(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		first := runDoc(t, f, gw.Orthogonal, 5, [][2]int{{0, 2}, {0, 3}, {0, 4}, {1, 2}}...)
		if first.Edge(0).Bends().Len() != 2 {
			t.Fatalf("flow %d: no run to start with", f)
		}
		next := func() *gw.Document {
			return runDoc(t, f, gw.Orthogonal, 5, [][2]int{{0, 2}, {0, 1}, {1, 2}, {3, 2}, {4, 2}}...)
		}
		fresh, reused := next(), next()
		l := layout.New(first.Caps())
		for _, d := range []*gw.Document{first, reused} {
			if _, err := l.Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
		}
		if positions(reused) != positions(fresh) {
			t.Errorf("flow %d: reused %s, new %s", f, positions(reused), positions(fresh))
		}
	}
}
