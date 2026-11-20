package dom

import (
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit"
)

// funcOf is the one place the binding makes a js.Func. Each it makes is
// kept for the life of the program and none is released: the object's
// functions at Install, and the event listeners the first time a handle
// is mounted. Mount, Load, Annotate, SetMode, the redraws and Destroy
// make none.
func funcOf(fn func(js.Value, []js.Value) any) js.Func {
	listeners.count++
	return js.FuncOf(fn)
}

// FuncsMade is how many JavaScript functions the binding has made, for
// a test that they are made at start-up alone: the count after Install
// and the first Mount stays the same through every later call.
func FuncsMade() int { return listeners.count }

// listenerSet is the binding's event listeners, one function per event
// for every handle in the page, and the handles they route to. Each host
// element listens with the element functions; the window, while any
// handle is mounted, with the pointer functions, which go to the handle
// holding the pointer, or, for a hover, to the one under it.
type listenerSet struct {
	count   int  // the functions funcOf has made, the object's among them
	made    bool // the listeners are made
	element [elementEvents]js.Func
	window  [windowEvents]js.Func
	handles []*Handle // mounted, in the order they were
	hover   *Handle   // the handle the last hover went to, nil for none
}

// listeners is the page's one set: the functions are made once, when
// the first handle is mounted, and kept. It is the package's one
// variable: one set of listeners serves every handle in the page, and a
// host written in Go mounts without Install, so no value of its own
// could hold it.
var listeners listenerSet //nolint:gochecknoglobals // the page's one set of listeners, shared by every handle

// The events a host element listens for, by index.
const (
	evPointerDown = iota
	evWheel
	evDoubleClick
	evContextMenu
	evKeyDown
	evInput
	evBlur
	evClick
	elementEvents
)

// elementEventName is the DOM's name for one of them.
func elementEventName(ev int) string {
	switch ev {
	case evPointerDown:
		return "pointerdown"
	case evWheel:
		return "wheel"
	case evDoubleClick:
		return "dblclick"
	case evContextMenu:
		return "contextmenu"
	case evKeyDown:
		return "keydown"
	case evInput:
		return "input"
	case evBlur:
		return "blur"
	}
	return "click"
}

// The events the window listens for, by index.
const (
	evPointerMove = iota
	evPointerUp
	evPointerCancel
	windowEvents
)

// windowEventName is the DOM's name for one of them.
func windowEventName(ev int) string {
	switch ev {
	case evPointerMove:
		return "pointermove"
	case evPointerUp:
		return "pointerup"
	}
	return "pointercancel"
}

// elementEvent is one of a host element's listeners.
type elementEvent struct{ ev int }

// windowEvent is one of the window's listeners.
type windowEvent struct{ ev int }

// make makes the functions, the first time.
func (l *listenerSet) make() {
	if l.made {
		return
	}
	l.made = true
	for i := range l.element {
		l.element[i] = funcOf((&elementEvent{ev: i}).run)
	}
	for i := range l.window {
		l.window[i] = funcOf((&windowEvent{ev: i}).run)
	}
}

// add starts routing the page's events to a handle just mounted: its
// element listens, and the window does once a first handle is there.
func (l *listenerSet) add(h *Handle) {
	l.make()
	for i, f := range l.element {
		// The input's blur does not bubble: the element hears it as it
		// goes down to the input.
		h.el.Call("addEventListener", elementEventName(i), f, i == evBlur)
	}
	if len(l.handles) == 0 {
		w := js.Global()
		for i, f := range l.window {
			w.Call("addEventListener", windowEventName(i), f)
		}
	}
	l.handles = append(l.handles, h)
}

// remove stops routing to a handle being destroyed, and the window
// stops listening once no handle is left.
func (l *listenerSet) remove(h *Handle) {
	for i, f := range l.element {
		h.el.Call("removeEventListener", elementEventName(i), f, i == evBlur)
	}
	// bound: the handles mounted
	for i, x := range l.handles {
		if x == h {
			n := len(l.handles) - 1
			copy(l.handles[i:], l.handles[i+1:])
			l.handles[n] = nil // the handle goes, not kept by the array
			l.handles = l.handles[:n]
			break
		}
	}
	if l.hover == h {
		l.hover = nil
	}
	if len(l.handles) == 0 {
		w := js.Global()
		for i, f := range l.window {
			w.Call("removeEventListener", windowEventName(i), f)
		}
	}
}

// at is the handle mounted in the element, or nil.
func (l *listenerSet) at(el js.Value) *Handle {
	// bound: the handles mounted
	for _, h := range l.handles {
		if h.el.Equal(el) {
			return h
		}
	}
	return nil
}

// under is the handle whose element holds the pointer event's target,
// or, for an event sent to the window itself, the element at its point;
// nil for none.
func (l *listenerSet) under(ev js.Value) *Handle {
	node := ev.Get("target")
	if node.Type() != js.TypeObject || !node.Get("nodeType").Truthy() {
		node = js.Global().Get("document").Call("elementFromPoint", ev.Get("clientX").Float(), ev.Get("clientY").Float())
	}
	if node.Type() != js.TypeObject {
		return nil
	}
	// bound: the handles mounted
	for _, h := range l.handles {
		if h.el.Call("contains", node).Bool() {
			return h
		}
	}
	return nil
}

