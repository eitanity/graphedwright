package graphedwright_test

import (
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// The kinds of the test schema, by index.
const (
	kSource gw.KindIndex = iota
	kNode
	kLeaf
)

// The edge kinds of the test schema, by index.
const (
	eTo gw.KindIndex = iota
	eOutcome
	eAny
)

// testSchema: sources (roots, at most three), nodes with an in port and
// an out port that takes two edges, a number attribute p with a default
// and a choice attribute mode, and leaves; edges "to" from a source to a
// node, "outcome" from a node to a node or a leaf, and "any" between any
// kinds. Acyclic.
func testSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 100, H: 24}, Root: true, Max: 3},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36},
				Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right, Max: 2}},
				Attrs: []gw.AttrSpec{
					{Key: "p", Kind: gw.Number, Required: true, Default: "0.5"},
					{Key: "mode", Kind: gw.Choice, Choices: []string{"absolute", "sequential"}, Default: "absolute"},
					{Key: "note", Kind: gw.Text},
				}},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 120, H: 36}, Leaf: true},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "to", From: []gw.KindIndex{kSource}, To: []gw.KindIndex{kNode}, Arrow: true},
			{Name: "outcome", From: []gw.KindIndex{kNode}, To: []gw.KindIndex{kNode, kLeaf}, Weighted: true, Route: gw.Curve,
				Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number}}},
			{Name: "any"},
		},
		Acyclic: true,
	}
}

func newDoc(t *testing.T, caps gw.Caps) *gw.Document {
	t.Helper()
	d, err := gw.New(testSchema(), caps)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// apply applies a command that must succeed.
func apply(t *testing.T, d *gw.Document, c gw.Command) gw.Change {
	t.Helper()
	ch, err := d.Apply(c)
	if err != nil {
		t.Fatalf("%+v: %v", c, err)
	}
	return ch
}

// refuse applies a command that must fail with the message, leaving the
// document as it was.
func refuse(t *testing.T, d *gw.Document, c gw.Command, want string) {
	t.Helper()
	before := snapshot(d)
	_, err := d.Apply(c)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("op %d: got %v, want %q", c.Op, err, want)
	}
	if after := snapshot(d); after != before {
		t.Fatalf("op %d: a refused command changed the document:\n%s\n%s", c.Op, before, after)
	}
}

func addVertex(t *testing.T, d *gw.Document, kind gw.KindIndex, id string) gw.VertexIndex {
	t.Helper()
	ch := apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: id})
	return ch.Vertices[0]
}

func addEdge(t *testing.T, d *gw.Document, kind gw.KindIndex, from, to gw.VertexIndex, fp, tp gw.PortIndex) gw.EdgeIndex {
	t.Helper()
	ch := apply(t, d, gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: fp, ToPort: tp})
	return ch.Edges[0]
}

// chain builds source s → node a → node b → leaf c and returns the
// vertex indices.
func chain(t *testing.T, d *gw.Document) (s, a, b, c gw.VertexIndex) {
	t.Helper()
	s = addVertex(t, d, kSource, "s")
	a = addVertex(t, d, kNode, "a")
	b = addVertex(t, d, kNode, "b")
	c = addVertex(t, d, kLeaf, "c")
	addEdge(t, d, eTo, s, a, gw.None, 0)
	addEdge(t, d, eOutcome, a, b, 1, 0)
	addEdge(t, d, eOutcome, b, c, 1, gw.None)
	return s, a, b, c
}

// snapshot writes the live content of a document in a fixed form, so
// that two states can be compared.
func snapshot(d *gw.Document) string {
	var b strings.Builder
	for i := range d.Vertices {
		v := &d.Vertices[i]
		if !v.Live {
			continue
		}
		b.WriteString("v " + strconv.Itoa(i) + " " + string(v.ID) + " k" + strconv.Itoa(int(v.Kind)) + " " + strconv.Quote(v.Label))
		b.WriteString(" @" + ftoa(v.Pos.X) + "," + ftoa(v.Pos.Y) + " " + ftoa(v.Size.W) + "x" + ftoa(v.Size.H))
		if v.Placed {
			b.WriteString(" placed")
		}
		if v.Parent != gw.None {
			b.WriteString(" in " + strconv.Itoa(int(v.Parent)))
		}
		for _, p := range v.Ports {
			b.WriteString(" port " + p.Name + "/" + strconv.Itoa(int(p.Side)) + "/" + strconv.Itoa(p.Max))
		}
		writeAttrs(&b, v.Attrs)
		b.WriteByte('\n')
	}
	for i := range d.Edges {
		e := &d.Edges[i]
		if !e.Live {
			continue
		}
		b.WriteString("e " + strconv.Itoa(i) + " " + string(e.ID) + " k" + strconv.Itoa(int(e.Kind)) + " " + strconv.Itoa(int(e.From)) + ":" + strconv.Itoa(int(e.FromPort)) + "->" + strconv.Itoa(int(e.To)) + ":" + strconv.Itoa(int(e.ToPort)) + " " + strconv.Quote(e.Label) + " w" + ftoa(e.Weight))
		writeAttrs(&b, e.Attrs)
		b.WriteByte('\n')
	}
	return b.String()
}

func writeAttrs(b *strings.Builder, attrs []gw.Attr) {
	for _, a := range attrs {
		b.WriteString(" " + a.Key + "=" + strconv.Quote(a.Value))
	}
}

func ftoa(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }
