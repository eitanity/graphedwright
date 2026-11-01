// Package layout places the vertices of a document in layers from left
// to right, or from top to bottom when the document's flow says so:
// roots first, each vertex one layer past its furthest
// predecessor, the order within a layer chosen to cross few edges, and
// the positions on a grid. Vertices the host or the user placed stay
// where they are and the rest are laid out around them.
//
// A container's contents are laid out first, deepest container first,
// as a graph of their own: an edge into or out of the container counts
// at the level it crosses, between the vertices or containers there
// that hold its ends. The container is then one node of the level it is
// in, the size of what it holds, and the positions are composed from
// the top down. A placed vertex in a container is fixed in the
// container's layout; when the container is placed as well, both stay
// where they are.
//
// The passes work in one frame, the layers along x and the order within
// a layer along y; a top-to-bottom flow swaps the axes on the way in and
// out.
package layout

import (
	"errors"
	"math"
	"strconv"
	"unicode/utf8"

	gw "code.eitanity.com/graphedwright"
)

// Options shape one layout. A Pitch, Gap, Grid or Sweeps that is not
// positive, zero or NaN among them, takes the default: a layer
// pitch of 160 units (96 for a top-to-bottom flow, whose layers are as
// deep as their vertices are tall), a gap of 24 between vertices in a
// layer, a grid of 8, and 8 crossing-reduction sweeps. All lays out
// every vertex, placed or not.
type Options struct {
	Pitch  float64
	Gap    float64
	Grid   float64
	Sweeps int
	All    bool
}

func (o Options) filled() Options {
	if !(o.Pitch > 0) {
		o.Pitch = 160
	}
	if !(o.Gap > 0) {
		o.Gap = 24
	}
	if !(o.Grid > 0) {
		o.Grid = 8
	}
	if o.Sweeps <= 0 {
		o.Sweeps = 8
	}
	return o
}

// Result reports a layout: the number of layers, the edge crossings of
// the order kept, and the edges reversed to break cycles, which a host
// can mark. Reversed is the layouter's scratch, valid until the next Run.
type Result struct {
	Layers    int
	Crossings int64
	Reversed  []gw.EdgeIndex
}

// dummiesPerEdge bounds the dummy vertices: the budget is this many per
// edge capacity, checked before any are made.
const dummiesPerEdge = 8

// downPitch is the default layer pitch of a top-to-bottom flow.
const downPitch = 96

// columnGap is the least room between the widest nodes of two adjacent
// layers; layers are the pitch apart or further when their nodes need it.
const columnGap = 40

// labelChar is the width a label's character is taken to need, the
// renderer's default, and labelPad the room either side of a label.
const (
	labelChar = 7
	labelPad  = 8
)

// labelRoom gives each layer the room the text in the gap before it
// needs: the widest of the edges without dummies between it and the
// layer before, whose label and kind's stereotype are drawn in that gap,
// one above the other at the middle of the edge, or, for an orthogonal
// edge whose source has others to the same layer, of its last leg, past
// its turn halfway. A top-to-bottom flow puts them beside their edges,
// so it needs none.
func (l *Layouter) labelRoom(d *gw.Document) {
	// bound: the layers
	for k := 0; k <= l.layers; k++ {
		l.lg[k] = 0
	}
	if l.down {
		return
	}
	// The orthogonal edges between adjacent layers out of each node, whose
	// texts go on their last legs when more than one leave it, in bary,
	// which ordering has not yet used.
	for i := range l.n {
		l.bary[i] = 0
	}
	for s := range l.s {
		e := l.segE[s]
		a, b := l.layer[l.segF[s]], l.layer[l.segT[s]]
		if l.dN[e] == 0 && (a-b == 1 || b-a == 1) && d.Schema.EdgeKinds[d.Edges[e].Kind].Route == gw.Orthogonal {
			l.bary[l.eFrom[e]]++
		}
	}
	for s := range l.s {
		e := l.segE[s]
		a, b := l.layer[l.segF[s]], l.layer[l.segT[s]]
		if l.dN[e] != 0 || (a-b != 1 && b-a != 1) {
			continue
		}
		k := max(a, b)
		kind := d.Schema.EdgeKinds[d.Edges[e].Kind]
		chars := max(utf8.RuneCountInString(d.Edges[e].Label), utf8.RuneCountInString(kind.Stereotype))
		w := float64(float64(chars)*labelChar) + 2*labelPad
		if kind.Route == gw.Orthogonal && l.bary[l.eFrom[e]] > 1 {
			// Its text is drawn on its last leg, the half of the gap
			// past its turn, which it shares with the others.
			w *= 2
		}
		l.lg[k] = math.Max(l.lg[k], w)
	}
}

