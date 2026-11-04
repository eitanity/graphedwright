package layout

import (
	"cmp"
	"math"
	"slices"

	gw "code.eitanity.com/graphedwright"
)

// Once every vertex is placed and every route recorded, an edge whose
// drawn route still passes through a vertex or container that holds
// neither of its ends is given a detour: a route of level and upright
// legs, in the frame, round what is in the way. The renderer draws a
// level leg of a bent edge as a line, an upright leg of an elbow as a
// line, and an upright leg of a curve that runs a little forward as a
// near-upright S, so the detour is drawn as found. Its first leg leaves
// the side of its source that the renderer starts the edge from, and its
// last arrives at the side of its target where it ends.
//
// The detour is found on the grid of lines a gap outside each obstacle's
// sides and through both ends, by a shortest-path search in which each
// turn costs as much as a long leg, so that the route turns as little as
// it can. The obstacles are the vertices that hold neither end and are
// not inside another obstacle: the other members of each container that
// holds an end, and what stands at the top beside them. Only those near
// the ends' box are taken, a fixed number at most; with more, or with no
// route on the grid, the edge keeps its route.

// detourObs is the most obstacles a detour is found round.
const detourObs = 40

// detourWalls is the most containers holding an end whose sides a
// detour keeps off.
const detourWalls = 8

// detourCoords is the most grid lines on each axis: two per obstacle,
// four per wall, and one through each end.
const detourCoords = 2*detourObs + 4*detourWalls + 2

// detourStates is the search's states: a grid point and the way the
// route arrived there.
const detourStates = 4 * detourCoords * detourCoords

// detourGap is how far outside an obstacle the first detour runs.
const detourGap = float64(gw.ContainerPad)

// detourTurn is the cost of a turn, in units of length.
const detourTurn = 96

// detourClear is how near an obstacle a detour may run: more than a
// curve's lean, less than the gap.
const detourClear = 2

// curveRun is how far forward an upright leg of a curved edge runs, so
// that the curve, which pulls half its run either way, is drawn almost
// upright and does not loop.
const curveRun = 1

// minPull is the least pull of a curve that loops, as the renderer
// draws it.
const minPull = 24

// curveSteps is how many lines a curved leg is checked as.
const curveSteps = 12

// detour holds the search's working arrays, made once.
type detour struct {
	obs    [detourObs]frameBox
	nObs   int
	walls  [detourWalls]frameBox // the containers that hold an end
	nWalls int
	gap    float64 // how far outside an obstacle this detour runs
	xs, ys [detourCoords]float64
	nx, ny int
	dist   []float64 // per state
	prev   []int32   // per state, the state it was reached from, or -1
	heap   []int32   // states by distance
	hpos   []int32   // each state's place in heap, or -1
	path   [detourCoords * 2]gw.Point
	byX    []int32    // the live vertices that hold nothing, by their boxes' near sides
	wide   []int32    // the live containers
	boxes  []frameBox // each vertex's box in the frame, as indexed
	maxW   float64    // the widest box in byX
}

func newDetour(vertices int) *detour {
	return &detour{
		dist: make([]float64, detourStates), prev: make([]int32, detourStates),
		heap: make([]int32, 0, detourStates), hpos: make([]int32, detourStates),
		byX: make([]int32, 0, vertices), boxes: make([]frameBox, vertices), wide: make([]int32, 0, vertices),
	}
}

// index sorts the live vertices that hold nothing by the near side of
// their boxes along the flow, with the widest of them, and keeps the
// containers apart, so that hit looks only at the vertices a line can
// reach: those starting no more than the widest before it ends.
func (l *Layouter) index(d *gw.Document) {
	dt := l.dt
	dt.byX, dt.wide, dt.maxW = dt.byX[:0], dt.wide[:0], 0
	for i := range d.VertexSlots() {
		v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if !v.Live() {
			continue
		}
		b := l.frame(v)
		dt.boxes[i] = b
		if d.Kinds().Vertex(v.Kind()).Container() {
			dt.wide = append(dt.wide, int32(i)) //nolint:gosec // bounded by the capacity
			continue
		}
		dt.byX = append(dt.byX, int32(i)) //nolint:gosec // bounded by the capacity
		dt.maxW = math.Max(dt.maxW, b.x1-b.x0)
	}
	slices.SortFunc(dt.byX, dt.nearer)
}

