package layout_test

import (
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

const (
	kSource gw.KindIndex = iota
	kNode
)

func schema(acyclic bool) gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 96, H: 24}, Root: true},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "to", Arrow: true, Route: gw.Curve}},
		Acyclic:   acyclic,
	}
}

// randomDAG makes a document of n vertices and about m edges that go
// from a lower index to a higher one, from a seed.
func randomDAG(t testing.TB, seed uint64, n, m int, caps gw.Caps) *gw.Document {
	t.Helper()
	d, err := gw.New(schema(true), caps)
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(seed, 1))
	for i := 0; i < n; i++ {
		kind := kNode
		if i < max(1, n/10) {
			kind = kSource
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, Text: "v" + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	for tries := 0; d.LiveEdges() < m && tries < 10*m; tries++ {
		a, b := r.IntN(n), r.IntN(n)
		if a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		_, _ = d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: gw.VertexIndex(a), To: gw.VertexIndex(b), FromPort: gw.None, ToPort: gw.None}) //nolint:gosec // below n
	}
	return d
}

// checkInvariants: no two live vertices overlap, every edge that was
// not reversed goes further along the flow, and every position is on
// the grid.
func checkInvariants(t *testing.T, name string, d *gw.Document, res layout.Result, opts layout.Options) {
	t.Helper()
	grid := opts.Grid
	if grid == 0 {
		grid = 8
	}
	for i := range d.Vertices {
		a := &d.Vertices[i]
		if !a.Live {
			continue
		}
		if !a.Placed && (math.Mod(a.Pos.X, grid) != 0 || math.Mod(a.Pos.Y, grid) != 0) {
			t.Errorf("%s: %s off the grid at %v", name, a.ID, a.Pos)
		}
		for j := i + 1; j < len(d.Vertices); j++ {
			b := &d.Vertices[j]
			if b.Live && math.Abs(a.Pos.X-b.Pos.X) < (a.Size.W+b.Size.W)/2 && math.Abs(a.Pos.Y-b.Pos.Y) < (a.Size.H+b.Size.H)/2 {
				t.Errorf("%s: %s at %v overlaps %s at %v", name, a.ID, a.Pos, b.ID, b.Pos)
			}
		}
	}
	for i := range d.Edges {
		e := &d.Edges[i]
		if !e.Live || reversed(res, gw.EdgeIndex(i)) || d.Vertices[e.From].Placed || d.Vertices[e.To].Placed { //nolint:gosec // bounded
			continue
		}
		if lay(d, d.Vertices[e.To].Pos) <= lay(d, d.Vertices[e.From].Pos) {
			t.Errorf("%s: edge %s goes backwards: %v to %v", name, e.ID, d.Vertices[e.From].Pos, d.Vertices[e.To].Pos)
		}
	}
}

// lay is how far along the document's flow a point lies: its x for
// left to right, its y for top to bottom.
func lay(d *gw.Document, p gw.Point) float64 {
	if d.Flow == gw.TopToBottom {
		return p.Y
	}
	return p.X
}

func reversed(res layout.Result, e gw.EdgeIndex) bool {
	for _, r := range res.Reversed {
		if r == e {
			return true
		}
	}
	return false
}

func positions(d *gw.Document) string {
	var b strings.Builder
	for i := range d.Vertices {
		if v := &d.Vertices[i]; v.Live {
			b.WriteString(string(v.ID) + "@" + strconv.FormatFloat(v.Pos.X, 'g', -1, 64) + "," + strconv.FormatFloat(v.Pos.Y, 'g', -1, 64) + " ")
		}
	}
	return b.String()
}