// Layouter holds the working arrays, sized from the capacities once.
type Layouter struct {
	caps     gw.Caps
	maxN     int // real vertices plus the dummy budget
	maxS     int // segments: an edge plus its dummies
	layer    []int32
	fixed    []bool // layer fixed by a placed vertex
	order    []int32
	best     []int32
	pos      []int32
	bary     []float64
	y        []float64
	x        []float64
	h        []float64
	topo     []int32
	indeg    []int32
	stack    []int32
	mark     []uint8
	segF     []int32 // segment ends over the layered graph's nodes
	segT     []int32
	segE     []int32 // the edge a segment belongs to
	off      []int32 // CSR offsets for the adjacency of one direction
	adj      []int32
	keys     []int32 // layer starts in order
	fen      []int32 // Fenwick tree for the crossing count
	byTo     []int32
	rev      []gw.EdgeIndex
	liveReal []bool // the vertices of the level being laid out
	pin      []bool // the placed ones among them
	eOn      []bool // the edges of the level, between the nodes that hold their ends
	eFrom    []int32
	eTo      []int32
	members  int       // vertices in the level
	edges    int       // the document's edge slots, live or not
	w        []float64 // node widths, as h is heights
	lw, lx   []float64 // each layer's widest node and its x
	lg       []float64 // the room the labels in the gap before each layer need
	fx, fy   []float64 // a placed vertex's position in its level
	bw, bh   []float64 // a container's size once what it holds is laid out; 0 before
	cx, cy   []float64 // where that box's centre fell from the container's centre
	rx, ry   []float64 // a vertex's position from its container's centre, or the document's
	depth    []int32
	dFirst   []int32          // an edge's first dummy node in the level it is laid out in
	dN       []int32          // and how many it has
	eLevel   []gw.VertexIndex // the container whose level gave an edge its bends, or None
	route    [][]gw.Point     // the bends each edge was given this run, in the document's slab, until placed
	rOff     []int32          // where in its route an edge's dummies' bends start, after room for its inside runs
	rLen     []int32          // and how many there are
	runs     [insidePts]gw.Point
	n        int // nodes in the layered graph
	s        int // segments
	realN    int // the document's vertex slots, live or not
	layers   int
	x0       float64 // the x of layer zero
	down     bool    // the flow is top to bottom: the frame's axes are swapped
	sorted   bool    // the level has no placed vertex, so each layer's order runs down in y
}

