package edit

import (
	gw "code.eitanity.com/graphedwright"
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
	e.selV = append(e.selV, v)
}

// selectEdge makes x the selection, or adds it.
func (e *Editor) selectEdge(x gw.EdgeIndex, add bool) {
	for i, y := range e.selE {
		if y == x {
			if add {
				e.selE = append(e.selE[:i], e.selE[i+1:]...)
			}
			return
		}
	}
	if !add {
		e.clearSelection()
	}
	e.selE = append(e.selE, x)
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
			e.selV = append(e.selV[:i], e.selV[i+1:]...)
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
	for i := range e.d.Vertices {
		v := &e.d.Vertices[i]
		if v.Live && v.Pos.X >= box.X && v.Pos.X <= box.X+box.W && v.Pos.Y >= box.Y && v.Pos.Y <= box.Y+box.H {
			e.selectVertex(gw.VertexIndex(i), true) //nolint:gosec // bounded by the capacity
		}
	}
	for i := range e.d.Edges {
		x := &e.d.Edges[i]
		if x.Live && e.isSelectedV(x.From) && e.isSelectedV(x.To) {
			e.selectEdge(gw.EdgeIndex(i), true) //nolint:gosec // bounded by the capacity
		}
	}
}

// dropDead removes deleted elements from the selection.
func (e *Editor) dropDead() {
	n := 0
	for _, v := range e.selV {
		if e.d.Vertices[v].Live {
			e.selV[n] = v
			n++
		}
	}
	e.selV = e.selV[:n]
	m := 0
	for _, x := range e.selE {
		if e.d.Edges[x].Live {
			e.selE[m] = x
			m++
		}
	}
	e.selE = e.selE[:m]
}