// TestInvariantsOnRandomDAGs: seeds 1 to 8, each laid out with and
// without sweeps: the invariants hold, the sweeps never raise the
// crossing count, the layout is the same twice and after Compact.
func TestInvariantsOnRandomDAGs(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		name := "seed " + strconv.FormatUint(seed, 10)
		d := randomDAG(t, seed, 40, 70, gw.Caps{})
		l := layout.New(d.Caps())
		none, err := l.Run(d, layout.Options{Sweeps: 1})
		if err != nil {
			t.Fatal(err)
		}
		res, err := l.Run(d, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		checkInvariants(t, name, d, res, layout.Options{})
		if res.Crossings > none.Crossings {
			t.Errorf("%s: %d crossings after sweeps, %d after one", name, res.Crossings, none.Crossings)
		}
		if len(res.Reversed) != 0 || res.Layers < 2 {
			t.Errorf("%s: reversed %v layers %d", name, res.Reversed, res.Layers)
		}
		first := positions(d)
		if _, err := l.Run(d, layout.Options{}); err != nil || positions(d) != first {
			t.Errorf("%s: a second layout differs", name)
		}
		if _, err := d.Apply(gw.Command{Op: gw.DeleteVertex, Vertex: 3}); err != nil {
			t.Fatal(err)
		}
		if _, err := l.Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
		before := positions(d)
		d.Compact()
		if _, err := l.Run(d, layout.Options{}); err != nil || positions(d) != before {
			t.Errorf("%s: layout after Compact differs:\n%s\n%s", name, before, positions(d))
		}
	}
}

// TestPlacedStay: a placed vertex keeps its position and its layer
// comes from its x; All replaces every position.
func TestPlacedStay(t *testing.T) {
	d := randomDAG(t, 3, 12, 16, gw.Caps{})
	l := layout.New(d.Caps())
	if _, err := l.Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	pin := gw.Point{X: 333, Y: 77}
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: 5, Pos: pin}); err != nil {
		t.Fatal(err)
	}
	res, err := l.Run(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Vertices[5].Pos != pin || !d.Vertices[5].Placed {
		t.Errorf("placed vertex moved to %v", d.Vertices[5].Pos)
	}
	checkInvariants(t, "placed", d, res, layout.Options{})
	if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	if d.Vertices[5].Pos == pin || d.Vertices[5].Placed {
		t.Errorf("all left the pin: %v placed %v", d.Vertices[5].Pos, d.Vertices[5].Placed)
	}
}

// TestCycleReversed: a document that may hold cycles has the edge that
// closes one reversed for the layout, and the rest laid out forward.
func TestCycleReversed(t *testing.T) {
	d, err := gw.New(schema(false), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode, ID: gw.ID(string(rune('a' + i)))}); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range [][2]gw.VertexIndex{{0, 1}, {1, 2}, {2, 0}} {
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: p[0], To: p[1], FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	l := layout.New(d.Caps())
	res, err := l.Run(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Reversed) != 1 || res.Reversed[0] != 2 || res.Layers != 3 {
		t.Errorf("reversed %v layers %d", res.Reversed, res.Layers)
	}
	checkInvariants(t, "cycle", d, res, layout.Options{})
	// The reversed edge from c back to a passes b's layer, so it is
	// routed around b, against the flow.
	if len(d.Edges[2].Bends) == 0 {
		t.Error("the reversed edge has no route around the layer it crosses")
	}
	checkBends(t, "cycle", d, res)
}

