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

// Properties describes the one selected vertex or edge; ok is false
// when the selection is not exactly one. It allocates, for a panel to
// call on a selection change, not on every pointer move.
func (e *Editor) Properties() (Properties, bool) {
	v, x, ok := e.one()
	if !ok {
		return Properties{}, false
	}
	p := Properties{Vertex: v, Edge: x}
	var specs []gw.AttrSpec
	var attrs []gw.Attr
	if v != gw.None {
		vx := &e.d.Vertices[v]
		kind := e.d.Schema.VertexKinds[vx.Kind]
		p.ID, p.Kind, p.Label, specs, attrs = vx.ID, kind.Name, vx.Label, kind.Attrs, vx.Attrs
	} else {
		ed := &e.d.Edges[x]
		kind := e.d.Schema.EdgeKinds[ed.Kind]
		p.ID, p.Kind, p.Label, specs, attrs = ed.ID, kind.Name, ed.Label, kind.Attrs, ed.Attrs
		p.From, p.To = e.d.Vertices[ed.From].ID, e.d.Vertices[ed.To].ID
	}
	p.Attrs = make([]Property, 0, len(specs)+len(attrs))
	for _, s := range specs {
		p.Attrs = append(p.Attrs, Property{Key: s.Key, Value: value(attrs, s.Key), Kind: s.Kind, Choices: s.Choices, Required: s.Required, Known: true})
	}
	for _, a := range attrs {
		if !named(specs, a.Key) {
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

// value is the value of the attribute with the key, or "".
func value(attrs []gw.Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

// named reports whether the specifications name the key.
func named(specs []gw.AttrSpec, key string) bool {
	for _, s := range specs {
		if s.Key == key {
			return true
		}
	}
	return false
}
