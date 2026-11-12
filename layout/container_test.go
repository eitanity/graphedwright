package layout_test

import (
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

const kZone = kNode + 1

// nested makes a random DAG of n vertices and about m edges, then puts
// vertices into zones: some zones at the top, some in other zones, and
// every vertex either at the top or in a zone, from a seed.
func nested(t *testing.T, seed uint64, n, m, zones int) *gw.Document {
	t.Helper()
	s := schema(true)
	s.VertexKinds = append(s.VertexKinds, gw.VertexKind{Name: "zone", Size: gw.Size{W: 60, H: 40}, Container: true})
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	src := randomDAG(t, seed, n, m, gw.Caps{})
	for i := range src.VertexSlots() {
		v := src.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: v.Kind(), Text: v.Label()}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range src.EdgeSlots() {
		e := src.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e.From(), To: e.To(), FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	r := rand.New(rand.NewPCG(seed, 2))
	first := gw.VertexIndex(d.VertexSlots()) //nolint:gosec // below the capacity
	for z := 0; z < zones; z++ {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kZone, Text: "z" + strconv.Itoa(z)})
		if err != nil {
			t.Fatal(err)
		}
		if z > 0 && r.IntN(2) == 0 {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: ch.Vertices[0], To: first + gw.VertexIndex(r.IntN(z))}); err != nil { //nolint:gosec // below the zones
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < n; i++ {
		if k := r.IntN(zones + 1); k < zones {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: gw.VertexIndex(i), To: first + gw.VertexIndex(k)}); err != nil { //nolint:gosec // below n
				t.Fatal(err)
			}
		}
	}
	return d
}

// inside reports whether a's box lies within b's.
func inside(a, b gw.VertexView) bool {
	return a.Pos().X-a.Size().W/2 >= b.Pos().X-b.Size().W/2 && a.Pos().X+a.Size().W/2 <= b.Pos().X+b.Size().W/2 &&
		a.Pos().Y-a.Size().H/2 >= b.Pos().Y-b.Size().H/2 && a.Pos().Y+a.Size().H/2 <= b.Pos().Y+b.Size().H/2
}

// TestContainers: on random DAGs with nested zones, every vertex lies
// inside its zone, no two vertices of the same level overlap, so no
// vertex lies inside a zone it is not in, and the layout is the same
// twice.
func TestContainers(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		name := "seed " + strconv.FormatUint(seed, 10)
		d := nested(t, seed, 30, 50, 5)
		checkContainers(t, name, d)
	}
}

// TestContainersOwnSize: zones given their own sizes, some larger than
// what they hold, are laid out at those sizes as TestContainers checks.
func TestContainersOwnSize(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		name := "own size, seed " + strconv.FormatUint(seed, 10)
		d := nested(t, seed, 30, 50, 5)
		for z := 30; z < 35; z += 2 {
			if _, err := d.Apply(gw.Command{Op: gw.Resize, Vertex: gw.VertexIndex(z), Size: gw.Size{W: 500, H: 260}}); err != nil { //nolint:gosec // below the vertices
				t.Fatal(err)
			}
		}
		checkContainers(t, name, d)
		if v := d.Vertex(30); v.Size().W < 500 || v.Size().H < 260 {
			t.Errorf("%s: zone z0 %v below its own size", name, v.Size())
		}
	}
}

// checkContainers lays d out and checks every vertex is inside its
// zone, no two of the same level overlap, and a second layout is the
// same.
func checkContainers(t *testing.T, name string, d *gw.Document) {
	t.Helper()
	l := layout.New(d.Caps())
	if _, err := l.Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	for i := range d.VertexSlots() {
		a := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if !a.Live() {
			continue
		}
		if a.Parent() != gw.None && !inside(a, d.Vertex(a.Parent())) {
			t.Errorf("%s: %s %v %v outside its zone %s %v %v", name, a.Label(), a.Pos(), a.Size(), d.Vertex(a.Parent()).Label(), d.Vertex(a.Parent()).Pos(), d.Vertex(a.Parent()).Size())
		}
		for j := i + 1; j < d.VertexSlots(); j++ {
			b := d.Vertex(gw.VertexIndex(j)) //nolint:gosec // bounded by the capacity
			if b.Live() && a.Parent() == b.Parent() && overlap(a, b) {
				t.Errorf("%s: %s %v %v overlaps %s %v %v", name, a.Label(), a.Pos(), a.Size(), b.Label(), b.Pos(), b.Size())
			}
		}
	}
	first := positions(d)
	if _, err := l.Run(d, layout.Options{All: true}); err != nil || positions(d) != first {
		t.Errorf("%s: a second layout differs", name)
	}
}

