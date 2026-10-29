package dom

import (
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// mount builds the host element's content: the drawing, the palette
// in edit mode, the zoom buttons, the hidden label input, and the
// listeners.
func (h *handle) mount() {
	doc := js.Global().Get("document")
	h.el.Set("tabIndex", 0)
	h.el.Get("classList").Call("add", "graphedwright-host")
	h.el.Set("innerHTML", "")
	h.palette = doc.Call("createElement", "div")
	h.palette.Set("className", "graphedwright-palette")
	h.el.Call("appendChild", h.palette)
	drawing := doc.Call("createElement", "div")
	drawing.Set("className", "graphedwright-drawing")
	h.el.Call("appendChild", drawing)
	h.zoomBar(doc)
	h.input = doc.Call("createElement", "input")
	h.input.Set("className", "graphedwright-label")
	h.input.Get("style").Set("display", "none")
	h.el.Call("appendChild", h.input)
	h.buildPalette()
	h.renderAll()
	// Moves and releases are taken from the window, so that a drag goes
	// on outside the drawing and ends wherever the button comes up.
	h.listen(drawing, "pointerdown", h.pointer(edit.PointerDown))
	h.listen(js.Global(), "pointermove", h.pointer(edit.PointerMove))
	h.listen(js.Global(), "pointerup", h.pointer(edit.PointerUp))
	h.listen(js.Global(), "pointercancel", h.pointerCancel)
	h.listen(drawing, "wheel", h.wheel)
	h.listen(drawing, "dblclick", h.doubleClick)
	h.listen(h.el, "keydown", h.keydown)
	h.listen(h.input, "keydown", h.inputKey)
	h.listen(h.input, "blur", h.inputBlur)
}

// unmount removes the listeners, the content and the functions.
func (h *handle) unmount() {
	for _, f := range h.funcs {
		f.Release()
	}
	h.funcs = nil
	h.el.Set("innerHTML", "")
}

// listen adds an event listener whose function is released on unmount.
func (h *handle) listen(target js.Value, event string, fn func(js.Value, []js.Value) any) {
	f := js.FuncOf(fn)
	h.funcs = append(h.funcs, f)
	target.Call("addEventListener", event, f)
}

// zoomBar makes the buttons that zoom in, out and to the drawing, in
// either mode.
func (h *handle) zoomBar(doc js.Value) {
	bar := doc.Call("createElement", "div")
	bar.Set("className", "graphedwright-zoom")
	h.el.Call("appendChild", bar)
	for _, b := range []struct {
		text, title string
		f           float64 // the zoom factor, or 0 to fit
	}{{"+", "zoom in (Ctrl+=)", zoomFactor}, {"\u2212", "zoom out (Ctrl+-)", 1 / zoomFactor}, {"Fit", "fit the drawing (Ctrl+0)", 0}} {
		el := doc.Call("createElement", "button")
		el.Set("type", "button")
		el.Set("textContent", b.text)
		el.Set("title", b.title)
		el.Call("setAttribute", "aria-label", b.title)
		bar.Call("appendChild", el)
		f := b.f
		h.listen(el, "click", func(js.Value, []js.Value) any {
			if f == 0 {
				h.after(h.ed.Fit(), nil)
			} else {
				h.after(h.ed.Zoom(f), nil)
			}
			return nil
		})
	}
}

// zoomFactor is what one press of a zoom button zooms by.
const zoomFactor = 1.25

// buildPalette makes a button per vertex kind, which a press starts
// placing, and per edge kind, which a press chooses for the edges drawn
// from then on, or unchooses.
func (h *handle) buildPalette() {
	doc := js.Global().Get("document")
	h.palette.Set("innerHTML", "")
	h.edgeItems = h.edgeItems[:0]
	if h.ed.Mode() != edit.Edit {
		return
	}
	for i, k := range h.d.Schema.VertexKinds {
		h.paletteButton(doc, k.Name, "vertex", gw.KindIndex(i), false) //nolint:gosec // bounded by the schema
	}
	for i, k := range h.d.Schema.EdgeKinds {
		h.edgeItems = append(h.edgeItems, h.paletteButton(doc, k.Name, "edge", gw.KindIndex(i), true)) //nolint:gosec // bounded by the schema
	}
	h.markChosen()
}

func (h *handle) paletteButton(doc js.Value, name, class string, kind gw.KindIndex, isEdge bool) js.Value {
	b := doc.Call("createElement", "button")
	b.Set("type", "button")
	b.Set("className", "graphedwright-item graphedwright-"+class)
	b.Set("textContent", name)
	b.Set("title", "drag onto the drawing")
	if isEdge {
		b.Set("title", "draw new edges as "+name+"; press again to let the shapes choose")
	}
	h.palette.Call("appendChild", b)
	h.listen(b, "pointerdown", func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		args[0].Call("preventDefault")
		// The palette press is the pointer whose release drops the item.
		if !h.ptr.track(edit.PointerDown, args[0].Get("pointerId").Int()) {
			return nil
		}
		// preventDefault keeps the focus where it was, so the label
		// input gets no blur: commit its text here, as Enter in it does.
		// The editor hears first, so the blur that hiding causes finds
		// nothing left to commit.
		if h.ed.State() == edit.EditingText {
			ch, err := h.ed.Handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
			h.hideInput()
			h.after(ch, err)
		}
		ch, err := h.ed.Handle(edit.Event{Kind: edit.PaletteStart, Item: kind, IsEdge: isEdge})
		h.markChosen()
		h.after(ch, err)
		return nil
	})
	return b
}

