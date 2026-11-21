package edit

import (
	"math"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/graphedwright/svg"
)

// selectVertex makes v the selection, or adds it when adding; a vertex
// already selected stays, so a drag of a group starts from any of them.
func (e *Editor) selectVertex(v gw.VertexIndex, add bool) {
	if e.isSelectedV(v) {
		if add {
			e.removeV(v)
		}
		return
	}
	if !add {
		e.clearSelection()
	}
	e.selV = push(e.selV, v)
}

// selectEdge makes x the selection, or adds it.
func (e *Editor) selectEdge(x gw.EdgeIndex, add bool) {
	for i, y := range e.selE {
		if y == x {
			if add {
				e.selE = e.selE[:i+copy(e.selE[i:], e.selE[i+1:])]
			}
			return
		}
	}
	if !add {
		e.clearSelection()
	}
	e.selE = push(e.selE, x)
	e.selectMerged(x)
}

// isSelectedE reports whether edge x is selected.
func (e *Editor) isSelectedE(x gw.EdgeIndex) bool {
	for _, y := range e.selE {
		if y == x {
			return true
		}
	}
	return false
}

func (e *Editor) isSelectedV(v gw.VertexIndex) bool {
	for _, x := range e.selV {
		if x == v {
			return true
		}
	}
	return false
}

func (e *Editor) removeV(v gw.VertexIndex) {
	for i, x := range e.selV {
		if x == v {
			e.selV = e.selV[:i+copy(e.selV[i:], e.selV[i+1:])]
			return
		}
	}
}

func (e *Editor) clearSelection() {
	e.selV, e.selE = e.selV[:0], e.selE[:0]
}

// selectIn selects the live vertices whose centre lies in the box and
// the edges with both ends selected.
func (e *Editor) selectIn(box gw.Viewport, add bool) {
	if !add {
		e.clearSelection()
	}
	for i := range e.d.VertexSlots() {
		v := e.d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if v.Live() && v.Pos().X >= box.X && v.Pos().X <= box.X+box.W && v.Pos().Y >= box.Y && v.Pos().Y <= box.Y+box.H {
			e.selectVertex(gw.VertexIndex(i), true) //nolint:gosec // bounded by the capacity
		}
	}
	for i := range e.d.EdgeSlots() {
		x := e.d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		if x.Live() && e.isSelectedV(x.From()) && e.isSelectedV(x.To()) {
			e.selectEdge(gw.EdgeIndex(i), true) //nolint:gosec // bounded by the capacity
		}
	}
}

// Settle drops from the selection what is deleted, or hidden in a fold,
// after a change the editor did not make: the host's own command, or
// its undo. The editor settles after its own.
func (e *Editor) Settle() { e.dropDead() }

// dropDead removes deleted elements from the selection, and those a
// fold hides.
func (e *Editor) dropDead() {
	n := 0
	for _, v := range e.selV {
		if e.d.Vertex(v).Live() && !e.d.Hidden(v) {
			e.selV[n] = v
			n++
		}
	}
	e.selV = e.selV[:n]
	m := 0
	for _, x := range e.selE {
		if e.d.Edge(x).Live() && !e.d.EdgeHidden(x) {
			e.selE[m] = x
			m++
		}
	}
	e.selE = e.selE[:m]
}

// context settles the selection for a context request and notes it: at
// the point, or from the keyboard on the selection.
func (e *Editor) context(ev Event) error {
	var p gw.Point
	var hit svg.Hit
	if ev.Keyboard {
		p, hit = e.onSelection()
	} else {
		p = e.toDoc(ev.Point)
		hit = e.hitAt(p)
	}
	return e.fire(gesture.Event{Kind: gesture.EvContext, Hit: hitOf(hit)}, input{p: p, hit: hit, ev: ev})
}

// onSelection is where a request from the keyboard is: the centre of
// the box around the selected vertices and the ends of the selected
// edges, on the first selected vertex, or else edge; with nothing
// selected, the view's centre, on the background.
func (e *Editor) onSelection() (gw.Point, svg.Hit) {
	hit := background()
	switch {
	case len(e.selV) > 0:
		hit.Kind, hit.Vertex = svg.VertexHit, e.selV[0]
	case len(e.selE) > 0:
		hit.Kind, hit.Edge = svg.EdgeHit, e.selE[0]
	default:
		v := e.d.View()
		return gw.Point{X: v.X + float64(v.W/2), Y: v.Y + float64(v.H/2)}, hit
	}
	lo := gw.Point{X: math.Inf(1), Y: math.Inf(1)}
	hi := gw.Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, v := range e.selV {
		x := e.d.Vertex(v)
		half := gw.Point{X: float64(x.Size().W / 2), Y: float64(x.Size().H / 2)}
		lo, hi = widen(lo, hi, gw.Point{X: x.Pos().X - half.X, Y: x.Pos().Y - half.Y})
		lo, hi = widen(lo, hi, gw.Point{X: x.Pos().X + half.X, Y: x.Pos().Y + half.Y})
	}
	for _, x := range e.selE {
		lo, hi = widen(lo, hi, e.d.Vertex(e.d.Edge(x).From()).Pos())
		lo, hi = widen(lo, hi, e.d.Vertex(e.d.Edge(x).To()).Pos())
	}
	return gw.Point{X: (lo.X + hi.X) / 2, Y: (lo.Y + hi.Y) / 2}, hit
}

// widen is the box from lo to hi grown to hold p.
func widen(lo, hi, p gw.Point) (gw.Point, gw.Point) {
	return gw.Point{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y)}, gw.Point{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y)}
}
