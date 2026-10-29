package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// A vertex's position is its centre; its shape is drawn around it. The
// geometry here is shared by the renderer and the hit test, so that a
// point names the element that was drawn there.

// portRadius is the radius of a port's circle and of its hit region.
const portRadius = 4

// edgeTolerance is how far from an edge's path a point still hits it.
const edgeTolerance = 4

// outlineBand is how far either side of the outline of a vertex without
// ports a point starts an edge rather than a drag.
const outlineBand = 6

// edgeSamples is the number of segments an edge's path is sampled into
// for the hit test.
const edgeSamples = 16

// PortAt is the absolute position of port p of vertex v, as drawn.
func PortAt(v *gw.Vertex, p gw.PortIndex) gw.Point { return portAt(v, p) }

// portAt is the absolute position of port p of vertex v: ports on a side
// are spaced evenly along it.
func portAt(v *gw.Vertex, p gw.PortIndex) gw.Point {
	side := v.Ports[p].Side
	n, k := 0, 0
	for i := range v.Ports {
		if v.Ports[i].Side == side {
			if gw.PortIndex(i) == p { //nolint:gosec // bounded by the ports per vertex
				k = n
			}
			n++
		}
	}
	t := float64(k+1) / float64(n+1)
	w, h := v.Size.W, v.Size.H
	switch side {
	case gw.Left:
		return gw.Point{X: v.Pos.X - float64(w/2), Y: v.Pos.Y - float64(h/2) + float64(h*t)}
	case gw.Right:
		return gw.Point{X: v.Pos.X + float64(w/2), Y: v.Pos.Y - float64(h/2) + float64(h*t)}
	case gw.Top:
		return gw.Point{X: v.Pos.X - float64(w/2) + float64(w*t), Y: v.Pos.Y - float64(h/2)}
	case gw.Bottom:
		return gw.Point{X: v.Pos.X - float64(w/2) + float64(w*t), Y: v.Pos.Y + float64(h/2)}
	}
	return v.Pos
}

// ends are the points an edge leaves from and arrives at: its ports, or
// without them, for a straight edge, where the line towards the other
// end, or towards the nearest of the points it passes through, leaves
// each shape, and for a curve or an elbow the right and left edge
// centres, or the bottom and top in a top-to-bottom flow, the way the
// layers run; for a backward edge, the left and right, or the top and
// bottom.
func ends(d *gw.Document, e *gw.Edge, through []gw.Point) (a, b gw.Point) {
	from, to := &d.Vertices[e.From], &d.Vertices[e.To]
	if d.Schema.EdgeKinds[e.Kind].Route == gw.Straight && e.From != e.To {
		a, b = from.Pos, to.Pos
		if e.FromPort != gw.None {
			a = portAt(from, e.FromPort)
		}
		if e.ToPort != gw.None {
			b = portAt(to, e.ToPort)
		}
		ta, tb := b, a
		if n := len(through); n > 0 {
			ta, tb = through[0], through[n-1]
		}
		ca, cb := a, b
		if e.FromPort == gw.None {
			ca = clip(d.Schema.VertexKinds[from.Kind].Shape, from, ta)
		}
		if e.ToPort == gw.None {
			cb = clip(d.Schema.VertexKinds[to.Kind].Shape, to, tb)
		}
		return ca, cb
	}
	down := d.Flow == gw.TopToBottom
	fromH, fromW, toH, toW := float64(from.Size.H/2), float64(from.Size.W/2), float64(to.Size.H/2), float64(to.Size.W/2)
	if backward(d, e) {
		fromH, fromW, toH, toW = -fromH, -fromW, -toH, -toW
	}
	switch {
	case e.FromPort != gw.None:
		a = portAt(from, e.FromPort)
	case down:
		a = gw.Point{X: from.Pos.X, Y: from.Pos.Y + fromH}
	default:
		a = gw.Point{X: from.Pos.X + fromW, Y: from.Pos.Y}
	}
	switch {
	case e.ToPort != gw.None:
		b = portAt(to, e.ToPort)
	case down:
		b = gw.Point{X: to.Pos.X, Y: to.Pos.Y - toH}
	default:
		b = gw.Point{X: to.Pos.X - toW, Y: to.Pos.Y}
	}
	return a, b
}