// TestCrossingsCounted: the complete bipartite graph K(3,3) lies in two
// layers with C(3,2)² = 9 crossings in any order, and of two edges that
// cross in the order the vertices were added, the sweeps uncross one
// layer against the other.
func TestCrossingsCounted(t *testing.T) {
	bipartite := func(edges [][2]gw.VertexIndex, n int) (*gw.Document, layout.Result) {
		t.Helper()
		d, err := gw.New(schema(true), gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2*n; i++ {
			kind := kNode
			if i < n {
				kind = kSource
			}
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, Text: "v" + strconv.Itoa(i)}); err != nil {
				t.Fatal(err)
			}
		}
		for _, p := range edges {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: p[0], To: p[1], FromPort: gw.None, ToPort: gw.None}); err != nil {
				t.Fatal(err)
			}
		}
		res, err := layout.New(d.Caps()).Run(d, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return d, res
	}
	var all [][2]gw.VertexIndex
	for i := gw.VertexIndex(0); i < 3; i++ {
		for j := gw.VertexIndex(3); j < 6; j++ {
			all = append(all, [2]gw.VertexIndex{i, j})
		}
	}
	if _, res := bipartite(all, 3); res.Layers != 2 || res.Crossings != 9 {
		t.Errorf("K(3,3): layers %d crossings %d, want 2 and 9", res.Layers, res.Crossings)
	}
	d, res := bipartite([][2]gw.VertexIndex{{0, 3}, {1, 2}}, 2)
	v := d.Vertices
	if res.Crossings != 0 || (v[0].Pos.Y < v[1].Pos.Y) != (v[3].Pos.Y < v[2].Pos.Y) {
		t.Errorf("crossed pair: %d crossings, sources at y %v %v, targets at y %v %v", res.Crossings, v[0].Pos.Y, v[1].Pos.Y, v[3].Pos.Y, v[2].Pos.Y)
	}
}

// TestDummyBudget: an edge spanning more layers than the budget allows
// is refused with the numbers.
func TestDummyBudget(t *testing.T) {
	d, err := gw.New(schema(true), gw.Caps{Vertices: 16, Edges: 1})
	if err != nil {
		t.Fatal(err)
	}
	var prev gw.VertexIndex = gw.None
	for i := 0; i < 12; i++ {
		v, _ := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode, ID: gw.ID("n" + strconv.Itoa(i))})
		if i > 0 {
			if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: v.Vertices[0], Pos: gw.Point{X: float64(i) * 160}}); err != nil {
				t.Fatal(err)
			}
		}
		prev = v.Vertices[0]
	}
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: 0, To: prev, FromPort: gw.None, ToPort: gw.None}); err != nil {
		t.Fatal(err)
	}
	l := layout.New(d.Caps())
	_, err = l.Run(d, layout.Options{})
	if err == nil || !strings.Contains(err.Error(), "layout needs 9 dummy vertices, over the budget of 8") {
		t.Errorf("got %v", err)
	}
	big := layout.New(gw.Caps{Vertices: 4, Edges: 1})
	if _, err := big.Run(d, layout.Options{}); err == nil || !strings.Contains(err.Error(), "exceeds the layouter's capacities") {
		t.Errorf("capacity: %v", err)
	}
}

// TestOutOfRange: capacities no document can have make a layouter of
// the defaults, which lays out what fits and refuses what does not, and
// a pitch, gap or grid that is NaN lays out as the default does.
func TestOutOfRange(t *testing.T) {
	d := randomDAG(t, 3, 40, 70, gw.Caps{})
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	want := positions(d)
	for _, caps := range []gw.Caps{{Vertices: -1, Edges: -1}, {Vertices: math.MaxInt, Edges: math.MaxInt}, {Vertices: gw.MaxCap + 1, Edges: 1 << 30}} {
		if _, err := layout.New(caps).Run(d, layout.Options{}); err != nil || positions(d) != want {
			t.Errorf("caps %+v: %v", caps, err)
		}
	}
	big := randomDAG(t, 3, 5000, 10, gw.Caps{Vertices: 5000})
	if _, err := layout.New(gw.Caps{Vertices: -1}).Run(big, layout.Options{}); err == nil || !strings.Contains(err.Error(), "exceeds the layouter's capacities") {
		t.Errorf("beyond the default: %v", err)
	}
	nan := math.NaN()
	for _, opts := range []layout.Options{{Pitch: nan}, {Gap: nan}, {Grid: nan}} {
		if _, err := layout.New(d.Caps()).Run(d, opts); err != nil || positions(d) != want {
			t.Errorf("options %+v: %v", opts, err)
		}
	}
}