// markChosen shows which edge kind's button is chosen, pressed and
// with the class graphedwright-chosen.
func (h *handle) markChosen() {
	chosen := h.ed.EdgeKind()
	for i, b := range h.edgeItems {
		on := gw.KindIndex(i) == chosen //nolint:gosec // bounded by the schema
		b.Get("classList").Call("toggle", "graphedwright-chosen", on)
		b.Call("setAttribute", "aria-pressed", strconv.FormatBool(on))
	}
}

// drawing is the element the SVG lives in.
func (h *handle) drawing() js.Value { return h.el.Call("querySelector", ".graphedwright-drawing") }

// pointer converts a pointer event to the editor's. The point is mapped
// into document units here, through the SVG's own screen matrix, which
// knows the viewport and the letterboxing the browser applied; the
// editor therefore sees document units and applies no viewport.
func (h *handle) pointer(kind edit.EventKind) func(js.Value, []js.Value) any {
	return func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		ev := args[0]
		if !h.ptr.track(kind, ev.Get("pointerId").Int()) {
			return nil
		}
		p := h.toDoc(ev.Get("clientX").Float(), ev.Get("clientY").Float())
		e := edit.Event{Kind: kind, Point: p, Button: ev.Get("button").Int(), Modifiers: modifiers(ev)}
		if kind == edit.PointerDown {
			h.el.Call("focus")
			ev.Call("preventDefault")
		}
		ch, err := h.ed.Handle(e)
		h.after(ch, err)
		return nil
	}
}

// escapeKey is the key that cancels what is in progress.
const escapeKey = "Escape"

// pointerCancel ends the gesture of the pointer that is down when the
// browser takes it, for a scroll or a system gesture: the editor cancels
// it as it does for Escape, so a drag goes back to where it began.
func (h *handle) pointerCancel(_ js.Value, args []js.Value) any {
	if len(args) == 0 || !h.ptr.cancel(args[0].Get("pointerId").Int()) {
		return nil
	}
	ch, err := h.ed.Handle(edit.Event{Kind: edit.Key, Key: escapeKey})
	h.after(ch, err)
	return nil
}

func modifiers(ev js.Value) edit.Modifiers {
	var m edit.Modifiers
	if ev.Get("shiftKey").Truthy() {
		m |= edit.Shift
	}
	if ev.Get("ctrlKey").Truthy() || ev.Get("metaKey").Truthy() {
		m |= edit.Ctrl
	}
	if ev.Get("altKey").Truthy() {
		m |= edit.Alt
	}
	return m
}

