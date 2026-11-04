package graphedwright

import "strconv"

// Kinds is a schema compiled: checked once, and copied into storage it
// alone holds, so that nothing can change it afterwards. Any number of
// documents share one. Its kinds are read by index, through values,
// and their lists by position.
type Kinds struct {
	vertex    []VertexKind
	edge      []EdgeKind
	acyclic   bool
	connected bool
}

// Compile checks a schema and copies it. It refuses a schema without a
// vertex kind, with a name twice or a kind or port out of range, or
// with an attribute whose default does not fit it, naming the kind.
// The checks that depend on a document's capacities are New's.
func Compile(s Schema) (*Kinds, error) {
	if err := checkSchema(s); err != nil {
		return nil, err
	}
	k := &Kinds{vertex: make([]VertexKind, len(s.VertexKinds)), edge: make([]EdgeKind, len(s.EdgeKinds)), acyclic: s.Acyclic, connected: s.Connected}
	for i, v := range s.VertexKinds {
		v.Ports = clonePorts(v.Ports)
		v.Attrs = cloneSpecs(v.Attrs)
		v.Contains = cloneKinds(v.Contains)
		v.Compartments = cloneStrings(v.Compartments)
		k.vertex[i] = v
	}
	for i, e := range s.EdgeKinds {
		e.From, e.To = cloneKinds(e.From), cloneKinds(e.To)
		e.Attrs = cloneSpecs(e.Attrs)
		k.edge[i] = e
	}
	return k, nil
}

// clonePorts is a copy of ports, nil for none.
func clonePorts(s []Port) []Port {
	if len(s) == 0 {
		return nil
	}
	return append([]Port(nil), s...)
}

// cloneKinds is a copy of kind indices, nil for none.
func cloneKinds(s []KindIndex) []KindIndex {
	if len(s) == 0 {
		return nil
	}
	return append([]KindIndex(nil), s...)
}

// cloneStrings is a copy of strings, nil for none.
func cloneStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return append([]string(nil), s...)
}

// cloneSpecs is a copy of attribute specifications and their choices,
// nil for none.
func cloneSpecs(specs []AttrSpec) []AttrSpec {
	if len(specs) == 0 {
		return nil
	}
	specs = append([]AttrSpec(nil), specs...)
	for i := range specs {
		specs[i].Choices = cloneStrings(specs[i].Choices)
	}
	return specs
}

// fits refuses a kind with more ports or attributes than the
// capacities allow.
func (k *Kinds) fits(caps Caps) error {
	for _, v := range k.vertex {
		where := "vertex kind " + strconv.Quote(v.Name)
		if len(v.Ports) > caps.PortsPerVertex {
			return schemaError(where, itoa(len(v.Ports))+" ports exceed the limit of "+itoa(caps.PortsPerVertex))
		}
		if len(v.Attrs) > caps.AttrsPerElement {
			return schemaError(where, itoa(len(v.Attrs))+" attributes exceed the limit of "+itoa(caps.AttrsPerElement))
		}
	}
	for _, e := range k.edge {
		if len(e.Attrs) > caps.AttrsPerElement {
			return schemaError("edge kind "+strconv.Quote(e.Name), itoa(len(e.Attrs))+" attributes exceed the limit of "+itoa(caps.AttrsPerElement))
		}
	}
	return nil
}

// VertexKinds is the number of vertex kinds.
func (k *Kinds) VertexKinds() int {
	if k == nil {
		return 0
	}
	return len(k.vertex)
}

// EdgeKinds is the number of edge kinds.
func (k *Kinds) EdgeKinds() int {
	if k == nil {
		return 0
	}
	return len(k.edge)
}

// Acyclic reports whether the graph must stay a DAG.
func (k *Kinds) Acyclic() bool { return k != nil && k.acyclic }

// Connected reports whether every vertex should be reachable from a
// root kind: a warning, not a refusal.
func (k *Kinds) Connected() bool { return k != nil && k.connected }

// Vertex is the vertex kind at the index; out of range, every read of
// it is the zero value.
func (k *Kinds) Vertex(i KindIndex) VertexKindView {
	if k == nil || i < 0 || int(i) >= len(k.vertex) {
		return VertexKindView{}
	}
	return VertexKindView{&k.vertex[i]}
}

// Edge is the edge kind at the index; out of range, every read of it is
// the zero value.
func (k *Kinds) Edge(i KindIndex) EdgeKindView {
	if k == nil || i < 0 || int(i) >= len(k.edge) {
		return EdgeKindView{}
	}
	return EdgeKindView{&k.edge[i]}
}

// VertexNamed is the index of the vertex kind with the name, or None.
func (k *Kinds) VertexNamed(name string) KindIndex {
	// bound: the kinds, at most their number
	for i := range k.VertexKinds() {
		if k.vertex[i].Name == name {
			return KindIndex(i) //nolint:gosec // bounded by the kinds
		}
	}
	return None
}