// TestDummyBudgetWide: 64 vertices placed at x = 0 and 64 at 528 000
// layers' pitch, within the layer budget of 528 384, each of the first
// joined to each of the second, need 4096 × 527 999 dummies, past 2^31;
// the layout is refused with the whole count, where an int has 32 bits
// too.
func TestDummyBudgetWide(t *testing.T) {
	d, err := gw.New(schema(true), gw.Caps{Edges: 65536})
	if err != nil {
		t.Fatal(err)
	}
	const k = 64
	for i := range 2 * k {
		x := 0.0
		if i >= k {
			x = 528000 * 160
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: gw.VertexIndex(i), Pos: gw.Point{X: x, Y: float64(50 * i)}}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range k {
		for j := k; j < 2*k; j++ {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: gw.VertexIndex(i), To: gw.VertexIndex(j), FromPort: gw.None, ToPort: gw.None}); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err = layout.New(d.Caps()).Run(d, layout.Options{})
	if err == nil || !strings.Contains(err.Error(), "layout needs 2162683904 dummy vertices") {
		t.Errorf("got %v", err)
	}
}

// TestLayerBudget: two vertices placed at x = 0 and MaxCoord, with no
// edge between them, lie 2^20 layers apart, beyond the arrays by layer;
// the layout is refused with both numbers.
func TestLayerBudget(t *testing.T) {
	d, err := gw.New(schema(true), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range []float64{0, gw.MaxCoord} {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: gw.VertexIndex(i), Pos: gw.Point{X: x}}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = layout.New(d.Caps()).Run(d, layout.Options{})
	if err == nil || !strings.Contains(err.Error(), "layout needs 1048577 layers, over the budget of 135168") {
		t.Errorf("got %v", err)
	}
}

// TestLayoutBudget: 2 000 vertices and 5 000 edges lay out in under
// 200 ms, skipped under the race detector.
func TestLayoutBudget(t *testing.T) {
	if os.Getenv("GRAPHEDWRIGHT_RACE") == "1" {
		t.Skip("timing under the race detector")
	}
	d := randomDAG(t, 9, 2000, 5000, gw.Caps{Vertices: 2000, Edges: 5000})
	l := layout.New(d.Caps())
	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := l.Run(d, layout.Options{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	per := time.Duration(res.NsPerOp())
	t.Logf("layout of %d vertices and %d edges: %v", d.LiveVertices(), d.LiveEdges(), per)
	if per > 200*time.Millisecond {
		t.Errorf("layout took %v, over the 200 ms budget", per)
	}
}

// TestAllocLayout: a layouter runs without allocating.
func TestAllocLayout(t *testing.T) {
	d := randomDAG(t, 4, 30, 50, gw.Caps{})
	l := layout.New(d.Caps())
	if n := testing.AllocsPerRun(10, func() {
		if _, err := l.Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
	}); n > 0 {
		t.Errorf("%g allocations per run", n)
	}
}

// TestLabelRoom: an edge between adjacent layers has its label drawn in
// the gap between them, so the gap is as wide as the label and its room
// either side; in a top-to-bottom flow, labels are beside their edges
// and the layers keep the pitch.
func TestLabelRoom(t *testing.T) {
	const label = "a label of thirty characters.."
	for _, flow := range []string{"left-to-right", "top-to-bottom"} {
		d, err := gw.New(schema(false), gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None, Text: label}); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: flow}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		a, b := d.Vertices[0], d.Vertices[1]
		gap, want := b.Pos.X-a.Pos.X-(a.Size.W+b.Size.W)/2, float64(len(label)*7+16)
		if d.Flow == gw.TopToBottom {
			gap, want = b.Pos.Y-a.Pos.Y, 96
		}
		if gap < want || gap >= want+8 {
			t.Errorf("%s: the gap is %g, want %g out to the grid", flow, gap, want)
		}
	}
}