// nearer orders two vertices by the near side of their boxes.
func (dt *detour) nearer(a, b int32) int {
	return cmp.Compare(dt.boxes[a].x0, dt.boxes[b].x0)
}

// detours gives each edge whose drawn route passes through a vertex or
// container that holds neither end a detour round them. A document with
// no container is left as it is: its layers keep its routes clear.
func (l *Layouter) detours(d *gw.Document) {
	if !hasContainer(d) {
		return
	}
	l.index(d)
	for i := range d.EdgeSlots() {
		e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		ei := gw.EdgeIndex(i)        //nolint:gosec // bounded by the capacity
		if !e.Live() || e.From() == e.To() || e.FromPort() != gw.None || e.ToPort() != gw.None {
			continue
		}
		if k, n := d.Parallel(ei); n > 1 {
			if k == 0 {
				l.lanes(d, ei)
			}
			continue
		}
		if l.crosses(d, ei) {
			l.reroute(d, ei, 0, 1)
		}
	}
}

// maxLanes is the most edges between two vertices that are given
// detours together.
const maxLanes = 8

// lanes gives the edges drawn in lanes between the ends of edge e, the
// first of them, detours if any of them passes through a vertex: each
// its own, the further round what is in the way the later it comes, so
// that they run apart as their lanes do.
func (l *Layouter) lanes(d *gw.Document, e gw.EdgeIndex) {
	var group [maxLanes]gw.EdgeIndex
	n, hit := 0, false
	ed := d.Edge(e)
	for _, v := range [2]gw.VertexIndex{ed.From(), ed.To()} {
		// bound: the out list visits each edge of the vertex once
		for o := d.FirstOut(v); o != gw.None; o = d.NextOut(o) {
			k, m := d.Parallel(o)
			if m < 2 || !sameEnds(d, e, o) {
				continue
			}
			if m > maxLanes {
				return
			}
			group[k], n = o, m
			hit = hit || l.crosses(d, o)
		}
	}
	if !hit {
		return
	}
	for j, o := range group[:n] {
		l.reroute(d, o, j, n)
	}
}

// sameEnds reports whether edges e and o join the same two vertices,
// either way.
func sameEnds(d *gw.Document, e, o gw.EdgeIndex) bool {
	a, b := d.Edge(e), d.Edge(o)
	return a.From() == b.From() && a.To() == b.To() || a.From() == b.To() && a.To() == b.From()
}

func hasContainer(d *gw.Document) bool {
	for i := range d.VertexSlots() {
		if d.Vertex(gw.VertexIndex(i)).Live() && d.Kinds().Vertex(d.Vertex(gw.VertexIndex(i)).Kind()).Container() { //nolint:gosec // bounded by the capacity
			return true
		}
	}
	return false
}

// drawn is an edge as the renderer draws it, in the frame: its route,
// whether it runs backward, and its ends.
type drawn struct {
	route gw.Route
	back  bool
	a, b  gw.Point
}

func (l *Layouter) drawnOf(d *gw.Document, e gw.EdgeView) drawn {
	fb, tb := l.frame(d.Vertex(e.From())), l.frame(d.Vertex(e.To()))
	fx, tx := (fb.x0+fb.x1)/2, (tb.x0+tb.x1)/2
	w := drawn{route: d.Kinds().Edge(e.Kind()).Route(), back: tx < fx}
	w.a, w.b = gw.Point{X: fx, Y: fb.cy}, gw.Point{X: tx, Y: tb.cy}
	if w.route != gw.Straight {
		w.a.X, w.b.X = fb.x1, tb.x0
		if w.back {
			w.a.X, w.b.X = fb.x0, tb.x1
		}
	}
	return w
}

// crosses reports whether edge e, drawn through its bends, passes
// through a vertex that holds neither of its ends.
func (l *Layouter) crosses(d *gw.Document, e gw.EdgeIndex) bool {
	ed := d.Edge(e)
	w := l.drawnOf(d, ed)
	l.walls(d, ed)
	var through [1]gw.Point
	bends := ed.Bends()
	n, onLane := bends.Len(), false
	if k, m := d.Parallel(e); m > 1 {
		if lane, ok := w.lane(k, m); ok {
			through[0], onLane, n = lane, true, 1
		}
	}
	var pts [curveSteps + 1]gw.Point
	// bound: the edge's legs, one more than its bends
	for k := 0; k <= n; k++ {
		p, q := w.a, w.b
		if k > 0 {
			p = l.point(bends, through[:], onLane, k-1)
		}
		if k < n {
			q = l.point(bends, through[:], onLane, k)
		}
		m := legPoints(w, n > 0, p, q, &pts)
		// bound: the leg's points, curveSteps + 1 at most
		for j := 1; j < m; j++ {
			if l.hit(d, ed, pts[j-1], pts[j]) != gw.None || l.dt.onWall(pts[j-1], pts[j]) {
				return true
			}
		}
	}
	return false
}

