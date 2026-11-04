package graphedwright_test

import (
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// kindsSchema sets every field of a vertex kind, an edge kind and an
// attribute specification to a value other than its zero.
func kindsSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40},
				Ports: []gw.Port{{Name: "in", Side: gw.Left, Max: 2}, {Name: "out", Side: gw.Right}},
				Attrs: []gw.AttrSpec{
					{Key: "p", Kind: gw.Number, Required: true, Default: "0.5", Bounded: true, Min: 0.25, Max: 1},
					{Key: "c", Kind: gw.Choice, Choices: []string{"x", "y"}},
				},
				Root: true, Leaf: true, Max: 3, Dashed: true, Keyword: "block",
				Compartments: []string{"c"}, MaxWidth: 200, LabelFormat: "{p}", EditAttr: "p"},
			{Name: "zone", Size: gw.Size{W: 100, H: 60}, Container: true, Contains: []gw.KindIndex{0}},
			{Name: "any", Size: gw.Size{W: 100, H: 60}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "link", From: []gw.KindIndex{0}, To: []gw.KindIndex{0, 1}, Route: gw.Orthogonal,
			Arrow: true, Head: gw.HollowHead, Tail: gw.HollowDiamond, Dashed: true, Stereotype: "uses",
			Attrs: []gw.AttrSpec{{Key: "w", Kind: gw.Number}}, Weighted: true, LabelFormat: "{w}", EditAttr: "w"}},
		Acyclic:   true,
		Connected: true,
	}
}

// TestKindsRead: every read of a compiled kind is the schema's.
func TestKindsRead(t *testing.T) {
	k, err := gw.Compile(kindsSchema())
	if err != nil {
		t.Fatal(err)
	}
	if k.VertexKinds() != 3 || k.EdgeKinds() != 1 || !k.Acyclic() || !k.Connected() {
		t.Errorf("counts and rules: %d %d %v %v", k.VertexKinds(), k.EdgeKinds(), k.Acyclic(), k.Connected())
	}
	v := k.Vertex(0)
	if !v.Valid() || v.Name() != "box" || v.Shape() != gw.RoundRect || v.Size() != (gw.Size{W: 120, H: 40}) ||
		!v.Root() || !v.Leaf() || v.Max() != 3 || !v.Dashed() || v.Container() || v.Keyword() != "block" ||
		v.MaxWidth() != 200 || v.LabelFormat() != "{p}" || v.EditAttr() != "p" {
		t.Error("vertex kind fields")
	}
	if v.PortCount() != 2 || v.Port(0) != (gw.Port{Name: "in", Side: gw.Left, Max: 2}) || v.Port(1).Name != "out" {
		t.Error("vertex kind ports")
	}
	if v.CompartmentCount() != 1 || v.Compartment(0) != "c" || v.AttrCount() != 2 {
		t.Error("vertex kind compartments and attributes")
	}
	a := v.Attr(0)
	if !a.Valid() || a.Key() != "p" || a.Kind() != gw.Number || !a.Required() || a.Default() != "0.5" || !a.Bounded() || a.Min() != 0.25 || a.Max() != 1 {
		t.Error("attribute fields")
	}
	if !a.Accepts("1") || a.Accepts("1.5") || a.Accepts("0.2") {
		t.Error("attribute accepts")
	}
	c := v.Attr(1)
	if c.Kind() != gw.Choice || c.ChoiceCount() != 2 || c.Choice(0) != "x" || c.Choice(1) != "y" {
		t.Error("attribute choices")
	}
	zone, anyKind := k.Vertex(1), k.Vertex(2)
	if !zone.Container() || zone.ContainsCount() != 1 || zone.Contains(0) != 0 || !zone.Holds(0) || zone.Holds(1) {
		t.Error("a container that names what it holds")
	}
	if !anyKind.Holds(0) || !anyKind.Holds(2) || v.Holds(0) {
		t.Error("a container that names none holds any; a vertex that is no container holds none")
	}
	e := k.Edge(0)
	if !e.Valid() || e.Name() != "link" || e.Route() != gw.Orthogonal || !e.Arrow() || e.Head() != gw.HollowHead ||
		e.Tail() != gw.HollowDiamond || !e.Dashed() || e.Stereotype() != "uses" || !e.Weighted() ||
		e.LabelFormat() != "{w}" || e.EditAttr() != "w" || e.AttrCount() != 1 || e.Attr(0).Key() != "w" {
		t.Error("edge kind fields")
	}
	if e.FromCount() != 1 || e.From(0) != 0 || e.ToCount() != 2 || e.To(0) != 0 || e.To(1) != 1 {
		t.Error("edge kind ends")
	}
	if !e.Allows(0, 1) || e.Allows(1, 0) || e.Allows(0, 2) {
		t.Error("edge kind allows")
	}
	if k.VertexNamed("zone") != 1 || k.VertexNamed("nope") != gw.None || k.EdgeNamed("link") != 0 || k.EdgeNamed("nope") != gw.None {
		t.Error("kinds by name")
	}
}

