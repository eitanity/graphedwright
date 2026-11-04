package graphedwright_test

import (
	"math"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestSchemaRefusals: every rule of schema validation is reached, and a
// capacity out of range is refused.
func TestSchemaRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		mod  func(*gw.Schema)
		want string
	}{
		{"no kinds", func(s *gw.Schema) { s.VertexKinds = nil }, "no vertex kinds"},
		{"empty vertex name", func(s *gw.Schema) { s.VertexKinds[0].Name = "" }, "vertex kind 0: empty name"},
		{"duplicate vertex name", func(s *gw.Schema) { s.VertexKinds[2].Name = "node" }, `vertex kind "node": duplicate name`},
		{"too many ports", func(s *gw.Schema) {
			s.VertexKinds[1].Ports = make([]gw.Port, 9)
			for i := range s.VertexKinds[1].Ports {
				s.VertexKinds[1].Ports[i].Name = string(rune('a' + i))
			}
		}, "9 ports exceed the limit of 8"},
		{"nameless port", func(s *gw.Schema) { s.VertexKinds[1].Ports[0].Name = "" }, "port 0 needs a name"},
		{"duplicate port", func(s *gw.Schema) { s.VertexKinds[1].Ports[1].Name = "in" }, `port "in" duplicated`},
		{"negative max", func(s *gw.Schema) { s.VertexKinds[0].Max = -1 }, "max, shape and size must be in range"},
		{"empty edge name", func(s *gw.Schema) { s.EdgeKinds[1].Name = "" }, "edge kind 1: empty name"},
		{"duplicate edge name", func(s *gw.Schema) { s.EdgeKinds[1].Name = "to" }, `edge kind "to": duplicate name`},
		{"edge kind range", func(s *gw.Schema) { s.EdgeKinds[0].To = []gw.KindIndex{7} }, `edge kind "to": vertex kind 7 out of range`},
		{"route range", func(s *gw.Schema) { s.EdgeKinds[0].Route = 9 }, "route, head and tail must be in range"},
		{"head range", func(s *gw.Schema) { s.EdgeKinds[0].Head = gw.BarHead + 1 }, "route, head and tail must be in range"},
		{"tail range", func(s *gw.Schema) { s.EdgeKinds[0].Tail = gw.CircledPlus + 1 }, "route, head and tail must be in range"},
		{"contains without container", func(s *gw.Schema) { s.VertexKinds[0].Contains = []gw.KindIndex{1} }, "holds kinds without being a container"},
		{"contains range", func(s *gw.Schema) {
			s.VertexKinds[0].Container = true
			s.VertexKinds[0].Contains = []gw.KindIndex{9}
		}, "contains vertex kind 9 out of range"},
		{"shape range", func(s *gw.Schema) { s.VertexKinds[0].Shape = gw.DataStore + 1 }, "max, shape and size must be in range"},
		{"too many attrs", func(s *gw.Schema) {
			s.VertexKinds[1].Attrs = make([]gw.AttrSpec, 17)
			for i := range s.VertexKinds[1].Attrs {
				s.VertexKinds[1].Attrs[i].Key = string(rune('a' + i))
			}
		}, "17 attributes exceed the limit of 16"},
		{"empty attr key", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Key = "" }, "attribute 2 has an empty key"},
		{"duplicate attr", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Key = "p" }, `attribute "p" duplicated`},
		{"choice without choices", func(s *gw.Schema) { s.VertexKinds[1].Attrs[1].Choices = nil }, `attribute "mode" of kind choice has no choices`},
		{"required without default", func(s *gw.Schema) { s.VertexKinds[1].Attrs[0].Default = "" }, `attribute "p" is required without a default`},
		{"control in vertex name", func(s *gw.Schema) { s.VertexKinds[0].Name = "a\nb" }, `name "a\nb" holds the control character U+000a`},
		{"control in edge name", func(s *gw.Schema) { s.EdgeKinds[0].Name = "t\x00" }, "control character U+0000"},
		{"control in stereotype", func(s *gw.Schema) { s.EdgeKinds[0].Stereotype = "\x01" }, "stereotype holds the control character U+0001"},
		{"control in keyword", func(s *gw.Schema) { s.VertexKinds[1].Keyword = "\x01" }, "keyword holds the control character U+0001"},
		{"keyword too low", func(s *gw.Schema) {
			s.VertexKinds[1].Keyword = "«block»"
			s.VertexKinds[1].Size.H = 24
		}, "has a keyword and is under 32 high"},
		{"compartment undeclared", func(s *gw.Schema) { s.VertexKinds[1].Compartments = []string{"note", "body"} }, `compartment "body" is not an attribute of the kind`},
		{"compartment twice", func(s *gw.Schema) { s.VertexKinds[1].Compartments = []string{"note", "note"} }, `compartment "note" is named twice`},
		{"compartments on a pill", func(s *gw.Schema) {
			s.VertexKinds[0].Attrs = []gw.AttrSpec{{Key: "note"}}
			s.VertexKinds[0].Compartments = []string{"note"}
		}, "has compartments and is not a rectangle or rounded rectangle"},
		{"compartments on a container", func(s *gw.Schema) {
			s.VertexKinds[1].Container = true
			s.VertexKinds[1].Compartments = []string{"note"}
		}, "has compartments and is not a rectangle or rounded rectangle"},
		{"max width negative", func(s *gw.Schema) { s.VertexKinds[1].MaxWidth = -1 }, "max width must be finite and non-negative"},
		{"max width NaN", func(s *gw.Schema) { s.VertexKinds[1].MaxWidth = math.NaN() }, "max width must be finite and non-negative"},
		{"max width infinite", func(s *gw.Schema) { s.VertexKinds[1].MaxWidth = math.Inf(1) }, "max width must be finite and non-negative"},
		{"control in port", func(s *gw.Schema) { s.VertexKinds[1].Ports[0].Name = "i\tn" }, `port "i\tn" holds the control character`},
		{"control in attr key", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Key = "k\x7f" }, `attribute key "k\x7f" may hold only`},
		{"upper attr key", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Key = "Note" }, `attribute key "Note" may hold only`},
		{"reserved attr key", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Key = "kind" }, `attribute key "kind" is reserved`},
		{"control in choice", func(s *gw.Schema) { s.VertexKinds[1].Attrs[1].Choices[0] = "a\x1f" }, `attribute "mode" choice holds the control character U+001f`},
		{"control in default", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Default = "\x03" }, "default holds the control character U+0003"},
		{"default not accepted", func(s *gw.Schema) { s.VertexKinds[1].Attrs[0].Default = "x" }, `attribute "p" has a default it does not accept: "x"`},
		{"vertex edits an undeclared attr", func(s *gw.Schema) { s.VertexKinds[1].EditAttr = "body" }, `vertex kind "node": edits attribute "body", which is not an attribute of the kind`},
		{"edge edits an undeclared attr", func(s *gw.Schema) { s.EdgeKinds[1].EditAttr = "q" }, `edge kind "outcome": edits attribute "q", which is not an attribute of the kind`},
		{"bounded text", func(s *gw.Schema) { s.VertexKinds[1].Attrs[2].Bounded = true }, `attribute "note" is bounded and not a number`},
		{"bounds crossed", func(s *gw.Schema) { s.VertexKinds[1].Attrs[0].Bounded, s.VertexKinds[1].Attrs[0].Min = true, 1 }, `attribute "p" has bounds that hold no number`},
		{"bound NaN", func(s *gw.Schema) {
			s.VertexKinds[1].Attrs[0].Bounded, s.VertexKinds[1].Attrs[0].Max = true, math.NaN()
		}, `attribute "p" has bounds that hold no number`},
		{"min infinite above", func(s *gw.Schema) {
			s.VertexKinds[1].Attrs[0].Bounded, s.VertexKinds[1].Attrs[0].Min, s.VertexKinds[1].Attrs[0].Max = true, math.Inf(1), math.Inf(1)
		}, `attribute "p" has bounds that hold no number`},
		{"max infinite below", func(s *gw.Schema) {
			s.VertexKinds[1].Attrs[0].Bounded, s.VertexKinds[1].Attrs[0].Min, s.VertexKinds[1].Attrs[0].Max = true, math.Inf(-1), math.Inf(-1)
		}, `attribute "p" has bounds that hold no number`},
		{"default out of bounds", func(s *gw.Schema) { s.VertexKinds[1].Attrs[0].Bounded, s.VertexKinds[1].Attrs[0].Max = true, 0.4 }, `attribute "p" has a default it does not accept: "0.5"`},
	} {
		s := testSchema()
		tc.mod(&s)
		_, err := makeDoc(s, gw.Caps{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, err := makeDoc(testSchema(), gw.Caps{Vertices: -1}); err == nil || !strings.Contains(err.Error(), "capacity vertices -1") {
		t.Errorf("negative capacity: %v", err)
	}
	// Each capacity within its range, but together too much memory.
	for _, tc := range []struct {
		caps gw.Caps
		want string
	}{
		{gw.Caps{Edges: 1 << 24, BendsPerEdge: 1 << 24}, "the largest part is edges × bends per edge"},
		{gw.Caps{Vertices: 1 << 24, AttrsPerElement: 1 << 10}, "the largest part is vertices × attributes per element"},
		{gw.Caps{Vertices: 1 << 22, UndoLayouts: 1 << 10}, "the largest part is undo layouts × vertices"},
		{gw.Caps{UndoDepth: 1 << 24, PortsPerVertex: 1 << 10}, "the largest part is undo depth × ports per vertex"},
		{gw.Caps{Edges: 1 << 24}, "over the limit of 1024 MiB; the largest part is edges"},
	} {
		if _, err := makeDoc(testSchema(), tc.caps); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: got %v, want %q", tc.caps, err, tc.want)
		}
	}
	// A large document within the memory is still accepted.
	if _, err := makeDoc(testSchema(), gw.Caps{Vertices: 1 << 18, Edges: 1 << 18}); err != nil {
		t.Errorf("a quarter of a million vertices and edges: %v", err)
	}
}

// TestDefaults: zero capacities take the defaults, a new vertex has its
// kind's size, ports and default attributes, and minted IDs are unique
// and never reused, even after a delete.
func TestDefaults(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	if d.Caps() != gw.DefaultCaps() {
		t.Errorf("caps %+v", d.Caps())
	}
	a := addVertex(t, d, kNode, "")
	v := d.Vertex(a)
	if v.ID() != "v1" || v.Size() != (gw.Size{W: 120, H: 36}) || v.Ports().Len() != 2 || v.Attrs().Len() != 2 || v.Attrs().At(0) != (gw.Attr{Key: "p", Value: "0.5"}) || v.Placed() {
		t.Errorf("vertex %+v", v)
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: a})
	b := addVertex(t, d, kNode, "")
	if d.Vertex(b).ID() != "v2" || d.VertexByID("v1") != gw.None || d.VertexByID("v2") != b {
		t.Errorf("ids after delete: %q %q", d.Vertex(a).ID(), d.Vertex(b).ID())
	}
	refuse(t, d, gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "v1"}, `id "v1" already used`)
	addVertex(t, d, kNode, "v3")
	c := addVertex(t, d, kNode, "")
	if d.Vertex(c).ID() != "v4" {
		t.Errorf("minting skips a host id: %q", d.Vertex(c).ID())
	}
	if d.LiveVertices() != 3 || d.LiveEdges() != 0 {
		t.Errorf("live %d %d", d.LiveVertices(), d.LiveEdges())
	}
}