// toDoc maps a client point into document units through the SVG's
// screen matrix; without an SVG the point is taken as it is.
func (h *handle) toDoc(x, y float64) gw.Point {
	if !h.svgEl.Truthy() {
		return gw.Point{X: x, Y: y}
	}
	// A drawing of no size has a matrix with no inverse, and inverse()
	// would throw.
	ctm := h.svgEl.Call("getScreenCTM")
	if !ctm.Truthy() || ctm.Get("a").Float()*ctm.Get("d").Float()-ctm.Get("b").Float()*ctm.Get("c").Float() == 0 {
		return gw.Point{X: x, Y: y}
	}
	pt := h.svgEl.Call("createSVGPoint")
	pt.Set("x", x)
	pt.Set("y", y)
	q := pt.Call("matrixTransform", ctm.Call("inverse"))
	return gw.Point{X: q.Get("x").Float(), Y: q.Get("y").Float()}
}

// wheel forwards a scroll, in pixels: as document units through the
// SVG's scale to pan, so that the drawing moves as far as the scroll on
// screen at any zoom, or as they are with Ctrl to zoom.
func (h *handle) wheel(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	ev.Call("preventDefault")
	// The deltas are read before the mode: Firefox gives a mouse wheel's
	// in lines only to a page that asks for the mode first.
	dx, dy := ev.Get("deltaX").Float(), ev.Get("deltaY").Float()
	switch ev.Get("deltaMode").Int() {
	case 1:
		dx, dy = dx*lineHeight, dy*lineHeight
	case 2:
		pageH := h.drawing().Get("clientHeight").Float()
		dx, dy = dx*pageH, dy*pageH
	}
	m := modifiers(ev)
	if m&edit.Ctrl == 0 && h.svgEl.Truthy() {
		if ctm := h.svgEl.Call("getScreenCTM"); ctm.Truthy() && ctm.Get("a").Float() > 0 && ctm.Get("d").Float() > 0 {
			dx, dy = dx/ctm.Get("a").Float(), dy/ctm.Get("d").Float()
		}
	}
	p := h.toDoc(ev.Get("clientX").Float(), ev.Get("clientY").Float())
	ch, err := h.ed.Handle(edit.Event{Kind: edit.Wheel, Point: p, Delta: dy, DeltaX: dx, Modifiers: m})
	h.after(ch, err)
	return nil
}

// lineHeight is the pixels a wheel scroll counted in lines moves by.
const lineHeight = 16

func (h *handle) keydown(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	key := ev.Get("key").String()
	switch key {
	case "Delete", "Backspace", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Tab", "Enter", escapeKey:
		ev.Call("preventDefault")
	case "=", "+", "-", "_", "0":
		// With Ctrl these zoom the drawing, not the page.
		if modifiers(ev)&edit.Ctrl != 0 {
			ev.Call("preventDefault")
		}
	}
	ch, err := h.ed.Handle(edit.Event{Kind: edit.Key, Key: key, Modifiers: modifiers(ev)})
	h.after(ch, err)
	if h.ed.State() == edit.EditingText {
		// A character that began the editing is the input's first, not
		// typed into it again by the browser.
		ev.Call("preventDefault")
		h.showInput()
	}
	return nil
}

// doubleClick edits the label of what is under the pointer.
func (h *handle) doubleClick(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	ev.Call("preventDefault")
	p := h.toDoc(ev.Get("clientX").Float(), ev.Get("clientY").Float())
	ch, err := h.ed.Handle(edit.Event{Kind: edit.DoubleClick, Point: p})
	h.after(ch, err)
	if h.ed.State() == edit.EditingText {
		h.showInput()
	}
	return nil
}

