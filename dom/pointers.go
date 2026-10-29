package dom

import "code.eitanity.com/graphedwright/edit"

// pointers keeps the pointer that is down, from its press to its release
// or cancel. Only it drives the editor: a second finger's press, moves
// and release are dropped, so it cannot end the first one's gesture.
type pointers struct {
	down bool
	id   int
}

// track reports whether an event of kind from pointer id goes to the
// editor. A press while another pointer is down is a second finger and is
// dropped, with its moves and release. A press by the pointer already
// down means its release was lost, outside the window, and starts afresh.
// Moves with no pointer down are hovers.
func (p *pointers) track(kind edit.EventKind, id int) bool {
	other := p.down && id != p.id
	switch kind {
	case edit.PointerDown:
		if other {
			return false
		}
		p.down, p.id = true, id
	case edit.PointerUp:
		if !p.down || other {
			return false
		}
		p.down = false
	case edit.PointerMove:
		return !other
	case edit.Wheel, edit.Key, edit.PaletteStart, edit.Blur, edit.Resize, edit.TextDone, edit.TextCancel, edit.DoubleClick:
	}
	return true
}

// cancel reports whether pointer id, cancelled by the browser, was the
// one down, and lets it go.
func (p *pointers) cancel(id int) bool {
	if !p.down || id != p.id {
		return false
	}
	p.down = false
	return true
}