// backward reports whether an edge runs against the layers, to a vertex
// earlier in the flow than the one it leaves. It leaves the near side of
// its vertex and arrives at the far side, and its legs pull the way it
// runs, so that it passes back through its bends with no hook at each;
// beside an edge the other way between the same two vertices, its lane
// keeps it apart. An edge with a port keeps the side its port is on.
func backward(d *gw.Document, e *gw.Edge) bool {
	if e.FromPort != gw.None || e.ToPort != gw.None {
		return false
	}
	from, to := d.Vertices[e.From].Pos, d.Vertices[e.To].Pos
	if d.Flow == gw.TopToBottom {
		return to.Y < from.Y
	}
	return to.X < from.X
}

// frame is the axes an edge's route is worked out in: the routes below
// are written for layers that run left to right, and a top-to-bottom
// flow swaps x and y on the way in and on the way out, so that a curve
// leaves and arrives vertically and an elbow turns at the middle in y.
// back is whether the edge is backward, so that its curves pull the
// other way.
type frame struct{ down, back bool }

// frameOf is the frame of an edge in the document's flow.
func frameOf(d *gw.Document, e *gw.Edge) frame {
	return frame{down: d.Flow == gw.TopToBottom, back: backward(d, e)}
}

// t takes a point into the frame or back out of it.
func (f frame) t(p gw.Point) gw.Point {
	if f.down {
		return gw.Point{X: p.Y, Y: p.X}
	}
	return p
}

// controls are the two control points of a curved edge: horizontal
// tangents, each pulled half the horizontal distance from its end the
// way the edge runs, so that a curve between layers bends smoothly and
// never turns back, however short. A curve whose end is not ahead the
// way it runs, as between two vertices in one layer, is pulled out by
// minPull at least and loops.
func (f frame) controls(a, b gw.Point) (c1, c2 gw.Point) {
	run := b.X - a.X
	if f.back {
		run = -run
	}
	pull := math.Abs(run) / 2
	if run <= 0 {
		pull = math.Max(pull, minPull)
	}
	if f.back {
		pull = -pull
	}
	return gw.Point{X: a.X + pull, Y: a.Y}, gw.Point{X: b.X - pull, Y: b.Y}
}

// minPull is the least pull of a curve that loops.
const minPull = 24

// at is the point of a route at parameter t in [0, 1].
func (f frame) at(route gw.Route, a, b gw.Point, t float64) gw.Point {
	switch route {
	case gw.Straight:
		return gw.Point{X: a.X + float64((b.X-a.X)*t), Y: a.Y + float64((b.Y-a.Y)*t)}
	case gw.Curve:
		c1, c2 := f.controls(a, b)
		u := 1 - t
		return gw.Point{
			X: float64(u*u*u*a.X) + float64(3*u*u*t*c1.X) + float64(3*u*t*t*c2.X) + float64(t*t*t*b.X),
			Y: float64(u*u*u*a.Y) + float64(3*u*u*t*c1.Y) + float64(3*u*t*t*c2.Y) + float64(t*t*t*b.Y),
		}
	case gw.Orthogonal:
		// Three legs: across to the middle, down, across to the end.
		mid := float64((a.X + b.X) / 2)
		switch {
		case t < 1.0/3:
			return gw.Point{X: a.X + float64((mid-a.X)*t*3), Y: a.Y}
		case t < 2.0/3:
			return gw.Point{X: mid, Y: a.Y + float64((b.Y-a.Y)*(t-1.0/3)*3)}
		default:
			return gw.Point{X: mid + float64((b.X-mid)*(t-2.0/3)*3), Y: b.Y}
		}
	}
	return a
}

