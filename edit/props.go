package edit

import (
	"errors"

	gw "code.eitanity.com/graphedwright"
)

// Property is one attribute of the element a properties panel shows:
// its key and value, and what its kind's schema says it may be. An
// attribute the schema does not name is Text and not Known.
type Property struct {
	Key      string
	Value    string
	Kind     gw.AttrKind
	Choices  []string
	Required bool
	Known    bool
	Bounded  bool    // a number from Min to Max
	Min, Max float64 // the bounds of a bounded number
}

// Properties describe the one selected vertex or edge for a host's
// properties panel: what it is, its label, and its attributes, the
// kind's in the schema's order, then any others it carries.
type Properties struct {
	Vertex   gw.VertexIndex // None for an edge
	Edge     gw.EdgeIndex   // None for a vertex
	ID       gw.ID
	Kind     string
	Label    string
	From, To gw.ID // an edge's ends
	Attrs    []Property
}

// ErrNotOne is the refusal of a property edit without exactly one
// vertex or edge selected.
var ErrNotOne = errors.New("select one vertex or edge")

// ErrNotVertex is the error of a port command without one vertex
// selected.
var ErrNotVertex = errors.New("select one vertex")

// Properties describes the one selected vertex or edge; ok is false
// when the selection is not exactly one. It allocates, for a panel to
// call on a selection change, not on every pointer move.
func (e *Editor) Properties() (Properties, bool) {
	v, x, ok := e.one()
	if !ok {
		return Properties{}, false
	}
	p := Properties{Vertex: v, Edge: x}
	var specs gw.AttrSpecList
	var attrs gw.AttrList
	if v != gw.None {
		vx := e.d.Vertex(v)
		kind := e.d.Kinds().Vertex(vx.Kind())
		p.ID, p.Kind, p.Label, specs, attrs = vx.ID(), kind.Name(), vx.Label(), kind.Attrs(), vx.Attrs()
	} else {
		ed := e.d.Edge(x)
		kind := e.d.Kinds().Edge(ed.Kind())
		p.ID, p.Kind, p.Label, specs, attrs = ed.ID(), kind.Name(), ed.Label(), kind.Attrs(), ed.Attrs()
		p.From, p.To = e.d.Vertex(ed.From()).ID(), e.d.Vertex(ed.To()).ID()
	}
	p.Attrs = make([]Property, 0, specs.Len()+attrs.Len())
	// bound: the kind's attributes
	for i := range specs.Len() {
		s := specs.At(i)
		choices := make([]string, s.Choices().Len())
		for c := range choices {
			choices[c] = s.Choices().At(c)
		}
		if len(choices) == 0 {
			choices = nil
		}
		p.Attrs = append(p.Attrs, Property{Key: s.Key(), Value: attrs.Value(s.Key()), Kind: s.Kind(), Choices: choices, Required: s.Required(), Known: true, Bounded: s.Bounded(), Min: s.Min(), Max: s.Max()})
	}
	// bound: the element's attributes
	for i := range attrs.Len() {
		if a := attrs.At(i); !named(specs, a.Key) {
			p.Attrs = append(p.Attrs, Property{Key: a.Key, Value: a.Value, Kind: gw.Text})
		}
	}
	return p, true
}

// SetLabel relabels the one selected vertex or edge, as a command.
func (e *Editor) SetLabel(text string) (gw.Change, error) {
	v, x, ok := e.one()
	if !ok {
		return gw.Change{}, ErrNotOne
	}
	return e.command(gw.Command{Op: gw.Relabel, Vertex: v, Edge: x, Text: text})
}

// SetAttr sets an attribute of the one selected vertex or edge, as a
// command; an empty value removes it. The schema's refusal of a value
// comes back as the error.
func (e *Editor) SetAttr(key, value string) (gw.Change, error) {
	v, x, ok := e.one()
	if !ok {
		return gw.Change{}, ErrNotOne
	}
	return e.command(gw.Command{Op: gw.SetAttr, Vertex: v, Edge: x, Key: key, Value: value})
}

// AddPort gives the one selected vertex a port of its own.
func (e *Editor) AddPort(p gw.Port) (gw.Change, error) {
	v, _, ok := e.one()
	if !ok || v == gw.None {
		return gw.Change{}, ErrNotVertex
	}
	return e.command(gw.Command{Op: gw.AddPort, Vertex: v, Port: p})
}

// DeletePort removes the named port of the one selected vertex's own.
func (e *Editor) DeletePort(name string) (gw.Change, error) {
	v, _, ok := e.one()
	if !ok || v == gw.None {
		return gw.Change{}, ErrNotVertex
	}
	return e.command(gw.Command{Op: gw.DeletePort, Vertex: v, Text: name})
}

// one is the one selected vertex, or edge with Vertex None.
func (e *Editor) one() (gw.VertexIndex, gw.EdgeIndex, bool) {
	switch {
	case len(e.selV) == 1 && len(e.selE) == 0:
		return e.selV[0], gw.None, true
	case len(e.selE) == 1 && len(e.selV) == 0:
		return gw.None, e.selE[0], true
	}
	return gw.None, gw.None, false
}

// command applies a command outside an event and reports its change.
func (e *Editor) command(c gw.Command) (gw.Change, error) {
	e.changeV, e.changeE, e.layout = e.changeV[:0], e.changeE[:0], false
	err := e.apply(c)
	return gw.Change{Vertices: e.changeV, Edges: e.changeE, Layout: e.layout}, err
}

// named reports whether the specifications name the key.
func named(specs gw.AttrSpecList, key string) bool {
	// bound: the kind's attributes
	for i := range specs.Len() {
		if specs.At(i).Key() == key {
			return true
		}
	}
	return false
}
