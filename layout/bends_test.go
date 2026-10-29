package layout_test

import (
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// checkBends: an edge has bends exactly when a vertex of its level lies
// in a layer between its ends; they come in pairs, run strictly between
// its ends in the edge's own direction, against the flow for one
// reversed to break a cycle, and every leg between two bends misses
// every vertex but the ends and the containers that hold them.
func checkBends(t *testing.T, name string, d *gw.Document, res layout.Result) {
	t.Helper()
	for i := range d.Edges {
		e := &d.Edges[i]
		if !e.Live {
			continue
		}
		from, to := &d.Vertices[e.From], &d.Vertices[e.To]
		dir := 1.0
		if reversed(res, gw.EdgeIndex(i)) { //nolint:gosec // bounded
			dir = -1
		}
		if len(e.Bends)%2 != 0 {
			t.Errorf("%s: edge %s has %d bends", name, e.ID, len(e.Bends))
		}
		if between(d, e) && len(e.Bends) == 0 {
			t.Errorf("%s: edge %s from %v to %v crosses a layer without bends", name, e.ID, from.Pos, to.Pos)
		}
		for k, p := range e.Bends {
			if dir*lay(d, p) <= dir*lay(d, from.Pos) || dir*lay(d, p) >= dir*lay(d, to.Pos) || (k > 0 && dir*lay(d, p) < dir*lay(d, e.Bends[k-1])) {
				t.Errorf("%s: edge %s from %v to %v: bend %d at %v out of order", name, e.ID, from.Pos, to.Pos, k, p)
			}
		}
		for k := 1; k < len(e.Bends); k++ {
			for j := range d.Vertices {
				v := &d.Vertices[j]
				u := gw.VertexIndex(j) //nolint:gosec // bounded
				if !v.Live || d.Within(e.From, u) || d.Within(e.To, u) {
					continue
				}
				if crosses(e.Bends[k-1], e.Bends[k], v) {
					t.Errorf("%s: edge %s leg %v to %v crosses %s at %v %v", name, e.ID, e.Bends[k-1], e.Bends[k], v.Label, v.Pos, v.Size)
				}
			}
		}
	}
}

// between reports whether a vertex at the edge's level, holding neither
// end, has its centre between the edge's ends along the flow, by more
// than a grid step: a container's centre sits a little off its layer's,
// by the room its title takes, and a layer between is further than that.
func between(d *gw.Document, e *gw.Edge) bool {
	level := gw.VertexIndex(gw.None)
	// bound: the parent chain
	for c := d.Vertices[e.From].Parent; c != gw.None; c = d.Vertices[c].Parent {
		if d.Within(e.To, c) {
			level = c
			break
		}
	}
	a, b := hold(d, e.From, level), hold(d, e.To, level)
	lo, hi := lay(d, d.Vertices[a].Pos), lay(d, d.Vertices[b].Pos)
	if lo > hi {
		lo, hi = hi, lo
	}
	for j := range d.Vertices {
		v := &d.Vertices[j]
		if v.Live && v.Parent == level && lay(d, v.Pos) > lo+8 && lay(d, v.Pos) < hi-8 {
			return true
		}
	}
	return false
}

// hold is the vertex directly in c that is v or holds it.
func hold(d *gw.Document, v, c gw.VertexIndex) gw.VertexIndex {
	// bound: the parent chain
	for d.Vertices[v].Parent != c {
		v = d.Vertices[v].Parent
	}
	return v
}

// crosses reports whether the segment pq passes through the inside of
// the vertex's box, by clipping it to the box shrunk by half a unit.
func crosses(p, q gw.Point, v *gw.Vertex) bool {
	x0, x1 := v.Pos.X-v.Size.W/2+0.5, v.Pos.X+v.Size.W/2-0.5
	y0, y1 := v.Pos.Y-v.Size.H/2+0.5, v.Pos.Y+v.Size.H/2-0.5
	lo, hi := 0.0, 1.0
	dx, dy := q.X-p.X, q.Y-p.Y
	for _, c := range [4][2]float64{{-dx, p.X - x0}, {dx, x1 - p.X}, {-dy, p.Y - y0}, {dy, y1 - p.Y}} {
		den, num := c[0], c[1]
		if den == 0 {
			if num < 0 {
				return false
			}
			continue
		}
		t := num / den
		if den < 0 {
			lo = max(lo, t)
		} else {
			hi = min(hi, t)
		}
		if lo > hi {
			return false
		}
	}
	return true
}

// TestBendsOnRandomDAGs: on the random DAGs, flat and nested, every
// long edge is routed through bends clear of the vertices it passes.
func TestBendsOnRandomDAGs(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		name := "seed " + strconv.FormatUint(seed, 10)
		d := randomDAG(t, seed, 40, 70, gw.Caps{})
		res, err := layout.New(d.Caps()).Run(d, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		checkBends(t, name, d, res)
		n := nested(t, seed, 30, 50, 5)
		res, err = layout.New(n.Caps()).Run(n, layout.Options{All: true})
		if err != nil {
			t.Fatal(err)
		}
		checkBends(t, name+" nested", n, res)
	}
}

// TestBendsCleared: a second layout replaces the bends rather than
// adding to the slab, and an edge whose ends come to adjacent layers
// loses them.
func TestBendsCleared(t *testing.T) {
	d, err := gw.New(schema(true), gw.Caps{Edges: 4, BendsPerEdge: 2})
	if err != nil {
		t.Fatal(err)
	}
	var v [4]gw.VertexIndex
	for i := range v {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode})
		if err != nil {
			t.Fatal(err)
		}
		v[i] = ch.Vertices[0]
	}
	edge := func(a, b gw.VertexIndex) gw.EdgeIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: a, To: b, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Edges[0]
	}
	edge(v[0], v[1])
	edge(v[1], v[2])
	edge(v[2], v[3])
	long := edge(v[0], v[3])
	l := layout.New(d.Caps())
	for range 3 {
		if _, err := l.Run(d, layout.Options{}); err != nil {
			t.Fatal(err)
		}
		if len(d.Edges[long].Bends) != 4 {
			t.Fatalf("the edge across two layers has bends %v", d.Edges[long].Bends)
		}
	}
	if _, err := d.Apply(gw.Command{Op: gw.DeleteVertex, Vertex: v[1]}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	if len(d.Edges[long].Bends) != 0 {
		t.Errorf("between adjacent layers: bends %v", d.Edges[long].Bends)
	}
}

