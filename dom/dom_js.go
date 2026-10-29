// Package dom is the browser binding: it mounts a document in a host
// element, forwards the element's pointer, wheel and keyboard events to
// the editor, patches the SVG from each change, and exposes the
// graphedwright object to JavaScript. It is the only package that
// touches the browser, and it uses the part of syscall/js that both
// the Go and the TinyGo toolchains implement.
package dom

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// The callbacks a host can register, by index.
const (
	cbChange = iota
	cbSelect
	cbError
	cbEditLabel
	cbCount
)

// handle is one mounted document with everything the binding keeps
// for it.
type handle struct {
	id        int
	el        js.Value // the host element
	svgEl     js.Value // the SVG inside it
	palette   js.Value
	edgeItems []js.Value // the palette's edge kind buttons, by kind
	grid      float64    // the grid create was given, for each editor load makes
	input     js.Value
	d         *gw.Document
	ed        *edit.Editor
	r         *svg.Renderer
	lay       *layout.Layouter
	schemas   []doc.Named
	schema    string
	caps      gw.Caps
	notesV    []svg.Annotation
	notesE    []svg.Annotation
	opts      svg.Options
	buf       bytes.Buffer
	funcs     []js.Func
	cbs       [cbCount]js.Value
	patch     bool     // patch the DOM by element, else re-render the whole drawing
	lastSel   int      // the selection's size at the last report, to notice a change
	ptr       pointers // the pointer that drives the editor
}

// binding is the page's state: the handles and the schemas known.
type binding struct {
	handles map[int]*handle
	next    int
	schemas []doc.Named
	funcs   []js.Func
}

var state = &binding{handles: map[int]*handle{}, next: 1, schemas: doc.Builtin()} //nolint:gochecknoglobals // the page's one binding

// Register adds a schema a page can name in create, for a host built
// in Go that embeds its own kinds.
func Register(name string, s gw.Schema) {
	state.schemas = append(state.schemas, doc.Named{Name: name, Schema: s})
}

// Main installs the graphedwright object and keeps the program alive
// for its callbacks.
func Main() {
	obj := js.Global().Get("Object").New()
	for _, f := range []struct {
		name string
		fn   func(js.Value, []js.Value) any
	}{
		{"create", create}, {"load", withHandle(load)}, {"export", withHandle(export)}, {"svg", withHandle(svgText)},
		{"setMode", withHandle(setMode)}, {"layout", withHandle(relayout)}, {"annotate", withHandle(annotate)},
		{"on", withHandle(on)}, {"undo", withHandle(undo)}, {"redo", withHandle(redo)}, {"destroy", withHandle(destroy)},
		{"properties", withHandle(properties)}, {"setLabel", withHandle(setLabel)}, {"setAttr", withHandle(setAttr)},
		{"zoom", withHandle(zoom)}, {"fit", withHandle(fit)},
		{"registerSchema", registerSchema},
	} {
		fn := js.FuncOf(f.fn)
		state.funcs = append(state.funcs, fn)
		obj.Set(f.name, fn)
	}
	js.Global().Set("graphedwright", obj)
	select {}
}

// result is what every call returns: {ok: value} or {error: text}.
func result(v any, err error) js.Value {
	out := js.Global().Get("Object").New()
	if err != nil {
		out.Set("error", err.Error())
		return out
	}
	out.Set("ok", v)
	return out
}

func fail(msg string) js.Value { return result(nil, errors.New(msg)) }

// withHandle looks the handle up from the first argument.
func withHandle(f func(*handle, []js.Value) js.Value) func(js.Value, []js.Value) any {
	return func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Type() != js.TypeNumber {
			return fail("the first argument is the handle create returned")
		}
		h, ok := state.handles[args[0].Int()]
		if !ok {
			return fail("no such handle " + strconv.Itoa(args[0].Int()))
		}
		return f(h, args[1:])
	}
}

