package layout

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// An edge between adjacent layers is drawn from end to end across the
// gap between them, and its label at its middle, as wide as the label
// room makes the gap. Where its ends are at different heights the edge
// climbs across the gap, and the label, level, or turned along it top
// to bottom, crosses its own line and the edges fanning out of the ends
// beside it. So a labelled curved or orthogonal edge whose ends are
// further apart across the flow than a line of text runs level out of
// its quieter end, the one at the side with fewer ends, for as long as
// its label, through a pair of bends at that end's height, and climbs
// to the other end in the last runClimb of the gap. The label, taken
// from the middle pair of the route, lies on that run; the gap is given
// the room (climbRoom); and since every such run out of one side climbs
// in the same stretch, beside the far layer, the runs and the routes
// from the far side's ends keep their order.

// runClimb is the length along the flow in which a label run climbs to
// its far end.
const runClimb = 64

// runWidths finds the length of the label run each edge of the level
// would have, its label's room; 0 for an edge that has none: one between
// layers not adjacent, which has dummies, one without a label, and one
// with an end not spread at its side (endSideOf), as a straight one's or
// one not of the level.
func (l *Layouter) runWidths(d *gw.Document) {
	for i := range l.edges {
		l.runW[i] = 0 // an edge not of the level has its ends in layer 0, and no sides
		if a, b := l.layer[l.eFrom[i]], l.layer[l.eTo[i]]; a-b != 1 && b-a != 1 {
			continue
		}
		_, sf := l.endSideOf(d, i, 0)
		_, st := l.endSideOf(d, i, 1)
		if n := d.EdgeLabelRunes(gw.EdgeIndex(i)); n > 0 && sf != 0 && st != 0 { //nolint:gosec // bounded by the capacity
			l.runW[i] = float64(float64(n)*labelChar) + 2*labelPad
		}
	}
}

// climbs reports whether edge i, with a label run's length, has its ends
// further apart across the flow than a line of text, or is in line
// (inLine).
func (l *Layouter) climbs(i int) bool {
	return l.runW[i] > 0 && (l.inLine(i) || math.Abs(l.y[l.eFrom[i]]-l.y[l.eTo[i]]) > labelLine)
}

// inLine reports whether edge i, with a label run's length, has its ends
// within a line of text of each other across a top-to-bottom flow in a
// container. Its label, drawn level beside a line straight across the
// gap, would be as wide as its text, which the container, sized to what
// it holds, has no room for. So it is given a label run, along which
// its label is turned, as a climbing edge's is; and since the ends'
// spreads put them a little apart, the far end has a pair of bends
// level with it too, so that the route arrives level there, the step
// between the two made in the climb.
func (l *Layouter) inLine(i int) bool {
	return l.runW[i] > 0 && l.down && l.nested && math.Abs(l.y[l.eFrom[i]]-l.y[l.eTo[i]]) <= labelLine
}

// climbRoom gives the gap each climbing edge crosses (climbs) the room
// for its label run, a grid step off its end and runClimb before the far
// one.
func (l *Layouter) climbRoom(g float64) {
	for i := range l.edges {
		if l.climbs(i) {
			k := max(l.layer[l.eFrom[i]], l.layer[l.eTo[i]])
			l.lg[k] = math.Max(l.lg[k], l.runW[i]+runClimb+g)
		}
	}
}

// runPairs is endPairs, but with the label run of edge i (labelRun) in
// place of the pair at its end, recorded in lRun.
func (l *Layouter) runPairs(d *gw.Document, i int, g float64, route gw.Route) (sIn, sOut gw.Point, sOK bool, tIn, tOut gw.Point, tOK bool) {
	sIn, sOut, sOK, tIn, tOut, tOK = l.endPairs(i, g, route)
	end, in, out, ok := l.labelRun(d, i, g)
	l.lRun[i] = -1
	switch {
	case !ok:
	case end == 0:
		sIn, sOut, sOK, l.lRun[i] = in, out, true, 0
	default:
		tIn, tOut, tOK, l.lRun[i] = in, out, true, 1
	}
	if ok && l.inLine(i) {
		in, out := l.farPair(i, 1-end, g)
		if end == 0 {
			tIn, tOut, tOK = in, out, true
		} else {
			sIn, sOut, sOK = in, out, true
		}
		l.lRun[i] += 2
	}
	return sIn, sOut, sOK, tIn, tOut, tOK
}

// farPair is the pair of bends at the far end of an edge in line
// (inLine) from its label run, at its source for end 0 or its target for
// end 1: the inner a grid step out of that end's node and the outer two,
// both at the end's height, short of the run's outer bend, which is
// runClimb from the band.
func (l *Layouter) farPair(i, end int, g float64) (inner, outer gw.Point) {
	node, toward := l.eFrom[i], l.eTo[i]
	if end == 1 {
		node, toward = toward, node
	}
	dir := 1.0
	if l.layer[toward] < l.layer[node] {
		dir = -1
	}
	exit := l.x[node] + float64(dir*float64(l.w[node]/2))
	x, y := l.swap(exit+float64(dir*g), l.y[node])
	inner = gw.Point{X: x, Y: y}
	x, y = l.swap(exit+float64(2*dir*g), l.y[node])
	return inner, gw.Point{X: x, Y: y}
}

// labelRun is the pair of bends of climbing edge i's label run: at its
// source for end 0 or its target for end 1, the inner a grid step out of
// that end's node and the outer runClimb short of the far layer's band,
// both at the end's height; the room climbRoom gave the gap holds it.
// ok is false for an edge that does not climb, and one with a placed
// end, which stands where it was put rather than in its layer's band.
func (l *Layouter) labelRun(d *gw.Document, i int, g float64) (end int, inner, outer gw.Point, ok bool) {
	if !l.climbs(i) || l.pin[l.eFrom[i]] || l.pin[l.eTo[i]] {
		return 0, inner, outer, false
	}
	vf, sf := l.endSideOf(d, i, 0) // both spread, as runWidths gave it a run
	vt, st := l.endSideOf(d, i, 1)
	node, toward := l.eFrom[i], l.eTo[i]
	if l.sideN[2*int(vt)+(st+1)/2] < l.sideN[2*int(vf)+(sf+1)/2] {
		node, toward, end = toward, node, 1
	}
	dir := 1.0
	if l.layer[toward] < l.layer[node] {
		dir = -1
	}
	exit := l.x[node] + float64(dir*float64(l.w[node]/2))
	kt := l.layer[toward]
	back := float64(l.lw[kt]/2) + runClimb
	// Out to the grid toward the far layer, which keeps the room
	// climbRoom gave the run.
	far := float64(math.Ceil((l.lx[kt]-float64(dir*back))/g) * g)
	if dir < 0 {
		far = float64(math.Floor((l.lx[kt]-float64(dir*back))/g) * g)
	}
	x, y := l.swap(exit+float64(dir*g), l.y[node])
	inner = gw.Point{X: x, Y: y}
	x, y = l.swap(far, l.y[node])
	return end, inner, gw.Point{X: x, Y: y}, true
}
