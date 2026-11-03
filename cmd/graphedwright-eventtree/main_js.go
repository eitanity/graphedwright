// Command graphedwright-eventtree is an example application written in
// Go: an editor of event trees, the diagrams that follow an initiating
// event through the events after it to its outcomes, each branch with
// the probability that it is taken.
//
// It uses graphedwright as any application would, through dom.Mount and
// a Host, and shows the patterns a host is built from:
//
//   - its own schema: events, outcomes that end a path, and branches
//     that carry a probability p;
//   - a figure computed from the document on every change and drawn
//     with Annotate: the probability of reaching each vertex;
//   - a rule kept with follow-up commands: a new branch splits its
//     event evenly, one SetAttr at a time, all undone in one step with
//     the edit that caused them;
//   - a check that flags, rather than changes: an event whose branches
//     do not sum to 1;
//   - its own interface around the drawing: a panel for the selection,
//     a status line for refusals, and Open and Save.
//
// Its page (web/eventtree.html) holds no script but the loader.
package main

import (
	"strconv"
	"strings"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/dom"
	"code.eitanity.com/graphedwright/svg"
)

// The kinds of the schema.
const (
	kEvent gw.KindIndex = iota
	kOutcome
)

const (
	schemaName = "event-tree"
	keyP       = "p" // a branch's probability
	tolerance  = 1e-6
)

// The document's capacities, and so the app's: every buffer below is
// made once at these sizes. The split of an event with n branches takes
// n commands after the edit, so the follow-up bound is the most
// branches a document can hold, and the undo log, which takes the edit
// and its split back as one step, is deep enough for one of those and
// more.
const (
	maxVertices = 256
	maxEdges    = 512
	undoDepth   = 2 * maxEdges
)

func schema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "event", Shape: gw.RoundRect, Size: gw.Size{W: 130, H: 40}},
			{Name: "outcome", Shape: gw.Rect, Size: gw.Size{W: 130, H: 40}, Leaf: true},
		},
		EdgeKinds: []gw.EdgeKind{{
			Name: "branch", From: []gw.KindIndex{kEvent}, To: []gw.KindIndex{kEvent, kOutcome}, Arrow: true,
			Attrs: []gw.AttrSpec{{Key: keyP, Kind: gw.Number}},
		}},
		Acyclic: true,
	}
}

// example is the tree the app opens with. Its "contained" event's
// branches sum to 0.9, which the app flags.
const example = `{"schema":"event-tree","vertices":[
 {"id":"fire","kind":"event","label":"fire starts"},
 {"id":"detected","kind":"event","label":"detected"},
 {"id":"contained","kind":"event","label":"contained"},
 {"id":"minor","kind":"outcome","label":"minor damage"},
 {"id":"major","kind":"outcome","label":"major damage"},
 {"id":"loss","kind":"outcome","label":"total loss"}],
 "edges":[
 {"id":"b1","kind":"branch","from":"fire","to":"detected","attrs":{"p":"0.8"}},
 {"id":"b2","kind":"branch","from":"fire","to":"loss","attrs":{"p":"0.2"}},
 {"id":"b3","kind":"branch","from":"detected","to":"contained","attrs":{"p":"0.5"}},
 {"id":"b4","kind":"branch","from":"detected","to":"major","attrs":{"p":"0.5"}},
 {"id":"b5","kind":"branch","from":"contained","to":"minor","attrs":{"p":"0.6"}},
 {"id":"b6","kind":"branch","from":"contained","to":"major","attrs":{"p":"0.3"}}]}`

// app is the host: it holds the handle, the page's own elements and the
// buffers the figures are computed in.
type app struct {
	dom.NoHost
	h                   *dom.Handle
	panel, status, file js.Value

	prob   []float64        // by vertex: the probability of reaching it
	indeg  []int            // by vertex: in edges not yet walked
	order  []gw.VertexIndex // the walk's queue
	notesV []svg.Annotation
	notesE []svg.Annotation
	num    []byte // scratch for formatting
}

