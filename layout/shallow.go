package layout

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// An edge's dummies are centred on their neighbours, so the leg from an
// end to the dummy next to it, or an edge between two adjacent layers,
// can have far to go across the flow in the room between two layers.
// Drawn as a curve leaving and arriving level, such a leg is all but
// flat, and edges in lanes beside it, offset across the flow, lie on it.
// So a curved or orthogonal leg that goes more than shallowRatio times
// as far across as its gap is deep is drawn as a level run, a run
// straight across the flow at an upright of its own in the gap, and a
// level run on: two more bends. A leg into a container goes as far
// across as where it goes on to inside it (reachY). The uprights stand zStep apart along
// the gap, stacked from a dummy's side, away from the vertex whose
// other edges fan out there: from the earlier layer's band edge, or
// back from the later one's when the dummy is there. In each stack they
// are ordered so that runs nested one inside the other do not cross:
// by the y each leaves and then the y it reaches, those going to a
// lower y first, then those going higher, highest first. A gap too
// shallow for its uprights and an arrowhead after them is deepened, the
// layers after it moved on together. A straight edge is
// drawn as lines, which do not lie on each other, and keeps its legs.

// shallowRatio is how many times its gap's depth a leg must go across
// the flow to be given an upright.
const shallowRatio = 4

// zStep is the room along the flow between two uprights in a gap, and
// zArrow the least room left for the leg into an end.
const (
	zStep  = 8
	zArrow = 16
)

// gapEdges is the room between layer k-1's band and layer k's along
// the flow: between where a dummy's bends stand past the earlier band
// and before the later one, half a grid step out and out to the grid.
func (l *Layouter) gapEdges(k int32, g float64) (lo, hi float64) {
	lo = float64(math.Ceil((l.lx[k-1]+float64(l.lw[k-1]/2)+float64(g/2))/g) * g)
	hi = float64(math.Floor((l.lx[k]-float64(l.lw[k]/2)-float64(g/2))/g) * g)
	return lo, hi
}

// legOf is edge i's leg at its source (end 0) or its target (end 1), in
// the route's order: from the source to its first dummy, or from the
// last dummy to the target; for an edge without dummies, end 1 is the
// whole edge. A backward edge's dummies run from its target's layer.
// ok is false for end 0 of an edge without dummies.
func (l *Layouter) legOf(i, end int) (from, to int32, ok bool) {
	f, t := l.eFrom[i], l.eTo[i]
	if l.dN[i] == 0 {
		return f, t, end == 1
	}
	first, last := l.dFirst[i], l.dFirst[i]+l.dN[i]-1
	if l.layer[f] > l.layer[t] {
		first, last = last, first
	}
	if end == 0 {
		return f, first, true
	}
	return last, t, true
}

// shallowEnds finds the legs of the level's edges that are given an
// upright, deepens the gaps that need it, and records each upright's
// x in zAt, by 2*edge+end; NaN where there is none. A level with a
// placed vertex keeps its layers where they are, and its legs as they
// are.
func (l *Layouter) shallowEnds(d *gw.Document, opts Options) {
	g := opts.Grid
	for i := range 2 * l.edges {
		l.zAt[i] = math.NaN()
	}
	if l.anyPinned() {
		return
	}
	m := l.shallowLegs(d, g)
	l.sortLegs(m)
	// bound: the legs found, in runs of one gap
	for a := 0; a < m; {
		k := l.zGap[a]
		b, nhi := a, 0
		// bound: the legs found
		for b < m && l.zGap[b] == k {
			if l.zHi[b] {
				nhi++
			}
			b++
		}
		lo, hi := l.gapEdges(k, g)
		if need := float64(zStep*(b-a+1)) + zArrow; hi-lo < need {
			l.deepen(k, float64(math.Ceil((need-(hi-lo))/g)*g))
			lo, hi = l.gapEdges(k, g)
		}
		rlo, rhi := 0, 0
		// bound: the legs of the gap
		for s := a; s < b; s++ {
			if l.zHi[s] {
				l.zAt[l.zIdx[s]] = hi - float64(zStep*(nhi-rhi))
				rhi++
			} else {
				rlo++
				l.zAt[l.zIdx[s]] = lo + float64(zStep*rlo)
			}
		}
		a = b
	}
}