// holding is the handle the pointer is down in, or nil.
func (l *listenerSet) holding(id int) *Handle {
	// bound: the handles mounted
	for _, h := range l.handles {
		if h.ptr.down && h.ptr.id == id {
			return h
		}
	}
	return nil
}

// run routes an event of a host element's to its handle, and within it
// by where the event happened: the drawing, the palette, the zoom
// buttons or the label input.
func (x *elementEvent) run(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	h := listeners.at(ev.Get("currentTarget"))
	if h == nil || h.destroyed {
		return nil
	}
	target := ev.Get("target")
	if h.input.Equal(target) {
		return h.inputEvent(x.ev, ev)
	}
	if h.drawEl.Call("contains", target).Bool() && h.drawingEvent(x.ev, ev) {
		return nil
	}
	switch x.ev {
	case evPointerDown:
		if b := button(h.palette, target, ".graphedwright-item"); b.Truthy() {
			h.palettePress(b, ev)
		}
	case evKeyDown:
		h.keydown(ev)
	case evClick:
		if b := button(h.zoom, target, "[data-zoom]"); b.Truthy() {
			h.zoomPress(b)
		}
	}
	return nil
}

// inputEvent passes an event of the label input's to the handle.
func (h *Handle) inputEvent(kind int, ev js.Value) any {
	switch kind {
	case evKeyDown:
		h.inputKey(ev)
	case evInput:
		h.inputGrow()
	case evBlur:
		h.inputBlur()
	}
	return nil
}

// drawingEvent passes an event of the drawing's to the handle, and
// reports whether it took it.
func (h *Handle) drawingEvent(kind int, ev js.Value) bool {
	switch kind {
	case evPointerDown:
		h.pointer(edit.PointerDown, ev)
	case evWheel:
		h.wheel(ev)
	case evDoubleClick:
		h.doubleClick(ev)
	case evContextMenu:
		h.contextMenu(ev)
	default:
		return false
	}
	return true
}

// button is the button matching the selector that the target is or is
// in, within the container, or null.
func button(container, target js.Value, selector string) js.Value {
	if target.Type() != js.TypeObject || !target.Get("closest").Truthy() {
		return js.Null()
	}
	b := target.Call("closest", selector)
	if !b.Truthy() || !container.Call("contains", b).Bool() {
		return js.Null()
	}
	return b
}

// run routes a pointer event of the window's: to the handle the pointer
// is down in, so that a drag goes on outside its drawing and reaches no
// other; or, a hover, to the handle under the pointer, and to the one
// the last hover went to, once, so that it sees the pointer leave.
func (x *windowEvent) run(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	ev := args[0]
	id := ev.Get("pointerId").Int()
	h := listeners.holding(id)
	switch x.ev {
	case evPointerMove:
		if h == nil {
			h = listeners.under(ev)
			if last := listeners.hover; last != nil && last != h {
				listeners.hover = nil
				last.pointer(edit.PointerMove, ev)
			}
			listeners.hover = h
		}
		if h != nil {
			h.pointer(edit.PointerMove, ev)
		}
	case evPointerUp:
		if h != nil {
			h.pointer(edit.PointerUp, ev)
		}
	case evPointerCancel:
		if h != nil {
			h.pointerCancel(ev)
		}
	}
	return nil
}

// palettePress starts placing the button's vertex kind, or chooses or
// unchooses its edge kind: the button carries the kind as data-kind,
// and data-edge for an edge kind.
func (h *Handle) palettePress(b, ev js.Value) any {
	isEdge := b.Call("hasAttribute", "data-edge").Bool()
	n := h.d.Kinds().VertexKinds()
	if isEdge {
		n = h.d.Kinds().EdgeKinds()
	}
	kind, err := strconv.Atoi(b.Call("getAttribute", "data-kind").String())
	if err != nil || kind < 0 || kind >= n {
		return nil
	}
	ev.Call("preventDefault")
	// The palette press is the pointer whose release drops the item.
	if !h.ptr.track(edit.PointerDown, ev.Get("pointerId").Int()) {
		return nil
	}
	// preventDefault keeps the focus where it was, so the label input
	// gets no blur.
	h.commitInput()
	ch, errEd := h.ed.Handle(edit.Event{Kind: edit.PaletteStart, Item: gw.KindIndex(kind), IsEdge: isEdge}) //nolint:gosec // bounded by the kinds above
	h.markChosen()
	h.report(ch, errEd)
	return nil
}

// zoomPress zooms by the button's data-zoom, "in", "out" or "fit",
// through the handle's own calls, so that a click is dropped as any
// event is while a Host method runs, and grouped.
func (h *Handle) zoomPress(b js.Value) any {
	switch b.Call("getAttribute", "data-zoom").String() {
	case zoomIn:
		_ = h.Zoom(zoomFactor) // discarded: a refusal drops the click, as any event is dropped while a Host method runs or after destroy
	case zoomOut:
		_ = h.Zoom(1 / zoomFactor) // discarded: as for in
	case zoomFit:
		_ = h.Fit() // discarded: as for in
	}
	return nil
}

// What a zoom button does, as its data-zoom says.
const (
	zoomIn  = "in"
	zoomOut = "out"
	zoomFit = "fit"
)
