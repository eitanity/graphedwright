// Package doc reads and writes a document as JSON: a hand-written
// encoder, and a bounded decoder that never recurses, refuses unknown
// keys with their path, and checks every size against the capacities
// before it allocates. The schema is named, not embedded; the host
// supplies the schemas it has.
package doc

import (
	"errors"
	"strconv"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

// Named is a schema the host supplies under a name.
type Named struct {
	Name   string
	Schema gw.Schema
}

// Options shape a decode: the schemas known, the capacities of the
// document made, and whether to lay out a document whose vertices lack
// positions (on by default; NoLayout turns it off).
type Options struct {
	Schemas  []Named
	Caps     gw.Caps
	NoLayout bool
}

// Decode reads a document. The text is refused beyond MaxInput before
// anything is parsed; every element goes through Apply, so the
// schema's rules and the capacities hold for a decoded document as for
// an edited one. The commands that built it are not in its undo log.
func Decode(data []byte, opts Options) (*gw.Document, error) {
	d, _, err := DecodeNamed(data, opts)
	return d, err
}

// DecodeNamed is Decode that also returns the name of the document's
// schema, which a host needs to encode it again.
func DecodeNamed(data []byte, opts Options) (*gw.Document, string, error) {
	if len(data) > MaxInput {
		return nil, "", errors.New("document of " + strconv.Itoa(len(data)) + " bytes exceeds the limit of " + strconv.Itoa(MaxInput))
	}
	caps := opts.Caps
	if caps.LabelBytes == 0 {
		caps.LabelBytes = gw.DefaultCaps().LabelBytes
	}
	s := &scanner{data: data, path: make([]byte, 0, 64), maxS: caps.LabelBytes}
	dec := &decoder{s: s, opts: opts}
	if err := dec.root(); err != nil {
		return nil, "", err
	}
	if dec.d == nil {
		return nil, "", errors.New("document has no schema")
	}
	if dec.unplaced && !opts.NoLayout {
		if _, err := layout.New(dec.d.Caps()).Run(dec.d, layout.Options{}); err != nil {
			return nil, "", err
		}
	}
	dec.d.ClearUndo()
	return dec.d, dec.name, nil
}

// decoder holds the document being built and what the root has seen.
type decoder struct {
	s        *scanner
	opts     Options
	d        *gw.Document
	name     string // the schema's
	unplaced bool
	seen     uint8 // bits for the root keys, each allowed once
	parents  []parentOf
}

// parentOf is a vertex's container, named before every vertex is read,
// and the path to report it at.
type parentOf struct {
	v    gw.VertexIndex
	id   string
	path string
}

// Root keys, as bits in seen.
const (
	sawSchema uint8 = 1 << iota
	sawVertices
	sawEdges
	sawView
	sawFlow
)

// root reads the top-level object. The schema must come before the
// vertices and edges, since they are validated against it.
func (dec *decoder) root() error {
	s := dec.s
	s.push("document")
	if err := s.open('{'); err != nil {
		return err
	}
	for s.peek() != '}' {
		k, err := s.key()
		if err != nil {
			return err
		}
		if err := dec.rootKey(k); err != nil {
			return err
		}
		more, err := s.more('}')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	if s.peek() != 0 {
		return s.errorf("text after the document")
	}
	return nil
}

func (dec *decoder) rootKey(k string) error {
	s := dec.s
	switch k {
	case "schema":
		if dec.seen&sawSchema != 0 {
			return s.errorf(`"schema" given twice`)
		}
		dec.seen |= sawSchema
		return dec.schema()
	case "vertices":
		if dec.seen&sawVertices != 0 {
			return s.errorf(`"vertices" given twice`)
		}
		dec.seen |= sawVertices
		if err := dec.vertices(); err != nil {
			return err
		}
		return dec.resolveParents()
	case "edges":
		if dec.seen&sawEdges != 0 {
			return s.errorf(`"edges" given twice`)
		}
		dec.seen |= sawEdges
		return dec.edges()
	case "view":
		if dec.seen&sawView != 0 {
			return s.errorf(`"view" given twice`)
		}
		dec.seen |= sawView
		return dec.view()
	case "flow":
		if dec.seen&sawFlow != 0 {
			return s.errorf(`"flow" given twice`)
		}
		dec.seen |= sawFlow
		return dec.flow()
	}
	return s.errorf("unknown key " + strconv.Quote(k))
}

// schema reads the schema's name and makes the document.
func (dec *decoder) schema() error {
	s := dec.s
	s.push(".schema")
	defer s.pop(len(".schema"))
	name, err := s.str()
	if err != nil {
		return err
	}
	for _, n := range dec.opts.Schemas {
		if n.Name == name {
			d, err := gw.New(n.Schema, dec.opts.Caps)
			if err != nil {
				return s.errorf(err.Error())
			}
			dec.d, dec.name = d, name
			return nil
		}
	}
	return s.errorf("schema " + strconv.Quote(name) + " not supplied")
}

// need refuses vertices or edges before the schema.
func (dec *decoder) need() error {
	if dec.d == nil {
		return dec.s.errorf(`"schema" must come first`)
	}
	return nil
}

// vertices reads the vertex array.
func (dec *decoder) vertices() error {
	if err := dec.need(); err != nil {
		return err
	}
	s := dec.s
	s.push(".vertices")
	defer s.pop(len(".vertices"))
	if err := s.open('['); err != nil {
		return err
	}
	for i := 0; s.peek() != ']'; i++ {
		seg := "[" + strconv.Itoa(i) + "]"
		s.push(seg)
		if err := dec.vertex(); err != nil {
			return err
		}
		s.pop(len(seg))
		more, err := s.more(']')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	return nil
}

// vertexFields gathers one vertex object before it is applied.
type vertexFields struct {
	cmd       gw.Command
	hasPos    bool
	placed    bool
	parent    string
	hasParent bool
}

// vertex reads one vertex object and applies it.
func (dec *decoder) vertex() error {
	s := dec.s
	if err := s.open('{'); err != nil {
		return err
	}
	f := vertexFields{cmd: gw.Command{Op: gw.AddVertex, Kind: gw.None}}
	var attrs []gw.Attr
	for s.peek() != '}' {
		k, err := s.key()
		if err != nil {
			return err
		}
		s.push("." + k)
		if attrs, err = dec.vertexKey(k, &f, attrs); err != nil {
			return err
		}
		s.pop(len(k) + 1)
		more, err := s.more('}')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	return dec.applyVertex(&f, attrs)
}

// applyVertex adds the gathered vertex, places it and sets its
// attributes.
func (dec *decoder) applyVertex(f *vertexFields, attrs []gw.Attr) error {
	s := dec.s
	if f.cmd.Kind == gw.None {
		return s.errorf(`"kind" missing`)
	}
	ch, err := dec.d.Apply(f.cmd)
	if err != nil {
		return s.errorf(err.Error())
	}
	v := ch.Vertices[0]
	if f.hasParent {
		dec.parents = append(dec.parents, parentOf{v: v, id: f.parent, path: string(s.path)})
	}
	if f.hasPos {
		dec.d.Place(v, f.cmd.Pos, f.placed)
	} else {
		dec.unplaced = true
	}
	for _, a := range attrs {
		if _, err := dec.d.Apply(gw.Command{Op: gw.SetAttr, Vertex: v, Key: a.Key, Value: a.Value}); err != nil {
			return s.errorf(err.Error())
		}
	}
	return nil
}

// vertexKey reads one key of a vertex object.
func (dec *decoder) vertexKey(k string, f *vertexFields, attrs []gw.Attr) ([]gw.Attr, error) {
	s := dec.s
	var err error
	switch k {
	case "id":
		var id string
		id, err = s.str()
		f.cmd.ID = gw.ID(id)
	case "kind":
		f.cmd.Kind, err = dec.vertexKind()
	case "label":
		f.cmd.Text, err = s.str()
	case "x":
		f.cmd.Pos.X, err = s.num()
		f.hasPos = true
	case "y":
		f.cmd.Pos.Y, err = s.num()
		f.hasPos = true
	case "w":
		f.cmd.Size.W, err = s.num()
	case "h":
		f.cmd.Size.H, err = s.num()
	case "placed":
		f.placed, err = s.boolean()
	case "parent":
		f.parent, err = s.str()
		f.hasParent = true
	case "attrs":
		return dec.attrs(attrs)
	default:
		err = s.errorf("unknown key " + strconv.Quote(k))
	}
	return attrs, err
}

// resolveParents puts each vertex that names a container in it, once
// every vertex is read, so that a container may come after what it
// holds.
func (dec *decoder) resolveParents() error {
	for _, p := range dec.parents {
		c := dec.d.VertexByID(gw.ID(p.id))
		if c == gw.None {
			return errors.New(p.path + ".parent: unknown vertex " + strconv.Quote(p.id))
		}
		if _, err := dec.d.Apply(gw.Command{Op: gw.SetParent, Vertex: p.v, To: c}); err != nil {
			return errors.New(p.path + ": " + err.Error())
		}
	}
	return nil
}

// vertexKind reads a vertex kind by name.
func (dec *decoder) vertexKind() (gw.KindIndex, error) {
	name, err := dec.s.str()
	if err != nil {
		return gw.None, err
	}
	for i, k := range dec.d.Schema.VertexKinds {
		if k.Name == name {
			return gw.KindIndex(i), nil //nolint:gosec // bounded by the schema
		}
	}
	return gw.None, dec.s.errorf("unknown vertex kind " + strconv.Quote(name))
}

// attrs reads an object of string values into the list, bounded by the
// capacity, the limit refused by Apply when exceeded.
func (dec *decoder) attrs(attrs []gw.Attr) ([]gw.Attr, error) {
	s := dec.s
	if err := s.open('{'); err != nil {
		return attrs, err
	}
	for s.peek() != '}' {
		k, err := s.key()
		if err != nil {
			return attrs, err
		}
		v, err := s.str()
		if err != nil {
			return attrs, err
		}
		if len(attrs) >= dec.d.Caps().AttrsPerElement {
			return attrs, s.errorf("more than " + strconv.Itoa(dec.d.Caps().AttrsPerElement) + " attributes")
		}
		attrs = append(attrs, gw.Attr{Key: k, Value: v})
		more, err := s.more('}')
		if err != nil {
			return attrs, err
		}
		if !more {
			break
		}
	}
	s.close()
	return attrs, nil
}

// edges reads the edge array.
func (dec *decoder) edges() error {
	if err := dec.need(); err != nil {
		return err
	}
	s := dec.s
	s.push(".edges")
	defer s.pop(len(".edges"))
	if err := s.open('['); err != nil {
		return err
	}
	for i := 0; s.peek() != ']'; i++ {
		seg := "[" + strconv.Itoa(i) + "]"
		s.push(seg)
		if err := dec.edge(); err != nil {
			return err
		}
		s.pop(len(seg))
		more, err := s.more(']')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	return nil
}

// edgeFields gathers one edge object; the ports are resolved once the
// ends are known.
type edgeFields struct {
	cmd              gw.Command
	fromPort, toPort string
	hasFromP, hasToP bool
	hasFrom, hasTo   bool
	hasKind          bool
	fromName, toName string
	kindName         string
	bends            []gw.Point
}

// edge reads one edge object and applies it.
func (dec *decoder) edge() error {
	s := dec.s
	if err := s.open('{'); err != nil {
		return err
	}
	f := edgeFields{cmd: gw.Command{Op: gw.AddEdge, Vertex: gw.None, FromPort: gw.None, ToPort: gw.None}}
	var attrs []gw.Attr
	for s.peek() != '}' {
		k, err := s.key()
		if err != nil {
			return err
		}
		s.push("." + k)
		if attrs, err = dec.edgeKey(k, &f, attrs); err != nil {
			return err
		}
		s.pop(len(k) + 1)
		more, err := s.more('}')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	if err := dec.resolveEdge(&f); err != nil {
		return err
	}
	ch, err := dec.d.Apply(f.cmd)
	if err != nil {
		return s.errorf(err.Error())
	}
	for _, a := range attrs {
		if _, err := dec.d.Apply(gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: ch.Edges[0], Key: a.Key, Value: a.Value}); err != nil {
			return s.errorf(err.Error())
		}
	}
	if len(f.bends) > 0 && !dec.d.SetBends(ch.Edges[0], f.bends) {
		return s.errorf("bends exceed the room left for them")
	}
	return nil
}

// edgeKey reads one key of an edge object.
func (dec *decoder) edgeKey(k string, f *edgeFields, attrs []gw.Attr) ([]gw.Attr, error) {
	s := dec.s
	var err error
	switch k {
	case "id":
		var id string
		id, err = s.str()
		f.cmd.ID = gw.ID(id)
	case "kind":
		f.kindName, err = s.str()
		f.hasKind = true
	case "from":
		f.fromName, err = s.str()
		f.hasFrom = true
	case "to":
		f.toName, err = s.str()
		f.hasTo = true
	case "from_port":
		f.fromPort, err = s.str()
		f.hasFromP = true
	case "to_port":
		f.toPort, err = s.str()
		f.hasToP = true
	case "label":
		f.cmd.Text, err = s.str()
	case "weight":
		f.cmd.Weight, err = s.num()
	case "bends":
		f.bends, err = dec.bends()
	case "attrs":
		return dec.attrs(attrs)
	default:
		err = s.errorf("unknown key " + strconv.Quote(k))
	}
	return attrs, err
}

// bends reads an edge's bends, an array of [x, y] pairs, no more than
// the document's bend slab holds.
func (dec *decoder) bends() ([]gw.Point, error) {
	s := dec.s
	caps := dec.d.Caps()
	limit := caps.Edges * caps.BendsPerEdge
	if err := s.open('['); err != nil {
		return nil, err
	}
	var pts []gw.Point
	for s.peek() != ']' {
		if len(pts) == limit {
			return nil, s.errorf("bends exceed the limit of " + strconv.Itoa(limit))
		}
		var p gw.Point
		if err := s.open('['); err != nil {
			return nil, err
		}
		var err error
		if p.X, err = s.num(); err != nil {
			return nil, err
		}
		if err := s.expect(','); err != nil {
			return nil, err
		}
		if p.Y, err = s.num(); err != nil {
			return nil, err
		}
		if err := s.expect(']'); err != nil {
			return nil, err
		}
		pts = append(pts, p)
		more, err := s.more(']')
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	s.close()
	return pts, nil
}

// resolveEdge turns the names of an edge's kind, ends and ports into
// indices.
func (dec *decoder) resolveEdge(f *edgeFields) error {
	s := dec.s
	if !f.hasKind || !f.hasFrom || !f.hasTo {
		return s.errorf(`"kind", "from" and "to" are required`)
	}
	f.cmd.Kind = gw.None
	for i, k := range dec.d.Schema.EdgeKinds {
		if k.Name == f.kindName {
			f.cmd.Kind = gw.KindIndex(i) //nolint:gosec // bounded by the schema
		}
	}
	if f.cmd.Kind == gw.None {
		return s.errorf("unknown edge kind " + strconv.Quote(f.kindName))
	}
	f.cmd.From = dec.d.VertexByID(gw.ID(f.fromName))
	f.cmd.To = dec.d.VertexByID(gw.ID(f.toName))
	if f.cmd.From == gw.None {
		return s.errorf("unknown vertex " + strconv.Quote(f.fromName))
	}
	if f.cmd.To == gw.None {
		return s.errorf("unknown vertex " + strconv.Quote(f.toName))
	}
	var err error
	if f.hasFromP {
		if f.cmd.FromPort, err = dec.port(f.cmd.From, f.fromPort); err != nil {
			return err
		}
	}
	if f.hasToP {
		f.cmd.ToPort, err = dec.port(f.cmd.To, f.toPort)
	}
	return err
}

// port finds a port of a vertex by name.
func (dec *decoder) port(v gw.VertexIndex, name string) (gw.PortIndex, error) {
	for i, p := range dec.d.Vertices[v].Ports {
		if p.Name == name {
			return gw.PortIndex(i), nil //nolint:gosec // bounded by the ports per vertex
		}
	}
	return gw.None, dec.s.errorf("vertex " + strconv.Quote(string(dec.d.Vertices[v].ID)) + " has no port " + strconv.Quote(name))
}

// flow reads the direction the layers run in.
func (dec *decoder) flow() error {
	if err := dec.need(); err != nil {
		return err
	}
	s := dec.s
	s.push(".flow")
	defer s.pop(len(".flow"))
	name, err := s.str()
	if err != nil {
		return err
	}
	f, ok := gw.FlowNamed(name)
	if !ok {
		return s.errorf("unknown flow " + strconv.Quote(name))
	}
	dec.d.Flow = f
	return nil
}

// viewRule is what is wrong with a viewport number, or "": a corner
// within gw.MaxCoord of the origin, a width and height not negative and
// at most gw.MaxCoord.
func viewRule(k string, v float64) string {
	if k == "w" || k == "h" {
		if !(v >= 0) || v > gw.MaxCoord {
			return k + " must not be negative and at most 1e9"
		}
		return ""
	}
	if !(v >= -gw.MaxCoord) || v > gw.MaxCoord {
		return k + " must be within 1e9 of the origin"
	}
	return ""
}

// view reads the viewport object.
func (dec *decoder) view() error {
	if err := dec.need(); err != nil {
		return err
	}
	s := dec.s
	s.push(".view")
	defer s.pop(len(".view"))
	if err := s.open('{'); err != nil {
		return err
	}
	for s.peek() != '}' {
		k, err := s.key()
		if err != nil {
			return err
		}
		var dst *float64
		switch k {
		case "x":
			dst = &dec.d.View.X
		case "y":
			dst = &dec.d.View.Y
		case "w":
			dst = &dec.d.View.W
		case "h":
			dst = &dec.d.View.H
		default:
			return s.errorf("unknown key " + strconv.Quote(k))
		}
		if *dst, err = s.num(); err != nil {
			return err
		}
		if msg := viewRule(k, *dst); msg != "" {
			return s.errorf(msg)
		}
		more, err := s.more('}')
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	s.close()
	return nil
}