func main() {
	page := js.Global().Get("document")
	a := &app{
		panel:  page.Call("getElementById", "panel"),
		status: page.Call("getElementById", "status"),
		file:   page.Call("getElementById", "file"),
		prob:   make([]float64, maxVertices),
		indeg:  make([]int, maxVertices),
		order:  make([]gw.VertexIndex, 0, maxVertices),
		notesV: make([]svg.Annotation, maxVertices),
		notesE: make([]svg.Annotation, maxEdges),
	}
	d, err := decode(example)
	if err != nil {
		a.say(err)
		return
	}
	h, err := dom.Mount(page.Call("getElementById", "drawing"), d, dom.Options{IDPrefix: "et-", MaxFollow: maxEdges}, a)
	if err != nil {
		a.say(err)
		return
	}
	a.h = h
	a.figures()
	a.Selected(nil, nil)
	button(page, "open", a.open)
	button(page, "save", a.save)
	select {}
}

func decode(text string) (*gw.Document, error) {
	return doc.Decode([]byte(text), doc.Options{Schemas: []doc.Named{{Name: schemaName, Schema: schema()}}, Caps: gw.Caps{Vertices: maxVertices, Edges: maxEdges, UndoDepth: undoDepth}})
}

// button calls f when the page's button is clicked.
func button(page js.Value, id string, f func()) {
	page.Call("getElementById", id).Call("addEventListener", "click", js.FuncOf(func(js.Value, []js.Value) any {
		f()
		return nil
	}))
}

// Changed keeps the rule: while an event has a branch with no p, its
// branches are set to an even split, one command per call, which the
// handle applies in the same undo step as the edit. Once the rule
// holds, it draws the figures again, once for the edit and its split.
func (a *app) Changed(gw.Change) (gw.Command, bool) {
	if c, ok := a.split(); ok {
		return c, true
	}
	a.figures()
	return gw.Command{}, false
}

// Selected shows the one selected vertex's probability and branches.
func (a *app) Selected(sv []gw.VertexIndex, _ []gw.EdgeIndex) {
	if len(sv) != 1 {
		a.panel.Set("textContent", "Select an event or an outcome to see its probability.")
		return
	}
	d, v := a.h.Document(), sv[0]
	var b strings.Builder
	b.WriteString(label(d, v) + ": P = " + a.fmt(a.prob[v]))
	for e := d.FirstOut(v); e != gw.None; e = d.NextOut(e) {
		p, _ := branchP(d, e)
		b.WriteString("\n  → " + label(d, d.Edges[e].To) + ", p = " + a.fmt(p))
	}
	a.panel.Set("textContent", b.String())
}

// Refused shows why an edit was not made.
func (a *app) Refused(err error) { a.say(err) }

func (a *app) say(err error) { a.status.Set("textContent", err.Error()) }

// figures computes the probability of reaching each vertex, walking the
// tree from its roots in topological order, and draws it on every
// vertex, with each branch's p on its edge. An event whose branches do
// not sum to 1 is flagged.
func (a *app) figures() {
	a.walk()
	a.draw()
}

// walk computes each vertex's probability.
func (a *app) walk() {
	d := a.h.Document()
	n := len(d.Vertices)
	a.order = a.order[:0]
	for v := range n {
		a.prob[v], a.indeg[v] = 0, 0
	}
	for e := range d.Edges {
		if d.Edges[e].Live {
			a.indeg[d.Edges[e].To]++
		}
	}
	for v := range n {
		if d.Vertices[v].Live && a.indeg[v] == 0 {
			a.prob[v] = 1
			a.order = append(a.order, gw.VertexIndex(v)) //nolint:gosec // bounded by the capacity
		}
	}
	// The schema keeps the graph acyclic, so each vertex is queued once.
	for i := 0; i < len(a.order); i++ {
		v := a.order[i]
		for e := d.FirstOut(v); e != gw.None; e = d.NextOut(e) {
			p, _ := branchP(d, e)
			to := d.Edges[e].To
			a.prob[to] += a.prob[v] * p
			if a.indeg[to]--; a.indeg[to] == 0 {
				a.order = append(a.order, to)
			}
		}
	}
}

