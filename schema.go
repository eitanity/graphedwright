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
	TypedHead              // an outlined triangle with two dots inside, as for typing
	BarHead                // an outlined triangle with a bar across its base, as for redefinition
)

// Tail is the decoration at an edge's start, where it leaves its
// source.
type Tail uint8

// The tails.
const (
	NoTail        Tail = iota
	FilledDiamond      // as for composition
	HollowDiamond      // as for aggregation
	CircledPlus        // as for containment
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
// required attribute needs a default, so that a new element has one. A
// bounded number must lie from Min to Max, inclusive; either may be
// infinite, for a number bounded on one side only.
type AttrSpec struct {
	Key      string
	Kind     AttrKind
	Choices  []string
	Required bool
	Default  string
	Bounded  bool
	Min, Max float64
}

// VertexKind is a palette entry: what a kind of vertex looks like, the
// ports it has, the attributes it takes, and whether it is a root (no
// incoming edge) or a leaf (no outgoing edge). Max bounds how many a
// document holds; 0 is unbounded. A dashed kind's outline is drawn
// dashed. A container kind holds other vertices (see Container); Contains
// names the kinds it may hold, empty for any. A keyword, such as «block»,
// is drawn on every vertex of the kind above its label, before a
// container's title, or above an actor's head; a kind with one, but for
// a container or an actor, is at least KeywordHeight high, room for
// both lines. Compartments names attributes the kind declares, each
// drawn with its value below the label of a rectangle or rounded
// rectangle, ruled off from what is above it; the vertex grows to hold
// them, as wide as MaxWidth at most, or its kind's width when that is 0.
// A label format builds the label drawn from the label as set and the
// attributes (see NextLabelPart). EditAttr names an attribute the kind
// declares that the editor's text input edits in place of the label,
// for a kind whose label format draws it; empty edits the label.
type VertexKind struct {
	Name         string
	Shape        Shape
	Size         Size
	Ports        []Port
	Attrs        []AttrSpec
	Root         bool
	Leaf         bool
	Max          int
	Dashed       bool
	Container    bool
	Contains     []KindIndex
	Keyword      string
	Compartments []string
	MaxWidth     float64
	LabelFormat  string
	EditAttr     string
}

// KeywordHeight is the least height of a vertex kind with a keyword,
// but for a container or an actor: two lines of text.
const KeywordHeight = 32

// EdgeKind is a palette entry for edges: the vertex kinds it may join at
// each end (empty is any), how it is routed and drawn, and the
// attributes it takes. Head says how an arrow ends, and Tail what is
// drawn where the edge starts; a stereotype, such as «include», is
// drawn on every edge of the kind beside its label. A label format
// builds the label drawn, and EditAttr names the attribute the text
// input edits, as a vertex kind's do.
type EdgeKind struct {
	Name        string
	From, To    []KindIndex
	Route       Route
	Arrow       bool
	Head        Head
	Tail        Tail
	Dashed      bool
	Stereotype  string
	Attrs       []AttrSpec
	Weighted    bool
	LabelFormat string
	EditAttr    string
}

// Schema says what a document may contain. The editor's palette is the
// schema's kinds; validation is the schema's rules.
type Schema struct {
	VertexKinds []VertexKind
	EdgeKinds   []EdgeKind
	Acyclic     bool // the graph must stay a DAG
	Connected   bool // every vertex reachable from a root kind: a warning, not a refusal
}

// checkSchema validates a schema: at least one kind, unique names, kind
// and port references in range, and every required attribute with a
// default. The capacities are checked by Kinds.fits.
func checkSchema(s Schema) error {
	if len(s.VertexKinds) == 0 {
		return schemaError("", "no vertex kinds")
	}
	for i, k := range s.VertexKinds {
		if err := checkVertexKind(s, i, k); err != nil {
			return err
		}
	}
	for i, k := range s.EdgeKinds {
		if err := checkEdgeKind(s, i, k); err != nil {
			return err
		}
	}
	return nil
}

func checkVertexKind(s Schema, i int, k VertexKind) error {
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
	if err := checkPorts(where, k.Ports); err != nil {
		return err
	}
	if err := checkText("keyword", k.Keyword); err != nil {
		return schemaError(where, err.Error())
	}
	if k.Keyword != "" && !k.Container && k.Shape != Actor && k.Size.H < KeywordHeight {
		return schemaError(where, "has a keyword and is under "+itoa(KeywordHeight)+" high, the room for it and the label")
	}
	if err := checkCompartments(k); err != nil {
		return schemaError(where, err.Error())
	}
	if err := checkLabelFormat(k.LabelFormat, k.Attrs); err != nil {
		return schemaError(where, err.Error())
	}
	if err := checkEditAttr(k.EditAttr, k.Attrs); err != nil {
		return schemaError(where, err.Error())
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
	return checkAttrSpecs(where, k.Attrs)
}

// checkCompartments refuses compartments on a shape that cannot hold
// them, a key the kind does not declare or names twice, and a width
// that is not a finite non-negative number.
func checkCompartments(k VertexKind) error {
	if !(k.MaxWidth >= 0) || math.IsInf(k.MaxWidth, 1) {
		return refusal("max width must be finite and non-negative")
	}
	if len(k.Compartments) == 0 {
		return nil
	}
	if k.Container || (k.Shape != Rect && k.Shape != RoundRect) {
		return refusal("has compartments and is not a rectangle or rounded rectangle")
	}
	for j, key := range k.Compartments {
		if _, ok := spec(k.Attrs, key); !ok {
			return refusal("compartment " + strconv.Quote(key) + " is not an attribute of the kind")
		}
		for _, other := range k.Compartments[:j] {
			if other == key {
				return refusal("compartment " + strconv.Quote(key) + " is named twice")
			}
		}
	}
	return nil
}

// checkPorts validates a kind's ports: each named, on a side, with a
// non-negative max, and no name twice.
func checkPorts(where string, ports []Port) error {
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

func checkEdgeKind(s Schema, i int, k EdgeKind) error {
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
	if err := checkLabelFormat(k.LabelFormat, k.Attrs); err != nil {
		return schemaError(where, err.Error())
	}
	if err := checkEditAttr(k.EditAttr, k.Attrs); err != nil {
		return schemaError(where, err.Error())
	}
	for j := range i {
		if s.EdgeKinds[j].Name == k.Name {
			return schemaError(where, "duplicate name")
		}
	}
	if k.Route > Orthogonal || k.Head > BarHead || k.Tail > CircledPlus {
		return schemaError(where, "route, head and tail must be in range")
	}
	for _, ends := range [2][]KindIndex{k.From, k.To} {
		for _, kind := range ends {
			if kind < 0 || int(kind) >= len(s.VertexKinds) {
				return schemaError(where, "vertex kind "+itoa(int(kind))+" out of range")
			}
		}
	}
	return checkAttrSpecs(where, k.Attrs)
}

// checkAttrSpecs validates a kind's attribute specifications: unique
// keys, choices for a choice, a default that fits its own kind, and a
// default for every required attribute.
func checkAttrSpecs(where string, specs []AttrSpec) error {
	for i, a := range specs {
		if err := checkSpecText(i, a); err != nil {
			return schemaError(where, err.Error())
		}
		for j := range i {
			if specs[j].Key == a.Key {
				return schemaError(where, "attribute "+strconv.Quote(a.Key)+" duplicated")
			}
		}
		if err := checkSpecValues(a); err != nil {
			return schemaError(where, err.Error())
		}
	}
	return nil
}

// checkSpecValues refuses an attribute specification whose kind is out
// of range, a choice without choices, bounds that do not fit, and a
// default that is missing when required or that the kind refuses.
func checkSpecValues(a AttrSpec) error {
	if a.Kind > Choice || (a.Kind == Choice && len(a.Choices) == 0) {
		return refusal("attribute " + strconv.Quote(a.Key) + " of kind choice has no choices")
	}
	if err := checkBounds(a); err != nil {
		return err
	}
	if a.Required && a.Default == "" {
		return refusal("attribute " + strconv.Quote(a.Key) + " is required without a default")
	}
	if a.Default != "" && !a.accepts(a.Default) {
		return refusal("attribute " + strconv.Quote(a.Key) + " has a default it does not accept: " + strconv.Quote(a.Default))
	}
	return nil
}

// checkEditAttr refuses an attribute to edit in place of the label
// that the kind does not declare.
func checkEditAttr(key string, specs []AttrSpec) error {
	if key == "" {
		return nil
	}
	if _, ok := spec(specs, key); !ok {
		return refusal("edits attribute " + strconv.Quote(key) + ", which is not an attribute of the kind")
	}
	return nil
}

// checkBounds refuses bounds on an attribute that is not a number, and
// bounds that are NaN or that hold no number.
func checkBounds(a AttrSpec) error {
	if !a.Bounded {
		return nil
	}
	if a.Kind != Number {
		return refusal("attribute " + strconv.Quote(a.Key) + " is bounded and not a number")
	}
	if !(a.Min <= a.Max) || math.IsInf(a.Min, 1) || math.IsInf(a.Max, -1) {
		return refusal("attribute " + strconv.Quote(a.Key) + " has bounds that hold no number")
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
		return err == nil && !math.IsNaN(x) && !math.IsInf(x, 0) && (!a.Bounded || (a.Min <= x && x <= a.Max))
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
