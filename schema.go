package graphedwright

import (
	"math"
	"strconv"
)

// Shape is how a vertex kind is drawn.
type Shape uint8

// The shapes of a vertex.
const (
	Rect Shape = iota
	RoundRect
	Pill
	Diamond
	Circle    // an ellipse filling the size
	Actor     // a stick figure, its label below it
	DataStore // a box open at both ends: a line above and a line below
)

// Route is how an edge kind is drawn between its ends.
type Route uint8

// The routes of an edge.
const (
	Straight Route = iota
	Curve
	Orthogonal
)

// Head is how an edge kind's arrow ends.
type Head uint8

// The arrowheads.
const (
	FilledHead Head = iota // a filled triangle
	OpenHead               // two strokes, the edge drawn to the tip
	HollowHead             // an outlined triangle, as for generalisation
)

// AttrKind says what an attribute's value may be.
type AttrKind uint8

// The kinds of attribute value.
const (
	Text   AttrKind = iota // any text
	Number                 // a finite number
	Choice                 // one of Choices
)

// AttrSpec describes an attribute a kind allows: its key, what its
// value may be, whether it must be present and what it starts as. A
// required attribute needs a default, so that a new element has one.
type AttrSpec struct {
	Key      string
	Kind     AttrKind
	Choices  []string
	Required bool
	Default  string
}

// VertexKind is a palette entry: what a kind of vertex looks like, the
// ports it has, the attributes it takes, and whether it is a root (no
// incoming edge) or a leaf (no outgoing edge). Max bounds how many a
// document holds; 0 is unbounded. A dashed kind's outline is drawn
// dashed. A container kind holds other vertices (see Container); Contains
// names the kinds it may hold, empty for any.
type VertexKind struct {
	Name      string
	Shape     Shape
	Size      Size
	Ports     []Port
	Attrs     []AttrSpec
	Root      bool
	Leaf      bool
	Max       int
	Dashed    bool
	Container bool
	Contains  []KindIndex
}

// EdgeKind is a palette entry for edges: the vertex kinds it may join at
// each end (empty is any), how it is routed and drawn, and the
// attributes it takes. Head says how an arrow ends; a stereotype, such
// as «include», is drawn on every edge of the kind beside its label.
type EdgeKind struct {
	Name       string
	From, To   []KindIndex
	Route      Route
	Arrow      bool
	Head       Head
	Dashed     bool
	Stereotype string
	Attrs      []AttrSpec
	Weighted   bool
}

// Schema says what a document may contain. The editor's palette is the
// schema's kinds; validation is the schema's rules.
type Schema struct {
	VertexKinds []VertexKind
	EdgeKinds   []EdgeKind
	Acyclic     bool // the graph must stay a DAG
	Connected   bool // every vertex reachable from a root kind: a warning, not a refusal
}

// checkSchema validates a schema against the capacities: at least one
// kind, unique names, kind and port references in range, ports and
// attributes within the capacities, and every required attribute with
// a default.
func checkSchema(s Schema, caps Caps) error {
	if len(s.VertexKinds) == 0 {
		return schemaError("", "no vertex kinds")
	}
	for i, k := range s.VertexKinds {
		if err := checkVertexKind(s, i, k, caps); err != nil {
			return err
		}
	}
	for i, k := range s.EdgeKinds {
		if err := checkEdgeKind(s, i, k, caps); err != nil {
			return err
		}
	}
	return nil
}

func checkVertexKind(s Schema, i int, k VertexKind, caps Caps) error {
	where := "vertex kind " + strconv.Quote(k.Name)
	if k.Name == "" {
		return schemaError("vertex kind "+itoa(i), "empty name")
	}
	if err := checkName("name", k.Name); err != nil {
		return schemaError(where, err.Error())
	}
	for j := range i {
		if s.VertexKinds[j].Name == k.Name {
			return schemaError(where, "duplicate name")
		}
	}
	if err := checkPorts(where, k.Ports, caps); err != nil {
		return err
	}
	if len(k.Contains) > 0 && !k.Container {
		return schemaError(where, "holds kinds without being a container")
	}
	for _, kind := range k.Contains {
		if kind < 0 || int(kind) >= len(s.VertexKinds) {
			return schemaError(where, "contains vertex kind "+itoa(int(kind))+" out of range")
		}
	}
	if k.Max < 0 || k.Shape > DataStore || !(k.Size.W >= 0) || !(k.Size.H >= 0) || math.IsInf(k.Size.W, 1) || math.IsInf(k.Size.H, 1) {
		return schemaError(where, "max, shape and size must be in range")
	}
	return checkAttrSpecs(where, k.Attrs, caps)
}