// draw annotates each vertex with its probability, flagging an event
// whose branches do not sum to 1, and each branch with its p.
func (a *app) draw() {
	d := a.h.Document()
	n := len(d.Vertices)
	clear(a.notesV)
	clear(a.notesE)
	for v := range n {
		if !d.Vertices[v].Live {
			continue
		}
		a.notesV[v] = svg.Annotation{Text: "P = " + a.fmt(a.prob[v])}
		if sum, out := sumOut(d, gw.VertexIndex(v)); out > 0 && (sum < 1-tolerance || sum > 1+tolerance) { //nolint:gosec // bounded by the capacity
			a.notesV[v] = svg.Annotation{Text: "P = " + a.fmt(a.prob[v]) + ", branches sum to " + a.fmt(sum), Class: "unbalanced"}
		}
	}
	for e := range d.Edges {
		if d.Edges[e].Live {
			if p, ok := branchP(d, gw.EdgeIndex(e)); ok { //nolint:gosec // bounded by the capacity
				a.notesE[e] = svg.Annotation{Text: "p = " + a.fmt(p)}
			}
		}
	}
	if err := a.h.Annotate(a.notesV[:n], a.notesE[:len(d.Edges)]); err != nil {
		a.say(err)
	}
}

// split is the next command of the even split, if an event has a branch
// with no p: first a sibling whose p is not the share, then the branch
// itself, so that the rule holds until the last command and stops there.
func (a *app) split() (gw.Command, bool) {
	d := a.h.Document()
	for e := range d.Edges {
		if !d.Edges[e].Live {
			continue
		}
		if _, ok := branchP(d, gw.EdgeIndex(e)); ok { //nolint:gosec // bounded by the capacity
			continue
		}
		from := d.Edges[e].From
		_, out := sumOut(d, from)
		share := strconv.FormatFloat(1/float64(out), 'g', -1, 64) // in full, so that the shares sum to 1
		for s := d.FirstOut(from); s != gw.None; s = d.NextOut(s) {
			if v, ok := attr(d, s); ok && v != share {
				return gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: s, Key: keyP, Value: share}, true
			}
		}
		return gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: gw.EdgeIndex(e), Key: keyP, Value: share}, true //nolint:gosec // bounded by the capacity
	}
	return gw.Command{}, false
}

// open replaces the document with the JSON in the file box.
func (a *app) open() {
	d, err := decode(a.file.Get("value").String())
	if err != nil {
		a.say(err)
		return
	}
	if err := a.h.Load(d); err != nil {
		a.say(err)
		return
	}
	a.status.Set("textContent", "")
	a.figures()
	a.Selected(nil, nil)
}

// save writes the document as JSON into the file box.
func (a *app) save() {
	var b strings.Builder
	if err := doc.Encode(a.h.Document(), schemaName, &b); err != nil {
		a.say(err)
		return
	}
	a.file.Set("value", b.String())
}

// sumOut is the sum of p over the vertex's branches, and how many it has.
func sumOut(d *gw.Document, v gw.VertexIndex) (sum float64, n int) {
	for e := d.FirstOut(v); e != gw.None; e = d.NextOut(e) {
		p, _ := branchP(d, e)
		sum += p
		n++
	}
	return sum, n
}

// branchP is the branch's p, and whether it has one.
func branchP(d *gw.Document, e gw.EdgeIndex) (float64, bool) {
	v, ok := attr(d, e)
	if !ok {
		return 0, false
	}
	p, err := strconv.ParseFloat(v, 64)
	return p, err == nil
}

func attr(d *gw.Document, e gw.EdgeIndex) (string, bool) {
	for _, at := range d.Edges[e].Attrs {
		if at.Key == keyP {
			return at.Value, true
		}
	}
	return "", false
}

func label(d *gw.Document, v gw.VertexIndex) string {
	if l := d.Vertices[v].Label; l != "" {
		return l
	}
	return string(d.Vertices[v].ID)
}

// fmt formats a probability to four significant figures.
func (a *app) fmt(p float64) string {
	a.num = strconv.AppendFloat(a.num[:0], p, 'g', 4, 64)
	return string(a.num)
}
