package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// A label at the middle of its route can cross another edge, another
// edge's label or a vertex: edges that fan out of one vertex run close
// together near it, and edges that share a short gap share its middle.
// Once a layout has placed everything, PlaceLabels moves each such label
// along its own route to the nearest place where it is clear, and keeps
// it there (gw.Document.SetLabelAt), so that every drawing of the
// layout, and its hit test, finds it there without searching. A label
// clear at its middle stays there; one clear nowhere goes where it
// crosses fewest, if that is fewer than at its middle, a vertex counted
// as placeVertex lines: a label across a line is read, one on a box
// hides it.
// A lane's label, at its lane's point, and an orthogonal edge's on its
// own last leg, are not moved.

// placeSamples is how many legs of a route the placer tests a label's
// box against: enough to follow a curve to within a few units.
const placeSamples = 32

// placeSteps is how many places either side of the middle a label is
// tried at, each placeStep of its route further, and placeEnds how near
// either end of its route it may go, as a fraction of the route.
const (
	placeSteps = 38
	placeStep  = 0.025
	placeEnds  = 0.02
)

// placeVertex is what a vertex under a label counts for, in lines
// crossed: more than a label crosses in all but the densest drawing.
const placeVertex = 64

// PlaceEdges is the most edges a document may have for PlaceLabels to
// place its labels. Each place a label is tried at is tested against
// every other edge, so the work grows as the square of the edges: with
// 128 edges, each label crossed, it takes some 150 ms natively on a
// desktop machine. Past this a document's labels stay where they are.
const PlaceEdges = 128

// PlaceLabels moves each label of d that crosses another edge, label or
// vertex where it is to the nearest place along its route where it
// crosses none, in edge order, each label placed before it where it was
// put. It keeps nothing of its own and allocates nothing. A document
// with more than PlaceEdges edges, or locked (gw.Document.SetLabelAt),
// is left as it is.
func PlaceLabels(d *gw.Document) {
	if d.EdgeSlots() > PlaceEdges {
		return
	}
	for i := range d.EdgeSlots() {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by PlaceEdges
		if d.EdgeLabelRunes(e) == 0 {
			continue // a deleted edge's label is refused its place by SetLabelAt
		}
		var lane [1]gw.Point
		w := wayOf(d, e, &lane)
		move(d, e, crossings(d, e, drawnLabel(d, e, &w, w.middle(d.Kinds().Edge(d.Edge(e).Kind()).Route()))))
	}
}

// move tries edge e's label, which crosses c things where it is, at
// its middle on the other side of its line, then at places either side
// of the middle, nearer first and toward the target first, each on its
// own side and then the other, and keeps the first that crosses
// nothing, or failing that the first that crosses fewest, if fewer than
// c; where c is 0 it stays.
func move(d *gw.Document, e gw.EdgeIndex, c int) {
	var lane [1]gw.Point
	w := wayOf(d, e, &lane)
	if w.n > 1 || w.ownLeg {
		return
	}
	route := d.Kinds().Edge(d.Edge(e).Kind()).Route()
	t0 := middleT(w.f, w.through)
	best, bestT := c, 0.0
	// bound: placeSteps places either side and the middle, which the
	// two signs try alike and its own side crosses c, never fewer;
	// stopping at the first clear one saves work only, as a later one is
	// never fewer
	for k := 0; k <= placeSteps && best > 0; k++ {
		for _, sign := range [2]float64{1, -1} {
			t := t0 + float64(sign*float64(k)*placeStep)
			if t < placeEnds || t > 1-placeEnds {
				continue
			}
			if n, flip := bothSides(d, e, &w, along(w.f, route, w.a, w.through, w.b, t)); n < best {
				best, bestT = n, t
				if flip {
					bestT = -t
				}
			}
		}
	}
	d.SetLabelAt(e, bestT) // refused where none was better: bestT is 0
}

// bothSides is how many things edge e's label at mid on its way w
// crosses on its own side of its line, or on the other, flip, where that
// crosses fewer.
func bothSides(d *gw.Document, e gw.EdgeIndex, w *way, mid gw.Point) (n int, flip bool) {
	w.flip = false
	n = crossings(d, e, drawnLabel(d, e, w, mid))
	w.flip = true
	if o := crossings(d, e, drawnLabel(d, e, w, mid)); o < n {
		return o, true
	}
	return n, false
}

// drawnLabel is the box edge e's label takes, as drawn, at mid on its
// way w.
func drawnLabel(d *gw.Document, e gw.EdgeIndex, w *way, mid gw.Point) gw.Viewport {
	b, turned := labelBoxAt(d, e, w, mid)
	return drawnBox(d, e, b, w.side(), turned)
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
// other edges' labels where they now are, and, placeVertex each, the
// vertices that hold nothing.
func crossings(d *gw.Document, e gw.EdgeIndex, box gw.Viewport) int {
	n := 0
	for i := range d.VertexSlots() {
		v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if v.Live() && !d.Kinds().Vertex(v.Kind()).Container() && boxesMeet(box, vertexBox(v)) {
			n += placeVertex
		}
	}
	for i := range d.EdgeSlots() {
		o := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		if o == e || !d.Edge(o).Live() {
			continue
		}
		var lane [1]gw.Point
		w := wayOf(d, o, &lane)
		route := d.Kinds().Edge(d.Edge(o).Kind()).Route()
		if d.EdgeLabelRunes(o) > 0 {
			if boxesMeet(box, drawnLabel(d, o, &w, w.middle(route))) {
				n++
			}
		}
		if w.crosses(route, box) {
			n++
		}
	}
	return n
}

// crosses reports whether the route through w as drawn, sampled, passes
// through box.
func (w *way) crosses(route gw.Route, box gw.Viewport) bool {
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