// create(el, {schema, mode, caps, patch}) mounts an empty document of
// the schema in the element and returns the handle.
func create(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("create(element, options)")
	}
	opts := js.Undefined()
	if len(args) > 1 {
		opts = args[1]
	}
	schemas, name, err := schemaOption(opts)
	if err != nil {
		return result(nil, err)
	}
	caps := capsOption(opts)
	prefix := str(opts, "idPrefix")
	if !token(prefix) {
		return fail("idPrefix may hold only letters, digits, '-' and '_'")
	}
	d, err := doc.Decode([]byte(`{"schema":`+strconv.Quote(name)+`}`), doc.Options{Schemas: schemas, Caps: caps})
	if err != nil {
		return result(nil, err)
	}
	h := &handle{id: state.next, el: args[0], d: d, r: svg.New(), lay: layout.New(d.Caps()), schemas: schemas, schema: name, caps: d.Caps(), patch: true}
	state.next++
	// The renderer's own IDs are the handle's, so that two editors in a
	// page keep their arrowheads apart; the document's IDs take the
	// host's prefix, if it gives one.
	h.opts.IDPrefix, h.opts.DocIDPrefix = "gw"+strconv.Itoa(h.id)+"-", prefix
	mode := edit.Edit
	if str(opts, "mode") == "view" {
		mode = edit.View
	}
	if get(opts, "patch").Type() == js.TypeBoolean && !get(opts, "patch").Bool() {
		h.patch = false
	}
	h.grid = float(opts, "grid")
	h.ed = edit.New(d, mode, h.grid)
	h.notesV = make([]svg.Annotation, d.Caps().Vertices)
	h.notesE = make([]svg.Annotation, d.Caps().Edges)
	h.mount()
	state.handles[h.id] = h
	return result(h.id, nil)
}

// registerSchema(name, schemaObject) adds a schema described from
// JavaScript.
func registerSchema(_ js.Value, args []js.Value) any {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeObject {
		return fail("registerSchema(name, schema)")
	}
	s, err := schemaFromJS(args[1])
	if err != nil {
		return result(nil, err)
	}
	Register(args[0].String(), s)
	return result(true, nil)
}

// schemaOption resolves the schema option: a name the binding knows,
// or an object describing one, which is then known under its name or
// "custom".
func schemaOption(opts js.Value) ([]doc.Named, string, error) {
	v := get(opts, "schema")
	switch v.Type() {
	case js.TypeString:
		for _, n := range state.schemas {
			if n.Name == v.String() {
				return state.schemas, v.String(), nil
			}
		}
		return nil, "", errors.New("schema " + strconv.Quote(v.String()) + " not known; registerSchema it or pass an object")
	case js.TypeObject:
		s, err := schemaFromJS(v)
		if err != nil {
			return nil, "", err
		}
		name := str(v, "name")
		if name == "" {
			name = "custom"
		}
		return append([]doc.Named{{Name: name, Schema: s}}, state.schemas...), name, nil
	case js.TypeUndefined, js.TypeNull, js.TypeBoolean, js.TypeNumber, js.TypeSymbol, js.TypeFunction:
	}
	return state.schemas, "basic", nil
}

// capsOption reads {caps: {vertices, edges, ...}}.
func capsOption(opts js.Value) gw.Caps {
	c := get(opts, "caps")
	return gw.Caps{
		Vertices: count(c, "vertices"), Edges: count(c, "edges"), PortsPerVertex: count(c, "portsPerVertex"),
		AttrsPerElement: count(c, "attrsPerElement"), LabelBytes: count(c, "labelBytes"), UndoDepth: count(c, "undoDepth"),
		UndoLayouts: count(c, "undoLayouts"), BendsPerEdge: count(c, "bendsPerEdge"),
	}
}

// load(g, document) replaces the document with one decoded from a JSON
// string or an object, which is stringified in JavaScript.
func load(h *handle, args []js.Value) js.Value {
	if len(args) < 1 {
		return fail("load(handle, document)")
	}
	text, err := textOf(args[0])
	if err != nil {
		return result(nil, err)
	}
	d, name, err := doc.DecodeNamed([]byte(text), doc.Options{Schemas: h.schemas, Caps: h.caps})
	if err != nil {
		return result(nil, err)
	}
	h.d, h.schema = d, name
	h.ed = edit.New(d, h.ed.Mode(), h.grid)
	h.lay = layout.New(d.Caps())
	clear(h.notesV)
	clear(h.notesE)
	h.fitView()
	h.renderAll()
	return result(true, nil)
}

// textOf is a string argument as it is, or an object as JSON.
func textOf(v js.Value) (string, error) {
	switch v.Type() {
	case js.TypeString:
		return v.String(), nil
	case js.TypeObject:
		return js.Global().Get("JSON").Call("stringify", v).String(), nil
	case js.TypeUndefined, js.TypeNull, js.TypeBoolean, js.TypeNumber, js.TypeSymbol, js.TypeFunction:
	}
	return "", errors.New("a document is a JSON string or an object")
}

// export(g) returns the document as an object.
func export(h *handle, _ []js.Value) js.Value {
	h.buf.Reset()
	if err := doc.Encode(h.d, h.schema, &h.buf); err != nil {
		return result(nil, err)
	}
	return result(js.Global().Get("JSON").Call("parse", h.buf.String()), nil)
}

