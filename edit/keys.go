package edit

import (
	"unicode/utf8"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/graphedwright/svg"
)

// key handles a key press: Delete, the arrows, undo and redo, Enter or
// F2 and a printable character to edit a label, Escape and Tab, and
// with Ctrl, = or + to zoom in, - to zoom out and 0 to fit the drawing.
// The zoom keys act in any state; the gesture machine decides the rest.
func (e *Editor) key(ev Event) error {
	if ev.Modifiers&Ctrl != 0 {
		switch ev.Key {
		case "=", "+":
			e.zoomBy(keyZoom)
			return nil
		case "-", "_":
			e.zoomBy(1 / keyZoom)
			return nil
		case "0":
			e.fit()
			return nil
		}
	}
	_, _, one := e.one()
	return e.fire(gesture.Event{Kind: keyKind(ev), One: one}, input{ev: ev})
}

// keyKind is the gesture machine's name for a key press.
func keyKind(ev Event) gesture.EventKind {
	ctrl := ev.Modifiers&Ctrl != 0
	switch ev.Key {
	case "Escape":
		return gesture.EvEscape
	case "Tab":
		return gesture.EvTab
	case "Delete", "Backspace":
		return gesture.EvDelete
	case "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown":
		return gesture.EvNudge
	case "z", "Z":
		if ctrl && ev.Modifiers&Shift != 0 {
			return gesture.EvRedo
		}
		if ctrl {
			return gesture.EvUndo
		}
	case "y", "Y":
		if ctrl {
			return gesture.EvRedo
		}
	case "Enter", "F2":
		return gesture.EvStartText
	}
	if printable(ev.Key) && !ctrl {
		return gesture.EvType
	}
	return gesture.EvOtherKey
}

// printable reports whether a key names one character that types
// something, as against a key such as "Shift" or "ArrowLeft": one rune,
// not a control character.
func printable(key string) bool {
	r, n := utf8.DecodeRuneInString(key)
	return n == len(key) && n > 0 && r != utf8.RuneError && r >= ' ' && (r < 0x7f || r >= 0xa0)
}

// doubleClick edits the label of the vertex or edge at the point,
// selecting it alone.
func (e *Editor) doubleClick(ev Event) error {
	p := e.toDoc(ev.Point)
	hit := svg.HitTest(e.d, p)
	return e.fire(gesture.Event{Kind: gesture.EvDoubleClick, Hit: hitOf(hit)}, input{p: p, hit: hit, ev: ev})
}

// deleteSelection removes the selected edges, then the vertices with
// their edges, as one step to undo.
func (e *Editor) deleteSelection() error {
	e.d.Group()
	defer e.d.EndGroup()
	for _, x := range e.selE {
		if e.d.Edges[x].Live {
			if err := e.apply(gw.Command{Op: gw.DeleteEdge, Edge: x}); err != nil {
				return err
			}
		}
	}
	for _, v := range e.selV {
		if e.d.Vertices[v].Live {
			if err := e.apply(gw.Command{Op: gw.DeleteVertex, Vertex: v}); err != nil {
				return err
			}
		}
	}
	e.clearSelection()
	return nil
}

// nudgeKey moves the selected vertices by one grid unit in the arrow's
// direction.
func (e *Editor) nudgeKey(key string) error {
	switch key {
	case "ArrowLeft":
		return e.nudge(-e.grid, 0)
	case "ArrowRight":
		return e.nudge(e.grid, 0)
	case "ArrowUp":
		return e.nudge(0, -e.grid)
	case "ArrowDown":
		return e.nudge(0, e.grid)
	}
	return nil
}

// nudge moves the selected vertices by dx and dy, as one step to undo.
func (e *Editor) nudge(dx, dy float64) error {
	e.d.Group()
	defer e.d.EndGroup()
	for _, v := range e.selV {
		pos := e.d.Vertices[v].Pos
		if err := e.apply(gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: pos.X + dx, Y: pos.Y + dy}}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Editor) undo() {
	if ch, ok := e.d.Undo(); ok {
		e.record(ch)
		e.dropDead()
	}
}

func (e *Editor) redo() {
	if ch, ok := e.d.Redo(); ok {
		e.record(ch)
		e.dropDead()
	}
}

// tab moves the selection to the next live vertex in index order,
// wrapping; from nothing, to the first.
func (e *Editor) tab() {
	start := -1
	if len(e.selV) > 0 {
		start = int(e.selV[len(e.selV)-1])
	}
	n := len(e.d.Vertices)
	// bound: the vertex slots
	for k := 1; k <= n; k++ {
		i := (start + k) % n
		if e.d.Vertices[i].Live {
			e.selectVertex(gw.VertexIndex(i), false) //nolint:gosec // bounded by the capacity
			return
		}
	}
}

// startText begins editing the label of the one selected element, with
// the key typed to begin it when the event was a printable key.
func (e *Editor) startText(k gesture.EventKind, key string) {
	v, x, ok := e.one()
	if !ok {
		return
	}
	typed := ""
	if k == gesture.EvType {
		typed = key
	}
	e.editV, e.editE, e.typed = v, x, typed
}

// relabel relabels the element being edited with the host's final text.
func (e *Editor) relabel(text string) error {
	return e.apply(gw.Command{Op: gw.Relabel, Vertex: e.editV, Edge: e.editE, Text: text})
}

// paletteStart begins placing a vertex kind from the palette, or for an
// edge kind chooses it for the edges drawn from now on.
func (e *Editor) paletteStart(ev Event) error {
	k := gesture.EvPaletteVertex
	if ev.IsEdge {
		k = gesture.EvPaletteEdge
	}
	return e.fire(gesture.Event{Kind: k}, input{ev: ev})
}

// chooseEdge chooses an edge kind for new edges, or unchooses it when it
// was chosen, as a tool palette does.
func (e *Editor) chooseEdge(item gw.KindIndex) {
	k := gw.KindIndex(gw.None)
	if item != e.edgeKind && item >= 0 && int(item) < len(e.d.Schema.EdgeKinds) {
		k = item
	}
	e.edgeKind = k
}