// New makes a layouter for documents with the capacities. A capacity
// no document can have, below 1 or beyond gw.MaxCap, takes the default,
// so that the working arrays' sizes stay within an int32; Run refuses a
// document beyond the layouter's capacities.
func New(caps gw.Caps) *Layouter {
	caps = fillCaps(caps)
	maxN := caps.Vertices + dummiesPerEdge*caps.Edges
	maxS := caps.Edges * (dummiesPerEdge + 1)
	return &Layouter{
		caps: caps, maxN: maxN, maxS: maxS,
		layer: make([]int32, maxN), fixed: make([]bool, caps.Vertices), order: make([]int32, maxN), best: make([]int32, maxN),
		pos: make([]int32, maxN), bary: make([]float64, maxN), y: make([]float64, maxN), x: make([]float64, maxN), h: make([]float64, maxN),
		topo: make([]int32, caps.Vertices), indeg: make([]int32, caps.Vertices), stack: make([]int32, caps.Vertices), mark: make([]uint8, caps.Vertices),
		segF: make([]int32, maxS), segT: make([]int32, maxS), segE: make([]int32, maxS), off: make([]int32, maxN+1), adj: make([]int32, maxS),
		keys: make([]int32, maxN+1), fen: make([]int32, maxN+1), byTo: make([]int32, maxS), rev: make([]gw.EdgeIndex, 0, caps.Edges),
		liveReal: make([]bool, caps.Vertices), pin: make([]bool, caps.Vertices),
		eOn: make([]bool, caps.Edges), eFrom: make([]int32, caps.Edges), eTo: make([]int32, caps.Edges),
		w: make([]float64, maxN), lw: make([]float64, maxN+1), lx: make([]float64, maxN+1), lg: make([]float64, maxN+1),
		fx: make([]float64, caps.Vertices), fy: make([]float64, caps.Vertices), bw: make([]float64, caps.Vertices), bh: make([]float64, caps.Vertices),
		rx: make([]float64, caps.Vertices), ry: make([]float64, caps.Vertices), depth: make([]int32, caps.Vertices),
		cx: make([]float64, caps.Vertices), cy: make([]float64, caps.Vertices),
		dFirst: make([]int32, caps.Edges), dN: make([]int32, caps.Edges), eLevel: make([]gw.VertexIndex, caps.Edges),
		route: make([][]gw.Point, caps.Edges), rOff: make([]int32, caps.Edges), rLen: make([]int32, caps.Edges),
	}
}

func fillCaps(c gw.Caps) gw.Caps {
	def := gw.DefaultCaps()
	if c.Vertices < 1 || c.Vertices > gw.MaxCap {
		c.Vertices = def.Vertices
	}
	if c.Edges < 1 || c.Edges > gw.MaxCap {
		c.Edges = def.Edges
	}
	return c
}

// Run lays the document out and sets the positions through Place. The
// document's capacities must fit the layouter's.
func (l *Layouter) Run(d *gw.Document, opts Options) (Result, error) {
	if !(opts.Pitch > 0) && d.Flow == gw.TopToBottom {
		opts.Pitch = downPitch
	}
	opts = opts.filled()
	if len(d.Vertices) > l.caps.Vertices || len(d.Edges) > l.caps.Edges {
		return Result{}, errors.New("document exceeds the layouter's capacities")
	}
	l.realN = len(d.Vertices)
	l.down = d.Flow == gw.TopToBottom
	deepest := 0
	for i := range d.Vertices {
		v := &d.Vertices[i]
		l.fixed[i] = !opts.All && v.Live && v.Placed
		l.bw[i], l.bh[i], l.cx[i], l.cy[i] = 0, 0, 0, 0
		if v.Live {
			l.depth[i] = int32(d.Depth(gw.VertexIndex(i))) //nolint:gosec // bounded by the capacity
			deepest = max(deepest, int(l.depth[i]))
		}
	}
	l.rev = l.rev[:0]
	clear(l.route)
	d.ClearBends()
	crossings, err := l.contents(d, opts, deepest)
	if err != nil {
		return Result{}, err
	}
	_, cr, err := l.level(d, opts, gw.None)
	if err != nil {
		return Result{}, err
	}
	crossings += cr
	l.bends(d, gw.None, 0, 0, opts)
	l.compose(d, deepest)
	// The routes hold for the ends as they are when recorded, so the
	// containers are fitted first: fitted after, an edge at a container
	// would find its end resized and lose its route.
	d.Fit()
	l.placeBends(d)
	return Result{Layers: l.layers, Crossings: crossings, Reversed: l.rev}, nil
}