// way is the route an edge is drawn along: its frame, its ends, and the
// points it passes through, which are its bends, or for one of several
// edges between the same two vertices (gw.Document.Parallel), a lane:
// a point midway, a lane's height from the next, so that each runs and
// is labelled apart from the others and an edge back beside one forward
// does not lie on it. k and n are its lane and how many there are.
type way struct {
	f       frame
	a, b    gw.Point
	through []gw.Point
	k, n    int
}

// laneGap is how far apart lanes run: a label's height, so that the
// labels above them stack.
const laneGap = labelHeight

// wayOf is the way of edge i; lane holds its lane's point.
func wayOf(d *gw.Document, i gw.EdgeIndex, lane *[1]gw.Point) way {
	e := &d.Edges[i]
	w := way{f: frameOf(d, e), through: e.Bends, n: 1}
	w.a, w.b = ends(d, e, e.Bends)
	k, n := d.Parallel(i)
	if n < 2 {
		return w
	}
	p, q := w.f.t(w.a), w.f.t(w.b)
	run := q.X - p.X
	if w.f.back {
		run = -run
	}
	if run <= 0 {
		return w // it loops, and its lane would only add a turn
	}
	if d.Schema.EdgeKinds[e.Kind].Route == gw.Straight {
		p, q = w.f.t(d.Vertices[e.From].Pos), w.f.t(d.Vertices[e.To].Pos)
	}
	off := float64((float64(k) - float64(float64(n-1)/2)) * laneGap)
	lane[0] = w.f.t(gw.Point{X: float64((p.X + q.X) / 2), Y: float64((p.Y+q.Y)/2) + off})
	w.through, w.k, w.n = lane[:], k, n
	w.a, w.b = ends(d, e, w.through)
	return w
}

// An edge with bends is drawn through them in legs, from a to the first
// bend, between bends, and from the last to b, each leg drawn as the
// route draws a whole edge: a line, a curve leaving and arriving level,
// or an elbow at its middle.

// leg is the k-th of the len(bends)+1 legs from a through the bends to b.
func leg(a gw.Point, bends []gw.Point, b gw.Point, k int) (p, q gw.Point) {
	p, q = a, b
	if k > 0 {
		p = bends[k-1]
	}
	if k < len(bends) {
		q = bends[k]
	}
	return p, q
}

// legRoute is how a leg, in the frame, is drawn: as the edge's route,
// except a level leg of a bent edge, which is a line, so that a run past
// a layer stays level however short it is.
func legRoute(route gw.Route, bent bool, p, q gw.Point) gw.Route {
	if bent && p.Y == q.Y {
		return gw.Straight
	}
	return route
}

// along is the point at parameter t in [0, 1] of the route from a
// through the bends to b, each leg an equal share of t.
func along(f frame, route gw.Route, a gw.Point, bends []gw.Point, b gw.Point, t float64) gw.Point {
	n := len(bends) + 1
	s := float64(t * float64(n))
	k := min(int(s), n-1)
	p, q := leg(a, bends, b, k)
	p, q = f.t(p), f.t(q)
	return f.t(f.at(legRoute(route, len(bends) > 0, p, q), p, q, s-float64(k)))
}

// middle is where an edge's label goes: the middle of its route, or
// with bends the middle of the leg ending at bend len/2 rounded up to
// odd, which for the pairs a layout gives is the run past the middle
// layer the edge crosses, level and clear of its vertices. In a lane it
// is the lane's point; in a top-to-bottom flow, whose labels are beside
// their lanes, moved along it by a label's height a lane, so that they
// stack.
func (w *way) middle(route gw.Route) gw.Point {
	if w.n > 1 {
		p := w.f.t(w.through[0])
		if w.f.down {
			p.X += float64((float64(w.k) - float64(float64(w.n-1)/2)) * labelHeight)
		}
		return w.f.t(p)
	}
	return middle(w.f, route, w.a, w.through, w.b)
}

func middle(f frame, route gw.Route, a gw.Point, bends []gw.Point, b gw.Point) gw.Point {
	if len(bends) < 2 {
		return along(f, route, a, bends, b, 0.5)
	}
	k := len(bends)/2 | 1
	p, q := f.t(bends[k-1]), f.t(bends[k])
	return f.t(f.at(legRoute(route, true, p, q), p, q, 0.5))
}

