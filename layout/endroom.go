package layout

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// The ends of the curved and orthogonal edges at one side of a vertex
// are spread along it, in the order of where their routes head
// (gw.Document.EndSlot). A bend within that spread is taken to be one
// the layout brings level with its end, and is passed by; so a long
// edge whose dummy next to the vertex stood within it, and so was not
// brought level, would leave its end in an order its route does not
// keep, and cross the others there. Such a dummy is moved out of the
// spread, the way it is off the middle, pushing those beyond it in its
// layer on; one at the middle, where the route runs straight on from the
// end, stays.

// endSpreads finds how far off the middle of its vertex's side the
// ends there spread across the flow, for each end of the level's edges
// that is drawn to a side among others (gw.Document.EndOffset): the
// level's other ends at that side, a step of EndGap or a share of the
// side apart.
func (l *Layouter) endSpreads(d *gw.Document) {
	clear(l.sideN)
	for i := range l.edges {
		for end := range 2 {
			if v, side := l.endSideOf(d, i, end); side != 0 {
				l.sideN[2*int(v)+(side+1)/2]++
			}
		}
	}
	for i := range l.edges {
		for end := range 2 {
			l.spread[2*i+end] = 0
			v, side := l.endSideOf(d, i, end)
			if side == 0 {
				continue
			}
			n := int(l.sideN[2*int(v)+(side+1)/2])
			if n > gw.MaxEndSlots {
				continue
			}
			size := d.Vertex(gw.VertexIndex(v)).Size()
			across := size.H
			if l.down {
				across = size.W
			}
			step := math.Min(gw.EndGap, across/float64(n+1))
			l.spread[2*i+end] = float64(float64(n-1)/2) * step
		}
	}
}

// endSideOf is the vertex edge i's end, at its target for end 1 and its
// source for 0, is drawn to, and the side of it: 1 facing on along the
// flow, -1 back, or 0 where the end is not spread: an edge not of the
// level, or run in from a container holding the vertex, a loop, a
// straight edge or one with a port there. An edge leaves its source's on
// side and arrives at its target's back side, the other way round when
// it runs to an earlier layer and has no port.
func (l *Layouter) endSideOf(d *gw.Document, i, end int) (int32, int) {
	if !l.eOn[i] {
		return 0, 0
	}
	e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	v, port, side := e.From(), e.FromPort(), 1
	if end == 1 {
		v, port, side = e.To(), e.ToPort(), -1
	}
	at := l.eFrom[i]
	if end == 1 {
		at = l.eTo[i]
	}
	if at != int32(v) || port != gw.None || e.From() == e.To() || d.Kinds().Edge(e.Kind()).Route() == gw.Straight { //nolint:gosec // bounded by the capacity
		return 0, 0
	}
	if e.FromPort() == gw.None && e.ToPort() == gw.None && l.layer[l.eTo[i]] < l.layer[l.eFrom[i]] {
		side = -side
	}
	return at, side
}

// endDummy is edge i's dummy next to its end, at its target for end 1
// and its source for 0: the first of its dummies from the source's
// layer.
func (l *Layouter) endDummy(i, end int) int32 {
	first, last := l.dFirst[i], l.dFirst[i]+l.dN[i]-1
	if l.layer[l.eFrom[i]] > l.layer[l.eTo[i]] {
		first, last = last, first
	}
	if end == 1 {
		return last
	}
	return first
}

// clearsSpreads is whether edge i's dummies, in one row at y across the
// flow, leave the dummy next to each end out of the spread there, or at
// its middle, once snapped to grid g.
func (l *Layouter) clearsSpreads(i int, y, g float64) bool {
	for end := range 2 {
		if l.spreadOff(i, end, y, g) != 0 {
			return false
		}
	}
	return true
}

// spreadOff is the way across the flow, -1 or 1, that edge i's dummy
// next to its end, at y and snapped to grid g, stands off the middle of
// the spread there, when it is within it; and 0 when it is not, or is at
// the middle.
func (l *Layouter) spreadOff(i, end int, y, g float64) int {
	s := l.spread[2*i+end]
	if s == 0 {
		return 0
	}
	v := l.eFrom[i]
	if end == 1 {
		v = l.eTo[i]
	}
	off := l.snapped(y, g) - l.snappedAt(v, g)
	if math.Abs(off) <= straightTol || math.Abs(off) > s+straightTol {
		return 0
	}
	if off < 0 {
		return -1
	}
	return 1
}

// snapped is y on grid g; snappedAt is node v's y as it will be drawn: a
// placed vertex's where it is, the others' on the grid.
func (l *Layouter) snapped(y, g float64) float64 { return float64(math.Round(y/g) * g) }

func (l *Layouter) snappedAt(v int32, g float64) float64 {
	if l.isFixed(v) {
		return l.y[v]
	}
	return l.snapped(l.y[v], g)
}

// clearSpreads moves each long edge's dummy next to an end that stands
// within the spread there, off its middle, the grid steps out of it
// that way that take it clear, and pushes the nodes beyond it in its
// layer on by the gap, up to a placed one. It is whether any moved.
func (l *Layouter) clearSpreads(gap, g float64) bool {
	moved := false
	for i := range l.edges {
		if l.dN[i] == 0 {
			continue
		}
		for end := range 2 {
			u := l.endDummy(i, end)
			way := l.spreadOff(i, end, l.y[u], g)
			if way == 0 {
				continue
			}
			v := l.eFrom[i]
			if end == 1 {
				v = l.eTo[i]
			}
			out := float64(math.Floor(l.spread[2*i+end]/g)+1) * g
			l.y[u] = l.snappedAt(v, g) + float64(float64(way)*out)
			l.pushOn(u, way, gap)
			moved = true
		}
	}
	return moved
}

// pushOn pushes the nodes after node u in its layer, the way across the
// flow given, 1 for those after it and -1 for those before, the gap
// clear of the one before them, up to a placed one.
func (l *Layouter) pushOn(u int32, way int, gap float64) {
	k := l.layer[u]
	p := l.keys[k] + l.pos[u]
	// bound: the nodes of one layer
	for q := p + int32(way); q >= l.keys[k] && q < l.keys[k+1]; q += int32(way) { //nolint:gosec // -1 or 1
		a, b := l.order[q-int32(way)], l.order[q] //nolint:gosec // -1 or 1
		room := float64(l.h[a]/2) + gap + float64(l.h[b]/2)
		if float64(way)*(l.y[b]-l.y[a]) >= room-straightTol || l.isFixed(b) {
			return
		}
		l.y[b] = l.y[a] + float64(float64(way)*room)
	}
}