// contents lays out what each container holds, deepest first, and sizes
// the container to it. It returns the crossings of the orders kept.
func (l *Layouter) contents(d *gw.Document, opts Options, deepest int) (int64, error) {
	crossings := int64(0)
	// bound: the deepest nesting, at most the vertex count
	for k := deepest; k >= 0; k-- {
		for i := range d.Vertices {
			c := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
			if !d.Vertices[i].Live || int(l.depth[i]) != k || !d.Schema.VertexKinds[d.Vertices[i].Kind].Container {
				continue
			}
			n, cr, err := l.level(d, opts, c)
			if err != nil {
				return 0, err
			}
			if n > 0 {
				crossings += cr
				l.box(d, c, opts)
				l.bends(d, c, l.cx[c], l.cy[c], opts)
			}
		}
	}
	return crossings, nil
}

// compose places every vertex from the top level, just laid out, down:
// each at its container's centre plus its offset; a placed vertex at the
// top level stays where it is.
func (l *Layouter) compose(d *gw.Document, deepest int) {
	for i := range d.Vertices {
		if l.liveReal[i] {
			l.rx[i], l.ry[i] = l.swap(l.x[i], l.y[i])
		}
	}
	// bound: the deepest nesting, at most the vertex count
	for k := 0; k <= deepest; k++ {
		for i := range d.Vertices {
			v := &d.Vertices[i]
			if !v.Live || int(l.depth[i]) != k {
				continue
			}
			if v.Parent != gw.None {
				l.rx[i] += l.rx[v.Parent]
				l.ry[i] += l.ry[v.Parent]
			}
			if v.Parent != gw.None || !l.fixed[i] {
				d.Place(gw.VertexIndex(i), gw.Point{X: l.rx[i], Y: l.ry[i]}, l.fixed[i]) //nolint:gosec // bounded by the capacity
			}
		}
	}
}

// placeBends makes the bends of the edges laid out in a container
// absolute, from its composed centre, and records every route for the
// ends where they now are. It works from the points the layouter
// reserved rather than the edges' Bends, which hide a route while an
// end is not where it was recorded for, as after a container is fitted.
func (l *Layouter) placeBends(d *gw.Document) {
	for i := range d.Edges {
		pts := l.route[i]
		if pts == nil {
			continue
		}
		outer := pts[l.rOff[i] : l.rOff[i]+l.rLen[i]]
		if c := l.eLevel[i]; c != gw.None {
			for j := range outer {
				outer[j].X += l.rx[c]
				outer[j].Y += l.ry[c]
			}
		}
		n := l.withRuns(d, gw.EdgeIndex(i), pts) //nolint:gosec // bounded by the capacity
		d.SetBends(gw.EdgeIndex(i), pts[:n])     //nolint:gosec // bounded by the capacity
		l.route[i] = nil
	}
}

// withRuns puts edge e's inside runs round the bends its dummies gave it,
// in pts, and returns how many points the route then has: the runs out
// of the containers holding its source, the bends, and the runs into
// the containers holding its target.
func (l *Layouter) withRuns(d *gw.Document, e gw.EdgeIndex, pts []gw.Point) int {
	ed := &d.Edges[e]
	off, n := int(l.rOff[e]), int(l.rLen[e])
	from, to := d.Vertices[ed.From].Pos, d.Vertices[ed.To].Pos
	farFrom, farTo := to, from // the point each end's runs come from outside
	if n > 0 {
		farFrom, farTo = pts[off], pts[off+n-1]
	}
	// With the flow, the target's runs go to higher x and the source's,
	// walked from outside in, to lower; against it, the other way.
	back := l.reversed(e)
	half := len(l.runs) / 2
	src := l.insideRuns(d, e, ed.From, farFrom, !back, l.runs[:half])
	dst := l.insideRuns(d, e, ed.To, farTo, back, l.runs[half:])
	if src+n+dst > len(pts) {
		return copy(pts, pts[off:off+n]) // no room: the bends alone
	}
	copy(pts[src:], pts[off:off+n])
	// The source's runs lead out, innermost container first.
	for k := range src {
		pts[k] = l.runs[src-1-k]
	}
	copy(pts[src+n:], l.runs[half:half+dst])
	return src + n + dst
}