// point is bend k in the frame: of bends, or, on a lane, of through,
// already in the frame.
func (l *Layouter) point(bends gw.PointList, through []gw.Point, onLane bool, k int) gw.Point {
	if onLane {
		return through[k]
	}
	b := bends.At(k)
	x, y := l.swap(b.X, b.Y)
	return gw.Point{X: x, Y: y}
}

// laneGap is how far apart the renderer runs the lanes of the edges
// between two vertices: a label's height.
const laneGap = 20

// lane is the point the renderer draws the k-th of m edges between the
// same two vertices through, in the frame, and whether it draws it
// through one: not when the edge loops back.
func (w drawn) lane(k, m int) (gw.Point, bool) {
	run := w.b.X - w.a.X
	if w.back {
		run = -run
	}
	if run <= 0 {
		return gw.Point{}, false
	}
	off := float64((float64(k) - float64(float64(m-1)/2)) * laneGap)
	return gw.Point{X: float64((w.a.X + w.b.X) / 2), Y: float64((w.a.Y+w.b.Y)/2) + off}, true
}

// legPoints writes the leg from p to q, as the renderer draws it, into
// pts as a line through them, and returns their count.
func legPoints(w drawn, bent bool, p, q gw.Point, pts *[curveSteps + 1]gw.Point) int {
	route := w.route
	if bent && p.Y == q.Y {
		route = gw.Straight
	}
	switch route {
	case gw.Orthogonal:
		mid := float64((p.X + q.X) / 2)
		pts[0], pts[1], pts[2], pts[3] = p, gw.Point{X: mid, Y: p.Y}, gw.Point{X: mid, Y: q.Y}, q
		return 4
	case gw.Curve:
		run := q.X - p.X
		if w.back {
			run = -run
		}
		pull := math.Abs(run) / 2
		if run <= 0 {
			pull = math.Max(pull, minPull)
		}
		if w.back {
			pull = -pull
		}
		c1, c2 := gw.Point{X: p.X + pull, Y: p.Y}, gw.Point{X: q.X - pull, Y: q.Y}
		for j := range curveSteps + 1 {
			t := float64(j) / curveSteps
			u := 1 - t
			pts[j] = gw.Point{
				X: float64(u*u*u*p.X) + float64(3*u*u*t*c1.X) + float64(3*u*t*t*c2.X) + float64(t*t*t*q.X),
				Y: float64(u*u*u*p.Y) + float64(3*u*u*t*c1.Y) + float64(3*u*t*t*c2.Y) + float64(t*t*t*q.Y),
			}
		}
		return curveSteps + 1
	case gw.Straight:
	}
	pts[0], pts[1] = p, q
	return 2
}

// hit is a vertex that holds neither end of edge e and that the line
// from p to q, in the frame, passes more than half a unit into, or None.
func (l *Layouter) hit(d *gw.Document, e gw.EdgeView, p, q gw.Point) gw.VertexIndex {
	dt := l.dt
	x0, x1 := math.Min(p.X, q.X), math.Max(p.X, q.X)
	for _, u := range dt.wide {
		if l.into(d, e, p, q, u) {
			return gw.VertexIndex(u)
		}
	}
	// The vertices that start before the line ends, back to those that
	// start the widest box before it starts.
	hi, _ := slices.BinarySearchFunc(dt.byX, x1, func(v int32, x float64) int {
		if dt.boxes[v].x0 <= x {
			return -1
		}
		return 1
	})
	// bound: the indexed vertices
	for k := hi - 1; k >= 0 && dt.boxes[dt.byX[k]].x0 >= x0-dt.maxW; k-- {
		if u := dt.byX[k]; l.into(d, e, p, q, u) {
			return gw.VertexIndex(u)
		}
	}
	return gw.None
}

