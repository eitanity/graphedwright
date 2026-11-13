package layout

import (
	"math"
	"slices"

	gw "code.eitanity.com/graphedwright"
)

// A note stands beside the vertex it explains (gw.Document.NoteTarget),
// across the flow: in that vertex's layer, next to it in the order, and
// a gap from it. Its anchors are left out of the level's edges, so it
// takes no part in the layering or the crossings, and the order it is
// put in costs none. A note beside a vertex in another container stands
// beside the member of its own level that holds that vertex. One that
// was placed stays where it is, and one whose target is not in its
// container at all is laid out as a vertex without edges is.

// noteTargets finds, for each member of the level, the node it stands
// beside: the member that is or holds its target, for an unplaced note
// with one in container c; -1 for the others. It puts each note in that
// node's layer, and reports whether there is any.
func (l *Layouter) noteTargets(d *gw.Document, c gw.VertexIndex) bool {
	any := false
	for i := range d.VertexSlots() {
		l.noteAt[i] = -1
		if !l.liveReal[i] || l.pin[i] {
			continue
		}
		t := d.NoteTarget(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if t == gw.None {
			continue
		}
		if at := holder(d, t, c); at >= 0 {
			l.noteAt[i], l.layer[i], any = at, l.layer[at], true
		}
	}
	return any
}

// noteOrder moves each note next to its target in their layer: the
// first after it, the second before, and so on outward; or the first
// before a target that is first in its layer with others after it,
// where nothing stands. The rest keep their order.
func (l *Layouter) noteOrder() {
	if !l.notes {
		return
	}
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		lo, hi := l.keys[k], l.keys[k+1]
		w := lo
		// bound: the nodes of one layer
		for p := lo; p < hi; p++ {
			u := l.order[p]
			if l.isNote(u) {
				continue
			}
			before := l.alone(lo, p) && !l.alone(p+1, hi)
			w = l.notesOf(u, lo, hi, w, before, true)
			l.best[w] = u
			w++
			w = l.notesOf(u, lo, hi, w, before, false)
		}
		copy(l.order[lo:hi], l.best[lo:hi])
	}
	l.setPositions()
}

// isNote reports whether node u is a note laid out beside a target.
func (l *Layouter) isNote(u int32) bool { return int(u) < l.realN && l.noteAt[u] >= 0 }

// alone reports whether the order from lo to hi holds only notes.
func (l *Layouter) alone(lo, hi int32) bool {
	// bound: the nodes of one layer
	for q := lo; q < hi; q++ {
		if !l.isNote(l.order[q]) {
			return false
		}
	}
	return true
}

// notesOf writes u's notes in the layer from lo to hi into best from w,
// those before u with ahead and those after it without, and returns
// where the writing ends. The j-th, from 0 in the order, goes before u
// when j is even with before, or odd without; those before are written
// outermost, the last, first.
func (l *Layouter) notesOf(u, lo, hi, w int32, before, ahead bool) int32 {
	m, j := int32(0), 0
	// bound: the nodes of one layer
	for q := lo; q < hi; q++ {
		v := l.order[q]
		if !l.isNote(v) || l.noteAt[v] != u {
			continue
		}
		if (j%2 == 0) == before == ahead {
			l.best[w+m] = v
			m++
		}
		j++
	}
	if ahead {
		slices.Reverse(l.best[w : w+m])
	}
	return w + m
}

// toward is the node next to note v in its layer on its target's side:
// the target, or another of its notes nearer to it.
func (l *Layouter) toward(v int32) int32 {
	p := l.keys[l.layer[v]] + l.pos[v]
	if l.pos[v] < l.pos[l.noteAt[v]] {
		return l.order[p+1]
	}
	return l.order[p-1]
}

// besideOff is how far across the flow note v stands from the node
// toward its target, either way: its own room, that node's and the gap.
func (l *Layouter) besideOff(v int32, gap float64) float64 {
	u := l.toward(v)
	off := float64(l.h[u]/2) + gap + float64(l.h[v]/2)
	if l.pos[v] < l.pos[u] {
		return -off
	}
	return off
}

// besideNotes moves the notes of layer lo to hi beside their targets,
// nearest first: those after a target in order, then those before it
// against it.
func (l *Layouter) besideNotes(lo, hi int32, gap float64) {
	// bound: the nodes of one layer
	for p := lo; p < hi; p++ {
		if v := l.order[p]; l.isNote(v) && l.pos[v] > l.pos[l.noteAt[v]] {
			l.y[v] = l.y[l.toward(v)] + l.besideOff(v, gap)
		}
	}
	// bound: the nodes of one layer
	for p := hi - 1; p >= lo; p-- {
		if v := l.order[p]; l.isNote(v) && l.pos[v] < l.pos[l.noteAt[v]] {
			l.y[v] = l.y[l.toward(v)] + l.besideOff(v, gap)
		}
	}
}

// besideTargets brings each note back beside its target once all are on
// grid g, nearest first: the most grid steps from the node toward its
// target that keep the gap or less, and more than nothing, between them.
// Nothing else stands between a note and its target in their layer, and
// a note only comes nearer.
func (l *Layouter) besideTargets(gap, g float64) {
	// bound: the layers
	for k := 0; k < l.layers; k++ {
		lo, hi := l.keys[k], l.keys[k+1]
		// bound: the nodes of one layer
		for p := lo; p < hi; p++ {
			if v := l.order[p]; l.isNote(v) && l.pos[v] > l.pos[l.noteAt[v]] {
				l.y[v] = l.y[l.toward(v)] + l.snappedOff(v, gap, g)
			}
		}
		// bound: the nodes of one layer
		for p := hi - 1; p >= lo; p-- {
			if v := l.order[p]; l.isNote(v) && l.pos[v] < l.pos[l.noteAt[v]] {
				l.y[v] = l.y[l.toward(v)] + l.snappedOff(v, gap, g)
			}
		}
	}
}

// snappedOff is besideOff in whole steps of grid g: no more than it,
// unless that leaves less than half a step past the two nodes' room, the
// most a container's box, fitted round what it holds, falls off its
// centre on the grid.
func (l *Layouter) snappedOff(v int32, gap, g float64) float64 {
	off := l.besideOff(v, gap)
	room := math.Abs(off) - gap + float64(g/2)
	steps := float64(math.Floor(math.Abs(off)/g) * g)
	if steps < room {
		steps = float64(math.Ceil(room/g) * g)
	}
	return math.Copysign(steps, off)
}
