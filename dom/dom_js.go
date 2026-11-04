// Package dom is the browser binding: it mounts a document in a host
// element, forwards the element's pointer, wheel and keyboard events to
// the editor, patches the SVG from each change, and reports to the
// host. A host written in Go mounts an editor with Mount and hears from
// it through its Host; Install exposes the same to JavaScript as the
// graphedwright object. It is the only package that touches the
// browser, and it uses the part of syscall/js that both the Go and the
// TinyGo toolchains implement.
package dom

import (
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

// The callbacks a page can register, by index.
const (
	cbChange = iota
	cbSelect
	cbError
	cbEditLabel
	cbCount
)

// jsHost is the host of an editor the page created: it hears the
// handle as any host does and passes what it hears to the page's
// callbacks, and keeps what only the JavaScript object needs.
type jsHost struct {
	h       *Handle
	id      int // the handle the page knows it by
	cbs     [cbCount]js.Value
	schemas []doc.Named
	schema  string
}

// binding is the graphedwright object's state: the editors created and
// the schemas known. Install makes one, and only the object's functions
// reach it, so the package holds none.
type binding struct {
	handles map[int]*jsHost
	next    int
	schemas []doc.Named
}

// The graphedwright object's functions, by index.
const (
	opCreate = iota
	opRegisterSchema
	opLoad
	opExport
	opSVG
	opSetMode
	opLayout
	opAnnotate
	opOn
	opUndo
	opRedo
	opDestroy
	opProperties
	opSetLabel
	opSetAttr
	opAddPort
	opDeletePort
	opZoom
	opFit
	opCount
)

// call is one of the object's functions: the binding it works on and
// which function it is.
type call struct {
	b  *binding
	op int
}

// Install sets the graphedwright object on the page's global object.
// A page may name the built-in schemas and those given here in create,
// as well as any it registers itself.
func Install(schemas ...doc.Named) {
	b := &binding{handles: map[int]*jsHost{}, next: 1, schemas: append(doc.Builtin(), schemas...)}
	names := [opCount]string{
		opCreate: "create", opRegisterSchema: "registerSchema", opLoad: "load", opExport: "export", opSVG: "svg",
		opSetMode: "setMode", opLayout: "layout", opAnnotate: "annotate", opOn: "on", opUndo: "undo", opRedo: "redo",
		opDestroy: "destroy", opProperties: "properties", opSetLabel: "setLabel", opSetAttr: "setAttr",
		opAddPort: "addPort", opDeletePort: "deletePort", opZoom: "zoom", opFit: "fit",
	}
	obj := js.Global().Get("Object").New()
	for op, name := range names {
		obj.Set(name, js.FuncOf((&call{b: b, op: op}).invoke))
	}
	js.Global().Set("graphedwright", obj)
}

// Main installs the graphedwright object, with the schemas given, and
// keeps the program alive for its callbacks.
func Main(schemas ...doc.Named) {
	Install(schemas...)
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

// fail is a refusal with a message.
func fail(msg string) js.Value { return result(nil, errors.New(msg)) }

// invoke runs the function: create and registerSchema as they are, the
// rest on the editor the first argument names.
func (c *call) invoke(_ js.Value, args []js.Value) any {
	switch c.op {
	case opCreate:
		return c.b.create(args)
	case opRegisterSchema:
		return c.b.registerSchema(args)
	}
	if len(args) == 0 || args[0].Type() != js.TypeNumber {
		return fail("the first argument is the handle create returned")
	}
	j, ok := c.b.handles[args[0].Int()]
	if !ok {
		return fail("no such handle " + strconv.Itoa(args[0].Int()))
	}
	args = args[1:]
	switch c.op {
	case opLoad:
		return load(j, args)
	case opExport:
		return export(j, args)
	case opSVG:
		return svgText(j, args)
	case opSetMode:
		return setMode(j, args)
	case opLayout:
		return relayout(j, args)
	case opAnnotate:
		return annotate(j, args)
	case opOn:
		return on(j, args)
	case opUndo:
		return result(j.h.Undo(), nil)
	case opRedo:
		return result(j.h.Redo(), nil)
	case opDestroy:
		return c.b.destroy(j)
	case opProperties:
		return properties(j, args)
	case opSetLabel:
		return setLabel(j, args)
	case opSetAttr:
		return setAttr(j, args)
	case opAddPort:
		return addPort(j, args)
	case opDeletePort:
		return deletePort(j, args)
	case opZoom:
		return zoom(j, args)
	case opFit:
		return fit(j, args)
	}
	return fail("no such function")
}

// create(el, {schema, mode, caps, grid, patch, expand, fullText,
// idPrefix}) mounts an empty document of the schema in the element and
// returns the handle.
func (b *binding) create(args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("create(element, options)")
	}
	opts := js.Undefined()
	if len(args) > 1 {
		opts = args[1]
	}
	schemas, name, err := b.schemaOption(opts)
	if err != nil {
		return result(nil, err)
	}
	prefix := str(opts, "idPrefix")
	if !token(prefix) {
		return fail("idPrefix may hold only letters, digits, '-' and '_'")
	}
	d, err := doc.Decode([]byte(`{"schema":`+strconv.Quote(name)+`}`), doc.Options{Schemas: schemas, Caps: capsOption(opts)})
	if err != nil {
		return result(nil, err)
	}
	id := b.next
	// The renderer's own IDs are the handle's, so that two editors in a
	// page keep their arrowheads apart; the document's IDs take the
	// host's prefix, if it gives one. A hovered vertex's cut texts are
	// drawn in full in the overlay, and each cut text is written in
	// full after it for the stylesheet, unless the page turns either off.
	j := &jsHost{id: id, schemas: schemas, schema: name}
	h, err := Mount(args[0], d, Options{
		View: str(opts, "mode") == "view", Grid: float(opts, "grid"), NoPatch: off(opts, "patch"),
		NoExpand: off(opts, "expand"), NoFullText: off(opts, "fullText"),
		IDPrefix: "gw" + strconv.Itoa(id) + "-", DocIDPrefix: prefix,
	}, j)
	if err != nil {
		return result(nil, err)
	}
	j.h = h
	b.next++
	b.handles[id] = j
	return result(id, nil)
}

// registerSchema(name, schemaObject) compiles a schema described from
// JavaScript and adds it, or refuses it, naming the kind at fault.
func (b *binding) registerSchema(args []js.Value) any {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeObject {
		return fail("registerSchema(name, schema)")
	}
	k, err := kindsFromJS(args[1])
	if err != nil {
		return result(nil, err)
	}
	b.schemas = append(b.schemas, doc.Named{Name: args[0].String(), Kinds: k})
	return result(true, nil)
}

// schemaOption resolves the schema option: a name the binding knows,
// or an object describing one, which is then known under its name or
// "custom".
func (b *binding) schemaOption(opts js.Value) ([]doc.Named, string, error) {
	v := get(opts, "schema")
	switch v.Type() {
	case js.TypeString:
		for _, n := range b.schemas {
			if n.Name == v.String() {
				return b.schemas, v.String(), nil
			}
		}
		return nil, "", errors.New("schema " + strconv.Quote(v.String()) + " not known; registerSchema it or pass an object")
	case js.TypeObject:
		k, err := kindsFromJS(v)
		if err != nil {
			return nil, "", err
		}
		name := str(v, "name")
		if name == "" {
			name = "custom"
		}
		return append([]doc.Named{{Name: name, Kinds: k}}, b.schemas...), name, nil
	case js.TypeUndefined, js.TypeNull, js.TypeBoolean, js.TypeNumber, js.TypeSymbol, js.TypeFunction:
	}
	return b.schemas, "basic", nil
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
func load(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 {
		return fail("load(handle, document)")
	}
	text, err := textOf(args[0])
	if err != nil {
		return result(nil, err)
	}
	d, name, err := doc.DecodeNamed([]byte(text), doc.Options{Schemas: j.schemas, Caps: j.h.d.Caps()})
	if err != nil {
		return result(nil, err)
	}
	if err := j.h.Load(d); err != nil {
		return result(nil, err)
	}
	j.schema = name
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
func export(j *jsHost, _ []js.Value) js.Value {
	j.h.buf.Reset()
	if err := doc.Encode(j.h.d, j.schema, &j.h.buf); err != nil {
		return result(nil, err)
	}
	return result(js.Global().Get("JSON").Call("parse", j.h.buf.String()), nil)
}

// svg(g) returns the current drawing as text.
func svgText(j *jsHost, _ []js.Value) js.Value {
	j.h.buf.Reset()
	if err := j.h.SVG(&j.h.buf); err != nil {
		return result(nil, err)
	}
	return result(j.h.buf.String(), nil)
}

// setMode(g, "view"|"edit").
func setMode(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail(`setMode(handle, "view" or "edit")`)
	}
	m := edit.Edit
	switch args[0].String() {
	case "view":
		m = edit.View
	case "edit":
	default:
		return fail(`mode is "view" or "edit"`)
	}
	if err := j.h.SetMode(m); err != nil {
		return result(nil, err)
	}
	return result(true, nil)
}

// layout(g, {all, flow}) lays the document out, in the flow named
// ("left-to-right" or "top-to-bottom") or the document's own.
func relayout(j *jsHost, args []js.Value) js.Value {
	all := len(args) > 0 && get(args[0], "all").Truthy()
	flow := ""
	if len(args) > 0 {
		flow = str(args[0], "flow")
	}
	if err := j.h.refuse(); err != nil {
		return result(nil, err)
	}
	res, err := j.h.relayout(layout.Options{All: all}, flow)
	if err != nil {
		return result(nil, err)
	}
	out := js.Global().Get("Object").New()
	out.Set("layers", res.Layers)
	out.Set("crossings", res.Crossings)
	return result(out, nil)
}

// zoom(g, factor) zooms the view about its centre, in by a factor above
// 1 and out by one below; a factor that is not a positive number, or a
// zoom past the view's bounds, changes nothing.
func zoom(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeNumber {
		return fail("zoom(handle, factor)")
	}
	if err := j.h.Zoom(args[0].Float()); err != nil {
		return result(nil, err)
	}
	return result(true, nil)
}

// fit(g) sets the view to the drawing's bounds.
func fit(j *jsHost, _ []js.Value) js.Value {
	if err := j.h.Fit(); err != nil {
		return result(nil, err)
	}
	return result(true, nil)
}

// properties(g) describes the one selected vertex or edge for a
// properties panel, or is null: {id, element ("vertex" or "edge"), kind,
// label, from, to (an edge's), attrs: [{key, value, kind ("text",
// "number" or "choice"), choices, required, known, and min and max for
// a bounded number, either of which may be infinite}]}.
func properties(j *jsHost, _ []js.Value) js.Value {
	p, ok := j.h.ed.Properties()
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
		if a.Bounded {
			x.Set("min", a.Min)
			x.Set("max", a.Max)
		}
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
func setLabel(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail("setLabel(handle, text)")
	}
	if err := j.h.refuse(); err != nil {
		return result(false, err)
	}
	j.h.d.Group()
	ch, err := j.h.ed.SetLabel(args[0].String())
	if back := j.h.after(ch, err); err == nil {
		err = back
	}
	return result(err == nil, err)
}

// setAttr(g, key, value) sets an attribute of the one selected vertex or
// edge; an empty value removes it.
func setAttr(j *jsHost, args []js.Value) js.Value {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeString {
		return fail("setAttr(handle, key, value)")
	}
	if err := j.h.refuse(); err != nil {
		return result(false, err)
	}
	j.h.d.Group()
	ch, err := j.h.ed.SetAttr(args[0].String(), args[1].String())
	if back := j.h.after(ch, err); err == nil {
		err = back
	}
	return result(err == nil, err)
}

// addPort(g, {name, side, max}) gives the selected vertex a port of
// its own.
func addPort(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("addPort(handle, {name, side, max})")
	}
	side, err := sideOf(str(args[0], "side"))
	if err != nil {
		return result(false, err)
	}
	if err := j.h.refuse(); err != nil {
		return result(false, err)
	}
	j.h.d.Group()
	ch, err := j.h.ed.AddPort(gw.Port{Name: str(args[0], "name"), Side: side, Max: count(args[0], "max")})
	if back := j.h.after(ch, err); err == nil {
		err = back
	}
	return result(err == nil, err)
}

// deletePort(g, name) removes a port of the selected vertex's own.
func deletePort(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail("deletePort(handle, name)")
	}
	if err := j.h.refuse(); err != nil {
		return result(false, err)
	}
	j.h.d.Group()
	ch, err := j.h.ed.DeletePort(args[0].String())
	if back := j.h.after(ch, err); err == nil {
		err = back
	}
	return result(err == nil, err)
}