// into reports whether the line from p to q passes more than half a
// unit into vertex u, which holds neither end of edge e.
func (l *Layouter) into(d *gw.Document, e gw.EdgeView, p, q gw.Point, u int32) bool {
	b := l.dt.boxes[u]
	if b.x1 < math.Min(p.X, q.X) || b.x0 > math.Max(p.X, q.X) || b.y1 < math.Min(p.Y, q.Y) || b.y0 > math.Max(p.Y, q.Y) {
		return false
	}
	v := gw.VertexIndex(u)
	if d.Within(e.From(), v) || d.Within(e.To(), v) {
		return false
	}
	b.x0, b.x1, b.y0, b.y1 = b.x0+0.5, b.x1-0.5, b.y0+0.5, b.y1-0.5
	return meets(p.X, p.Y, q.X, q.Y, b)
}

// reroute gives edge e, the j-th of n between its two vertices, a
// detour, if one is found, and keeps its route otherwise. The j-th runs
// j gaps further out than the first, and leaves and arrives off its
// ends' middles by as much as its lane is off the middle lane, so that
// edges between the same two vertices run apart. Where vertices stand
// too close for the gap, half of it is tried.
func (l *Layouter) reroute(d *gw.Document, e gw.EdgeIndex, j, n int) {
	ed := d.Edge(e)
	w := l.drawnOf(d, ed)
	dt := l.dt
	for _, base := range [2]float64{detourGap, detourGap / 2} {
		dt.gap = float64(base * float64(1+j))
		off := float64((float64(j) - float64(float64(n-1)/2)) * base)
		if m := l.find(d, ed, w, off); m >= 2 {
			l.setDetour(d, e, w, m)
			return
		}
	}
}

// find finds a detour for edge e, drawn as w, at the detour's gap and
// off its ends' middles by off, and returns how many corners it has, or
// 0 when there is none.
func (l *Layouter) find(d *gw.Document, ed gw.EdgeView, w drawn, off float64) int {
	dt := l.dt
	fwd := 1.0
	if w.back {
		fwd = -1
	}
	fb, tb := l.frame(d.Vertex(ed.From())), l.frame(d.Vertex(ed.To()))
	ax, bx := fb.x1, tb.x0
	if w.back {
		ax, bx = fb.x0, tb.x1
	}
	s := gw.Point{X: ax + float64(fwd*dt.gap), Y: w.a.Y + off}
	t := gw.Point{X: bx - float64(fwd*dt.gap), Y: w.b.Y + off}
	if !l.obstacles(d, ed, s, t) {
		return 0
	}
	// The obstacles are those near the ends; a route found round them
	// that meets another takes it in too, and is found again.
	// bound: detourRounds searches
	for round := 0; round < detourRounds; round++ {
		if !dt.grid(s, t) {
			return 0
		}
		m := dt.search(s, t, w.back, true)
		if m < 2 {
			m = dt.search(s, t, w.back, false)
		}
		if m < 2 {
			return 0
		}
		u := l.missed(d, ed, m)
		if u == gw.None {
			return m
		}
		if dt.nObs == detourObs {
			return 0
		}
		dt.obs[dt.nObs] = l.frame(d.Vertex(u))
		dt.nObs++
	}
	return 0
}

// detourRounds is how many times a detour is found again round an
// obstacle it met.
const detourRounds = 4

// missed is the obstacle that the detour's first m corners, with its
// legs to and from the ends, pass through, or None.
func (l *Layouter) missed(d *gw.Document, ed gw.EdgeView, m int) gw.VertexIndex {
	dt := l.dt
	// bound: the detour's legs
	for k := 1; k < m; k++ {
		if u := l.hit(d, ed, dt.path[k-1], dt.path[k]); u != gw.None {
			// bound: the parent chain, at most the vertex count
			for n := 0; !l.obstacle(d, ed, u) && n < d.VertexSlots(); n++ {
				u = d.Vertex(u).Parent()
			}
			return u
		}
	}
	return gw.None
}

// setDetour gives edge e the m corners of the detour just found, leaned
// forward on a curve's upright legs.
func (l *Layouter) setDetour(d *gw.Document, e gw.EdgeIndex, w drawn, m int) {
	dt := l.dt
	if w.route == gw.Curve {
		fwd := 1.0
		if w.back {
			fwd = -1
		}
		dt.lean(m, fwd)
	}
	bs := d.ReserveBends(e, m)
	if !bs.Valid() {
		return
	}
	for k := range m {
		x, y := l.swap(dt.path[k].X, dt.path[k].Y)
		bs.Set(k, gw.Point{X: x, Y: y})
	}
	bs.Keep(m)
}