// EdgeNamed is the index of the edge kind with the name, or None.
func (k *Kinds) EdgeNamed(name string) KindIndex {
	// bound: the kinds, at most their number
	for i := range k.EdgeKinds() {
		if k.edge[i].Name == name {
			return KindIndex(i) //nolint:gosec // bounded by the kinds
		}
	}
	return None
}

// VertexKindView reads one vertex kind of a Kinds (see VertexKind).
type VertexKindView struct{ k *VertexKind }

// Valid reports whether the view names a kind.
func (v VertexKindView) Valid() bool { return v.k != nil }

// Name is the kind's name.
func (v VertexKindView) Name() string {
	if v.k == nil {
		return ""
	}
	return v.k.Name
}

// Shape is how the kind is drawn.
func (v VertexKindView) Shape() Shape {
	if v.k == nil {
		return Rect
	}
	return v.k.Shape
}

// Size is the kind's size.
func (v VertexKindView) Size() Size {
	if v.k == nil {
		return Size{}
	}
	return v.k.Size
}

// PortCount is the number of the kind's ports.
func (v VertexKindView) PortCount() int {
	if v.k == nil {
		return 0
	}
	return len(v.k.Ports)
}

// Port is the jth port; out of range, the zero Port.
func (v VertexKindView) Port(j int) Port {
	if v.k == nil || j < 0 || j >= len(v.k.Ports) {
		return Port{}
	}
	return v.k.Ports[j]
}

// AttrCount is the number of attributes the kind takes.
func (v VertexKindView) AttrCount() int {
	if v.k == nil {
		return 0
	}
	return len(v.k.Attrs)
}

// Attr is the jth attribute's specification; out of range, every read
// of it is the zero value.
func (v VertexKindView) Attr(j int) AttrSpecView {
	if v.k == nil || j < 0 || j >= len(v.k.Attrs) {
		return AttrSpecView{}
	}
	return AttrSpecView{&v.k.Attrs[j]}
}

// Root reports whether a vertex of the kind takes no incoming edge.
func (v VertexKindView) Root() bool { return v.k != nil && v.k.Root }

// Leaf reports whether a vertex of the kind takes no outgoing edge.
func (v VertexKindView) Leaf() bool { return v.k != nil && v.k.Leaf }

// Max bounds how many of the kind a document holds; 0 is unbounded.
func (v VertexKindView) Max() int {
	if v.k == nil {
		return 0
	}
	return v.k.Max
}

// Dashed reports whether the kind's outline is drawn dashed.
func (v VertexKindView) Dashed() bool { return v.k != nil && v.k.Dashed }

// Container reports whether the kind holds other vertices.
func (v VertexKindView) Container() bool { return v.k != nil && v.k.Container }

// Holds reports whether a container of the kind may hold a vertex of
// the kind given: any, when it names none.
func (v VertexKindView) Holds(kind KindIndex) bool {
	return v.k != nil && v.k.Container && kindIn(v.k.Contains, kind)
}

// ContainsCount is the number of kinds the kind names as those it may
// hold.
func (v VertexKindView) ContainsCount() int {
	if v.k == nil {
		return 0
	}
	return len(v.k.Contains)
}

// Contains is the jth kind it may hold; out of range, None.
func (v VertexKindView) Contains(j int) KindIndex {
	if v.k == nil || j < 0 || j >= len(v.k.Contains) {
		return None
	}
	return v.k.Contains[j]
}

// Keyword is drawn on every vertex of the kind, such as «block».
func (v VertexKindView) Keyword() string {
	if v.k == nil {
		return ""
	}
	return v.k.Keyword
}

// CompartmentCount is the number of the kind's compartments.
func (v VertexKindView) CompartmentCount() int {
	if v.k == nil {
		return 0
	}
	return len(v.k.Compartments)
}

// Compartment is the key of the jth compartment; out of range, "".
func (v VertexKindView) Compartment(j int) string {
	if v.k == nil || j < 0 || j >= len(v.k.Compartments) {
		return ""
	}
	return v.k.Compartments[j]
}

// MaxWidth is the width a vertex with compartments grows to at most.
func (v VertexKindView) MaxWidth() float64 {
	if v.k == nil {
		return 0
	}
	return v.k.MaxWidth
}

// LabelFormat builds the label drawn.
func (v VertexKindView) LabelFormat() string {
	if v.k == nil {
		return ""
	}
	return v.k.LabelFormat
}

// EditAttr is the attribute the text input edits in place of the label.
func (v VertexKindView) EditAttr() string {
	if v.k == nil {
		return ""
	}
	return v.k.EditAttr
}

// EdgeKindView reads one edge kind of a Kinds (see EdgeKind).
type EdgeKindView struct{ k *EdgeKind }

// Valid reports whether the view names a kind.
func (e EdgeKindView) Valid() bool { return e.k != nil }

// Name is the kind's name.
func (e EdgeKindView) Name() string {
	if e.k == nil {
		return ""
	}
	return e.k.Name
}

