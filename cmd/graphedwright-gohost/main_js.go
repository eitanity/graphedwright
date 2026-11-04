// Command graphedwright-gohost is the browser smoke test's host written
// in Go: it mounts an editor with dom.Mount, with no JavaScript in the
// page but the loader, drives it with the page's own events, and writes
// what it saw into the page as JSON for the driver (web/smoke.py) to
// check. It never installs the graphedwright object.
package main

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/dom"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/svg"
)

// example is the document mounted: two boxes and an arrow between them.
const example = `{"schema":"basic","vertices":[
 {"id":"a","kind":"box","label":"a","x":120,"y":100,"placed":true},
 {"id":"b","kind":"box","label":"b","x":360,"y":100,"placed":true}],
 "edges":[{"id":"e","kind":"arrow","from":"a","to":"b"}]}`

// three is example with a third box, below.
const three = `{"schema":"basic","vertices":[
 {"id":"a","kind":"box","label":"a","x":120,"y":100,"placed":true},
 {"id":"b","kind":"box","label":"b","x":360,"y":100,"placed":true},
 {"id":"c","kind":"box","label":"c","x":120,"y":240,"placed":true}],
 "edges":[{"id":"e","kind":"arrow","from":"a","to":"b"}]}`

// What the host's Changed does.
const (
	followNone   = iota // asks for nothing
	followOnce          // tries to call back in, annotates, and asks once for a relabel
	followAlways        // asks for a relabel every time
	followBad           // asks for a relabel, then for a command the document refuses
)

// host is the test's Host: it records what it hears.
type host struct {
	dom.NoHost
	h         *dom.Handle
	mode      int
	changed   int  // Changed calls
	moving    int  // Changed calls while a gesture was under way
	gesture   bool // a follow-up was asked for during one
	first     gw.VertexIndex
	reentered error // h.Apply from inside Changed
	locked    error // the document's Apply from inside Changed
	annotated error // h.Annotate from inside Changed
	refusals  []string
	selected  string // the last selection Selected was given, as the IDs it names
}

// Changed records the change and does what the mode says.
func (t *host) Changed(ch gw.Change) (gw.Command, bool) {
	t.changed++
	switch t.mode {
	case followOnce:
		if t.h.Editor().State() != edit.Idle {
			// A vertex following the pointer: asking now is refused.
			t.moving++
			t.gesture = true
			return gw.Command{Op: gw.Relabel, Vertex: ch.Vertices[0], Text: "mid-drag"}, true
		}
		if t.first != gw.None || len(ch.Vertices) == 0 {
			return gw.Command{}, false
		}
		t.first = ch.Vertices[0]
		_, t.reentered = t.h.Apply(gw.Command{Op: gw.Relabel, Vertex: t.first, Text: "again"})
		_, t.locked = t.h.Document().Apply(gw.Command{Op: gw.Relabel, Vertex: t.first, Text: "again"})
		notes := make([]svg.Annotation, t.h.Document().VertexSlots())
		notes[t.first] = svg.Annotation{Text: "seen by Go"}
		t.annotated = t.h.Annotate(notes, nil)
		return gw.Command{Op: gw.Relabel, Vertex: t.first, Text: "moved"}, true
	case followAlways:
		return gw.Command{Op: gw.Relabel, Vertex: 0, Text: "x" + strconv.Itoa(t.changed)}, true
	case followBad:
		if t.changed == 1 {
			return gw.Command{Op: gw.Relabel, Vertex: 0, Text: "good"}, true
		}
		if t.changed == 2 {
			return gw.Command{Op: gw.Relabel, Vertex: 9999, Text: "bad"}, true
		}
	}
	return gw.Command{}, false
}

// Selected records the selection's IDs, then writes over the copy it
// was given, which must change nothing in the editor.
func (t *host) Selected(sv []gw.VertexIndex, se []gw.EdgeIndex) {
	d := t.h.Document()
	t.selected = ""
	for i, v := range sv {
		t.selected += string(d.Vertex(v).ID())
		sv[i] = gw.None
	}
	for i, x := range se {
		t.selected += string(d.Edge(x).ID())
		se[i] = gw.None
	}
}

// Refused records a refusal.
func (t *host) Refused(err error) { t.refusals = append(t.refusals, err.Error()) }

// report is the checks made, in order.
type report struct{ b strings.Builder }