// obstacles finds what a detour from s to t, for edge e, goes round: the
// vertices that hold neither end and stand in no other such vertex, that
// meet the box round s and t grown by those that meet it. It reports
// false when there are too many, or when s or t is inside one.
func (l *Layouter) obstacles(d *gw.Document, e gw.EdgeView, s, t gw.Point) bool {
	r := frameBox{x0: math.Min(s.X, t.X), x1: math.Max(s.X, t.X), y0: math.Min(s.Y, t.Y), y1: math.Max(s.Y, t.Y)}
	dt := l.dt
	l.walls(d, e)
	// bound: the box grows at most twice
	for pass := 0; pass < 3; pass++ {
		dt.nObs = 0
		grown := r
		for i := range d.VertexSlots() {
			u := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
			if !l.obstacle(d, e, u) {
				continue
			}
			b := l.frame(d.Vertex(gw.VertexIndex(i))) //nolint:gosec // bounded by the capacity
			if b.x1 < r.x0-dt.gap || b.x0 > r.x1+dt.gap || b.y1 < r.y0-dt.gap || b.y0 > r.y1+dt.gap {
				continue
			}
			if dt.nObs == detourObs || (b.x0 < s.X && s.X < b.x1 && b.y0 < s.Y && s.Y < b.y1) || (b.x0 < t.X && t.X < b.x1 && b.y0 < t.Y && t.Y < b.y1) {
				return false
			}
			dt.obs[dt.nObs] = b
			dt.nObs++
			grown = frameBox{x0: math.Min(grown.x0, b.x0), x1: math.Max(grown.x1, b.x1), y0: math.Min(grown.y0, b.y0), y1: math.Max(grown.y1, b.y1)}
		}
		if grown == r {
			break
		}
		r = grown
	}
	return true
}

// walls finds the containers that hold an end of e: a detour may cross
// their sides, to get in or out, but not run along them.
func (l *Layouter) walls(d *gw.Document, e gw.EdgeView) {
	dt := l.dt
	dt.nWalls = 0
	for _, v := range [2]gw.VertexIndex{e.From(), e.To()} {
		// bound: the parent chain, at most detourWalls containers
		for p := d.Vertex(v).Parent(); p != gw.None && dt.nWalls < detourWalls; p = d.Vertex(p).Parent() {
			dt.walls[dt.nWalls] = l.frame(d.Vertex(p))
			dt.nWalls++
		}
	}
}

// obstacle reports whether u is one of the vertices a detour for e goes
// round: live, holding neither end, and in none that holds neither.
func (l *Layouter) obstacle(d *gw.Document, e gw.EdgeView, u gw.VertexIndex) bool {
	v := d.Vertex(u)
	if !v.Live() || d.Within(e.From(), u) || d.Within(e.To(), u) {
		return false
	}
	p := v.Parent()
	return p == gw.None || d.Within(e.From(), p) || d.Within(e.To(), p)
}

// grid sets the grid lines: through s and t, and a gap outside each
// obstacle's sides, sorted and each once. It reports false when there
// is nothing to go round.
func (dt *detour) grid(s, t gw.Point) bool {
	if dt.nObs == 0 {
		return false
	}
	dt.nx, dt.ny = 0, 0
	dt.addX(s.X)
	dt.addX(t.X)
	dt.addY(s.Y)
	dt.addY(t.Y)
	for _, b := range dt.obs[:dt.nObs] {
		dt.addX(b.x0 - dt.gap)
		dt.addX(b.x1 + dt.gap)
		dt.addY(b.y0 - dt.gap)
		dt.addY(b.y1 + dt.gap)
	}
	for _, b := range dt.walls[:dt.nWalls] {
		for _, x := range [4]float64{b.x0 - dt.gap, b.x0 + dt.gap, b.x1 - dt.gap, b.x1 + dt.gap} {
			dt.addX(x)
		}
		for _, y := range [4]float64{b.y0 - dt.gap, b.y0 + dt.gap, b.y1 - dt.gap, b.y1 + dt.gap} {
			dt.addY(y)
		}
	}
	return true
}

