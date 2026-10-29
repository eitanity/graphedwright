package layout

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// An edge into or out of a container is laid out at the level it
// crosses, between the nodes there that hold its ends, so its route
// reaches the container and no further. Inside, a straight run to the
// member that holds its end would cross the members laid out before it.
// So once the layout is fitted, the run inside each container the edge
// crosses goes round them instead: along the free band above or below
// what the container holds, across to the gap before the member's
// layer, along that gap to the member's height, and in. The four points
// keep the bends in pairs and in the edge's own direction. A run is
// left straight where going round would break either, or where the leg
// into the band would itself cross a vertex.

// maxCross is how many containers on each side of an edge are routed
// inside; one nested deeper keeps the straight run inside it.
const maxCross = 2

// insidePts is the most points an edge's inside runs take, both sides.
const insidePts = 2 * maxCross * 4

// crossed counts the containers, up to maxCross, that hold v below the
// level of container c, the ones an edge at that level from or to v
// enters.
func crossed(d *gw.Document, v, c gw.VertexIndex) int {
	n := 0
	// bound: the parent chain is acyclic, so at most the vertex count
	for p, k := d.Vertices[v].Parent, 0; p != gw.None && p != c && n < maxCross && k <= len(d.Vertices); p, k = d.Vertices[p].Parent, k+1 {
		n++
	}
	return n
}

// frameBox is a vertex's box in the frame's axes: layers along x.
type frameBox struct{ x0, x1, y0, y1, cy float64 }

func (l *Layouter) frame(v *gw.Vertex) frameBox {
	x, y := l.swap(v.Pos.X, v.Pos.Y)
	w, h := l.swap(v.Size.W, v.Size.H)
	return frameBox{x0: x - float64(w/2), x1: x + float64(w/2), y0: y - float64(h/2), y1: y + float64(h/2), cy: y}
}

// run is one inside run being found: the edge, and the frame point it
// comes from outside the container.
type run struct {
	e      gw.EdgeIndex
	fx, fy float64
	up     bool // the run must go to a lower x, as for an edge reversed at its level
}

// inside writes into buf the run inside container c to its member m. It
// returns how many points it wrote: none when the far point is not beyond
// the side of c the run's direction enters from, when no member stands
// between that side and m, or when no band, gap and entry leg are clear.
func (l *Layouter) inside(d *gw.Document, c, m gw.VertexIndex, r run, buf []gw.Point) int {
	if len(buf) < 4 {
		return 0
	}
	mb, cb := l.frame(&d.Vertices[m]), l.frame(&d.Vertices[c])
	low := !r.up // entered from the low side, going to higher x
	if (low && r.fx >= cb.x0) || (!low && r.fx <= cb.x1) {
		return 0
	}
	sp := l.span(d, c, m, mb, low)
	near, top, bottom, col0, col1 := sp.near, sp.top, sp.bottom, sp.col0, sp.col1
	g := float64(gw.ContainerPad) / 2
	if math.IsInf(near, 0) {
		// m is first on that side, so only the members of its own layer
		// can be in the way: come in level at m's height, from just outside.
		return l.alongPad(d, r, cb, mb, low, g, buf)
	}
	in, gx, last := cb.x0+g, (near+col0)/2, float64((near+col0)/4)+float64(mb.x0/2)
	if !low {
		in, gx, last = cb.x1-g, (near+col1)/2, float64((near+col1)/4)+float64(mb.x1/2)
	}
	bands := [2]float64{top - g, bottom + g}
	if math.Abs(r.fy-bands[1]) < math.Abs(r.fy-bands[0]) {
		bands[0], bands[1] = bands[1], bands[0] // the band on the far point's side first
	}
	for _, by := range bands {
		if !l.clearRun(d, c, m, gx, by, mb.cy) || !l.clearLeg(d, r.e, r.fx, r.fy, in, by) {
			continue
		}
		pts := [4][2]float64{{in, by}, {gx, by}, {gx, mb.cy}, {last, mb.cy}}
		for k, p := range pts {
			x, y := l.swap(p[0], p[1])
			buf[k] = gw.Point{X: x, Y: y}
		}
		return 4
	}
	return 0
}

// span is what the members of a container other than m leave round it:
// near, the facing edge of those between the side the run enters from
// and m, infinite when there are none; top and bottom, the extent of
// them all and m; and col0 and col1, the extent of m's layer.
type span struct{ near, top, bottom, col0, col1 float64 }

func (l *Layouter) span(d *gw.Document, c, m gw.VertexIndex, mb frameBox, low bool) span {
	sp := span{near: math.Inf(-1), top: mb.y0, bottom: mb.y1, col0: mb.x0, col1: mb.x1}
	if !low {
		sp.near = math.Inf(1)
	}
	for i := range d.Vertices {
		o := &d.Vertices[i]
		if !o.Live || o.Parent != c || gw.VertexIndex(i) == m { //nolint:gosec // bounded by the capacity
			continue
		}
		ob := l.frame(o)
		sp.top, sp.bottom = math.Min(sp.top, ob.y0), math.Max(sp.bottom, ob.y1)
		switch {
		case ob.x1 <= mb.x0 && low:
			sp.near = math.Max(sp.near, ob.x1)
		case ob.x0 >= mb.x1 && !low:
			sp.near = math.Min(sp.near, ob.x0)
		case ob.x1 > mb.x0 && ob.x0 < mb.x1: // in m's layer
			sp.col0, sp.col1 = math.Min(sp.col0, ob.x0), math.Max(sp.col1, ob.x1)
		}
	}
	return sp
}