// TestContainerPlaced: a placed vertex in a placed zone stays where it
// is through a layout that leaves placed vertices, while the others in
// the zone are laid out around it.
func TestContainerPlaced(t *testing.T) {
	d := nested(t, 2, 12, 16, 1)
	z := gw.VertexIndex(12)
	l := layout.New(d.Caps())
	if _, err := l.Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	var in gw.VertexIndex = gw.None
	for i := range 12 {
		if d.Vertex(gw.VertexIndex(i)).Parent() == z { //nolint:gosec // bounded by the capacity
			in = gw.VertexIndex(i)
			break
		}
	}
	if in == gw.None {
		t.Fatal("no vertex in the zone")
	}
	at := gw.Point{X: d.Vertex(in).Pos().X + 40, Y: d.Vertex(in).Pos().Y - 200}
	d.Place(in, at, true)
	d.Place(z, d.Vertex(z).Pos(), true)
	d.Fit()
	if _, err := l.Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	if d.Vertex(in).Pos() != at || !d.Vertex(in).Placed() || !inside(d.Vertex(in), d.Vertex(z)) {
		t.Errorf("placed vertex in a placed zone moved: %v, was %v; zone %v %v", d.Vertex(in).Pos(), at, d.Vertex(z).Pos(), d.Vertex(z).Size())
	}
}

// overlap reports whether two vertices' boxes overlap.
func overlap(a, b gw.VertexView) bool {
	return math.Abs(a.Pos().X-b.Pos().X) < (a.Size().W+b.Size().W)/2 && math.Abs(a.Pos().Y-b.Pos().Y) < (a.Size().H+b.Size().H)/2
}

// TestContainerArrowRoom: edges from one vertex to each of six stacked
// in a zone are routed round the ones in the way, left to right, and
// each routed, in either flow, comes to its member level from at least
// an arrowhead's length out. The run along the zone's padding had its
// last bend 4 short of the member's side.
func TestContainerArrowRoom(t *testing.T) {
	const head = 9
	s := schema(true)
	s.VertexKinds = append(s.VertexKinds, gw.VertexKind{Name: "zone", Size: gw.Size{W: 60, H: 40}, Container: true})
	for _, flow := range []string{"left-to-right", "top-to-bottom"} {
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []gw.KindIndex{kSource, kZone, kNode, kNode, kNode, kNode, kNode, kNode} {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: k}); err != nil {
				t.Fatal(err)
			}
		}
		for v := gw.VertexIndex(2); v < 8; v++ {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: 0, To: v, FromPort: gw.None, ToPort: gw.None}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: flow}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		bent := 0
		for i := range d.EdgeSlots() {
			e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
			if e.Bends().Len() == 0 {
				continue
			}
			bent++
			last, to := e.Bends().At(e.Bends().Len()-1), d.Vertex(e.To())
			run, level := to.Pos().X-to.Size().W/2-last.X, last.Y == to.Pos().Y
			if d.Flow() == gw.TopToBottom {
				run, level = to.Pos().Y-to.Size().H/2-last.Y, last.X == to.Pos().X
			}
			if run < head || !level {
				t.Errorf("%s: edge %d comes to %v from %v, %g out", flow, i, to.Pos(), last, run)
			}
		}
		if bent == 0 && d.Flow() == gw.LeftToRight {
			t.Errorf("%s: no edge was routed round the members", flow)
		}
	}
}