// lastFrom is where an edge's last leg starts: its last bend, or a.
func lastFrom(a gw.Point, bends []gw.Point) gw.Point {
	if n := len(bends); n > 0 {
		return bends[n-1]
	}
	return a
}

// headLength is how far an arrowhead reaches back from an edge's end.
const headLength = 9

// shortened is where an edge's stroke ends when its kind has a closed
// arrowhead: the head's length back from the end along the last leg, as
// it is drawn, so that a thick stroke never shows past the head's tip.
// An open head is two strokes the edge runs up to; a leg shorter than
// the head is left as it is.
func (w *way) shortened(kind gw.EdgeKind) gw.Point {
	f := w.f
	if !kind.Arrow || kind.Head == gw.OpenHead {
		return w.b
	}
	a, b := f.t(lastFrom(w.a, w.through)), f.t(w.b)
	var dx, dy float64
	switch legRoute(kind.Route, len(w.through) > 0, a, b) {
	case gw.Straight:
		dx, dy = b.X-a.X, b.Y-a.Y
	case gw.Curve:
		_, c2 := f.controls(a, b)
		dx, dy = b.X-c2.X, b.Y-c2.Y
	case gw.Orthogonal:
		dx, dy = b.X-float64((a.X+b.X)/2), 0
	}
	l := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	if l <= headLength {
		return f.t(b)
	}
	return f.t(gw.Point{X: b.X - float64(dx/l*headLength), Y: b.Y - float64(dy/l*headLength)})
}

// band is the vertex's outline band: outlineBand, or a quarter of its
// smaller side when that is less, so a small vertex keeps an inside to
// drag it by.
func band(v *gw.Vertex) float64 {
	return math.Min(outlineBand, math.Min(v.Size.W, v.Size.H)/4)
}

// outlineDistance is how far p lies from the outline of the shape drawn
// for the vertex, negative inside: to the box for the rectangles, to
// the side in p's quadrant for a diamond, along the radius scaled to
// the smaller semi-axis for an ellipse.
func outlineDistance(shape gw.Shape, v *gw.Vertex, p gw.Point) float64 {
	dx, dy := math.Abs(p.X-v.Pos.X), math.Abs(p.Y-v.Pos.Y)
	a, b := float64(v.Size.W/2), float64(v.Size.H/2)
	if a > 0 && b > 0 {
		switch shape {
		case gw.Diamond:
			return (dx/a + dy/b - 1) / math.Sqrt(1/(a*a)+1/(b*b))
		case gw.Circle:
			return (math.Sqrt(dx*dx/(a*a)+dy*dy/(b*b)) - 1) * math.Min(a, b)
		case gw.Rect, gw.RoundRect, gw.Pill, gw.Actor, gw.DataStore:
		}
	}
	return math.Max(dx-a, dy-b)
}

// clip is where the line from the vertex's centre towards p leaves the
// shape drawn for it; the centre itself when p is the centre.
func clip(shape gw.Shape, v *gw.Vertex, p gw.Point) gw.Point {
	dx, dy := p.X-v.Pos.X, p.Y-v.Pos.Y
	a, b := v.Size.W/2, v.Size.H/2
	if dx == 0 && dy == 0 || !(a > 0) || !(b > 0) {
		return v.Pos
	}
	var t float64
	switch shape {
	case gw.Diamond:
		t = 1 / (math.Abs(dx)/a + math.Abs(dy)/b)
	case gw.Circle:
		t = 1 / math.Sqrt(dx*dx/(a*a)+dy*dy/(b*b))
	case gw.Rect, gw.RoundRect, gw.Pill, gw.Actor, gw.DataStore:
		t = math.Min(a/math.Abs(dx), b/math.Abs(dy))
	}
	return gw.Point{X: v.Pos.X + float64(dx*t), Y: v.Pos.Y + float64(dy*t)}
}

