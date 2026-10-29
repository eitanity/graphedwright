package graphedwright_test

import (
	"math"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestRefusals: every refusal a command can meet is reached, names its
// rule, and leaves the document untouched.
func TestRefusals(t *testing.T) {
	d := newDoc(t, gw.Caps{Vertices: 7, Edges: 4, AttrsPerElement: 3, LabelBytes: 8})
	s, a, b, c := chain(t, d)
	long := strings.Repeat("x", 9)
	for _, tc := range []struct {
		name string
		cmd  gw.Command
		want string
	}{
		{"vertex kind range", gw.Command{Op: gw.AddVertex, Kind: 9}, "vertex kind 9 out of range"},
		{"label too long", gw.Command{Op: gw.AddVertex, Kind: kNode, Text: long}, "label of 9 bytes exceeds the limit of 8"},
		{"edge kind range", gw.Command{Op: gw.AddEdge, Kind: 5, From: a, To: b}, "edge kind 5 out of range"},
		{"edge from dead", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: 9, To: b}, "vertex 9 is not live"},
		{"kinds not allowed", gw.Command{Op: gw.AddEdge, Kind: eTo, From: s, To: s}, `edge kind "to" cannot join "source" to "source"`},
		{"into a root", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: s, FromPort: 1, ToPort: gw.None}, `vertex "s" of kind "source" takes no incoming edge`},
		{"out of a leaf", gw.Command{Op: gw.AddEdge, Kind: eAny, From: c, To: b, FromPort: gw.None, ToPort: 0}, `vertex "c" of kind "leaf" takes no outgoing edge`},
		{"port needed", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: b, FromPort: gw.None, ToPort: 0}, `vertex "a" has ports; the edge must name one`},
		{"port range", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: b, FromPort: 7, ToPort: 0}, `port 7 of vertex "a" out of range`},
		{"cycle", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: b, To: a, FromPort: 1, ToPort: 0}, "edge would make a cycle: b → a → b"},
		{"self loop", gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: a, FromPort: 1, ToPort: 0}, "edge would make a cycle: a → a"},
		{"delete dead vertex", gw.Command{Op: gw.DeleteVertex, Vertex: 9}, "vertex 9 is not live"},
		{"delete dead edge", gw.Command{Op: gw.DeleteEdge, Edge: 9}, "edge 9 is not live"},
		{"move dead", gw.Command{Op: gw.Move, Vertex: 9}, "vertex 9 is not live"},
		{"move nan", gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: nan()}}, "position must be finite"},
		{"move far", gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{Y: -1.5e9}}, "position must be finite and within 1e9 of the origin"},
		{"add nan", gw.Command{Op: gw.AddVertex, Kind: kNode, Pos: gw.Point{X: nan()}}, "position must be finite and within 1e9"},
		{"add infinite", gw.Command{Op: gw.AddVertex, Kind: kNode, Pos: gw.Point{Y: math.Inf(-1)}}, "position must be finite"},
		{"add far", gw.Command{Op: gw.AddVertex, Kind: kNode, Pos: gw.Point{X: 1e308}}, "position must be finite"},
		{"add negative size", gw.Command{Op: gw.AddVertex, Kind: kNode, Size: gw.Size{W: -500, H: -500}}, "size must be positive and finite, at most 1e9"},
		{"add nan size", gw.Command{Op: gw.AddVertex, Kind: kNode, Size: gw.Size{W: nan(), H: 1}}, "size must be positive"},
		{"add half a size", gw.Command{Op: gw.AddVertex, Kind: kNode, Size: gw.Size{W: 10}}, "size must be positive"},
		{"resize huge", gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 1e308, H: 1}}, "size must be positive and finite, at most 1e9"},
		{"edge weight infinite", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, Weight: math.Inf(1)}, "weight must be finite, not negative and at most 1e9"},
		{"edge weight nan", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, Weight: nan()}, "weight must be finite"},
		{"edge weight negative", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, Weight: -3}, "weight must be finite"},
		{"edge weight huge", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, Weight: 2e9}, "weight must be finite"},
		{"resize zero", gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 0, H: 1}}, "size must be positive and finite"},
		{"relabel dead edge", gw.Command{Op: gw.Relabel, Vertex: gw.None, Edge: 9, Text: "x"}, "edge 9 is not live"},
		{"relabel too long", gw.Command{Op: gw.Relabel, Vertex: a, Text: long}, "label of 9 bytes"},
		{"attr empty key", gw.Command{Op: gw.SetAttr, Vertex: a}, "attribute key must not be empty"},
		{"attr not a number", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "p", Value: "x"}, `attribute "p" of vertex "a" must be a number: "x"`},
		{"attr not a choice", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "mode", Value: "x"}, `attribute "mode" of vertex "a" must be one of absolute, sequential`},
		{"attr required", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "p", Value: ""}, `attribute "p" of vertex "a" is required`},
		{"attr too long", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: long}, "attribute value of 9 bytes"},
		{"edge attr not a number", gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: 1, Key: "p", Value: "x"}, `attribute "p" of edge "e2" must be a number`},
		{"ports of dead", gw.Command{Op: gw.SetPorts, Vertex: 9}, "vertex 9 is not live"},
		{"id control", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "a\nb"}, `id "a\nb" holds the control character U+000a at byte 1`},
		{"id del", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "a\x7f"}, "control character U+007f"},
		{"id utf8", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "a\xff"}, "is not valid UTF-8"},
		{"id too long", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: gw.ID(long)}, "id of 9 bytes exceeds the limit of 8"},
		{"edge id control", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, ID: "\r"}, "control character U+000d"},
		{"label control", gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "x\x01"}, "label holds the control character U+0001 at byte 1"},
		{"label utf8", gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "\xc3"}, "label is not valid UTF-8"},
		{"label nonchar", gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "ab\uffff"}, "label holds the noncharacter U+FFFF at byte 2"},
		{"value nonchar", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: "\ufffe"}, "attribute value holds the noncharacter U+FFFE at byte 0"},
		{"id nonchar", gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "x\uffff"}, `id "x\uffff" holds the noncharacter U+FFFF at byte 1`},
		{"edge label control", gw.Command{Op: gw.AddEdge, Kind: eAny, From: a, To: b, FromPort: 1, ToPort: 0, Text: "\x1b"}, "control character U+001b"},
		{"relabel control", gw.Command{Op: gw.Relabel, Vertex: a, Text: "\x00"}, "control character U+0000"},
		{"attr key control", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "no\tte", Value: "x"}, `attribute key "no\tte" may hold only lowercase letters, digits, '-' and '_'`},
		{"attr key space", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "a b", Value: "x"}, `attribute key "a b" may hold only`},
		{"attr key upper", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "Note", Value: "x"}, `attribute key "Note" may hold only`},
		{"attr key utf8", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "é", Value: "x"}, `may hold only`},
		{"attr key kind", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "kind", Value: "x"}, `attribute key "kind" is reserved for the drawing`},
		{"attr key from", gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: 1, Key: "from", Value: "x"}, `attribute key "from" is reserved`},
		{"attr key to", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "to", Value: "x"}, `attribute key "to" is reserved`},
		{"attr key too long", gw.Command{Op: gw.SetAttr, Vertex: a, Key: long, Value: "x"}, "attribute key of 9 bytes"},
		{"attr value control", gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: "\x02"}, "attribute value holds the control character U+0002"},
		{"unknown op", gw.Command{Op: 99}, "unknown op 99"},
	} {
		t.Run(tc.name, func(t *testing.T) { refuse(t, d, tc.cmd, tc.want) })
	}
	// A port's limit: node a's out port takes two edges.
	addEdge(t, d, eOutcome, a, c, 1, gw.None)
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: c, FromPort: 1, ToPort: gw.None}, `port "out" of vertex "a" is full (2 of 2)`)
	// The edge capacity: four edges exist.
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: b, To: c, FromPort: 1, ToPort: gw.None}, "edge limit 4 reached")
	// The attribute capacity: a has p and mode; note makes three.
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "note", Value: "n"})
	refuse(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "more", Value: "m"}, `attribute limit 3 reached on vertex "a"`)
	// The kind's limit and the vertex capacity.
	addVertex(t, d, kSource, "s2")
	addVertex(t, d, kSource, "s3")
	refuse(t, d, gw.Command{Op: gw.AddVertex, Kind: kSource}, `kind "source" limit 3 reached`)
	addVertex(t, d, kNode, "n7")
	refuse(t, d, gw.Command{Op: gw.AddVertex, Kind: kNode}, "vertex limit 7 reached")
}