// svg(g) returns the current drawing as text, a document of its own
// and so without the page's ID prefixes.
func svgText(h *handle, _ []js.Value) js.Value {
	h.buf.Reset()
	h.ed.Overlay(&h.opts)
	opts := h.opts
	opts.IDPrefix, opts.DocIDPrefix = "", ""
	if err := h.r.Render(h.d, opts, &h.buf); err != nil {
		return result(nil, err)
	}
	return result(h.buf.String(), nil)
}

// setMode(g, "view"|"edit").
func setMode(h *handle, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail(`setMode(handle, "view" or "edit")`)
	}
	// SetMode cancels label editing, so the input goes with it: left
	// showing, it would take text the editor no longer wants. As for
	// Escape in the input, the editor hears first, so the blur that
	// hiding causes finds nothing to commit.
	if h.ed.State() == edit.EditingText {
		ch, err := h.ed.Handle(edit.Event{Kind: edit.TextCancel})
		h.hideInput()
		h.after(ch, err)
	}
	switch args[0].String() {
	case "view":
		h.ed.SetMode(edit.View)
	case "edit":
		h.ed.SetMode(edit.Edit)
	default:
		return fail(`mode is "view" or "edit"`)
	}
	h.renderAll()
	return result(true, nil)
}

// layout(g, {all, flow}) lays the document out, in the flow named
// ("left-to-right" or "top-to-bottom") or the document's own.
func relayout(h *handle, args []js.Value) js.Value {
	all := len(args) > 0 && get(args[0], "all").Truthy()
	text, flow := "", ""
	if all {
		text = "all"
	}
	if len(args) > 0 {
		flow = str(args[0], "flow")
	}
	if _, err := h.d.Apply(gw.Command{Op: gw.Layout, Text: text, Value: flow}); err != nil {
		return result(nil, err)
	}
	res, err := h.lay.Run(h.d, layout.Options{All: all})
	if err != nil {
		return result(nil, err)
	}
	h.fitView()
	h.renderAll()
	out := js.Global().Get("Object").New()
	out.Set("layers", res.Layers)
	out.Set("crossings", res.Crossings)
	return result(out, nil)
}

// fitView sets the viewport to the drawing's bounds when any vertex
// lies outside it, so a layout is never off the screen.
func (h *handle) fitView() {
	if h.d.LiveVertices() == 0 {
		// Nothing to fit: one unit to a pixel of the drawing, so that a
		// vertex placed in it is drawn at its size.
		w, ht := h.drawing().Get("clientWidth").Float(), h.drawing().Get("clientHeight").Float()
		if w <= 0 || ht <= 0 {
			w, ht = 800, 480
		}
		h.d.View = gw.Viewport{W: w, H: ht}
		return
	}
	b := svg.Bounds(h.d)
	// The bounds carry the renderer's margin; the vertices themselves
	// are what must be in view.
	x0, y0, x1, y1 := b.X+svg.Margin, b.Y+svg.Margin, b.X+b.W-svg.Margin, b.Y+b.H-svg.Margin
	v := h.d.View
	if v.W <= 0 || v.H <= 0 || x0 < v.X || y0 < v.Y || x1 > v.X+v.W || y1 > v.Y+v.H {
		h.d.View = b
	}
}

// zoom(g, factor) zooms the view about its centre, in by a factor above
// 1 and out by one below; a factor that is not a positive number, or a
// zoom past the view's bounds, changes nothing.
func zoom(h *handle, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeNumber {
		return fail("zoom(handle, factor)")
	}
	f := args[0].Float()
	if !(f > 0) || f-f != 0 {
		return fail("the zoom factor is a positive number")
	}
	h.after(h.ed.Zoom(f), nil)
	return result(true, nil)
}

// fit(g) sets the view to the drawing's bounds.
func fit(h *handle, _ []js.Value) js.Value {
	h.after(h.ed.Fit(), nil)
	return result(true, nil)
}

// properties(g) describes the one selected vertex or edge for a
// properties panel, or is null: {id, element ("vertex" or "edge"), kind,
// label, from, to (an edge's), attrs: [{key, value, kind ("text",
// "number" or "choice"), choices, required, known}]}.
func properties(h *handle, _ []js.Value) js.Value {
	p, ok := h.ed.Properties()
	if !ok {
		return result(nil, nil)
	}
	obj := js.Global().Get("Object")
	out := obj.New()
	out.Set("id", string(p.ID))
	out.Set("kind", p.Kind)
	out.Set("label", p.Label)
	out.Set("element", "vertex")
	if p.Edge != gw.None {
		out.Set("element", "edge")
		out.Set("from", string(p.From))
		out.Set("to", string(p.To))
	}
	attrs := js.Global().Get("Array").New()
	for _, a := range p.Attrs {
		x := obj.New()
		x.Set("key", a.Key)
		x.Set("value", a.Value)
		x.Set("kind", attrKind(a.Kind))
		choices := js.Global().Get("Array").New()
		for _, c := range a.Choices {
			choices.Call("push", c)
		}
		x.Set("choices", choices)
		x.Set("required", a.Required)
		x.Set("known", a.Known)
		attrs.Call("push", x)
	}
	out.Set("attrs", attrs)
	return result(out, nil)
}