// alongPad is the run to a member first on its side of its container,
// at the member's height: two points, half the padding outside the
// container and on its side, when the straight leg to the member would
// cross a vertex and the leg to the first point crosses none. The last
// leg, across the padding, is long enough for an arrowhead.
func (l *Layouter) alongPad(d *gw.Document, r run, cb, mb frameBox, low bool, g float64, buf []gw.Point) int {
	side, out := cb.x0, cb.x0-g
	if !low {
		side, out = cb.x1, cb.x1+g
	}
	// The edge is drawn toward the member's centre.
	if l.clearLeg(d, r.e, r.fx, r.fy, (mb.x0+mb.x1)/2, mb.cy) || !l.clearLeg(d, r.e, r.fx, r.fy, out, mb.cy) {
		return 0
	}
	for k, x := range [2]float64{out, side} {
		px, py := l.swap(x, mb.cy)
		buf[k] = gw.Point{X: px, Y: py}
	}
	return 2
}

// clearRun reports whether the run along x = gx from y0 to y1, in the
// frame, misses every member of c but m.
func (l *Layouter) clearRun(d *gw.Document, c, m gw.VertexIndex, gx, y0, y1 float64) bool {
	lo, hi := math.Min(y0, y1), math.Max(y0, y1)
	for i := range d.Vertices {
		o := &d.Vertices[i]
		if !o.Live || o.Parent != c || gw.VertexIndex(i) == m { //nolint:gosec // bounded by the capacity
			continue
		}
		if ob := l.frame(o); ob.x0 < gx && gx < ob.x1 && ob.y0 < hi && lo < ob.y1 {
			return false
		}
	}
	return true
}

// clearLeg reports whether the leg from (x0, y0) to (x1, y1), in the
// frame, misses every vertex that holds neither end of edge e.
func (l *Layouter) clearLeg(d *gw.Document, e gw.EdgeIndex, x0, y0, x1, y1 float64) bool {
	ed := &d.Edges[e]
	for i := range d.Vertices {
		v := &d.Vertices[i]
		u := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if !v.Live || d.Within(ed.From, u) || d.Within(ed.To, u) {
			continue
		}
		if meets(x0, y0, x1, y1, l.frame(v)) {
			return false
		}
	}
	return true
}

// meets reports whether the segment from (x0, y0) to (x1, y1) passes
// through the inside of box b, by clipping it to b.
func meets(x0, y0, x1, y1 float64, b frameBox) bool {
	lo, hi := 0.0, 1.0
	dx, dy := x1-x0, y1-y0
	for _, c := range [4][2]float64{{-dx, x0 - b.x0}, {dx, b.x1 - x0}, {-dy, y0 - b.y0}, {dy, b.y1 - y0}} {
		den, num := c[0], c[1]
		if den == 0 {
			if num <= 0 {
				return false
			}
			continue
		}
		t := num / den
		if den < 0 {
			lo = math.Max(lo, t)
		} else {
			hi = math.Min(hi, t)
		}
		if lo >= hi {
			return false
		}
	}
	return true
}

// insideRuns writes the inside runs of edge e to its end v, coming from
// far outside, into buf from the outermost container in, and returns the
// count. up says the runs go to lower x.
func (l *Layouter) insideRuns(d *gw.Document, e gw.EdgeIndex, v gw.VertexIndex, far gw.Point, up bool, buf []gw.Point) int {
	level := eLevelOf(d, e)
	var chain [maxCross]gw.VertexIndex // the members on the way in, innermost first
	var box [maxCross]gw.VertexIndex   // the containers holding them
	n := 0
	m := v
	// bound: the parent chain, at most maxCross containers
	for p := d.Vertices[v].Parent; p != gw.None && p != level && n < maxCross; p = d.Vertices[p].Parent {
		chain[n], box[n] = m, p
		n++
		m = p
	}
	r := run{e: e, up: up}
	r.fx, r.fy = l.swap(far.X, far.Y)
	w := 0
	for k := n - 1; k >= 0; k-- {
		if got := l.inside(d, box[k], chain[k], r, buf[w:]); got > 0 {
			w += got
			r.fx, r.fy = l.swap(buf[w-1].X, buf[w-1].Y) // the next container in is entered from here
		}
	}
	return w
}

// eLevelOf is the container whose level an edge is laid out at: the
// innermost holding both its ends, or None at the top.
func eLevelOf(d *gw.Document, e gw.EdgeIndex) gw.VertexIndex {
	ed := &d.Edges[e]
	// bound: the parent chain is acyclic, so at most the vertex count
	for c, k := d.Vertices[ed.From].Parent, 0; c != gw.None && k <= len(d.Vertices); c, k = d.Vertices[c].Parent, k+1 {
		if d.Within(ed.To, c) {
			return c
		}
	}
	return gw.None
}