// TestKindsOutOfRange: a kind, a port, an attribute, a choice, a
// compartment or an end read out of range, below 0 or at the count, is
// the zero value or None, and so is every read of nil Kinds.
func TestKindsOutOfRange(t *testing.T) {
	k, err := gw.Compile(kindsSchema())
	if err != nil {
		t.Fatal(err)
	}
	var none *gw.Kinds
	if none.VertexKinds() != 0 || none.EdgeKinds() != 0 || none.Acyclic() || none.Connected() || none.Vertex(0).Valid() || none.Edge(0).Valid() ||
		none.VertexNamed("box") != gw.None || none.EdgeNamed("link") != gw.None {
		t.Error("nil Kinds")
	}
	for _, i := range []gw.KindIndex{-1, 3} {
		v := k.Vertex(i)
		if v.Valid() || v.Name() != "" || v.Shape() != gw.Rect || v.Size() != (gw.Size{}) || v.PortCount() != 0 || v.Port(0) != (gw.Port{}) ||
			v.AttrCount() != 0 || v.Attr(0).Valid() || v.Root() || v.Leaf() || v.Max() != 0 || v.Dashed() || v.Container() ||
			v.Holds(0) || v.ContainsCount() != 0 || v.Contains(0) != gw.None || v.Keyword() != "" || v.CompartmentCount() != 0 ||
			v.Compartment(0) != "" || v.MaxWidth() != 0 || v.LabelFormat() != "" || v.EditAttr() != "" {
			t.Errorf("vertex kind %d read", i)
		}
	}
	for _, i := range []gw.KindIndex{-1, 1} {
		e := k.Edge(i)
		if e.Valid() || e.Name() != "" || e.Allows(0, 0) || e.FromCount() != 0 || e.From(0) != gw.None || e.ToCount() != 0 || e.To(0) != gw.None ||
			e.Route() != gw.Straight || e.Arrow() || e.Head() != gw.FilledHead || e.Tail() != gw.NoTail || e.Dashed() || e.Stereotype() != "" ||
			e.AttrCount() != 0 || e.Attr(0).Valid() || e.Weighted() || e.LabelFormat() != "" || e.EditAttr() != "" {
			t.Errorf("edge kind %d read", i)
		}
	}
	var a gw.AttrSpecView
	if a.Valid() || a.Key() != "" || a.Kind() != gw.Text || a.ChoiceCount() != 0 || a.Choice(0) != "" || a.Required() || a.Default() != "" ||
		a.Bounded() || a.Min() != 0 || a.Max() != 0 || a.Accepts("") {
		t.Error("zero attribute read")
	}
	v, e, zone, c := k.Vertex(0), k.Edge(0), k.Vertex(1), k.Vertex(0).Attr(1)
	for _, j := range []int{-1, 2} {
		if v.Port(j) != (gw.Port{}) || v.Attr(j).Valid() || c.Choice(j) != "" || e.To(j) != gw.None {
			t.Errorf("position %d read", j)
		}
	}
	for _, j := range []int{-1, 1} {
		if v.Compartment(j) != "" || zone.Contains(j) != gw.None || e.From(j) != gw.None || e.Attr(j).Valid() {
			t.Errorf("position %d read", j)
		}
	}
}