// bends routes each edge of the level just laid out that crosses a
// layer through its dummies, in the edge's own direction: for each, a
// bend at either side of the
// band the layer's widest node spans, half a grid step further and out
// to the grid, at the dummy's height on the grid. The run between the
// two is clear of the layer's nodes, which the dummy was kept a gap
// from, and the legs between bands cross no node. The points are from
// the centre of container c, whose box centre fell at ox, oy from the
// level's origin, or absolute at the top; compose makes them absolute.
func (l *Layouter) bends(d *gw.Document, c gw.VertexIndex, ox, oy float64, opts Options) {
	if l.members == 0 {
		return
	}
	g := opts.Grid
	l.sorted = !l.anyPinned()
	for i := range l.eOn[:l.edges] {
		if !l.eOn[i] {
			continue
		}
		e := &d.Edges[i]
		src, dst := 4*crossed(d, e.From, c), 4*crossed(d, e.To, c)
		sIn, sOut, sOK, tIn, tOut, tOK := l.endPairs(i, g, d.Schema.EdgeKinds[e.Kind].Route)
		ns, nt := 0, 0
		if sOK {
			ns = 2
		}
		if tOK {
			nt = 2
		}
		if l.dN[i] == 0 && src+dst+ns+nt == 0 {
			continue
		}
		all := d.ReserveBends(gw.EdgeIndex(i), src+ns+2*int(l.dN[i])+nt+dst) //nolint:gosec // bounded by the capacity
		if all == nil {
			continue
		}
		route := all[src : src+ns+2*int(l.dN[i])+nt]
		if sOK {
			route[0] = gw.Point{X: sIn.X - ox, Y: sIn.Y - oy}
			route[1] = gw.Point{X: sOut.X - ox, Y: sOut.Y - oy}
		}
		if tOK {
			route[len(route)-2] = gw.Point{X: tOut.X - ox, Y: tOut.Y - oy}
			route[len(route)-1] = gw.Point{X: tIn.X - ox, Y: tIn.Y - oy}
		}
		l.eLevel[i], l.route[i] = c, all
		l.rOff[i], l.rLen[i] = int32(src), int32(len(route)) //nolint:gosec // bounded by the budget
		l.dummyBends(i, g, ox, oy, route[ns:ns+2*int(l.dN[i])])
	}
}

// dummyBends writes the bends of edge i's dummies into pts, in the
// edge's own direction: a backward edge's dummies run from its target's
// layer, so its bends are taken last dummy first, each band right side
// first.
func (l *Layouter) dummyBends(i int, g, ox, oy float64, pts []gw.Point) {
	back := l.layer[l.eFrom[i]] > l.layer[l.eTo[i]]
	// bound: the dummies of one edge, below the budget
	for k := int32(0); k < l.dN[i]; k++ {
		j := l.dFirst[i] + k
		near, far := 2*k, 2*k+1
		if back {
			j = l.dFirst[i] + l.dN[i] - 1 - k
			near, far = far, near
		}
		lx, half := l.lx[l.layer[j]], float64(l.lw[l.layer[j]]/2)+float64(g/2)
		y := math.Round(l.y[j]/g) * g
		x, py := l.swap(math.Floor((lx-half)/g)*g, y)
		pts[near] = gw.Point{X: x - ox, Y: py - oy}
		x, py = l.swap(math.Ceil((lx+half)/g)*g, y)
		pts[far] = gw.Point{X: x - ox, Y: py - oy}
	}
}