func (r *report) check(name string, ok bool, detail string) {
	if r.b.Len() > 0 {
		r.b.WriteString(",")
	}
	r.b.WriteString(`{"name":` + strconv.Quote(name) + `,"ok":` + strconv.FormatBool(ok) + `,"detail":` + strconv.Quote(detail) + `}`)
}

func main() {
	page := js.Global().Get("document")
	r := &report{}
	run(page, r)
	page.Call("getElementById", "out").Set("textContent", "["+r.b.String()+"]")
}

func run(page js.Value, r *report) {
	el := page.Call("getElementById", "host")
	d, err := doc.Decode([]byte(example), doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		r.check("decode", false, err.Error())
		return
	}
	t := &host{first: gw.None}
	_, e1 := dom.Mount(js.Null(), d, dom.Options{}, t)
	_, e2 := dom.Mount(el, nil, dom.Options{}, t)
	_, e3 := dom.Mount(el, d, dom.Options{}, nil)
	_, e4 := dom.Mount(el, d, dom.Options{IDPrefix: "no spaces"}, t)
	c := d.Caps()
	_, e5 := dom.Mount(el, d, dom.Options{MaxFollow: -1}, t)
	_, e6 := dom.Mount(el, d, dom.Options{MaxFollow: c.Vertices + c.Edges + 1}, t)
	r.check("mount refuses a missing element, document or host, a bad prefix, and a follow-up bound out of range", e1 != nil && e2 != nil && e3 != nil && e4 != nil && e5 != nil && e6 != nil, "")
	h, err := dom.Mount(el, d, dom.Options{IDPrefix: "go-"}, t)
	if err != nil {
		r.check("mount", false, err.Error())
		return
	}
	t.h = h
	r.check("no graphedwright object", js.Global().Get("graphedwright").IsUndefined(), "")
	a := d.VertexByID("a")
	r.check("mounted", el.Call("querySelectorAll", "svg g.vertex").Length() == 2, el.Get("innerHTML").String())

	// A drag of a by pointer: one change, reported with a; a call back
	// in refused, the document locked, an annotation taken, and the
	// relabel asked for applied as a tail call.
	t.mode = followOnce
	before := d.Vertex(a).Pos()
	drag(el, "a", 80)
	moved := d.Vertex(a).Pos()
	r.check("a drag reaches Changed with the vertex", t.first == a && moved != before, "pos "+num(moved.X)+" from "+num(before.X))
	r.check("a call back in is refused", errors.Is(t.reentered, dom.ErrReentered), errText(t.reentered))
	r.check("the document is locked in Changed", errors.Is(t.locked, gw.ErrLocked), errText(t.locked))
	r.check("annotate works in Changed and is drawn", t.annotated == nil && strings.Contains(el.Get("textContent").String(), "seen by Go"), errText(t.annotated))
	r.check("the drag's progress is reported, and a follow-up during it refused", t.moving == 2 && t.gesture && len(t.refusals) == 2 && strings.Contains(t.refusals[0], "gesture"), "moving "+strconv.Itoa(t.moving)+", refusals "+strings.Join(t.refusals, "; "))
	r.check("the follow-up is applied, drawn and reported", d.Vertex(a).Label() == "moved" && t.changed == t.moving+2 && strings.Contains(el.Get("textContent").String(), "moved"), "label "+d.Vertex(a).Label()+", changes "+strconv.Itoa(t.changed))
	r.check("no lock left", !d.Locked(), "")

	// Ctrl+Z from the keyboard takes back the drag and its follow-up.
	t.mode = followNone
	el.Call("dispatchEvent", js.Global().Get("KeyboardEvent").New("keydown", obj(map[string]any{"key": "z", "ctrlKey": true, "bubbles": true})))
	r.check("one undo takes back the edit and its follow-up", d.Vertex(a).Pos() == before && d.Vertex(a).Label() == "a", "label "+d.Vertex(a).Label()+" pos "+num(d.Vertex(a).Pos().X))
	chains(page, el, h, t, a, r)
	lifecycle(el, h, t, r)
}