// shallowLegs gathers into zIdx, zGap, zKey, zKey2 and zHi the legs that go
// shallowRatio times their gap's depth across the flow, and returns how
// many there are.
func (l *Layouter) shallowLegs(d *gw.Document, g float64) int {
	m := 0
	for i := range l.edges {
		if !l.eOn[i] || d.Kinds().Edge(d.Edge(gw.EdgeIndex(i)).Kind()).Route() == gw.Straight { //nolint:gosec // bounded by the capacity
			continue
		}
		for end := range 2 {
			from, to, ok := l.legOf(i, end)
			early, late := from, to
			if l.layer[to] < l.layer[from] {
				early, late = to, from
			}
			k := l.layer[late]
			if !ok || k != l.layer[early]+1 {
				continue
			}
			lo, hi := l.gapEdges(k, g)
			y0, y1 := l.reachY(d, i, early, late, g), l.reachY(d, i, late, early, g)
			if math.Abs(y1-y0) <= float64(shallowRatio*(hi-lo)) {
				continue
			}
			key, key2 := y0, y1 // going lower, by the y it leaves and then the y it reaches
			if y1 > y0 {
				key, key2 = math.MaxFloat64/2-y0, -y1 // then going higher, highest first
			}
			l.zIdx[m], l.zGap[m], l.zKey[m], l.zKey2[m] = int32(2*i+end), k, key, key2 //nolint:gosec // bounded by the capacity
			l.zHi[m] = int(late) >= l.realN                                            // the dummy is in the later layer
			m++
		}
	}
	return m
}

// reachY is where edge i's leg from node from goes to at node v across
// the flow (legY), or, where v is a container holding the edge's end at
// this level, where the leg goes on to inside it: to the end, where it
// stands, or, where another member stands between the side the leg
// comes in at and the one on the way to the end, to the band past them
// all that the run inside takes round them (alongGap), the one nearer
// from.
func (l *Layouter) reachY(d *gw.Document, i int, v, from int32, g float64) float64 {
	y := l.legY(v, g)
	if int(v) >= l.realN {
		return y // a dummy
	}
	e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	end := e.From()
	if v == l.eTo[i] {
		end = e.To()
	}
	if end == gw.VertexIndex(v) {
		return y
	}
	at, m := y, end
	// bound: the parent chain is acyclic, so at most the vertex count
	for u, k := end, 0; u != gw.VertexIndex(v) && u != gw.None && k <= d.VertexSlots(); u, k = d.Vertex(u).Parent(), k+1 {
		_, across := l.swap(l.rx[u], l.ry[u]) // from its container's centre (box)
		at += across
		m = u
	}
	if band, ok := l.band(d, gw.VertexIndex(v), m, l.legY(from, g)-y, l.layer[from] < l.layer[v]); ok {
		return y + band
	}
	return at
}

// band is the band, across the flow from container c's centre, that a
// run in to its member m from far across, from the low side or the
// high, goes along round the other members (alongGap): none when no
// member stands between that side and m, where the run comes straight
// in.
func (l *Layouter) band(d *gw.Document, c, m gw.VertexIndex, far float64, low bool) (float64, bool) {
	ma, mc := l.swap(l.rx[m], l.ry[m])
	top, bottom, between := mc-float64(l.h[m]/2), mc+float64(l.h[m]/2), false
	toward := 1.0 // from m toward the side the run comes in at, along the flow
	if low {
		toward = -1
	}
	// m is among the members, but neither widens the span nor stands
	// between.
	for i := range d.VertexSlots() {
		u := gw.VertexIndex(i) //nolint:gosec // bounded by the capacity
		if o := d.Vertex(u); !o.Live() || o.Parent() != c {
			continue
		}
		a, across := l.swap(l.rx[u], l.ry[u])
		top, bottom = math.Min(top, across-float64(l.h[u]/2)), math.Max(bottom, across+float64(l.h[u]/2))
		between = between || float64(toward*(a-ma)) >= float64((l.w[u]+l.w[m])/2)
	}
	if !between {
		return 0, false
	}
	gap := float64(gw.ContainerPad) / 2
	if math.Abs(far-(bottom+gap)) < math.Abs(far-(top-gap)) {
		return bottom + gap, true
	}
	return top - gap, true
}

// legY is where a leg meets node v across the flow: a dummy's y on the
// grid, where its bends are, or a vertex's centre.
func (l *Layouter) legY(v int32, g float64) float64 {
	if int(v) >= l.realN {
		return math.Round(l.y[v]/g) * g
	}
	return l.y[v]
}

// sortLegs sorts the first m legs by gap and then key, by insertion:
// the legs that need an upright are few.
func (l *Layouter) sortLegs(m int) {
	// bound: the legs found
	for a := 1; a < m; a++ {
		idx, gap, key, key2, hi := l.zIdx[a], l.zGap[a], l.zKey[a], l.zKey2[a], l.zHi[a]
		b := a
		// bound: the legs before a
		for b > 0 && l.legAfter(b-1, gap, key, key2) {
			l.zIdx[b], l.zGap[b], l.zKey[b], l.zKey2[b], l.zHi[b] = l.zIdx[b-1], l.zGap[b-1], l.zKey[b-1], l.zKey2[b-1], l.zHi[b-1]
			b--
		}
		l.zIdx[b], l.zGap[b], l.zKey[b], l.zKey2[b], l.zHi[b] = idx, gap, key, key2, hi
	}
}