// endPairs gives edge i the pair of bends each of its ends needs, if
// any, to clear its band (clearance), source and then target, each with
// the inner bend first. Each end's route runs to the other end, or to
// the dummy next to it. The target's is found first, then the source's
// towards where the route then goes; and the target's again from the
// source's band edge if only the source needed a pair, since a straight
// route then comes from there.
func (l *Layouter) endPairs(i int, g float64, route gw.Route) (sIn, sOut gw.Point, sOK bool, tIn, tOut gw.Point, tOK bool) {
	f, t := l.eFrom[i], l.eTo[i]
	srcTo, dstFrom := t, f
	if l.dN[i] > 0 {
		srcTo, dstFrom = l.dFirst[i], l.dFirst[i]+l.dN[i]-1
		if l.layer[f] > l.layer[t] {
			srcTo, dstFrom = dstFrom, srcTo
		}
	}
	tIn, tOut, tOK = l.clearance(t, dstFrom, l.x[dstFrom], l.y[dstFrom], g, route)
	sx, sy := l.x[srcTo], l.y[srcTo]
	if tOK && l.dN[i] == 0 {
		sx, sy = l.frameOf(tOut)
	}
	sIn, sOut, sOK = l.clearance(f, srcTo, sx, sy, g, route)
	if sOK && !tOK && l.dN[i] == 0 {
		x, y := l.frameOf(sOut)
		tIn, tOut, tOK = l.clearance(t, dstFrom, x, y, g, route)
	}
	return sIn, sOut, sOK, tIn, tOut, tOK
}

// clearance gives the pair of bends, if they are needed, that keep the
// route from node end towards node toward, in an adjacent layer (the
// edge's other end, or its dummy next to end), out of the other nodes of
// end's layer. A layer's band is as wide as its widest node, so an end
// narrower than the band leaves or enters it some way in, and the route
// between there and the band's edge can pass another node of the layer:
// a container above or below the end, say. Where it would, the edge
// runs level from the end to the band's edge, half a grid step out and
// out to the grid as a dummy's bends are, through a bend halfway there
// and one at the edge, at the end's height. The run passes no other node
// of the layer, which are stacked apart from it, and the route on from
// the edge keeps to the room between the layers. The pair is a run as a
// dummy's is, so a label taken from the middle pair of a route lies on
// one. The route is taken to run to the point tx, ty in toward's layer,
// in the frame: toward's centre, or a band edge where the route has a
// bend. The points, the inner and the outer, are in the level's axes
// from its origin.
func (l *Layouter) clearance(end, toward int32, tx, ty, g float64, route gw.Route) (inner, outer gw.Point, ok bool) {
	k := l.layer[end]
	if k == l.layer[toward] {
		return gw.Point{}, gw.Point{}, false
	}
	side := 1.0 // the side of the band that faces the other layer
	if l.layer[toward] < k {
		side = -1
	}
	edge := l.lx[k] + float64(side*(float64(l.lw[k]/2)+float64(g/2)))
	if side > 0 {
		edge = math.Ceil(edge/g) * g
	} else {
		edge = math.Floor(edge/g) * g
	}
	exit := l.x[end] + float64(side*float64(l.w[end]/2))
	// The near edge of toward's band, where a curved or orthogonal route
	// arrives at the furthest in, so that its run there is the shortest.
	kt := l.layer[toward]
	near := l.lx[kt] - float64(side*float64(l.lw[kt]/2))
	if (edge-exit)*side <= 0 || !l.blocked(k, end, tx, ty, exit, edge, near, route) {
		return gw.Point{}, gw.Point{}, false
	}
	x, y := l.swap(float64((exit+edge)/2), l.y[end])
	inner = gw.Point{X: x, Y: y}
	x, y = l.swap(edge, l.y[end])
	return inner, gw.Point{X: x, Y: y}, true
}

// anyPinned reports whether the level being laid out has a placed
// vertex, which keeps its own height in its layer's order.
func (l *Layouter) anyPinned() bool {
	for i := range l.realN {
		if l.liveReal[i] && l.pin[i] {
			return true
		}
	}
	return false
}

// frameOf is a point of the level's axes in the frame.
func (l *Layouter) frameOf(p gw.Point) (x, y float64) { return l.swap(p.X, p.Y) }

