package dom

import (
	gw "code.eitanity.com/graphedwright"
)

// Reveal makes vertex v seen (edit.Editor.Reveal): it unfolds the folds
// it is in, as one step for Undo, selects it alone and centres it,
// drawn and reported to Changed as an edit is.
func (h *Handle) Reveal(v gw.VertexIndex) error {
	if err := h.refuse(); err != nil {
		return err
	}
	h.d.Group()
	ch, err := h.ed.Reveal(v)
	if back := h.after(ch, err); err == nil {
		err = back
	}
	return err
}

// Focus focuses on the selection's neighbourhood within depth edges,
// the way given (edit.Editor.Focus), and redraws; false, changing
// nothing, with nothing selected or a depth below 0.
func (h *Handle) Focus(depth int, way gw.Direction) (bool, error) {
	if err := h.refuse(); err != nil {
		return false, err
	}
	ok := h.ed.Focus(depth, way)
	if ok {
		h.renderAll()
	}
	return ok, nil
}

// Unfocus ends the focus and redraws.
func (h *Handle) Unfocus() error {
	if err := h.refuse(); err != nil {
		return err
	}
	h.ed.Unfocus()
	h.renderAll()
	return nil
}

// SetFocusHide leaves what is out of the focus undrawn, rather than
// dimmed, and redraws.
func (h *Handle) SetFocusHide(hide bool) error {
	if err := h.refuse(); err != nil {
		return err
	}
	h.ed.SetFocusHide(hide)
	h.renderAll()
	return nil
}