// inBox reports whether p lies within the vertex's box.
func inBox(v *gw.Vertex, p gw.Point) bool {
	return math.Abs(p.X-v.Pos.X) <= v.Size.W/2 && math.Abs(p.Y-v.Pos.Y) <= v.Size.H/2
}

// segmentDistance is the distance from p to the segment ab.
func segmentDistance(p, a, b gw.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := float64(dx*dx) + float64(dy*dy)
	t := 0.0
	if l2 > 0 {
		t = (float64((p.X-a.X)*dx) + float64((p.Y-a.Y)*dy)) / l2
		t = math.Max(0, math.Min(1, t))
	}
	qx, qy := a.X+float64(dx*t)-p.X, a.Y+float64(dy*t)-p.Y
	return math.Sqrt(float64(qx*qx) + float64(qy*qy))
}

// nearEdge reports whether p is within the tolerance of the edge's
// path: each leg sampled into edgeSamples segments, or one when it is a
// line.
func nearEdge(d *gw.Document, i gw.EdgeIndex, p gw.Point) bool {
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	route := d.Schema.EdgeKinds[d.Edges[i].Kind].Route
	f := wy.f
	p = f.t(p)
	for k := range len(wy.through) + 1 {
		u, w := leg(wy.a, wy.through, wy.b, k)
		u, w = f.t(u), f.t(w)
		r := legRoute(route, len(wy.through) > 0, u, w)
		n := edgeSamples
		if r == gw.Straight {
			n = 1
		}
		prev := u
		// bound: edgeSamples
		for i := 1; i <= n; i++ {
			next := f.at(r, u, w, float64(i)/float64(n))
			if segmentDistance(p, prev, next) <= edgeTolerance {
				return true
			}
			prev = next
		}
	}
	return false
}

// Margin is the space Bounds leaves around the vertices.
const Margin = 24

// Bounds is the box around every live vertex with a margin, which a
// viewport the host left empty takes; an empty document gets a unit box.
func Bounds(d *gw.Document) gw.Viewport { return bounds(d) }

// bounds is the box around every live vertex and every bend, with a
// margin, for a viewport the host left empty; an empty document gets a
// unit box. An actor's label, which is not cut, counts at the default
// character width.
// usable reports whether a viewport can be drawn: its corner within
// gw.MaxCoord of the origin, its width and height positive and at most
// gw.MaxCoord. Any other, the empty one included, gives way to the
// drawing's bounds.
func usable(v gw.Viewport) bool {
	in := func(x float64) bool { return x >= -gw.MaxCoord && x <= gw.MaxCoord }
	return in(v.X) && in(v.Y) && v.W > 0 && v.W <= gw.MaxCoord && v.H > 0 && v.H <= gw.MaxCoord
}

func bounds(d *gw.Document) gw.Viewport {
	const margin = Margin
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for i := range d.Vertices {
		v := &d.Vertices[i]
		if !v.Live {
			continue
		}
		half := v.Size.W / 2
		if d.Schema.VertexKinds[v.Kind].Shape == gw.Actor {
			half = math.Max(half, float64(runeCount(v.Label))*defaultCharWidth/2)
		}
		x0 = math.Min(x0, v.Pos.X-half)
		y0 = math.Min(y0, v.Pos.Y-float64(v.Size.H/2))
		x1 = math.Max(x1, v.Pos.X+half)
		y1 = math.Max(y1, v.Pos.Y+float64(v.Size.H/2))
	}
	for i := range d.Edges {
		if e := &d.Edges[i]; e.Live {
			for _, p := range e.Bends {
				x0, y0, x1, y1 = math.Min(x0, p.X), math.Min(y0, p.Y), math.Max(x1, p.X), math.Max(y1, p.Y)
			}
		}
	}
	if x0 > x1 {
		return gw.Viewport{W: 1, H: 1}
	}
	return gw.Viewport{X: x0 - margin, Y: y0 - margin, W: x1 - x0 + 2*margin, H: y1 - y0 + 2*margin}
}