// TestNumbersAllowed: positions and sizes up to MaxCoord are taken, and
// Place keeps to the same range.
func TestNumbersAllowed(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	v := addVertex(t, d, kNode, "a")
	apply(t, d, gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: gw.MaxCoord, Y: -gw.MaxCoord}})
	apply(t, d, gw.Command{Op: gw.Resize, Vertex: v, Size: gw.Size{W: gw.MaxCoord, H: 0.5}})
	apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kNode, Pos: gw.Point{X: -3, Y: 4}, Size: gw.Size{W: 1, H: 2}})
	if d.Place(v, gw.Point{X: 2e9}, true) || d.Place(v, gw.Point{Y: nan()}, true) || !d.Place(v, gw.Point{X: 5}, true) {
		t.Errorf("Place took a position out of range, or refused one in it")
	}
}

// TestTextAllowed: a label and a value may hold a tab, a newline, a
// carriage return and any valid UTF-8; the refusals above are of
// control characters only.
func TestTextAllowed(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	v := addVertex(t, d, kNode, "a")
	apply(t, d, gw.Command{Op: gw.Relabel, Vertex: v, Text: "one\ttwo\nthree\r é ✓ \x7f \ufffd \U0010fffd"})
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: v, Key: "note", Value: "a\nb\tc"})
	apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kNode, ID: "é-1 x"})
}