// TestBendsUndo: undo and redo restore the positions as before, and the
// bends follow them: a layout undone hides the bends it gave, redone
// shows them again, and a dragged end undone brings its edges' back.
func TestBendsUndo(t *testing.T) {
	d := randomDAG(t, 5, 30, 50, gw.Caps{})
	bentEdges := func() string {
		s := ""
		for i := range d.Edges {
			if len(d.Edges[i].Bends) > 0 {
				s += string(d.Edges[i].ID) + " "
			}
		}
		return s
	}
	before := positions(d)
	if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all"}); err != nil {
		t.Fatal(err)
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	after, bent := positions(d), bentEdges()
	if bent == "" {
		t.Fatal("no edge bent")
	}
	var long gw.EdgeIndex = gw.None
	for i := range d.Edges {
		if len(d.Edges[i].Bends) > 0 {
			long = gw.EdgeIndex(i) //nolint:gosec // bounded
			break
		}
	}
	end := d.Edges[long].To
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: end, Pos: gw.Point{X: 0, Y: -400}}); err != nil {
		t.Fatal(err)
	}
	if len(d.Edges[long].Bends) != 0 {
		t.Error("an edge whose end was dragged kept its bends")
	}
	d.Undo()
	if positions(d) != after || bentEdges() != bent {
		t.Errorf("the drag undone: bent %q, want %q", bentEdges(), bent)
	}
	d.Undo()
	if positions(d) != before || bentEdges() != "" {
		t.Errorf("the layout undone: positions differ or bent %q", bentEdges())
	}
	d.Redo()
	if positions(d) != after || bentEdges() != bent {
		t.Errorf("the layout redone: bent %q, want %q", bentEdges(), bent)
	}
}

// TestTopToBottom: on the random DAGs, flat and nested, a top-to-bottom
// flow keeps every invariant with the axes swapped: no overlap, every
// edge going down, every vertex inside its zone, every long edge routed
// clear of the vertices it passes; and its layers are the pitch for that
// flow apart.
func TestTopToBottom(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		name := "seed " + strconv.FormatUint(seed, 10) + " down"
		d := randomDAG(t, seed, 40, 70, gw.Caps{})
		d.Flow = gw.TopToBottom
		res, err := layout.New(d.Caps()).Run(d, layout.Options{})
		if err != nil {
			t.Fatal(err)
		}
		checkInvariants(t, name, d, res, layout.Options{})
		checkBends(t, name, d, res)
		if seed == 1 {
			ys := map[float64]bool{}
			for i := range d.Vertices {
				ys[d.Vertices[i].Pos.Y] = true
			}
			if len(ys) != res.Layers || !ys[96] {
				t.Errorf("%s: %d rows for %d layers, none at 96: %v", name, len(ys), res.Layers, ys)
			}
		}
		n := nested(t, seed, 30, 50, 5)
		n.Flow = gw.TopToBottom
		res, err = layout.New(n.Caps()).Run(n, layout.Options{All: true})
		if err != nil {
			t.Fatal(err)
		}
		checkBends(t, name+" nested", n, res)
		for i := range n.Vertices {
			a := &n.Vertices[i]
			if a.Live && a.Parent != gw.None && !inside(a, &n.Vertices[a.Parent]) {
				t.Errorf("%s nested: %s outside its zone", name, a.Label)
			}
			for j := i + 1; j < len(n.Vertices); j++ {
				if b := &n.Vertices[j]; a.Live && b.Live && a.Parent == b.Parent && overlap(a, b) {
					t.Errorf("%s nested: %s overlaps %s", name, a.Label, b.Label)
				}
			}
		}
	}
}

// TestContainerEdgeKeepsRoute: an edge from a container back past a
// vertex keeps the route its layout gave it. A route holds for the ends
// as they were when it was recorded, and the containers are fitted to
// what they hold at the end of a layout, so a route recorded before the
// fit was lost to the container's new size and the edge drawn straight
// through the vertex.
func TestContainerEdgeKeepsRoute(t *testing.T) {
	s := schema(false)
	s.VertexKinds = append(s.VertexKinds, gw.VertexKind{Name: "group", Shape: gw.RoundRect, Size: gw.Size{W: 200, H: 120}, Container: true})
	d, err := gw.New(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex, id string) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: id})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	edge := func(from, to gw.VertexIndex) {
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: from, To: to, FromPort: gw.None, ToPort: gw.None}); err != nil {
			t.Fatal(err)
		}
	}
	a, b, g := add(kNode, "a"), add(kNode, "b"), add(2, "g")
	for _, id := range []string{"x", "y"} {
		v := add(kNode, id)
		if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: g}); err != nil {
			t.Fatal(err)
		}
	}
	edge(a, b)
	edge(b, g)
	edge(g, a) // back past b
	res, err := layout.New(d.Caps()).Run(d, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if back := d.Edges[2]; len(back.Bends) == 0 {
		t.Errorf("the edge from the container back to a has no route; reversed %v", res.Reversed)
	}
	checkBends(t, "container cycle", d, res)
}
