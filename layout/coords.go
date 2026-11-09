package layout

import (
	"math"
)

// coordinates gives every node a y from its place in the order: a
// first stacking, then two passes centring each node on the median of
// its neighbours in the layer before (down) and after (up), overlaps
// pushed apart by the gap, placed vertices never moved; then each long
// edge's dummies brought into one row where they fit; then an x from its
// layer (placeLayers), which needs the ys for the room the label runs
// take (climbRoom); then everything snapped to the grid.
func (l *Layouter) coordinates(opts Options) {
	for i := range l.realN {
		if l.pin[i] {
			l.y[i] = l.fy[i]
		}
	}
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		y := 0.0
		// bound: the nodes of one layer
		for p := l.keys[k]; p < l.keys[k+1]; p++ {
			v := l.order[p]
			if !l.isFixed(v) {
				l.y[v] = y + float64(l.h[v]/2)
			}
			y += l.h[v] + opts.Gap
		}
	}
	l.buildAdj(false)
	// bound: the layers
	for k := 1; k < l.layers; k++ {
		l.centreLayer(k, opts)
	}
	l.buildAdj(true)
	// bound: the layers
	for k := l.layers - 2; k >= 0; k-- {
		l.centreLayer(k, opts)
	}
	l.columns(opts.Gap, opts.Grid)
	l.climbRoom(opts.Grid)
	l.placeLayers(opts)
	for i := range l.realN {
		if !l.pin[i] {
			l.x[i] = math.Round(l.x[i]/opts.Grid) * opts.Grid
			l.y[i] = math.Round(l.y[i]/opts.Grid) * opts.Grid
		}
	}
}

// placeLayers gives every node the x of its layer, a placed vertex its
// own. Layers are the pitch apart, or further where their widest nodes,
// or the labels between them and the runs beside those in a container
// (gapRoom), need the room, out to the grid so that snapping keeps it.
func (l *Layouter) placeLayers(opts Options) {
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		l.lw[k] = 0
	}
	for i := range l.n {
		if l.live(i) {
			l.lw[l.layer[i]] = math.Max(l.lw[l.layer[i]], l.w[i])
		}
	}
	l.lx[0] = l.x0
	// bound: the layers
	for k := 1; k < l.layers; k++ {
		step := math.Max(opts.Pitch, float64((l.lw[k-1]+l.lw[k])/2)+math.Max(columnGap, l.lg[k])) + l.rg[k]
		l.lx[k] = l.lx[k-1] + float64(math.Ceil(step/opts.Grid)*opts.Grid)
	}
	for i := range l.n {
		l.x[i] = l.lx[l.layer[i]]
		if i < l.realN && l.pin[i] {
			l.x[i] = l.fx[i]
		}
	}
}

// columns brings each long edge's dummies into one row where they fit,
// out of the spread of the ends next to them, and again once the dummies
// still within one are moved out (clearSpreads), so that the others
// follow.
func (l *Layouter) columns(gap, g float64) {
	l.straighten(gap, g)
	if l.clearSpreads(gap, g) {
		l.straighten(gap, g)
	}
}

func (l *Layouter) isFixed(v int32) bool { return int(v) < l.realN && l.pin[v] }

// centreLayer moves each unfixed node of layer k to the median y of its
// neighbours through the adjacency built, then pushes overlapping nodes
// apart in order, downwards past fixed ones.
func (l *Layouter) centreLayer(k int, opts Options) {
	lo, hi := l.keys[k], l.keys[k+1]
	// bound: the nodes of one layer
	for p := lo; p < hi; p++ {
		v := l.order[p]
		if l.isFixed(v) {
			continue
		}
		if m, ok := l.medianY(v); ok {
			l.y[v] = m
		}
	}
	// Overlaps: walk the order and push each unfixed node below the one
	// before it; a fixed node pushes the ones after it instead.
	floor := math.Inf(-1)
	// bound: the nodes of one layer
	for p := lo; p < hi; p++ {
		v := l.order[p]
		top := l.y[v] - float64(l.h[v]/2)
		if !l.isFixed(v) && top < floor {
			l.y[v] = floor + float64(l.h[v]/2)
		}
		floor = l.y[v] + float64(l.h[v]/2) + opts.Gap
	}
}

