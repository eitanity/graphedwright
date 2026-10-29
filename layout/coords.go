package layout

import (
	"math"
)

// coordinates gives every node an x from its layer and a y from its
// place in the order: a first stacking, then two passes centring each
// node on the median of its neighbours in the layer before (down) and
// after (up), overlaps pushed apart by the gap, placed vertices never
// moved; then everything snapped to the grid. Layers are the pitch
// apart, or further where their widest nodes, or the labels between
// them, need the room, out to the grid so that snapping keeps it.
func (l *Layouter) coordinates(opts Options) {
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
		step := math.Max(opts.Pitch, float64((l.lw[k-1]+l.lw[k])/2)+math.Max(columnGap, l.lg[k]))
		l.lx[k] = l.lx[k-1] + float64(math.Ceil(step/opts.Grid)*opts.Grid)
	}
	for i := range l.n {
		l.x[i] = l.lx[l.layer[i]]
		if i < l.realN && l.pin[i] {
			l.x[i], l.y[i] = l.fx[i], l.fy[i]
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
	for i := range l.realN {
		if !l.pin[i] {
			l.x[i] = math.Round(l.x[i]/opts.Grid) * opts.Grid
			l.y[i] = math.Round(l.y[i]/opts.Grid) * opts.Grid
		}
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
