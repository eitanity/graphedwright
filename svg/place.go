package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// A label at the middle of its route can cross another edge, another
// edge's label or a vertex: edges that fan out of one vertex run close
// together near it, and edges that share a short gap share its middle.
// Once a layout has placed everything, LabelPlacer moves each such label
// along its own route to the nearest place where it is clear, and keeps
// it there (gw.Document.SetLabelAt), so that every drawing of the
// layout, and its hit test, finds it there without searching. A label
// clear at its middle stays there; one clear nowhere goes where it
// crosses fewest, if that is fewer than at its middle.
// A lane's label, at its lane's point, and an orthogonal edge's on its
// own last leg, are not moved.

// placeSamples is how many legs of a route the placer tests a label's
// box against: enough to follow a curve to within a few units.
const placeSamples = 32

// placeSteps is how many places either side of the middle a label is
// tried at, each placeStep of its route further, and placeEnds how near
// either end of its route it may go, as a fraction of the route.
const (
	placeSteps = 16
	placeStep  = 0.05
	placeEnds  = 0.1
)

// LabelPlacer chooses the places of the labels of a laid-out document's
// edges. Its arrays are sized once, for a capacity of edges.
type LabelPlacer struct {
	hull  []gw.Viewport // each live edge's route's bounding box, as drawn
	label []gw.Viewport // each edge's label's box where it now is; zero W for none
}

// NewLabelPlacer makes a placer for documents of up to edges edges.
func NewLabelPlacer(edges int) *LabelPlacer {
	edges = max(edges, 0)
	return &LabelPlacer{hull: make([]gw.Viewport, edges), label: make([]gw.Viewport, edges)}
}

// Place moves each label of d that crosses another edge, label or
// vertex at its middle to the nearest place along its route where it
// crosses none, in edge order, each label placed before it where it was
// put. A document with more edges than the placer was made for, or one
// locked, is left as it is.
func (p *LabelPlacer) Place(d *gw.Document) {
	n := d.EdgeSlots()
	if n > len(p.hull) {
		return
	}
	for i := range n {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		p.hull[i], p.label[i] = gw.Viewport{}, gw.Viewport{}
		if !d.Edge(e).Live() {
			continue
		}
		p.hull[i] = routeHull(d, e)
		if d.EdgeLabelRunes(e) > 0 {
			p.label[i] = EdgeLabelBox(d, e)
		}
	}
	for i := range n {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if p.label[i].W == 0 {
			continue
		}
		if c := p.crossings(d, e, p.label[i]); c > 0 {
			p.move(d, e, c)
		}
	}
}

// move tries edge e's label, which crosses c things at its middle, at
// places either side of it, nearer first and toward the target first,
// and keeps the first that crosses nothing, or failing that the first
// that crosses fewest, if fewer than c.
func (p *LabelPlacer) move(d *gw.Document, e gw.EdgeIndex, c int) {
	var lane [1]gw.Point
	w := wayOf(d, e, &lane)
	if w.n > 1 || w.ownLeg {
		return
	}
	route := d.Kinds().Edge(d.Edge(e).Kind()).Route()
	t0 := middleT(w.f, w.through)
	best, bestT, bestBox := c, 0.0, gw.Viewport{}
	// bound: placeSteps places either side
	for k := 1; k <= placeSteps && best > 0; k++ {
		for _, sign := range [2]float64{1, -1} {
			t := t0 + float64(sign*float64(k)*placeStep)
			if t < placeEnds || t > 1-placeEnds {
				continue
			}
			mid := along(w.f, route, w.a, w.through, w.b, t)
			box := labelBoxAt(d, e, w.f, mid, w.side())
			if n := p.crossings(d, e, box); n < best {
				best, bestT, bestBox = n, t, box
				if n == 0 {
					break
				}
			}
		}
	}
	if bestT > 0 && d.SetLabelAt(e, bestT) {
		p.label[e] = bestBox
	}
}

// middleT is where along a route, as a fraction, middle puts its label.
func middleT(f frame, bends gw.PointList) float64 {
	n := bends.Len()
	if n < 2 {
		return 0.5
	}
	k := n/2 | 1
	p, q := f.t(bends.At(k-1)), f.t(bends.At(k))
	if p.X == q.X && p.Y != q.Y {
		if k >= 3 {
			k -= 2
		} else if k+2 < n {
			k += 2
		}
	}
	return float64(float64(k)+0.5) / float64(n+1)
}

// crossings counts what box crosses: the routes but edge e's own, the
// other edges' labels where they now are, and the vertices that hold
// nothing.
func (p *LabelPlacer) crossings(d *gw.Document, e gw.EdgeIndex, box gw.Viewport) int {
	n := 0
	for i := range d.VertexSlots() {
		v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if v.Live() && !d.Kinds().Vertex(v.Kind()).Container() && boxesMeet(box, vertexBox(v)) {
			n++
		}
	}
	for i := range d.EdgeSlots() {
		o := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if o == e || p.hull[i].W == 0 && p.hull[i].H == 0 {
			continue
		}
		if p.label[i].W > 0 && boxesMeet(box, p.label[i]) {
			n++
		}
		if boxesMeet(box, p.hull[i]) && routeCrosses(d, o, box) {
			n++
		}
	}
	return n
}

// routeHull is the bounding box of edge e's route as drawn, sampled.
func routeHull(d *gw.Document, e gw.EdgeIndex) gw.Viewport {
	var lane [1]gw.Point
	w := wayOf(d, e, &lane)
	route := d.Kinds().Edge(d.Edge(e).Kind()).Route()
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	// bound: placeSamples legs
	for k := 0; k <= placeSamples; k++ {
		q := along(w.f, route, w.a, w.through, w.b, float64(k)/placeSamples)
		x0, y0, x1, y1 = math.Min(x0, q.X), math.Min(y0, q.Y), math.Max(x1, q.X), math.Max(y1, q.Y)
	}
	return gw.Viewport{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// routeCrosses reports whether edge e's route as drawn, sampled, passes
// through box.
func routeCrosses(d *gw.Document, e gw.EdgeIndex, box gw.Viewport) bool {
	var lane [1]gw.Point
	w := wayOf(d, e, &lane)
	route := d.Kinds().Edge(d.Edge(e).Kind()).Route()
	prev := w.a
	// bound: placeSamples legs
	for k := 1; k <= placeSamples; k++ {
		q := along(w.f, route, w.a, w.through, w.b, float64(k)/placeSamples)
		if segmentMeets(prev, q, box) {
			return true
		}
		prev = q
	}
	return false
}

// segmentMeets reports whether the segment from p to q passes through
// the inside of box, by clipping it to the box.
func segmentMeets(p, q gw.Point, box gw.Viewport) bool {
	lo, hi := 0.0, 1.0
	dx, dy := q.X-p.X, q.Y-p.Y
	for _, c := range [4][2]float64{{-dx, p.X - box.X}, {dx, box.X + box.W - p.X}, {-dy, p.Y - box.Y}, {dy, box.Y + box.H - p.Y}} {
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

// boxesMeet reports whether two boxes share an inside.
func boxesMeet(a, b gw.Viewport) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// vertexBox is a vertex's box.
func vertexBox(v gw.VertexView) gw.Viewport {
	return gw.Viewport{X: v.Pos().X - float64(v.Size().W/2), Y: v.Pos().Y - float64(v.Size().H/2), W: v.Size().W, H: v.Size().H}
}