// attrKind names an attribute kind as properties gives it.
func attrKind(k gw.AttrKind) string {
	switch k {
	case gw.Number:
		return "number"
	case gw.Choice:
		return "choice"
	case gw.Text:
	}
	return "text"
}

// setLabel(g, text) relabels the one selected vertex or edge.
func setLabel(h *handle, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail("setLabel(handle, text)")
	}
	ch, err := h.ed.SetLabel(args[0].String())
	h.after(ch, err)
	return result(err == nil, err)
}

// setAttr(g, key, value) sets an attribute of the one selected vertex or
// edge; an empty value removes it.
func setAttr(h *handle, args []js.Value) js.Value {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeString {
		return fail("setAttr(handle, key, value)")
	}
	ch, err := h.ed.SetAttr(args[0].String(), args[1].String())
	h.after(ch, err)
	return result(err == nil, err)
}

// annotate(g, {id: {text, class, weight}}) sets the figures drawn on
// the elements named, replacing the last ones.
func annotate(h *handle, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("annotate(handle, {id: {text, class, weight}})")
	}
	clear(h.notesV)
	clear(h.notesE)
	keys := js.Global().Get("Object").Call("keys", args[0])
	for i := 0; i < keys.Length(); i++ {
		id := keys.Index(i).String()
		n := args[0].Get(id)
		note := svg.Annotation{Text: str(n, "text"), Class: str(n, "class"), Weight: float(n, "weight")}
		if v := h.d.VertexByID(gw.ID(id)); v != gw.None {
			h.notesV[v] = note
		} else if e := h.d.EdgeByID(gw.ID(id)); e != gw.None {
			h.notesE[e] = note
		}
	}
	h.renderAll()
	return result(true, nil)
}

// on(g, event, callback) registers a callback for "change", "select",
// "error" or "edit-label".
func on(h *handle, args []js.Value) js.Value {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeFunction {
		return fail("on(handle, event, callback)")
	}
	switch args[0].String() {
	case "change":
		h.cbs[cbChange] = args[1]
	case "select":
		h.cbs[cbSelect] = args[1]
	case "error":
		h.cbs[cbError] = args[1]
	case "edit-label":
		h.cbs[cbEditLabel] = args[1]
	default:
		return fail(`events are "change", "select", "error" and "edit-label"`)
	}
	return result(true, nil)
}

func undo(h *handle, _ []js.Value) js.Value {
	ch, ok := h.d.Undo()
	h.after(ch, nil)
	return result(ok, nil)
}

func redo(h *handle, _ []js.Value) js.Value {
	ch, ok := h.d.Redo()
	h.after(ch, nil)
	return result(ok, nil)
}

// destroy(g) removes the listeners and the elements and releases every
// function.
func destroy(h *handle, _ []js.Value) js.Value {
	h.unmount()
	delete(state.handles, h.id)
	return result(true, nil)
}

// token reports whether s holds only letters, digits, '-' and '_', as
// the renderer writes a prefix.
func token(s string) bool {
	// bound: the string
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

// get reads a property, undefined when the value is not an object.
func get(v js.Value, key string) js.Value {
	if v.Type() != js.TypeObject {
		return js.Undefined()
	}
	return v.Get(key)
}

// str reads a string property, "" when absent.
func str(v js.Value, key string) string {
	p := get(v, key)
	if p.Type() != js.TypeString {
		return ""
	}
	return p.String()
}

// float reads a number property, 0 when absent or not finite, so that
// NaN and Infinity take the default as a missing number does.
func float(v js.Value, key string) float64 {
	p := get(v, key)
	if p.Type() != js.TypeNumber {
		return 0
	}
	f := p.Float()
	if f != f || f-f != 0 {
		return 0
	}
	return f
}

// count reads a whole-number property as float does, clamped to
// ±(2^31 − 1) before the conversion, which Go leaves to the target
// beyond the int's range; the core then refuses what is out of its own.
func count(v js.Value, key string) int {
	return int(math.Max(-math.MaxInt32, math.Min(math.MaxInt32, float(v, key))))
}