// blocked reports whether the route from node end towards the point tx,
// ty would meet a real node of end's layer k other than end, before the
// band's edge. A straight route is the line from end's centre. A curved
// or orthogonal one leaves end's side, exit, at its height, towards a
// point no nearer than near, the near edge of the next band, and over
// the band from exit to edge it rises or falls by no more than a part of
// the way to ty: an orthogonal route not at all unless it turns in the
// band, which it does halfway to near; a curved one, whose controls are
// halfway along at either end's height, by at most 16u²/3 of it where u
// is the part of the way along that the band takes, and all of it from
// halfway on. The box from exit to the band's edge between end's height
// and that bound holds the route. Only nodes between those heights can
// meet it: where the layer's order runs down in y, the walk goes from
// end towards ty and stops at the first node past them; otherwise it
// takes the whole layer.
func (l *Layouter) blocked(k, end int32, tx, ty, exit, edge, near float64, route gw.Route) bool {
	x0, x1 := math.Min(exit, edge), math.Max(exit, edge)
	reach := l.y[end] + float64(l.part(end, tx, exit, edge, near, route)*(ty-l.y[end]))
	y0, y1 := math.Min(l.y[end], reach), math.Max(l.y[end], reach)
	lo, hi, step := l.walk(k, end, ty)
	// bound: the nodes of one layer
	for p := lo; p != hi; p += step {
		v := l.order[p]
		hw, hh := float64(l.w[v]/2), float64(l.h[v]/2)
		if step > 0 && lo != l.keys[k] && l.y[v]-hh > y1 || step < 0 && l.y[v]+hh < y0 {
			return false
		}
		if v == end || int(v) >= l.realN {
			continue
		}
		box := frameBox{x0: l.x[v] - hw, y0: l.y[v] - hh, x1: l.x[v] + hw, y1: l.y[v] + hh}
		if route == gw.Straight && meets(l.x[end], l.y[end], tx, ty, box) ||
			route != gw.Straight && box.x1 > x0 && box.x0 < x1 && box.y1 >= y0 && box.y0 <= y1 {
			return true
		}
	}
	return false
}

// part is how much of the way to its next point's height a route from
// node end goes within its band, from exit to edge (blocked).
func (l *Layouter) part(end int32, tx, exit, edge, near float64, route gw.Route) float64 {
	part := 1.0
	switch route {
	case gw.Straight:
		if run := math.Abs(tx - l.x[end]); run > 0 {
			part = math.Min(1, math.Abs(edge-l.x[end])/run)
		}
	case gw.Curve:
		if run := math.Abs(near - exit); run > 0 {
			if u := math.Abs(edge-exit) / run; u <= 0.5 {
				part = float64(16*u*u) / 3
			}
		}
	case gw.Orthogonal:
		if math.Abs(near-exit) >= 2*math.Abs(edge-exit) {
			part = 0
		}
	}
	return part
}

// walk is the order positions blocked visits in layer k, from lo
// towards hi by step: from end towards ty where the order runs down in
// y, or else the whole layer.
func (l *Layouter) walk(k, end int32, ty float64) (lo, hi, step int32) {
	lo, hi, step = l.keys[k], l.keys[k+1], 1
	if p := l.keys[k] + l.pos[end]; l.sorted && p < hi && l.order[p] == end {
		lo = p + 1
		if ty < l.y[end] {
			lo, hi, step = p-1, l.keys[k]-1, -1
		}
	}
	return lo, hi, step
}