// showInput puts the label input over the label being edited, as big
// as its box at the drawing's scale, holding the label selected, or the
// character typed to begin the editing with the caret after it.
func (h *handle) showInput() {
	v, e := h.ed.Editing()
	var text, id string
	var box gw.Viewport
	switch {
	case v != gw.None:
		text, id, box = h.d.Vertices[v].Label, string(h.d.Vertices[v].ID), svg.VertexLabelBox(h.d, v)
	case e != gw.None:
		text, id, box = h.d.Edges[e].Label, string(h.d.Edges[e].ID), svg.EdgeLabelBox(h.d, e)
	default:
		return
	}
	if cb := h.cbs[cbEditLabel]; cb.Type() == js.TypeFunction {
		arg := js.Global().Get("Object").New()
		arg.Set("id", id)
		arg.Set("text", text)
		h.emit(cbEditLabel, arg)
	}
	tl, br := h.toClient(gw.Point{X: box.X, Y: box.Y}), h.toClient(gw.Point{X: box.X + box.W, Y: box.Y + box.H})
	style := h.input.Get("style")
	style.Set("display", "block")
	style.Set("left", px(tl.X))
	style.Set("top", px(tl.Y))
	style.Set("width", px(br.X-tl.X))
	style.Set("height", px(br.Y-tl.Y))
	style.Set("fontSize", px((br.Y-tl.Y)*0.6))
	h.input.Call("focus")
	if typed := h.ed.Typed(); typed != "" {
		h.input.Set("value", typed)
		h.input.Call("setSelectionRange", len(typed), len(typed))
		return
	}
	h.input.Set("value", text)
	h.input.Call("select")
}

// px is a length in CSS pixels.
func px(x float64) string { return strconv.FormatFloat(x, 'f', 0, 64) + "px" }

// toClient maps document units to pixels within the host element,
// through the SVG's screen matrix.
func (h *handle) toClient(p gw.Point) gw.Point {
	if !h.svgEl.Truthy() {
		return p
	}
	ctm := h.svgEl.Call("getScreenCTM")
	if !ctm.Truthy() {
		return p
	}
	pt := h.svgEl.Call("createSVGPoint")
	pt.Set("x", p.X)
	pt.Set("y", p.Y)
	q := pt.Call("matrixTransform", ctm)
	host := h.el.Call("getBoundingClientRect")
	return gw.Point{X: q.Get("x").Float() - host.Get("left").Float(), Y: q.Get("y").Float() - host.Get("top").Float()}
}

func (h *handle) hideInput() {
	h.input.Get("style").Set("display", "none")
	h.el.Call("focus")
}

func (h *handle) inputKey(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	// The editor hears first: hiding the input blurs it, and a blur
	// while still editing keeps the text.
	switch ev.Get("key").String() {
	case "Enter":
		ev.Call("preventDefault")
		ch, err := h.ed.Handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
		h.hideInput()
		h.after(ch, err)
	case escapeKey:
		ev.Call("preventDefault")
		ch, err := h.ed.Handle(edit.Event{Kind: edit.TextCancel})
		h.hideInput()
		h.after(ch, err)
	}
	ev.Call("stopPropagation")
	return nil
}

// inputBlur keeps the text when the input loses the focus while still
// editing, as a click elsewhere does.
func (h *handle) inputBlur(js.Value, []js.Value) any {
	if h.ed.State() == edit.EditingText {
		h.input.Get("style").Set("display", "none")
		ch, err := h.ed.Handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
		h.after(ch, err)
	}
	return nil
}

// after patches the drawing from a change, reports an error, and tells
// the host what changed and what is selected.
func (h *handle) after(ch gw.Change, err error) {
	if err != nil {
		h.emit(cbError, err.Error())
	}
	if ch.Layout || !h.patch {
		h.renderAll()
	} else {
		h.patchElements(ch)
		h.renderOverlay()
	}
	if (len(ch.Vertices) > 0 || len(ch.Edges) > 0 || ch.Layout) && h.cbs[cbChange].Type() == js.TypeFunction {
		h.emit(cbChange, h.changeObject(ch))
	}
	sv, se := h.ed.Selected()
	if n := len(sv) + len(se); n != h.lastSel || n > 0 {
		h.lastSel = n
		if h.cbs[cbSelect].Type() == js.TypeFunction {
			h.emit(cbSelect, h.selectionObject(sv, se))
		}
	}
}