// legAfter reports whether leg b sorts after one of the gap and keys
// given.
func (l *Layouter) legAfter(b int, gap int32, key, key2 float64) bool {
	if l.zGap[b] != gap {
		return l.zGap[b] > gap
	}
	return l.zKey[b] > key || l.zKey[b] == key && l.zKey2[b] > key2
}

// deepen moves layer k and every layer after it, with their nodes, on
// along the flow by dx.
func (l *Layouter) deepen(k int32, dx float64) {
	// bound: the layers
	for j := int(k); j < l.layers; j++ {
		l.lx[j] += dx
	}
	for v := range l.n {
		if l.layer[v] >= k {
			l.x[v] += dx
		}
	}
}

// uprights is how many bends edge i's uprights take at its source and
// at its target: two for each it has.
func (l *Layouter) uprights(i int) (zs, zt int) {
	if !math.IsNaN(l.zAt[2*i]) {
		zs = 2
	}
	if !math.IsNaN(l.zAt[2*i+1]) {
		zt = 2
	}
	return zs, zt
}

// middle writes through w the bends of edge i between the pairs its
// ends may need (endPairs): the source's upright, the dummies' bends,
// and the target's upright, from src+ns, zs and zt bends each for the
// uprights; and records where the uprights start in zRt.
func (l *Layouter) middle(i int, g, ox, oy float64, w gw.BendWriter, src, ns, zs, zt int) {
	l.zRt[2*i], l.zRt[2*i+1] = -1, -1
	if zs > 0 {
		l.upright(i, 0, g, ox, oy, w, src+ns)
		l.zRt[2*i] = int32(ns) //nolint:gosec // bounded by the bends per edge
	}
	l.dummyBends(i, g, ox, oy, w, src+ns+zs)
	if zt > 0 {
		at := ns + zs + 2*int(l.dN[i])
		l.upright(i, 1, g, ox, oy, w, src+at)
		l.zRt[2*i+1] = int32(at) //nolint:gosec // bounded by the bends per edge
		if l.dN[i] == 0 {
			l.zRt[2*i] = int32(at) //nolint:gosec // the one upright is level with both ends
		}
	}
}

// upright writes through w from off the two bends of the upright of
// edge i's leg at end (legOf), in the route's order: level from where
// the leg starts across the flow to the upright, and across to where it
// ends.
func (l *Layouter) upright(i, end int, g, ox, oy float64, w gw.BendWriter, off int) {
	from, to, _ := l.legOf(i, end)
	u := l.zAt[2*i+end]
	x, y := l.swap(u, l.legY(from, g))
	w.Set(off, gw.Point{X: x - ox, Y: y - oy})
	x, y = l.swap(u, l.legY(to, g))
	w.Set(off+1, gw.Point{X: x - ox, Y: y - oy})
}

// uprightEnds brings the bends of edge e's uprights that are level with
// an end across the flow to where the route goes on from them, now that
// the vertices are placed and the runs inside the containers holding
// the ends are found: level with the nearest point of those runs, or
// with the end where it is drawn, off the middle of its side where
// other edges end there too (gw.Document.EndOffset), so that the level
// run between the upright and the end is a line of its own rather than
// on the others'. The route's src points before its bends are the
// source's runs, its n bends follow, and the target's runs take it to
// all. zRt holds where each upright starts among the bends: the
// source's first bend is level with the source, the target's second
// with the target, and an edge without dummies has one upright, both.
func (l *Layouter) uprightEnds(d *gw.Document, e gw.EdgeIndex, w gw.BendWriter, src, n, all int) {
	ed := d.Edge(e)
	for end := range 2 {
		at := l.zRt[2*int(e)+end]
		if at < 0 || int(at)+1 >= n {
			continue // none, or dropped for want of room
		}
		v, k, run, runs := ed.From(), src+int(at), src-1, src > 0
		if end == 1 {
			v, k, run, runs = ed.To(), src+int(at)+1, src+n, all > src+n
		}
		across := l.acrossOf(d.Vertex(v).Pos()) + d.EndOffset(e, end == 1)
		if runs {
			across = l.acrossOf(w.At(run))
		}
		p := w.At(k)
		if l.down {
			p.X = across
		} else {
			p.Y = across
		}
		w.Set(k, p)
		if !runs {
			l.setLevel(e, end, k, 1)
		}
	}
}

// acrossOf is a point's place across the flow.
func (l *Layouter) acrossOf(p gw.Point) float64 {
	if l.down {
		return p.X
	}
	return p.Y
}