// chains checks follow-ups that cannot be completed: past the bound, or
// refused by the document, each takes back its edit; and a host's own
// bound is the one kept.
func chains(page, el js.Value, h *dom.Handle, t *host, a gw.VertexIndex, r *report) {
	d := h.Document()
	// A host that always asks for more: past the bound, the edit and
	// every follow-up are taken back, and the take-back is reported.
	t.mode, t.changed, t.refusals = followAlways, 0, nil
	_, err := h.Apply(gw.Command{Op: gw.Relabel, Vertex: a, Text: "start"})
	r.check("a chain past the bound is taken back with its edit", strings.Contains(errText(err), "taken back") && t.changed == dom.DefaultMaxFollow+2 && len(t.refusals) == 1 && d.Vertex(a).Label() == "a", "changes "+strconv.Itoa(t.changed)+", refusals "+strings.Join(t.refusals, "; ")+", label "+d.Vertex(a).Label()+", "+errText(err))
	drawn := el.Call("querySelector", "#a").Get("textContent").String()
	r.check("nothing of it is drawn or left to undo or redo", !d.CanUndo() && !d.CanRedo() && drawn == "aseen by Go", "a drawn as "+strconv.Quote(drawn))

	// A follow-up the document refuses takes back the edit and the
	// follow-ups before it, and the refusal names both.
	t.mode, t.changed, t.refusals = followBad, 0, nil
	_, err = h.Apply(gw.Command{Op: gw.Relabel, Vertex: a, Text: "edit"})
	r.check("a refused follow-up takes back its edit", len(t.refusals) == 1 && strings.Contains(t.refusals[0], "refused, so the edit was taken back") && strings.Contains(t.refusals[0], "9999") && err != nil && d.Vertex(a).Label() == "a" && !d.CanUndo(), "refusals "+strings.Join(t.refusals, "; ")+", label "+d.Vertex(a).Label())
	t.mode = followNone

	// A host's own bound.
	el2 := page.Call("createElement", "div")
	el2.Get("style").Set("cssText", "width:300px;height:200px")
	page.Get("body").Call("appendChild", el2)
	d3, _ := doc.Decode([]byte(example), doc.Options{Schemas: doc.Builtin()})
	t3 := &host{first: gw.None, mode: followAlways}
	h3, err := dom.Mount(el2, d3, dom.Options{IDPrefix: "go3-", MaxFollow: 2}, t3)
	if err == nil {
		t3.h = h3
		_, err = h3.Apply(gw.Command{Op: gw.Relabel, Vertex: 0, Text: "start"})
		_ = h3.Destroy()
	}
	r.check("a host's own bound is the one kept", strings.Contains(errText(err), "taken back") && t3.changed == 2+2, "changes "+strconv.Itoa(t3.changed)+", "+errText(err))
}