// emit calls a host's callback as a microtask: after the call that
// caused it returns and before the next event. A callback that throws
// is reported by the browser and never reaches Go, where an exception
// would end the program, and a callback that calls back in finds the
// drawing finished.
func (h *handle) emit(which int, arg any) {
	cb := h.cbs[which]
	if cb.Type() != js.TypeFunction {
		return
	}
	js.Global().Call("queueMicrotask", cb.Call("bind", js.Null(), arg))
}

func (h *handle) changeObject(ch gw.Change) js.Value {
	out := js.Global().Get("Object").New()
	vs := js.Global().Get("Array").New()
	for _, v := range ch.Vertices {
		vs.Call("push", string(h.d.Vertices[v].ID))
	}
	es := js.Global().Get("Array").New()
	for _, e := range ch.Edges {
		es.Call("push", string(h.d.Edges[e].ID))
	}
	out.Set("vertices", vs)
	out.Set("edges", es)
	out.Set("layout", ch.Layout)
	return out
}

func (h *handle) selectionObject(sv []gw.VertexIndex, se []gw.EdgeIndex) js.Value {
	out := js.Global().Get("Object").New()
	vs := js.Global().Get("Array").New()
	for _, v := range sv {
		vs.Call("push", string(h.d.Vertices[v].ID))
	}
	es := js.Global().Get("Array").New()
	for _, e := range se {
		es.Call("push", string(h.d.Edges[e].ID))
	}
	out.Set("vertices", vs)
	out.Set("edges", es)
	return out
}

// renderAll replaces the whole drawing.
func (h *handle) renderAll() {
	h.ed.Overlay(&h.opts)
	h.opts.VertexNotes, h.opts.EdgeNotes = h.notesV, h.notesE
	h.buf.Reset()
	if err := h.r.Render(h.d, h.opts, &h.buf); err != nil {
		return
	}
	h.drawing().Set("innerHTML", h.buf.String())
	h.svgEl = h.drawing().Call("querySelector", "svg")
	h.buildPalette()
}

// renderOverlay replaces the overlay group alone.
func (h *handle) renderOverlay() {
	h.ed.Overlay(&h.opts)
	h.buf.Reset()
	if err := h.r.RenderOverlay(h.d, h.opts, &h.buf); err != nil {
		return
	}
	if g := h.svgEl.Call("querySelector", "g.overlay"); g.Truthy() {
		g.Set("outerHTML", h.buf.String())
	}
}

// patchElements replaces, removes or appends the elements a change
// names, by id.
func (h *handle) patchElements(ch gw.Change) {
	for _, v := range ch.Vertices {
		h.buf.Reset()
		if err := h.r.RenderVertex(h.d, h.opts, v, &h.buf); err != nil {
			continue
		}
		group := "g.vertices"
		if h.d.Schema.VertexKinds[h.d.Vertices[v].Kind].Container {
			group = "g.containers"
		}
		h.patchOne(string(h.d.Vertices[v].ID), h.d.Vertices[v].Live, group)
	}
	for _, e := range ch.Edges {
		h.buf.Reset()
		if err := h.r.RenderEdge(h.d, h.opts, e, &h.buf); err != nil {
			continue
		}
		h.patchOne(string(h.d.Edges[e].ID), h.d.Edges[e].Live, "g.edges")
	}
}

// patchOne puts the rendered fragment in h.buf in place of the element
// with the id, removes the element when the fragment is a tombstone's,
// or appends it to the group when it is new.
func (h *handle) patchOne(id string, live bool, group string) {
	// CSS.escape makes any id a valid selector; one built by hand from
	// document text could throw, and a throw ends the program.
	el := h.svgEl.Call("querySelector", "#"+js.Global().Get("CSS").Call("escape", h.opts.DocIDPrefix+id).String())
	switch {
	case !live && el.Truthy():
		el.Call("remove")
	case !live:
	case el.Truthy():
		el.Set("outerHTML", h.buf.String())
	default:
		if g := h.svgEl.Call("querySelector", group); g.Truthy() {
			g.Call("insertAdjacentHTML", "beforeend", h.buf.String())
		}
	}
}