// Allows reports whether an edge of the kind may join a vertex of the
// first kind to one of the second.
func (e EdgeKindView) Allows(from, to KindIndex) bool { return e.k != nil && e.k.allows(from, to) }

// FromCount is the number of vertex kinds an edge of the kind may
// leave: none for any.
func (e EdgeKindView) FromCount() int {
	if e.k == nil {
		return 0
	}
	return len(e.k.From)
}

// From is the jth vertex kind it may leave; out of range, None.
func (e EdgeKindView) From(j int) KindIndex {
	if e.k == nil || j < 0 || j >= len(e.k.From) {
		return None
	}
	return e.k.From[j]
}

// ToCount is the number of vertex kinds an edge of the kind may enter:
// none for any.
func (e EdgeKindView) ToCount() int {
	if e.k == nil {
		return 0
	}
	return len(e.k.To)
}

// To is the jth vertex kind it may enter; out of range, None.
func (e EdgeKindView) To(j int) KindIndex {
	if e.k == nil || j < 0 || j >= len(e.k.To) {
		return None
	}
	return e.k.To[j]
}

// Route is how the kind is drawn between its ends.
func (e EdgeKindView) Route() Route {
	if e.k == nil {
		return Straight
	}
	return e.k.Route
}

// Arrow reports whether the kind has an arrowhead.
func (e EdgeKindView) Arrow() bool { return e.k != nil && e.k.Arrow }

// Head is how the kind's arrow ends.
func (e EdgeKindView) Head() Head {
	if e.k == nil {
		return FilledHead
	}
	return e.k.Head
}

// Tail is the decoration at the kind's start.
func (e EdgeKindView) Tail() Tail {
	if e.k == nil {
		return NoTail
	}
	return e.k.Tail
}

// Dashed reports whether the kind is drawn dashed.
func (e EdgeKindView) Dashed() bool { return e.k != nil && e.k.Dashed }

// Stereotype is drawn on every edge of the kind, such as «include».
func (e EdgeKindView) Stereotype() string {
	if e.k == nil {
		return ""
	}
	return e.k.Stereotype
}

// AttrCount is the number of attributes the kind takes.
func (e EdgeKindView) AttrCount() int {
	if e.k == nil {
		return 0
	}
	return len(e.k.Attrs)
}

// Attr is the jth attribute's specification; out of range, every read
// of it is the zero value.
func (e EdgeKindView) Attr(j int) AttrSpecView {
	if e.k == nil || j < 0 || j >= len(e.k.Attrs) {
		return AttrSpecView{}
	}
	return AttrSpecView{&e.k.Attrs[j]}
}

// Weighted reports whether an edge of the kind is drawn as wide as its
// weight.
func (e EdgeKindView) Weighted() bool { return e.k != nil && e.k.Weighted }

// LabelFormat builds the label drawn.
func (e EdgeKindView) LabelFormat() string {
	if e.k == nil {
		return ""
	}
	return e.k.LabelFormat
}

// EditAttr is the attribute the text input edits in place of the label.
func (e EdgeKindView) EditAttr() string {
	if e.k == nil {
		return ""
	}
	return e.k.EditAttr
}

// AttrSpecView reads one attribute specification of a kind (see
// AttrSpec).
type AttrSpecView struct{ a *AttrSpec }

// Valid reports whether the view names a specification.
func (s AttrSpecView) Valid() bool { return s.a != nil }

// Key is the attribute's key.
func (s AttrSpecView) Key() string {
	if s.a == nil {
		return ""
	}
	return s.a.Key
}

// Kind says what the value may be.
func (s AttrSpecView) Kind() AttrKind {
	if s.a == nil {
		return Text
	}
	return s.a.Kind
}

// ChoiceCount is the number of choices.
func (s AttrSpecView) ChoiceCount() int {
	if s.a == nil {
		return 0
	}
	return len(s.a.Choices)
}

// Choice is the jth choice; out of range, "".
func (s AttrSpecView) Choice(j int) string {
	if s.a == nil || j < 0 || j >= len(s.a.Choices) {
		return ""
	}
	return s.a.Choices[j]
}

// Required reports whether the attribute must be present.
func (s AttrSpecView) Required() bool { return s.a != nil && s.a.Required }

// Default is what the attribute starts as.
func (s AttrSpecView) Default() string {
	if s.a == nil {
		return ""
	}
	return s.a.Default
}

// Bounded reports whether a number must lie from Min to Max.
func (s AttrSpecView) Bounded() bool { return s.a != nil && s.a.Bounded }

// Min is a bounded number's least value.
func (s AttrSpecView) Min() float64 {
	if s.a == nil {
		return 0
	}
	return s.a.Min
}

// Max is a bounded number's greatest value.
func (s AttrSpecView) Max() float64 {
	if s.a == nil {
		return 0
	}
	return s.a.Max
}

// Accepts reports whether the value fits the specification.
func (s AttrSpecView) Accepts(value string) bool { return s.a != nil && s.a.accepts(value) }