func (dt *detour) addX(v float64) { dt.nx = insert(dt.xs[:], dt.nx, v) }
func (dt *detour) addY(v float64) { dt.ny = insert(dt.ys[:], dt.ny, v) }

// insert puts v into the sorted first n of a, unless it is there, and
// returns the new count.
func insert(a []float64, n int, v float64) int {
	k := n
	// bound: the lines so far
	for k > 0 && a[k-1] > v {
		k--
	}
	if k > 0 && a[k-1] == v || n == len(a) {
		return n
	}
	copy(a[k+1:n+1], a[k:n])
	a[k] = v
	return n + 1
}

// The four ways a route arrives at a grid point.
const (
	east = iota
	west
	south
	north
)

// search finds the cheapest route on the grid from s, leaving it the way
// the edge runs, to t, and writes its corners, s first and t last, into
// path. It returns how many there are, or 0 when t cannot be reached.
// With onward set, the route never goes back against the way the edge
// runs, so that its bends keep the order of the layers it passes.
func (dt *detour) search(s, t gw.Point, back, onward bool) int {
	start, goal := dt.at(s), dt.at(t)
	if start < 0 || goal < 0 {
		return 0
	}
	ahead := int32(east)
	if back {
		ahead = west
	}
	for i := range dt.dist {
		dt.dist[i], dt.prev[i], dt.hpos[i] = math.Inf(1), -1, -1
	}
	dt.heap = dt.heap[:0]
	dt.relax(start*4+ahead, -1, 0)
	best, bestCost := int32(-1), math.Inf(1)
	// bound: each state leaves the heap once
	for len(dt.heap) > 0 {
		st := dt.pop()
		node, way := st/4, st%4
		cost := dt.dist[st]
		if cost >= bestCost {
			break
		}
		if node == goal {
			if way == ahead^1 {
				continue // it would turn back into its own last leg
			}
			if way != ahead {
				cost += detourTurn
			}
			if cost < bestCost {
				best, bestCost = st, cost
			}
			continue
		}
		dt.expand(st, node, way, cost, onward, ahead)
	}
	if best < 0 {
		return 0
	}
	return dt.trace(best)
}

// at is the grid point at p, or -1.
func (dt *detour) at(p gw.Point) int32 {
	ix, iy := -1, -1
	for i := range dt.nx {
		if dt.xs[i] == p.X {
			ix = i
		}
	}
	for i := range dt.ny {
		if dt.ys[i] == p.Y {
			iy = i
		}
	}
	if ix < 0 || iy < 0 {
		return -1
	}
	return int32(iy*dt.nx + ix) //nolint:gosec // within detourStates
}

// expand relaxes the states one leg on from state st, at grid point
// node, arrived at going way: to the next grid point each way but back,
// and with onward set, not against ahead, the way the edge runs.
func (dt *detour) expand(st, node, way int32, cost float64, onward bool, ahead int32) {
	ix, iy := int(node)%dt.nx, int(node)/dt.nx
	for to := int32(0); to < 4; to++ {
		if to == way^1 || onward && to == ahead^1 {
			continue // east and west, south and north, are pairs
		}
		jx, jy := ix, iy
		switch to {
		case east:
			jx++
		case west:
			jx--
		case south:
			jy++
		case north:
			jy--
		}
		if jx < 0 || jx >= dt.nx || jy < 0 || jy >= dt.ny {
			continue
		}
		p := gw.Point{X: dt.xs[ix], Y: dt.ys[iy]}
		q := gw.Point{X: dt.xs[jx], Y: dt.ys[jy]}
		if dt.blocked(p, q) {
			continue
		}
		c := cost + math.Abs(q.X-p.X) + math.Abs(q.Y-p.Y)
		if to != way {
			c += detourTurn
		}
		dt.relax(int32(jy*dt.nx+jx)*4+to, st, c) //nolint:gosec // within detourStates
	}
}

// blocked reports whether the line from p to q meets an obstacle, or
// runs within detourClear of one, as along a side that the line through an
// end happens to share.
func (dt *detour) blocked(p, q gw.Point) bool {
	for _, b := range dt.obs[:dt.nObs] {
		b.x0, b.x1, b.y0, b.y1 = b.x0-detourClear, b.x1+detourClear, b.y0-detourClear, b.y1+detourClear
		if meets(p.X, p.Y, q.X, q.Y, b) {
			return true
		}
	}
	return dt.onWall(p, q)
}

