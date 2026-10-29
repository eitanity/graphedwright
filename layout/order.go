package layout

import (
	"slices"
)

// initialOrder lists the nodes layer by layer, in index order within a
// layer, and sets keys to the start of each layer in order. Dead
// vertices are left out.
func (l *Layouter) initialOrder() {
	// bound: the layers
	for k := 0; k <= l.layers; k++ {
		l.keys[k] = 0
	}
	for i := range l.n {
		if l.live(i) {
			l.keys[l.layer[i]+1]++
		}
	}
	// bound: the layers
	for k := 1; k <= l.layers; k++ {
		l.keys[k] += l.keys[k-1]
	}
	for i := range l.n {
		if l.live(i) {
			k := l.layer[i]
			l.order[l.keys[k]] = int32(i) //nolint:gosec // below the budget
			l.keys[k]++
		}
	}
	// bound: the layers
	for k := l.layers; k > 0; k-- {
		l.keys[k] = l.keys[k-1]
	}
	l.keys[0] = 0
	l.setPositions()
}

// live reports whether node i is a live vertex or a dummy.
func (l *Layouter) live(i int) bool { return i >= l.realN || l.liveReal[i] }

// setPositions records each node's position within its layer.
func (l *Layouter) setPositions() {
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		// bound: the nodes of one layer
		for p := l.keys[k]; p < l.keys[k+1]; p++ {
			l.pos[l.order[p]] = p - l.keys[k]
		}
	}
}

// buildAdj makes the CSR adjacency of the segments in one direction:
// down lists each node's successors, up its predecessors.
func (l *Layouter) buildAdj(down bool) {
	// bound: the layered graph's nodes
	for i := 0; i <= l.n; i++ {
		l.off[i] = 0
	}
	for s := range l.s {
		from := l.segF[s]
		if !down {
			from = l.segT[s]
		}
		l.off[from+1]++
	}
	// bound: the layered graph's nodes
	for i := 1; i <= l.n; i++ {
		l.off[i] += l.off[i-1]
	}
	for s := range l.s {
		from, to := l.segF[s], l.segT[s]
		if !down {
			from, to = to, from
		}
		l.adj[l.off[from]] = to
		l.off[from]++
	}
	// bound: the layered graph's nodes
	for i := l.n; i > 0; i-- {
		l.off[i] = l.off[i-1]
	}
	l.off[0] = 0
}

// reduceCrossings sweeps the layers down and up, reordering each by the
// barycentre of its neighbours in the layer before, and keeps the order
// with the fewest crossings seen. It returns that count.
func (l *Layouter) reduceCrossings(sweeps int) int64 {
	best := l.countCrossings()
	copy(l.best[:l.n], l.order[:l.n])
	// bound: the sweeps asked for
	for sweep := 0; sweep < sweeps && best > 0; sweep++ {
		if sweep%2 == 0 {
			l.buildAdj(false)
			// bound: the layers
			for k := 1; k < l.layers; k++ {
				l.orderLayer(k)
			}
		} else {
			l.buildAdj(true)
			// bound: the layers
			for k := l.layers - 2; k >= 0; k-- {
				l.orderLayer(k)
			}
		}
		if c := l.countCrossings(); c < best {
			best = c
			copy(l.best[:l.n], l.order[:l.n])
		}
	}
	copy(l.order[:l.n], l.best[:l.n])
	l.setPositions()
	return best
}

// orderLayer sorts layer k by the mean position of each node's
// neighbours through the adjacency built, a node without any keeping
// its place; ties go by node index.
func (l *Layouter) orderLayer(k int) {
	lo, hi := l.keys[k], l.keys[k+1]
	// bound: the nodes of one layer
	for p := lo; p < hi; p++ {
		v := l.order[p]
		sum, cnt := 0.0, 0
		// bound: the segments at one node
		for a := l.off[v]; a < l.off[v+1]; a++ {
			sum += float64(l.pos[l.adj[a]])
			cnt++
		}
		if cnt == 0 {
			l.bary[v] = float64(l.pos[v])
		} else {
			l.bary[v] = sum / float64(cnt)
		}
	}
	slices.SortStableFunc(l.order[lo:hi], func(a, b int32) int {
		switch {
		case l.bary[a] < l.bary[b]:
			return -1
		case l.bary[a] > l.bary[b]:
			return 1
		case a < b:
			return -1
		case a > b:
			return 1
		}
		return 0
	})
	// bound: the nodes of one layer
	for p := lo; p < hi; p++ {
		l.pos[l.order[p]] = p - lo
	}
}

// countCrossings counts the crossings between every pair of adjacent
// layers: the segments of a layer pair sorted by the upper position
// then the lower, and for each the number of earlier segments whose
// lower position is greater, through a Fenwick tree. The count is an
// int64: m segments between two layers can cross m(m-1)/2 times, past
// 2^31 for a graph the capacities allow.
func (l *Layouter) countCrossings() int64 {
	total := int64(0)
	l.buildAdj(false)
	// bound: the layers
	for k := 1; k < l.layers; k++ {
		// Gather the segments into layer k by walking its nodes in order,
		// which sorts by the lower position for free; the upper positions
		// of one node's predecessors are sorted in place.
		m := 0
		// bound: the nodes of one layer
		for p := l.keys[k]; p < l.keys[k+1]; p++ {
			v := l.order[p]
			start := m
			// bound: the segments at one node
			for a := l.off[v]; a < l.off[v+1]; a++ {
				l.byTo[m] = l.pos[l.adj[a]]
				m++
			}
			slices.Sort(l.byTo[start:m])
		}
		total += l.inversions(l.byTo[:m], int(l.keys[k]-l.keys[k-1]))
	}
	return total
}

// inversions counts pairs i < j with a[i] > a[j] among values below
// size, with a Fenwick tree over the values.
func (l *Layouter) inversions(a []int32, size int) int64 {
	// bound: the tree's size
	for i := 0; i <= size; i++ {
		l.fen[i] = 0
	}
	count := int64(0)
	for i, v := range a {
		// Earlier values greater than v: i minus those at most v.
		at := int(v) + 1
		seen := int32(0)
		// bound: the tree's depth, log2(size)+1
		for j := at; j > 0; j -= j & -j {
			seen += l.fen[j]
		}
		count += int64(i) - int64(seen)
		// bound: the tree's depth
		for j := at; j <= size; j += j & -j {
			l.fen[j]++
		}
	}
	return count
}