// TestBandRoomGap: a zone holding a and, a layer on, b, beside y across
// the flow, stands out across it by the band room for an edge from
// outside to a or b, and not for one to the zone itself, or from the
// zone to a, as the document fits it; the layout makes room for it as
// the document does, so y stands the same gap off the zone either way.
func TestBandRoomGap(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		gap0, across0 := 0.0, 0.0
		for _, c := range []struct {
			name   string
			banded bool
		}{{"x to the zone", false}, {"x to b", true}, {"x to a", true}, {"and the zone to a", false}} {
			d, err := makeDoc(gw.Schema{
				VertexKinds: []gw.VertexKind{{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}}, {Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Container: true}},
				EdgeKinds:   []gw.EdgeKind{{Name: "line", Route: gw.Orthogonal}},
			}, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			add := func(k gw.KindIndex) gw.VertexIndex {
				t.Helper()
				ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: k})
				if err != nil {
					t.Fatal(err)
				}
				return ch.Vertices[0]
			}
			cmd := func(c gw.Command) {
				t.Helper()
				if _, err := d.Apply(c); err != nil {
					t.Fatal(err)
				}
			}
			edge := func(from, to gw.VertexIndex) {
				cmd(gw.Command{Op: gw.AddEdge, From: from, To: to, FromPort: gw.None, ToPort: gw.None})
			}
			z := add(1)
			a, b, x, y := add(0), add(0), add(0), add(0)
			cmd(gw.Command{Op: gw.SetParent, Vertex: a, To: z})
			cmd(gw.Command{Op: gw.SetParent, Vertex: b, To: z})
			edge(a, b)
			edge(x, y)
			switch c.name {
			case "x to b":
				edge(x, b)
			case "x to a":
				edge(x, a)
			case "and the zone to a":
				edge(x, z)
				edge(z, a)
			default:
				edge(x, z)
			}
			d.SetFlow(f)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			zv, yv := d.Vertex(z), d.Vertex(y)
			gap := math.Abs(zv.Pos().Y-yv.Pos().Y) - (zv.Size().H+yv.Size().H)/2
			across := zv.Size().H
			if f == gw.TopToBottom {
				gap = math.Abs(zv.Pos().X-yv.Pos().X) - (zv.Size().W+yv.Size().W)/2
				across = zv.Size().W
			}
			if gap0 == 0 {
				gap0, across0 = gap, across
			}
			want := across0
			if c.banded {
				want += 2 * gw.BandRoom
			}
			if gap != gap0 || across != want {
				t.Errorf("flow %d, %s: y %v off the zone, %v across, want %v off and %v across", f, c.name, gap, across, gap0, want)
			}
		}
	}
}

// TestTitleLinesLaidOut: a container whose title has two lines is laid
// out a line taller, with what it holds as far below its title as
// below one of one line, and the vertices beside it, across the flow
// left to right and after it along the flow top to bottom, as far off
// it as from one with a title of one line.
func TestTitleLinesLaidOut(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		var gap0, h0 float64
		for _, title := range []string{"zone", "zone\nof two lines"} {
			d, err := makeDoc(gw.Schema{
				VertexKinds: []gw.VertexKind{{Name: "node", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}}, {Name: "zone", Shape: gw.Rect, Size: gw.Size{W: 60, H: 40}, Container: true}},
				EdgeKinds:   []gw.EdgeKind{{Name: "line", Route: gw.Orthogonal}},
			}, gw.Caps{})
			if err != nil {
				t.Fatal(err)
			}
			add := func(k gw.KindIndex, text string) gw.VertexIndex {
				t.Helper()
				ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: k, Text: text})
				if err != nil {
					t.Fatal(err)
				}
				return ch.Vertices[0]
			}
			cmd := func(c gw.Command) {
				t.Helper()
				if _, err := d.Apply(c); err != nil {
					t.Fatal(err)
				}
			}
			z := add(1, title)
			a, x, y, w := add(0, "a"), add(0, "x"), add(0, "y"), add(0, "w")
			cmd(gw.Command{Op: gw.SetParent, Vertex: a, To: z})
			// x before the zone and y beside it, w after it.
			cmd(gw.Command{Op: gw.AddEdge, From: x, To: z, FromPort: gw.None, ToPort: gw.None})
			cmd(gw.Command{Op: gw.AddEdge, From: x, To: y, FromPort: gw.None, ToPort: gw.None})
			cmd(gw.Command{Op: gw.AddEdge, From: z, To: w, FromPort: gw.None, ToPort: gw.None})
			d.SetFlow(f)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			zv, av := d.Vertex(z), d.Vertex(a)
			top := zv.Pos().Y - zv.Size().H/2
			if got, want := av.Pos().Y-av.Size().H/2-top, gw.ContainerPad+d.TitleHeight(z); math.Abs(got-want) > 1e-9 {
				t.Errorf("flow %v, %q: what the zone holds %v below its top, want %v", f, title, got, want)
			}
			// The gap across the flow left to right, which the title is
			// across; along it top to bottom.
			other := d.Vertex(y)
			if f == gw.TopToBottom {
				other = d.Vertex(w)
			}
			gap := math.Abs(zv.Pos().Y-other.Pos().Y) - (zv.Size().H+other.Size().H)/2
			if h0 == 0 {
				gap0, h0 = gap, zv.Size().H
				continue
			}
			if math.Abs(gap-gap0) > 1e-9 || math.Abs(zv.Size().H-(h0+gw.LabelLine)) > 1e-9 {
				t.Errorf("flow %v, %q: gap %v, zone %v high; want %v, %v", f, title, gap, zv.Size().H, gap0, h0+gw.LabelLine)
			}
		}
	}
}