// onWall reports whether the line from p to q runs along a side of a
// container that holds an end.
func (dt *detour) onWall(p, q gw.Point) bool {
	for _, b := range dt.walls[:dt.nWalls] {
		if along(p.Y, q.Y, p.X, q.X, b.y0, b.y1, b.x0, b.x1) || along(p.X, q.X, p.Y, q.Y, b.x0, b.x1, b.y0, b.y1) {
			return true
		}
	}
	return false
}

// along reports whether a line level in its first axis, at a0 = a1,
// from b0 to b1 in the other, runs within half the gap of side s0 or
// s1, over the wall's extent from e0 to e1.
func along(a0, a1, b0, b1, s0, s1, e0, e1 float64) bool {
	if a0 != a1 || math.Max(b0, b1) <= e0 || math.Min(b0, b1) >= e1 {
		return false
	}
	return math.Abs(a0-s0) < detourGap/2 || math.Abs(a0-s1) < detourGap/2
}

// trace writes the corners of the route that ends in state st into path,
// from s, and returns their count.
func (dt *detour) trace(st int32) int {
	n := 0
	way := int32(-1)
	// bound: the chain of states, each reached once
	for k := 0; st >= 0 && k < detourStates; k++ {
		node := st / 4
		p := gw.Point{X: dt.xs[int(node)%dt.nx], Y: dt.ys[int(node)/dt.nx]}
		if prev := dt.prev[st]; prev < 0 || st%4 != way {
			if n == len(dt.path) {
				return 0
			}
			dt.path[n] = p
			n++
		}
		way = st % 4
		st = dt.prev[st]
	}
	// The chain runs from t back to s; a straight run keeps only its ends.
	// bound: half the corners
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		dt.path[i], dt.path[j] = dt.path[j], dt.path[i]
	}
	return n
}

// lean moves each upright leg's end forward by curveRun, and the route
// after it with it, so that a curve draws it almost upright.
func (dt *detour) lean(n int, fwd float64) {
	shift := 0.0
	// bound: the detour's corners
	for k := 1; k < n; k++ {
		if dt.path[k].X+shift == dt.path[k-1].X && dt.path[k].Y != dt.path[k-1].Y {
			shift += float64(fwd * curveRun)
		}
		dt.path[k].X += shift
	}
}

// relax lowers state st's distance to c, reached from state from, if
// that is lower.
func (dt *detour) relax(st, from int32, c float64) {
	if c >= dt.dist[st] {
		return
	}
	dt.dist[st], dt.prev[st] = c, from
	if dt.hpos[st] == -2 {
		return // settled: a search with no negative leg never lowers it
	}
	if dt.hpos[st] < 0 {
		dt.hpos[st] = int32(len(dt.heap)) //nolint:gosec // within detourStates
		dt.heap = append(dt.heap, st)
	}
	dt.up(int(dt.hpos[st]))
}

func (dt *detour) pop() int32 {
	h := dt.heap
	top := h[0]
	last := len(h) - 1
	h[0] = h[last]
	dt.hpos[h[0]] = 0
	dt.heap = h[:last]
	dt.hpos[top] = -2 // done
	if last > 0 {
		dt.down(0)
	}
	return top
}

func (dt *detour) up(i int) {
	h := dt.heap
	// bound: the heap's depth
	for i > 0 {
		p := (i - 1) / 2
		if dt.dist[h[p]] <= dt.dist[h[i]] {
			break
		}
		h[p], h[i] = h[i], h[p]
		dt.hpos[h[p]], dt.hpos[h[i]] = int32(p), int32(i) //nolint:gosec // within detourStates
		i = p
	}
}

func (dt *detour) down(i int) {
	h := dt.heap
	// bound: the heap's depth
	for {
		c := 2*i + 1
		if c >= len(h) {
			return
		}
		if c+1 < len(h) && dt.dist[h[c+1]] < dt.dist[h[c]] {
			c++
		}
		if dt.dist[h[i]] <= dt.dist[h[c]] {
			return
		}
		h[c], h[i] = h[i], h[c]
		dt.hpos[h[c]], dt.hpos[h[i]] = int32(c), int32(i) //nolint:gosec // within detourStates
		i = c
	}
}