// annotate(g, {id: {text, class, weight}}) sets the figures drawn on
// the elements named, replacing the last ones.
func annotate(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("annotate(handle, {id: {text, class, weight}})")
	}
	h := j.h
	if err := h.refuse(); err != nil {
		return result(nil, err)
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
func on(j *jsHost, args []js.Value) js.Value {
	if len(args) < 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeFunction {
		return fail("on(handle, event, callback)")
	}
	switch args[0].String() {
	case "change":
		j.cbs[cbChange] = args[1]
	case "select":
		j.cbs[cbSelect] = args[1]
	case "error":
		j.cbs[cbError] = args[1]
	case "edit-label":
		j.cbs[cbEditLabel] = args[1]
	default:
		return fail(`events are "change", "select", "error" and "edit-label"`)
	}
	return result(true, nil)
}

// destroy(g) removes the listeners and the elements and releases every
// function.
func (b *binding) destroy(j *jsHost) js.Value {
	if err := j.h.Destroy(); err != nil {
		return result(nil, err)
	}
	delete(b.handles, j.id)
	return result(true, nil)
}

// Changed passes a change to the page's "change" callback.
func (j *jsHost) Changed(ch gw.Change) (gw.Command, bool) {
	if j.cbs[cbChange].Type() == js.TypeFunction {
		j.emit(cbChange, j.changeObject(ch))
	}
	return gw.Command{}, false
}

// Selected passes the selection to the page's "select" callback.
func (j *jsHost) Selected(sv []gw.VertexIndex, se []gw.EdgeIndex) {
	if j.cbs[cbSelect].Type() == js.TypeFunction {
		j.emit(cbSelect, j.selectionObject(sv, se))
	}
}

// Refused passes a refusal's text to the page's "error" callback.
func (j *jsHost) Refused(err error) { j.emit(cbError, err.Error()) }

// EditingLabel passes the element and its label to the page's
// "edit-label" callback.
func (j *jsHost) EditingLabel(v gw.VertexIndex, e gw.EdgeIndex, text string) {
	if j.cbs[cbEditLabel].Type() != js.TypeFunction {
		return
	}
	id := ""
	if v != gw.None {
		id = string(j.h.d.Vertices[v].ID)
	} else if e != gw.None {
		id = string(j.h.d.Edges[e].ID)
	}
	arg := js.Global().Get("Object").New()
	arg.Set("id", id)
	arg.Set("text", text)
	j.emit(cbEditLabel, arg)
}

// emit calls a page's callback as a microtask: after the call that
// caused it returns and before the next event. A callback that throws
// is reported by the browser and never reaches Go, where an exception
// would end the program, and a callback that calls back in finds the
// drawing finished.
func (j *jsHost) emit(which int, arg any) {
	cb := j.cbs[which]
	if cb.Type() != js.TypeFunction {
		return
	}
	js.Global().Call("queueMicrotask", cb.Call("bind", js.Null(), arg))
}

func (j *jsHost) changeObject(ch gw.Change) js.Value {
	out := js.Global().Get("Object").New()
	vs := js.Global().Get("Array").New()
	for _, v := range ch.Vertices {
		vs.Call("push", string(j.h.d.Vertices[v].ID))
	}
	es := js.Global().Get("Array").New()
	for _, e := range ch.Edges {
		es.Call("push", string(j.h.d.Edges[e].ID))
	}
	out.Set("vertices", vs)
	out.Set("edges", es)
	out.Set("layout", ch.Layout)
	return out
}

func (j *jsHost) selectionObject(sv []gw.VertexIndex, se []gw.EdgeIndex) js.Value {
	out := js.Global().Get("Object").New()
	vs := js.Global().Get("Array").New()
	for _, v := range sv {
		vs.Call("push", string(j.h.d.Vertices[v].ID))
	}
	es := js.Global().Get("Array").New()
	for _, e := range se {
		es.Call("push", string(j.h.d.Edges[e].ID))
	}
	out.Set("vertices", vs)
	out.Set("edges", es)
	return out
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

// off reports whether a boolean option is given as false.
func off(v js.Value, key string) bool {
	o := get(v, key)
	return o.Type() == js.TypeBoolean && !o.Bool()
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
