package layout

import (
	gw "code.eitanity.com/graphedwright"
)

// topoOrder orders the live vertices so that every forward edge goes
// later, Kahn's way over the in-degrees. An edge that closes a cycle is
// taken as reversed for the layout and listed in rev: when the queue
// runs dry with vertices left, the lowest-indexed one left is released
// and its unprocessed in-edges are the reversed ones.
func (l *Layouter) topoOrder(d *gw.Document) {
	n := len(d.Vertices)
	for i := range n {
		l.indeg[i] = 0
		l.mark[i] = 0
	}
	for i := range d.Edges {
		if l.eOn[i] {
			l.indeg[l.eTo[i]]++
		}
	}
	top, done := 0, 0
	for i := range n {
		if l.liveReal[i] && l.indeg[i] == 0 {
			l.stack[top] = int32(i) //nolint:gosec // bounded by the capacity
			top++
		}
	}
	next := 0
	// bound: each vertex of the level enters the order once
	for done < l.members {
		if top == 0 {
			// A cycle: release the lowest vertex still waiting.
			// bound: n
			for next < n && (!l.liveReal[next] || l.mark[next] != 0) {
				next++
			}
			l.stack[top] = int32(next) //nolint:gosec // bounded by the capacity
			top++
			l.indeg[next] = 0
		}
		top--
		v := l.stack[top]
		if l.mark[v] != 0 {
			continue
		}
		l.mark[v] = 1
		l.topo[done] = v
		done++
		top = l.release(d, v, top)
	}
}

// release takes v's out-edges: one into a vertex already ordered is
// reversed, the others lower their target's in-degree, and a target
// at zero joins the stack. It returns the new stack top.
func (l *Layouter) release(d *gw.Document, v int32, top int) int {
	for i := range d.Edges {
		if !l.eOn[i] || l.eFrom[i] != v {
			continue
		}
		to := l.eTo[i]
		if l.mark[to] != 0 {
			l.rev = append(l.rev, gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
			continue
		}
		l.indeg[to]--
		if l.indeg[to] == 0 {
			l.stack[top] = to
			top++
		}
	}
	return top
}

// reversed reports whether the edge was taken as reversed.
func (l *Layouter) reversed(e gw.EdgeIndex) bool {
	for _, r := range l.rev {
		if r == e {
			return true
		}
	}
	return false
}

// assignLayers gives every vertex of the level a layer: a placed one
// the band its x falls in, the others one past their furthest
// predecessor, roots at zero, in topological order.
func (l *Layouter) assignLayers(opts Options) {
	l.fixedLayers(opts)
	l.layers = 0
	for _, v := range l.topo[:l.members] {
		if !l.pin[v] {
			l.layer[v] = l.longest(v)
		}
		if int(l.layer[v])+1 > l.layers {
			l.layers = int(l.layer[v]) + 1
		}
	}
}

// fixedLayers sets x0 to the leftmost placed vertex and gives every
// placed vertex the layer of the band its x falls in; the others start
// at zero.
func (l *Layouter) fixedLayers(opts Options) {
	l.x0 = 0
	first := true
	for i := range l.realN {
		if l.pin[i] && (first || l.fx[i] < l.x0) {
			l.x0, first = l.fx[i], false
		}
	}
	for i := range l.realN {
		l.layer[i] = 0
		if l.pin[i] {
			band := (l.fx[i] - l.x0) / opts.Pitch
			if band > 1<<20 {
				band = 1 << 20
			}
			l.layer[i] = int32(band + 0.5)
		}
	}
}

// longest is one past the furthest layer among v's predecessors over
// the edges not reversed, or zero.
func (l *Layouter) longest(v int32) int32 {
	layer := int32(0)
	for i := range l.eOn[:l.edges] {
		if l.eOn[i] && l.eTo[i] == v && !l.reversed(gw.EdgeIndex(i)) && l.layer[l.eFrom[i]]+1 > layer { //nolint:gosec // bounded by the capacity
			layer = l.layer[l.eFrom[i]] + 1
		}
	}
	return layer
}

// makeSegments turns every live edge into segments between adjacent
// layers, with a dummy node for each layer crossed, so that it is routed
// around the nodes there. A backward edge, one that closes a cycle, runs
// from its target's layer up to its source's like a forward one, so its
// dummies are kept clear of the nodes it passes too; a flat edge gets no
// segment. The layer and dummy counts are checked against the budget
// first.
func (l *Layouter) makeSegments(d *gw.Document) error {
	// The arrays by layer hold maxN+1 entries; a placed vertex's layer
	// follows its x, so vertices placed far apart can need more.
	if l.layers > l.maxN {
		return layersError(l.layers, l.maxN)
	}
	n := len(d.Vertices)
	// An int64: a placed vertex's layer reaches 2^20, so the edges'
	// spans can sum past 2^31.
	need := int64(0)
	for i := range d.Edges {
		span := int64(l.layer[l.eTo[i]]) - int64(l.layer[l.eFrom[i]])
		if span < 0 {
			span = -span
		}
		if l.eOn[i] && span > 1 {
			need += span - 1
		}
	}
	if budget := l.maxN - l.caps.Vertices; need > int64(budget) {
		return budgetError(need, budget)
	}
	l.n, l.s = n, 0
	for i := range d.Edges {
		l.dN[i] = 0
		if !l.eOn[i] {
			continue
		}
		from, to := l.eFrom[i], l.eTo[i]
		span := int(l.layer[to] - l.layer[from])
		if span < 0 {
			from, to, span = to, from, -span
		}
		if span < 1 {
			continue
		}
		l.dFirst[i], l.dN[i] = int32(l.n), int32(span-1) //nolint:gosec // below the budget
		first := l.layer[from]
		// bound: the layers
		for k := 1; k < span; k++ {
			l.layer[l.n] = first + int32(k) //nolint:gosec // below the budget
			l.h[l.n], l.w[l.n] = 0, 0
			l.segF[l.s], l.segT[l.s], l.segE[l.s] = from, int32(l.n), int32(i) //nolint:gosec // bounded by the capacities
			from = int32(l.n)                                                  //nolint:gosec // below the budget
			l.n++
			l.s++
		}
		l.segF[l.s], l.segT[l.s], l.segE[l.s] = from, to, int32(i) //nolint:gosec // bounded by the capacities
		l.s++
	}
	return nil
}
