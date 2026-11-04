package graphedwright

// View is the viewport: the part of the drawing shown. An empty one
// shows the whole drawing.
func (d *Document) View() Viewport { return d.view }

// SetView sets the viewport outside the command log: for a front end
// panning and zooming, and for a loader. It refuses, reporting false,
// while the document is locked, and a viewport whose corner is not
// within MaxCoord of the origin or whose width or height is negative or
// above MaxCoord.
func (d *Document) SetView(v Viewport) bool {
	if d.locked || !coord(v.X) || !coord(v.Y) || !span(v.W) || !span(v.H) {
		return false
	}
	d.view = v
	return true
}

// span reports whether x is a viewport's width or height: not negative
// and at most MaxCoord. It is false for NaN.
func span(x float64) bool { return x >= 0 && x <= MaxCoord }

// Flow is the way the layers run: set by a Layout command, or by
// SetFlow.
func (d *Document) Flow() Flow { return d.flow }

// SetFlow sets the way the layers run outside the command log: for a
// loader. It refuses, reporting false, while the document is locked,
// and a flow that is not one of the flows.
func (d *Document) SetFlow(f Flow) bool {
	if d.locked || f > TopToBottom {
		return false
	}
	d.flow = f
	return true
}