// checkPorts validates a kind's ports: within the capacity, each named,
// on a side, with a non-negative max, and no name twice.
func checkPorts(where string, ports []Port, caps Caps) error {
	if len(ports) > caps.PortsPerVertex {
		return schemaError(where, itoa(len(ports))+" ports exceed the limit of "+itoa(caps.PortsPerVertex))
	}
	for p, port := range ports {
		if port.Name == "" || port.Max < 0 || port.Side > Bottom {
			return schemaError(where, "port "+itoa(p)+" needs a name, a side and a non-negative max")
		}
		if err := checkName("port", port.Name); err != nil {
			return schemaError(where, err.Error())
		}
		for q := range p {
			if ports[q].Name == port.Name {
				return schemaError(where, "port "+strconv.Quote(port.Name)+" duplicated")
			}
		}
	}
	return nil
}

func checkEdgeKind(s Schema, i int, k EdgeKind, caps Caps) error {
	where := "edge kind " + strconv.Quote(k.Name)
	if k.Name == "" {
		return schemaError("edge kind "+itoa(i), "empty name")
	}
	if err := checkName("name", k.Name); err != nil {
		return schemaError(where, err.Error())
	}
	if err := checkText("stereotype", k.Stereotype); err != nil {
		return schemaError(where, err.Error())
	}
	for j := range i {
		if s.EdgeKinds[j].Name == k.Name {
			return schemaError(where, "duplicate name")
		}
	}
	if k.Route > Orthogonal || k.Head > HollowHead {
		return schemaError(where, "route and head must be in range")
	}
	for _, ends := range [2][]KindIndex{k.From, k.To} {
		for _, kind := range ends {
			if kind < 0 || int(kind) >= len(s.VertexKinds) {
				return schemaError(where, "vertex kind "+itoa(int(kind))+" out of range")
			}
		}
	}
	return checkAttrSpecs(where, k.Attrs, caps)
}

// checkAttrSpecs validates a kind's attribute specifications: unique
// keys within the capacity, choices for a choice, a default that fits
// its own kind, and a default for every required attribute.
func checkAttrSpecs(where string, specs []AttrSpec, caps Caps) error {
	if len(specs) > caps.AttrsPerElement {
		return schemaError(where, itoa(len(specs))+" attributes exceed the limit of "+itoa(caps.AttrsPerElement))
	}
	for i, a := range specs {
		if err := checkSpecText(i, a); err != nil {
			return schemaError(where, err.Error())
		}
		for j := range i {
			if specs[j].Key == a.Key {
				return schemaError(where, "attribute "+strconv.Quote(a.Key)+" duplicated")
			}
		}
		if a.Kind > Choice || (a.Kind == Choice && len(a.Choices) == 0) {
			return schemaError(where, "attribute "+strconv.Quote(a.Key)+" of kind choice has no choices")
		}
		if a.Required && a.Default == "" {
			return schemaError(where, "attribute "+strconv.Quote(a.Key)+" is required without a default")
		}
		if a.Default != "" && !a.accepts(a.Default) {
			return schemaError(where, "attribute "+strconv.Quote(a.Key)+" has a default it does not accept: "+strconv.Quote(a.Default))
		}
	}
	return nil
}

// checkSpecText refuses an attribute specification with an empty key,
// or whose key, choices or default XML cannot carry.
func checkSpecText(i int, a AttrSpec) error {
	if a.Key == "" {
		return refusal("attribute " + itoa(i) + " has an empty key")
	}
	if err := checkKey(a.Key); err != nil {
		return err
	}
	for _, c := range a.Choices {
		if err := checkText("attribute "+strconv.Quote(a.Key)+" choice", c); err != nil {
			return err
		}
	}
	return checkText("attribute "+strconv.Quote(a.Key)+" default", a.Default)
}

// accepts reports whether the value fits the specification's kind.
func (a AttrSpec) accepts(value string) bool {
	switch a.Kind {
	case Text:
		return true
	case Number:
		x, err := strconv.ParseFloat(value, 64)
		return err == nil && !math.IsNaN(x) && !math.IsInf(x, 0)
	case Choice:
		for _, c := range a.Choices {
			if c == value {
				return true
			}
		}
		return false
	}
	return false
}

// spec finds the attribute specification for a key, or ok = false.
func spec(specs []AttrSpec, key string) (AttrSpec, bool) {
	for _, a := range specs {
		if a.Key == key {
			return a, true
		}
	}
	return AttrSpec{}, false
}

// allows reports whether the edge kind may join the two vertex kinds.
func (k EdgeKind) allows(from, to KindIndex) bool {
	return kindIn(k.From, from) && kindIn(k.To, to)
}

// kindIn reports whether the kind is in the list, an empty list meaning any.
func kindIn(list []KindIndex, kind KindIndex) bool {
	if len(list) == 0 {
		return true
	}
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}