// level lays out the vertices directly in container c, or at the top
// with None, and the edges between what they hold. It returns how many
// vertices there were and the crossings of the order kept.
func (l *Layouter) level(d *gw.Document, opts Options, c gw.VertexIndex) (int, int64, error) {
	n := 0
	for i := range d.Vertices {
		v := &d.Vertices[i]
		m := v.Live && v.Parent == c
		l.liveReal[i], l.pin[i] = m, m && l.fixed[i]
		if !m {
			continue
		}
		n++
		l.w[i], l.h[i] = l.swap(v.Size.W, v.Size.H)
		if l.bw[i] > 0 {
			l.w[i], l.h[i] = l.swap(l.bw[i], l.bh[i])
		}
		if l.pin[i] {
			// A placed container stays where its box falls, so that
			// what is placed in it stays where it is.
			x, y := v.Pos.X+l.cx[i], v.Pos.Y+l.cy[i]
			if c != gw.None {
				x -= d.Vertices[c].Pos.X
				y -= d.Vertices[c].Pos.Y
			}
			l.fx[i], l.fy[i] = l.swap(x, y)
		}
	}
	l.members, l.edges = n, len(d.Edges)
	if n == 0 {
		return 0, 0, nil
	}
	for i := range d.Edges {
		e := &d.Edges[i]
		l.eOn[i] = false
		if !e.Live {
			continue
		}
		a, b := holder(d, e.From, c), holder(d, e.To, c)
		if a >= 0 && b >= 0 && a != b {
			l.eOn[i], l.eFrom[i], l.eTo[i] = true, a, b
		}
	}
	l.topoOrder(d)
	l.assignLayers(opts)
	if err := l.makeSegments(d); err != nil {
		return 0, 0, err
	}
	l.labelRoom(d)
	l.initialOrder()
	cr := l.reduceCrossings(opts.Sweeps)
	l.coordinates(opts)
	return n, cr, nil
}

// holder is the vertex directly in c, or at the top with None, that is
// v or holds it; -1 when v is not in c.
func holder(d *gw.Document, v, c gw.VertexIndex) int32 {
	// bound: the parent chain is acyclic, so at most the vertex count
	for n := 0; v != gw.None && n <= len(d.Vertices); n++ {
		p := d.Vertices[v].Parent
		if p == c {
			return int32(v)
		}
		v = p
	}
	return -1
}

// box sizes container c around its contents as just laid out, as the
// document fits it, its own size at least, with its centre on the grid, and keeps where that
// centre fell from the container's own and each content vertex's offset
// from it, all in the document's axes.
func (l *Layouter) box(d *gw.Document, c gw.VertexIndex, opts Options) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for i := range d.Vertices {
		if l.liveReal[i] {
			x, y := l.swap(l.x[i], l.y[i])
			w, h := l.swap(l.w[i], l.h[i])
			x0, x1 = math.Min(x0, x-float64(w/2)), math.Max(x1, x+float64(w/2))
			y0, y1 = math.Min(y0, y-float64(h/2)), math.Max(y1, y+float64(h/2))
		}
	}
	w, h := x1-x0+2*gw.ContainerPad, y1-y0+2*gw.ContainerPad+gw.ContainerTitle
	ex, ey := float64((x0+x1)/2), float64((y0+y1)/2)-gw.ContainerTitle/2
	// Grown to its own size from the top left corner, as the fit does.
	if own := d.Vertices[c].Min; w < own.W {
		ex, w = ex+float64((own.W-w)/2), own.W
	}
	if own := d.Vertices[c].Min; h < own.H {
		ey, h = ey+float64((own.H-h)/2), own.H
	}
	cx := float64(math.Round(ex/opts.Grid) * opts.Grid)
	cy := float64(math.Round(ey/opts.Grid) * opts.Grid)
	l.cx[c], l.cy[c] = cx, cy
	l.bw[c], l.bh[c] = w, h
	for i := range d.Vertices {
		if l.liveReal[i] {
			x, y := l.swap(l.x[i], l.y[i])
			l.rx[i], l.ry[i] = x-cx, y-cy
		}
	}
}

// swap turns a pair between the frame's axes and the document's: as it
// is for a left-to-right flow, swapped for top to bottom.
func (l *Layouter) swap(x, y float64) (float64, float64) {
	if l.down {
		return y, x
	}
	return x, y
}

func layersError(need, have int) error {
	return errors.New("layout needs " + strconv.Itoa(need) + " layers, over the budget of " + strconv.Itoa(have))
}

func budgetError(need int64, have int) error {
	return errors.New("layout needs " + strconv.FormatInt(need, 10) + " dummy vertices, over the budget of " + strconv.Itoa(have))
}