// medianY is the median y of a node's neighbours, gathered into the
// bary scratch and sorted by insertion, which is bounded by the degree.
func (l *Layouter) medianY(v int32) (float64, bool) {
	n := 0
	// bound: the segments at one node
	for a := l.off[v]; a < l.off[v+1]; a++ {
		y := l.y[l.adj[a]]
		j := n
		// bound: the neighbours gathered so far
		for j > 0 && l.bary[j-1] > y {
			l.bary[j] = l.bary[j-1]
			j--
		}
		l.bary[j] = y
		n++
	}
	if n == 0 {
		return 0, false
	}
	if n%2 == 1 {
		return l.bary[n/2], true
	}
	return (l.bary[n/2-1] + l.bary[n/2]) / 2, true
}

// straighten puts the dummies of each long edge in one row across the
// flow, where they fit between their neighbours in their layers, so that
// the edge runs on through the layers it passes instead of stepping
// between them. A dummy that does not fit may once another edge's have
// moved, so it passes over the edges again until one moves nothing.
func (l *Layouter) straighten(gap, g float64) {
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		// bound: the nodes of one layer
		for p := l.keys[k]; p < l.keys[k+1]; p++ {
			l.pos[l.order[p]] = p - l.keys[k]
		}
	}
	// bound: a pass for each edge, though one that moves nothing ends them
	for range l.edges {
		moved := false
		// bound: the document's edge slots
		for i := range l.edges {
			if l.dN[i] > 1 && l.straightenEdge(i, gap, g) {
				moved = true
			}
		}
		if !moved {
			return
		}
	}
}

// straightenEdge moves the n dummies from first to the row of the one
// that the most of them fit at, the first such from first, those of them
// that fit, and is whether any moved.
func (l *Layouter) straightenEdge(i int, gap, g float64) bool {
	first, n := l.dFirst[i], l.dN[i]
	at, most := 0.0, 0
	// bound: the edge's dummies
	for a := range n {
		if !l.clearsSpreads(i, l.y[first+a], g) {
			continue
		}
		if c := l.fitting(first, n, l.y[first+a], gap); c > most {
			at, most = l.y[first+a], c
		}
	}
	moved := false
	// bound: the edge's dummies
	for v := first; v < first+n; v++ {
		if l.y[v] != at && l.fitsAt(v, at, gap) {
			l.y[v], moved = at, true
		}
	}
	return moved
}

// fitting is how many of the n dummies from first fit at y.
func (l *Layouter) fitting(first, n int32, y, gap float64) int {
	c := 0
	// bound: the edge's dummies
	for v := first; v < first+n; v++ {
		if l.fitsAt(v, y, gap) {
			c++
		}
	}
	return c
}

// fitsAt is whether node v, moved to y, stays the gap clear of the nodes
// before and after it in its layer.
func (l *Layouter) fitsAt(v int32, y, gap float64) bool {
	k := l.layer[v]
	p := l.keys[k] + l.pos[v]
	half := float64(l.h[v] / 2)
	if p > l.keys[k] {
		u := l.order[p-1]
		if l.y[u]+float64(l.h[u]/2)+gap > y-half+straightTol {
			return false
		}
	}
	if p+1 < l.keys[k+1] {
		u := l.order[p+1]
		if l.y[u]-float64(l.h[u]/2)-gap < y+half-straightTol {
			return false
		}
	}
	return true
}

// straightTol is how far within the gap a node moved into a row is
// taken to fit: the rows are sums of the same sizes in another order.
const straightTol = 1e-6
