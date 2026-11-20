package dom

import (
	"math"
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// mount builds the host element's content: the palette, the drawing,
// the zoom buttons, the hidden label input, a textarea so that a label
// keeps its lines, and starts routing the page's events to the handle.
// Mount draws it, and the palette with it.
func (h *Handle) mount() {
	doc := js.Global().Get("document")
	h.el.Set("tabIndex", 0)
	h.el.Get("classList").Call("add", "graphedwright-host")
	h.el.Set("innerHTML", "")
	h.palette = doc.Call("createElement", "div")
	h.palette.Set("className", "graphedwright-palette")
	h.el.Call("appendChild", h.palette)
	h.drawEl = doc.Call("createElement", "div")
	h.drawEl.Set("className", "graphedwright-drawing")
	h.el.Call("appendChild", h.drawEl)
	h.zoomBar(doc)
	h.input = doc.Call("createElement", "textarea")
	h.input.Set("className", "graphedwright-label")
	h.input.Set("wrap", "off")
	h.input.Get("style").Set("display", "none")
	h.el.Call("appendChild", h.input)
	listeners.add(h)
}

// unmount stops the page's events reaching the handle and removes the
// content.
func (h *Handle) unmount() {
	listeners.remove(h)
	h.el.Set("innerHTML", "")
}

// zoomBar makes the buttons that zoom in, out and to the drawing, in
// either mode, each naming what it does as data-zoom.
func (h *Handle) zoomBar(doc js.Value) {
	h.zoom = doc.Call("createElement", "div")
	h.zoom.Set("className", "graphedwright-zoom")
	h.el.Call("appendChild", h.zoom)
	for _, b := range [...]struct{ text, title, zoom string }{
		{"+", "zoom in (Ctrl+=)", zoomIn}, {"\u2212", "zoom out (Ctrl+-)", zoomOut}, {"Fit", "fit the drawing (Ctrl+0)", zoomFit},
	} {
		el := doc.Call("createElement", "button")
		el.Set("type", "button")
		el.Set("textContent", b.text)
		el.Set("title", b.title)
		el.Call("setAttribute", "aria-label", b.title)
		el.Call("setAttribute", "data-zoom", b.zoom)
		h.zoom.Call("appendChild", el)
	}
}

// zoomFactor is what one press of a zoom button zooms by.
const zoomFactor = 1.25

// syncPalette makes the palette again when the mode or the kinds' names
// have changed since it was made, and shows the edge kind chosen.
func (h *Handle) syncPalette() {
	if !h.palBuilt || h.palMode != h.ed.Mode() || !sameNames(h.palKinds, h.d.Kinds()) {
		h.buildPalette()
	}
	h.markChosen()
}

// sameNames reports whether two schemas' kinds have the same names in
// the same order, all the palette shows of them.
func sameNames(a, b *gw.Kinds) bool {
	if a == b {
		return true
	}
	if a.VertexKinds() != b.VertexKinds() || a.EdgeKinds() != b.EdgeKinds() {
		return false
	}
	// bound: the vertex kinds
	for i := range a.VertexKinds() {
		if a.Vertex(gw.KindIndex(i)).Name() != b.Vertex(gw.KindIndex(i)).Name() { //nolint:gosec // bounded by the kinds
			return false
		}
	}
	// bound: the edge kinds
	for i := range a.EdgeKinds() {
		if a.Edge(gw.KindIndex(i)).Name() != b.Edge(gw.KindIndex(i)).Name() { //nolint:gosec // bounded by the kinds
			return false
		}
	}
	return true
}

// buildPalette makes a button per vertex kind, which a press starts
// placing, and per edge kind, which a press chooses for the edges drawn
// from then on, or unchooses. A button carries its kind as data-kind,
// and an edge kind's data-edge, for the element's one listener.
func (h *Handle) buildPalette() {
	h.palBuilt, h.palMode, h.palKinds = true, h.ed.Mode(), h.d.Kinds()
	doc := js.Global().Get("document")
	h.palette.Set("innerHTML", "")
	h.edgeItems = h.edgeItems[:0]
	if h.ed.Mode() != edit.Edit {
		return
	}
	kinds := h.d.Kinds()
	// bound: the vertex kinds
	for i := range kinds.VertexKinds() {
		h.paletteButton(doc, kinds.Vertex(gw.KindIndex(i)).Name(), "vertex", i, false) //nolint:gosec // bounded by the kinds
	}
	// bound: the edge kinds
	for i := range kinds.EdgeKinds() {
		h.edgeItems = append(h.edgeItems, h.paletteButton(doc, kinds.Edge(gw.KindIndex(i)).Name(), "edge", i, true)) //nolint:gosec // bounded by the kinds
	}
}

func (h *Handle) paletteButton(doc js.Value, name, class string, kind int, isEdge bool) js.Value {
	b := doc.Call("createElement", "button")
	b.Set("type", "button")
	b.Set("className", "graphedwright-item graphedwright-"+class)
	b.Set("textContent", name)
	b.Set("title", "drag onto the drawing")
	b.Call("setAttribute", "data-kind", strconv.Itoa(kind))
	if isEdge {
		b.Set("title", "draw new edges as "+name+"; press again to let the shapes choose")
		b.Call("setAttribute", "data-edge", "")
	}
	h.palette.Call("appendChild", b)
	return b
}

// markChosen shows which edge kind's button is chosen, pressed and
// with the class graphedwright-chosen.
func (h *Handle) markChosen() {
	chosen := h.ed.EdgeKind()
	for i, b := range h.edgeItems {
		on := gw.KindIndex(i) == chosen //nolint:gosec // bounded by the schema
		b.Get("classList").Call("toggle", "graphedwright-chosen", on)
		b.Call("setAttribute", "aria-pressed", strconv.FormatBool(on))
	}
}

// drawing is the element the SVG lives in.
func (h *Handle) drawing() js.Value { return h.drawEl }

// pointer converts a pointer event to the editor's. The point is mapped
// into document units here, through the SVG's own screen matrix, which
// knows the viewport and the letterboxing the browser applied; the
// editor therefore sees document units and applies no viewport.
func (h *Handle) pointer(kind edit.EventKind, ev js.Value) any {
	if !h.ptr.track(kind, ev.Get("pointerId").Int()) {
		return nil
	}
	p := h.toDoc(ev.Get("clientX").Float(), ev.Get("clientY").Float())
	e := edit.Event{Kind: kind, Point: p, Button: ev.Get("button").Int(), Modifiers: modifiers(ev)}
	if kind == edit.PointerDown {
		h.el.Call("focus")
		ev.Call("preventDefault")
	}
	h.handle(e)
	return nil
}

// commitInput commits the label input's text, as Enter in it does, when
// an event that keeps the focus where it was ends the editing. The
// editor hears first, so the blur that hiding causes finds nothing left
// to commit.
func (h *Handle) commitInput() {
	if h.ed.State() != edit.EditingText {
		return
	}
	ch, err := h.ed.Handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
	h.hideInput()
	h.report(ch, err)
}

// contextMenu takes the browser's contextmenu: the editor ends the
// gesture under way and settles the selection, whether or not the host
// takes the request, since the browser's menu may swallow the release.
// When the host takes it, the browser's menu is not shown and the host
// hears of it once the selection is drawn.
func (h *Handle) contextMenu(ev js.Value) any {
	if h.inHost || h.destroyed {
		return nil
	}
	if h.context {
		ev.Call("preventDefault")
	}
	client := gw.Point{X: ev.Get("clientX").Float(), Y: ev.Get("clientY").Float()}
	keyboard := h.fromKeyboard(ev, client)
	h.commitInput()
	h.handle(edit.Event{Kind: edit.Context, Point: h.toDoc(client.X, client.Y), Keyboard: keyboard})
	if !h.context || h.destroyed {
		return nil
	}
	req := h.ed.Context()
	if keyboard {
		client = h.toScreen(req.Point)
	}
	h.enter()
	h.host.Context(req, client)
	h.leave()
	return nil
}

// fromKeyboard reports whether a contextmenu came from the keyboard, the
// menu key or Shift+F10, which has no pointer position: not the
// secondary button, and a pointer event of no pointer type, or a point
// outside the drawing.
func (h *Handle) fromKeyboard(ev js.Value, p gw.Point) bool {
	if ev.Get("button").Int() == 2 {
		return false
	}
	if t := ev.Get("pointerType"); t.Type() == js.TypeString && t.String() == "" {
		return true
	}
	r := h.drawing().Call("getBoundingClientRect")
	return p.X < r.Get("left").Float() || p.X > r.Get("right").Float() || p.Y < r.Get("top").Float() || p.Y > r.Get("bottom").Float()
}

// escapeKey is the key that cancels what is in progress.
const escapeKey = "Escape"

// pointerCancel ends the gesture of the pointer that is down when the
// browser takes it, for a scroll or a system gesture: the editor cancels
// it as it does for Escape, so a drag goes back to where it began.
func (h *Handle) pointerCancel(ev js.Value) any {
	if !h.ptr.cancel(ev.Get("pointerId").Int()) {
		return nil
	}
	h.handle(edit.Event{Kind: edit.Key, Key: escapeKey})
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
func (h *Handle) toDoc(x, y float64) gw.Point {
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
func (h *Handle) wheel(ev js.Value) any {
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
	h.handle(edit.Event{Kind: edit.Wheel, Point: p, Delta: dy, DeltaX: dx, Modifiers: m})
	return nil
}

// lineHeight is the pixels a wheel scroll counted in lines moves by.
const lineHeight = 16

func (h *Handle) keydown(ev js.Value) any {
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
	h.handle(edit.Event{Kind: edit.Key, Key: key, Modifiers: modifiers(ev)})
	if h.ed.State() == edit.EditingText {
		// A character that began the editing is the input's first, not
		// typed into it again by the browser.
		ev.Call("preventDefault")
		h.showInput()
	}
	return nil
}

// doubleClick edits the label of what is under the pointer.
func (h *Handle) doubleClick(ev js.Value) any {
	ev.Call("preventDefault")
	p := h.toDoc(ev.Get("clientX").Float(), ev.Get("clientY").Float())
	h.handle(edit.Event{Kind: edit.DoubleClick, Point: p})
	if h.ed.State() == edit.EditingText {
		h.showInput()
	}
	return nil
}

// showInput puts the label input over the label being edited, as big
// as its box at the drawing's scale, its text at the drawing's size and
// its lines as far apart, holding the label selected, or the character
// typed to begin the editing with the caret after it.
func (h *Handle) showInput() {
	v, e := h.ed.Editing()
	text := h.ed.EditText()
	var box gw.Viewport
	switch {
	case v != gw.None:
		box = svg.VertexLabelBox(h.d, v)
	case e != gw.None:
		box = svg.EdgeLabelInputBox(h.d, e)
	default:
		return
	}
	h.enter()
	h.host.EditingLabel(v, e, text)
	h.leave()
	tl, br := h.toClient(gw.Point{X: box.X, Y: box.Y}), h.toClient(gw.Point{X: box.X + box.W, Y: box.Y + box.H})
	style := h.input.Get("style")
	style.Set("display", "block")
	style.Set("left", px(tl.X))
	style.Set("top", px(tl.Y))
	style.Set("width", px(br.X-tl.X))
	style.Set("height", px(br.Y-tl.Y))
	scale := 1.0
	if box.H > 0 {
		scale = (br.Y - tl.Y) / box.H
	}
	h.inputH = br.Y - tl.Y
	style.Set("fontSize", px(scale*labelFont))
	style.Set("lineHeight", px(scale*gw.LabelLine))
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
func (h *Handle) toClient(p gw.Point) gw.Point {
	q, ok := h.screen(p)
	if !ok {
		return p
	}
	host := h.el.Call("getBoundingClientRect")
	return gw.Point{X: q.X - host.Get("left").Float(), Y: q.Y - host.Get("top").Float()}
}

// toScreen maps document units to the page's client coordinates,
// through the SVG's screen matrix.
func (h *Handle) toScreen(p gw.Point) gw.Point {
	q, _ := h.screen(p)
	return q
}

// screen maps document units through the SVG's screen matrix, when
// there is one; without, the point is as it was.
func (h *Handle) screen(p gw.Point) (gw.Point, bool) {
	if !h.svgEl.Truthy() {
		return p, false
	}
	ctm := h.svgEl.Call("getScreenCTM")
	if !ctm.Truthy() {
		return p, false
	}
	pt := h.svgEl.Call("createSVGPoint")
	pt.Set("x", p.X)
	pt.Set("y", p.Y)
	q := pt.Call("matrixTransform", ctm)
	return gw.Point{X: q.Get("x").Float(), Y: q.Get("y").Float()}, true
}

func (h *Handle) hideInput() {
	h.input.Get("style").Set("display", "none")
	h.el.Call("focus")
}

// inputKey keeps the text on Enter and drops it on Escape; Shift-Enter
// starts a new line, as the textarea does by itself.
func (h *Handle) inputKey(ev js.Value) any {
	// The editor hears first: hiding the input blurs it, and a blur
	// while still editing keeps the text.
	switch ev.Get("key").String() {
	case "Enter":
		if ev.Get("shiftKey").Bool() {
			break
		}
		ev.Call("preventDefault")
		h.handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
		h.hideInput()
	case escapeKey:
		ev.Call("preventDefault")
		h.handle(edit.Event{Kind: edit.TextCancel})
		h.hideInput()
	}
	ev.Call("stopPropagation")
	return nil
}

// inputGrow makes the input tall enough for the lines typed into it,
// never less than the label's box it was put over.
func (h *Handle) inputGrow() any {
	h.input.Get("style").Set("height", px(math.Max(h.inputH, h.input.Get("scrollHeight").Float())))
	return nil
}

// labelFont is a label's size at the default font, which the input's
// text is drawn at, scaled as the drawing is.
const labelFont = 12

// inputBlur keeps the text when the input loses the focus while still
// editing, as a click elsewhere does.
func (h *Handle) inputBlur() any {
	if h.ed.State() == edit.EditingText {
		h.input.Get("style").Set("display", "none")
		h.handle(edit.Event{Kind: edit.TextDone, Text: h.input.Get("value").String()})
	}
	return nil
}

// renderAll replaces the whole drawing.
func (h *Handle) renderAll() {
	h.ed.Overlay(&h.opts)
	h.opts.VertexNotes, h.opts.EdgeNotes = h.notesV, h.notesE
	h.buf.Reset()
	if err := h.r.Render(h.d, h.opts, &h.buf); err != nil {
		return
	}
	h.drawing().Set("innerHTML", h.buf.String())
	h.svgEl = h.drawing().Call("querySelector", "svg")
	h.syncPalette()
}

// renderOverlay replaces the overlay group alone. The groups are found
// as children of the SVG root: an annotation's class is the host's and
// may be a group's name, and an element carrying it would come first.
func (h *Handle) renderOverlay() {
	h.ed.Overlay(&h.opts)
	h.buf.Reset()
	if err := h.r.RenderOverlay(h.d, h.opts, &h.buf); err != nil {
		return
	}
	if g := h.svgEl.Call("querySelector", ":scope > g.overlay"); g.Truthy() {
		g.Set("outerHTML", h.buf.String())
	}
}

// patchElements replaces, removes or appends the elements a change
// names, by id.
func (h *Handle) patchElements(ch gw.Change) {
	for _, v := range ch.Vertices {
		h.buf.Reset()
		if err := h.r.RenderVertex(h.d, h.opts, v, &h.buf); err != nil {
			continue
		}
		group := ":scope > g.vertices"
		if h.d.Kinds().Vertex(h.d.Vertex(v).Kind()).Container() {
			group = ":scope > g.containers"
		}
		h.patchOne(string(h.d.Vertex(v).ID()), h.d.Vertex(v).Live(), group)
	}
	for _, e := range ch.Edges {
		h.buf.Reset()
		if err := h.r.RenderEdge(h.d, h.opts, e, &h.buf); err != nil {
			continue
		}
		h.patchOne(string(h.d.Edge(e).ID()), h.d.Edge(e).Live(), ":scope > g.edges")
	}
}

// patchOne puts the rendered fragment in h.buf in place of the element
// with the id, removes the element when the fragment is a tombstone's,
// or appends it to the group, a child of the SVG root, when it is new.
func (h *Handle) patchOne(id string, live bool, group string) {
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
