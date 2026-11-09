package layout

import gw "code.eitanity.com/graphedwright"

// An edge between members of a container, across the gap between two of
// its layers, has its label drawn at its middle, in that gap, which the
// label room (labelRoom) makes as wide as the label. A run from outside
// the container to a member comes along that gap too (inside.go), and
// would cross the label there. So a labelled gap inside a container is
// gapRunRoom wider on each side, and a run along it keeps to that room,
// beside the layer of the member it goes to, off the label in the middle.
// Those from the near side and the far side would then pass each other
// along the band inside the container's padding and lie on each other.
// So a run from the far side keeps its band gapRunRoom/2 further out.
// That needs the band to stay half the padding off the container's side,
// for the detours (detour.go) not to take it to run along the side, as
// above a container's title; along a band in narrower padding, the runs
// from both sides keep five eighths of the way from their members'
// layers across the gap instead, off each other (inside).

// gapRunRoom is the room either side of a labelled gap in a container
// for the runs along it, two of the runs' steps.
const gapRunRoom = 16

// gapRoom gives each gap of container c's level that an edge with a
// label crosses, from one layer to the next, gapRunRoom on each side;
// none at the top level, which no run comes along.
func (l *Layouter) gapRoom(d *gw.Document, c gw.VertexIndex) {
	// bound: the layers
	for k := 0; k <= l.layers; k++ {
		l.rg[k] = 0
	}
	if c == gw.None {
		return
	}
	for s := range l.s {
		e := l.segE[s]
		a, b := l.layer[l.segF[s]], l.layer[l.segT[s]]
		if l.dN[e] == 0 && (a-b == 1 || b-a == 1) && d.EdgeLabelRunes(gw.EdgeIndex(e)) > 0 { //nolint:gosec // bounded by the capacity
			l.rg[max(a, b)] = 2 * gapRunRoom
		}
	}
}

// markGapRoom records, for each member of the level, whether the gaps
// before and after its layer have room for runs (gapRoom), for the runs
// found once the whole document is laid out.
func (l *Layouter) markGapRoom() {
	for i := range l.realN {
		if l.liveReal[i] {
			k := l.layer[i]
			l.roomB[i], l.roomA[i] = l.rg[k] > 0, l.rg[k+1] > 0 // the gap after the last layer has none
		}
	}
}

// gapSide is the way w along the band above what the container holds,
// out -1, or below it, out 1, to member m along a gap with room for runs,
// beside m's layer, whose side facing the gap is at col, from the near
// side (low), or from the far side with the band gapRunRoom/2 further
// out; where that would leave the band less than half the padding inside
// the box cb of the container, or the gap has no room, w as it is.
func (l *Layouter) gapSide(m gw.VertexIndex, low bool, col float64, cb frameBox, w bandWay, out float64) bandWay {
	const step = gapRunRoom / 2
	side := cb.y0
	if out > 0 {
		side = cb.y1
	}
	if float64((w.by+float64(out*step)-side)*-out) < gw.ContainerPad/2 {
		return w
	}
	switch {
	case low && l.roomB[m]:
		return bandWay{by: w.by, gx: col - step, last: col - step/2}
	case !low && l.roomA[m]:
		return bandWay{by: w.by + float64(out*step), gx: col + step, last: col + step/2}
	}
	return w
}