// lifecycle checks Apply from Go, Load and Destroy.
func lifecycle(el js.Value, h *dom.Handle, t *host, r *report) {
	// A vertex added from Go is drawn, and undone.
	_, err := h.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "from Go"})
	n := el.Call("querySelectorAll", "svg g.vertex").Length()
	h.Undo()
	r.check("Apply from Go is drawn and undone", err == nil && n == 3 && el.Call("querySelectorAll", "svg g.vertex").Length() == 2, "vertices "+strconv.Itoa(n)+" "+errText(err))

	// Load keeps the mode.
	d2, _ := doc.Decode([]byte(example), doc.Options{Schemas: doc.Builtin()})
	errMode := h.SetMode(edit.View)
	errLoad := h.Load(d2)
	r.check("Load keeps the mode", errMode == nil && errLoad == nil && h.Editor().Mode() == edit.View && h.Document() == d2, errText(errLoad))

	// Destroy empties the element, and later calls are refused.
	errDestroy := h.Destroy()
	_, errAfter := h.Apply(gw.Command{Op: gw.AddVertex, Kind: 0})
	r.check("Destroy empties the element and refuses later calls", errDestroy == nil && el.Get("innerHTML").String() == "" && errors.Is(errAfter, dom.ErrDestroyed) && errors.Is(h.Destroy(), dom.ErrDestroyed), errText(errAfter))

	// Mounted with room for two vertices, then given one with three: a
	// marquee over all three gives the host all of them, in a copy it
	// may write over.
	small, errSmall := doc.Decode([]byte(example), doc.Options{Schemas: doc.Builtin(), Caps: gw.Caps{Vertices: 2, Edges: 1}})
	big, errBig := doc.Decode([]byte(three), doc.Options{Schemas: doc.Builtin()})
	t2 := &host{first: gw.None}
	h2, errMount := dom.Mount(el, small, dom.Options{}, t2)
	if errMount != nil {
		r.check("mount a small document", false, errText(errors.Join(errMount, errSmall, errBig)))
		return
	}
	t2.h = h2
	errSmall = errors.Join(errSmall, errBig, h2.Load(big))
	marquee(el)
	sv, se := h2.Editor().Selected(make([]gw.VertexIndex, 0, 4), make([]gw.EdgeIndex, 0, 4))
	r.check("the host is given the whole selection after Load, in a copy of its own", errSmall == nil && t2.selected == "abce" && len(sv) == 3 && len(se) == 1 && sv[0] != gw.None && se[0] != gw.None, "host given "+strconv.Quote(t2.selected)+", editor holds "+strconv.Itoa(len(sv))+" "+errText(errSmall))
	_ = h2.Destroy()

	// An empty document loaded is shown one unit to a pixel, so that a
	// box added to it is drawn at its size, not scaled to fill.
	empty, errEmpty := doc.Decode([]byte(`{"schema":"basic"}`), doc.Options{Schemas: doc.Builtin()})
	full, errFull := doc.Decode([]byte(example), doc.Options{Schemas: doc.Builtin()})
	h4, errMount4 := dom.Mount(el, full, dom.Options{}, &host{first: gw.None})
	if err := errors.Join(errEmpty, errFull, errMount4); err != nil {
		r.check("mount a document", false, err.Error())
		return
	}
	errLoad4 := h4.Load(empty)
	_, errAdd := h4.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "box", Pos: gw.Point{X: 200, Y: 100}})
	scale := 0.0
	if s := el.Call("querySelector", "svg"); !s.IsNull() && el.Call("querySelector", "svg g.vertex").Truthy() {
		scale = s.Call("getScreenCTM").Get("a").Float()
	}
	errAdd = errors.Join(errLoad4, errAdd)
	r.check("an empty document loaded is shown one unit to a pixel", errAdd == nil && math.Abs(scale-1) < 0.01, "pixels to a unit "+num(scale)+" "+errText(errAdd))
	_ = h4.Destroy()
}

// marquee drags the main button from one corner of the drawing to the
// other.
func marquee(el js.Value) {
	s := el.Call("querySelector", "svg")
	box := s.Call("getBoundingClientRect")
	l, t := box.Get("left").Float()+2, box.Get("top").Float()+2
	r, b := box.Get("right").Float()-2, box.Get("bottom").Float()-2
	s.Call("dispatchEvent", pointerAt("pointerdown", l, t, 8))
	js.Global().Call("dispatchEvent", pointerAt("pointermove", (l+r)/2, (t+b)/2, 8))
	js.Global().Call("dispatchEvent", pointerAt("pointermove", r, b, 8))
	js.Global().Call("dispatchEvent", pointerAt("pointerup", r, b, 8))
}

// drag presses the main button on the element with the document id,
// moves dx pixels right and releases.
func drag(el js.Value, id string, dx float64) {
	v := el.Call("querySelector", "#"+js.Global().Get("CSS").Call("escape", id).String())
	box := v.Call("getBoundingClientRect")
	x, y := box.Get("left").Float()+box.Get("width").Float()/2, box.Get("top").Float()+box.Get("height").Float()/2
	v.Call("dispatchEvent", pointerAt("pointerdown", x, y, 7))
	js.Global().Call("dispatchEvent", pointerAt("pointermove", x+dx/2, y, 7))
	js.Global().Call("dispatchEvent", pointerAt("pointermove", x+dx, y, 7))
	js.Global().Call("dispatchEvent", pointerAt("pointerup", x+dx, y, 7))
}

// pointerAt is a pointer event of the main button, held, at the point.
func pointerAt(kind string, x, y float64, id int) js.Value {
	return js.Global().Get("PointerEvent").New(kind, obj(map[string]any{"clientX": x, "clientY": y, "pointerId": id, "button": 0, "buttons": 1, "isPrimary": true, "bubbles": true}))
}

// obj is a plain object with the fields given.
func obj(fields map[string]any) js.Value {
	o := js.Global().Get("Object").New()
	for k, v := range fields { // unordered: each field is set on its own
		o.Set(k, v)
	}
	return o
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