// TestCompileCopies: a schema changed after Compile changes nothing in
// the kinds, nor in a document of them; two documents of one Kinds
// share it; New refuses nil kinds.
func TestCompileCopies(t *testing.T) {
	s := kindsSchema()
	k, err := gw.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	d1, err1 := gw.New(k, gw.Caps{})
	d2, err2 := gw.New(k, gw.Caps{Vertices: 4})
	if err1 != nil || err2 != nil || d1.Kinds() != k || d2.Kinds() != k {
		t.Fatalf("two documents of one Kinds: %v %v", err1, err2)
	}
	vk, ek := &s.VertexKinds[0], &s.EdgeKinds[0]
	vk.Name, vk.Size.W, vk.Ports[0].Name, vk.Attrs[0].Key, vk.Attrs[1].Choices[0] = "changed", 1, "changed", "changed", "changed"
	vk.Compartments[0], s.VertexKinds[1].Contains[0] = "changed", 2
	ek.From[0], ek.To[1], ek.Attrs[0].Key = 1, 2, "changed"
	s.VertexKinds = append(s.VertexKinds[:1], gw.VertexKind{Name: "other"})
	v, e := k.Vertex(0), k.Edge(0)
	if v.Name() != "box" || v.Size().W != 120 || v.Port(0).Name != "in" || v.Attr(0).Key() != "p" || v.Attr(1).Choice(0) != "x" ||
		v.Compartment(0) != "c" || k.Vertex(1).Contains(0) != 0 || k.Vertex(1).Name() != "zone" ||
		e.From(0) != 0 || e.To(1) != 1 || e.Attr(0).Key() != "w" {
		t.Error("a change to the schema reached the compiled kinds")
	}
	if d1.Kinds().Vertex(0).Name() != "box" {
		t.Error("a change to the schema reached a document")
	}
	if _, err := gw.New(nil, gw.Caps{}); err == nil || !strings.Contains(err.Error(), "no kinds") {
		t.Errorf("New of nil kinds: %v", err)
	}
}

// TestKindsFit: Compile takes a schema whatever its ports' and
// attributes' number; New refuses kinds with more than its capacities
// allow, naming the kind.
func TestKindsFit(t *testing.T) {
	k, err := gw.Compile(kindsSchema())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		caps gw.Caps
		want string
	}{
		{gw.Caps{PortsPerVertex: 1}, `vertex kind "box": 2 ports exceed the limit of 1`},
		{gw.Caps{AttrsPerElement: 1}, `vertex kind "box": 2 attributes exceed the limit of 1`},
	} {
		if _, err := gw.New(k, c.caps); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want %q", c.caps, err, c.want)
		}
	}
	s := gw.Schema{VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 10, H: 10}}},
		EdgeKinds: []gw.EdgeKind{{Name: "link", Attrs: []gw.AttrSpec{{Key: "a"}, {Key: "b"}}}}}
	ke, err := gw.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gw.New(ke, gw.Caps{AttrsPerElement: 1}); err == nil || !strings.Contains(err.Error(), `edge kind "link": 2 attributes exceed the limit of 1`) {
		t.Errorf("edge attributes past the capacity: %v", err)
	}
	if _, err := gw.New(ke, gw.Caps{AttrsPerElement: 2}); err != nil {
		t.Errorf("at the capacity: %v", err)
	}
	if _, err := gw.New(k, gw.Caps{PortsPerVertex: 2, AttrsPerElement: 2}); err != nil {
		t.Errorf("at the capacities: %v", err)
	}
}
