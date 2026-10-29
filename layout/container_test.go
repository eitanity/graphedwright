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
	d, err := gw.New(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	src := randomDAG(t, seed, n, m, gw.Caps{})
	for i := range src.Vertices {
		v := &src.Vertices[i]
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: v.Kind, Text: v.Label}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range src.Edges {
		e := &src.Edges[i]
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e.From, To: e.To, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	r := rand.New(rand.NewPCG(seed, 2))
	first := gw.VertexIndex(len(d.Vertices)) //nolint:gosec // below the capacity
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
func inside(a, b *gw.Vertex) bool {
	return a.Pos.X-a.Size.W/2 >= b.Pos.X-b.Size.W/2 && a.Pos.X+a.Size.W/2 <= b.Pos.X+b.Size.W/2 &&
		a.Pos.Y-a.Size.H/2 >= b.Pos.Y-b.Size.H/2 && a.Pos.Y+a.Size.H/2 <= b.Pos.Y+b.Size.H/2
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
		if v := d.Vertices[30]; v.Size.W < 500 || v.Size.H < 260 {
			t.Errorf("%s: zone z0 %v below its own size", name, v.Size)
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
	for i := range d.Vertices {
		a := &d.Vertices[i]
		if !a.Live {
			continue
		}
		if a.Parent != gw.None && !inside(a, &d.Vertices[a.Parent]) {
			t.Errorf("%s: %s %v %v outside its zone %s %v %v", name, a.Label, a.Pos, a.Size, d.Vertices[a.Parent].Label, d.Vertices[a.Parent].Pos, d.Vertices[a.Parent].Size)
		}
		for j := i + 1; j < len(d.Vertices); j++ {
			b := &d.Vertices[j]
			if b.Live && a.Parent == b.Parent && overlap(a, b) {
				t.Errorf("%s: %s %v %v overlaps %s %v %v", name, a.Label, a.Pos, a.Size, b.Label, b.Pos, b.Size)
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
		if d.Vertices[i].Parent == z {
			in = gw.VertexIndex(i)
			break
		}
	}
	if in == gw.None {
		t.Fatal("no vertex in the zone")
	}
	at := gw.Point{X: d.Vertices[in].Pos.X + 40, Y: d.Vertices[in].Pos.Y - 200}
	d.Place(in, at, true)
	d.Place(z, d.Vertices[z].Pos, true)
	d.Fit()
	if _, err := l.Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	if d.Vertices[in].Pos != at || !d.Vertices[in].Placed || !inside(&d.Vertices[in], &d.Vertices[z]) {
		t.Errorf("placed vertex in a placed zone moved: %v, was %v; zone %v %v", d.Vertices[in].Pos, at, d.Vertices[z].Pos, d.Vertices[z].Size)
	}
}

// overlap reports whether two vertices' boxes overlap.
func overlap(a, b *gw.Vertex) bool {
	return math.Abs(a.Pos.X-b.Pos.X) < (a.Size.W+b.Size.W)/2 && math.Abs(a.Pos.Y-b.Pos.Y) < (a.Size.H+b.Size.H)/2
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
		d, err := gw.New(s, gw.Caps{})
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
		for i := range d.Edges {
			e := &d.Edges[i]
			if len(e.Bends) == 0 {
				continue
			}
			bent++
			last, to := e.Bends[len(e.Bends)-1], &d.Vertices[e.To]
			run, level := to.Pos.X-to.Size.W/2-last.X, last.Y == to.Pos.Y
			if d.Flow == gw.TopToBottom {
				run, level = to.Pos.Y-to.Size.H/2-last.Y, last.X == to.Pos.X
			}
			if run < head || !level {
				t.Errorf("%s: edge %d comes to %v from %v, %g out", flow, i, to.Pos, last, run)
			}
		}
		if bent == 0 && d.Flow == gw.LeftToRight {
			t.Errorf("%s: no edge was routed round the members", flow)
		}
	}
}