func nan() float64 { z := 0.0; return z / z }

// TestEdgeWithoutPorts: an edge into a vertex without ports names None,
// and a port index on it is refused.
func TestEdgeWithoutPorts(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	a := addVertex(t, d, kNode, "a")
	c := addVertex(t, d, kLeaf, "c")
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: a, To: c, FromPort: 1, ToPort: 0}, `port 0 of vertex "c" out of range`)
	e := addEdge(t, d, eOutcome, a, c, 1, gw.None)
	if d.Edges[e].ID != "e1" || d.EdgeByID("e1") != e || d.EdgeByID("e9") != gw.None {
		t.Errorf("edge %+v", d.Edges[e])
	}
}

// TestChange: the change names what a command touched, and deleting a
// vertex names its edges.
func TestChange(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, b, _ := chain(t, d)
	ch := apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 8, Y: 16}})
	if len(ch.Vertices) != 1 || ch.Vertices[0] != a || len(ch.Edges) != 2 || ch.Layout || !d.Vertices[a].Placed {
		t.Errorf("move change %+v", ch)
	}
	ch = apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: b})
	if len(ch.Vertices) != 1 || len(ch.Edges) != 2 || !ch.Layout || d.LiveEdges() != 1 {
		t.Errorf("delete change %+v, live edges %d", ch, d.LiveEdges())
	}
	ch = apply(t, d, gw.Command{Op: gw.Layout, Text: "all"})
	if !ch.Layout || d.Vertices[a].Placed || len(ch.Vertices) != 4 {
		t.Errorf("layout change %+v placed %v", ch, d.Vertices[a].Placed)
	}
}

// TestCycleNames: the refusal names the whole cycle the edge would
// close, and a long one is cut.
func TestCycleNames(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	vs := make([]gw.VertexIndex, 0, len(ids))
	for _, id := range ids {
		vs = append(vs, addVertex(t, d, kNode, id))
	}
	for i := 1; i < len(vs); i++ {
		addEdge(t, d, eOutcome, vs[i-1], vs[i], 1, 0)
	}
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: vs[3], To: vs[1], FromPort: 1, ToPort: 0}, "edge would make a cycle: d → b → c → d")
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: vs[11], To: vs[0], FromPort: 1, ToPort: 0}, "edge would make a cycle: l → a → b → c → d → e → f → g → h → … → l")
	// A deleted edge no longer closes a cycle; an undone delete does again.
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: 1})
	e := addEdge(t, d, eOutcome, vs[3], vs[1], 1, 0)
	apply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: e})
	d.Undo()
	d.Undo()
	d.Undo()
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: vs[3], To: vs[1], FromPort: 1, ToPort: 0}, "edge would make a cycle: d → b → c → d")
}

// TestParallel: of the edges between two vertices, either way, those
// without bends or ports are numbered in lanes by index; a self-loop and
// an edge with bends are alone; and adding one reports the others as
// changed, since their lanes move.
func TestParallel(t *testing.T) {
	d, err := gw.New(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "v", Size: gw.Size{W: 40, H: 20}}}, EdgeKinds: []gw.EdgeKind{{Name: "e"}}}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
			t.Fatal(err)
		}
	}
	add := func(from, to gw.VertexIndex) gw.Change {
		t.Helper()
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: from, To: to, FromPort: gw.None, ToPort: gw.None})
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}
	ab := add(0, 1).Edges[0]
	ba := add(1, 0).Edges[0]
	ac := add(0, 2).Edges[0]
	loop := add(0, 0).Edges[0]
	bent := add(0, 1).Edges[0]
	if !d.SetBends(bent, []gw.Point{{X: 10, Y: 10}}) {
		t.Fatal("bends refused")
	}
	ch := add(0, 1)
	ab2 := ch.Edges[0]
	if len(ch.Edges) != 4 { // the new edge, ab, ba and the bent one
		t.Errorf("adding a third edge between two vertices changed %v", ch.Edges)
	}
	for _, c := range []struct {
		e    gw.EdgeIndex
		k, n int
	}{{ab, 0, 3}, {ba, 1, 3}, {ab2, 2, 3}, {ac, 0, 1}, {loop, 0, 1}, {bent, 0, 1}} {
		if k, n := d.Parallel(c.e); k != c.k || n != c.n {
			t.Errorf("edge %d: lane %d of %d, want %d of %d", c.e, k, n, c.k, c.n)
		}
	}
}
