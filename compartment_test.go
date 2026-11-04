package graphedwright_test

import (
	"bytes"
	"slices"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
)

// compartmentSchema has a class kind that shows two of its attributes
// as compartments, keeps a third out of sight, and grows to 160 wide.
func compartmentSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{{
			Name: "class", Shape: gw.Rect, Size: gw.Size{W: 100, H: 40},
			Attrs:        []gw.AttrSpec{{Key: "fields"}, {Key: "methods"}, {Key: "note"}},
			Compartments: []string{"fields", "methods"}, MaxWidth: 160,
		}},
		EdgeKinds: []gw.EdgeKind{{Name: "uses"}},
	}
}

// TestCompartmentSize: a vertex is as high as its label's room and its
// compartments' lines, an empty one left out, as wide as its widest line
// up to the kind's MaxWidth, and never smaller than its kind.
func TestCompartmentSize(t *testing.T) {
	d, err := makeDoc(compartmentSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "Order", Pos: gw.Point{X: 200, Y: 100}})
	if err != nil {
		t.Fatal(err)
	}
	v := ch.Vertices[0]
	set := func(key, value string) gw.Change {
		t.Helper()
		ch, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: v, Key: key, Value: value})
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}
	line := func(n int) float64 { return float64(n)*gw.CompartmentLine + 2*gw.CompartmentPad }
	for _, tc := range []struct {
		key, value string
		want       gw.Size
	}{
		{"fields", "id", gw.Size{W: 100, H: gw.CompartmentTitle + line(1)}},
		{"fields", "id : Integer\ntotal : Real\r\nplaced: Date", gw.Size{W: 100, H: gw.CompartmentTitle + line(3)}},
		{"methods", "cancel()", gw.Size{W: 100, H: gw.CompartmentTitle + line(3) + line(1)}},
		// Seventeen characters, 119 wide with the inset.
		{"methods", "cancel(reason)   ", gw.Size{W: 17*gw.CharWidth + 2*gw.CompartmentInset, H: gw.CompartmentTitle + line(3) + line(1)}},
		{"methods", "a line far too long to be held by a vertex of the kind", gw.Size{W: 160, H: gw.CompartmentTitle + line(3) + line(1)}},
		{"methods", "", gw.Size{W: 100, H: gw.CompartmentTitle + line(3)}},
		{"fields", "", gw.Size{W: 100, H: 40}},
	} {
		set(tc.key, tc.value)
		if got := d.Vertices[v].Size; got != tc.want {
			t.Errorf("%s = %q: size %v, want %v", tc.key, tc.value, got, tc.want)
		}
		if got := d.Vertices[v].Pos; got != (gw.Point{X: 200, Y: 100}) {
			t.Errorf("%s = %q: moved to %v", tc.key, tc.value, got)
		}
	}
	// A key that is not a compartment's changes no size and leaves the
	// layout as it is.
	if ch := set("note", "a long note that would need a wide vertex to hold it"); ch.Layout || d.Vertices[v].Size != (gw.Size{W: 100, H: 40}) {
		t.Errorf("a key not shown: layout stale %v, size %v", ch.Layout, d.Vertices[v].Size)
	}
}

// TestCompartmentStale: a SetAttr that resizes a vertex reports it and
// its edges and marks the layout stale, and its undo restores the size
// and does the same; a resize is never below what the compartments
// need, and a saved document loads at the size it was saved at.
func TestCompartmentStale(t *testing.T) {
	d, err := makeDoc(compartmentSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	a := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "a", Text: "a", Pos: gw.Point{X: 100, Y: 100}}).Vertices[0]
	b := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "b", Text: "b", Pos: gw.Point{X: 300, Y: 100}}).Vertices[0]
	e := mustApply(t, d, gw.Command{Op: gw.AddEdge, Kind: 0, From: a, To: b, FromPort: gw.None, ToPort: gw.None}).Edges[0]
	before := d.Vertices[a].Size
	ch := mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "fields", Value: "x\ny\nz"})
	grown := d.Vertices[a].Size
	if !ch.Layout || grown == before || !slices.Contains(ch.Vertices, a) || !slices.Contains(ch.Edges, e) {
		t.Fatalf("set: layout stale %v, size %v from %v, change %+v", ch.Layout, grown, before, ch)
	}
	ch, ok := d.Undo()
	if !ok || !ch.Layout || d.Vertices[a].Size != before || !slices.Contains(ch.Vertices, a) {
		t.Fatalf("undo: layout stale %v, size %v, want %v", ch.Layout, d.Vertices[a].Size, before)
	}
	ch, ok = d.Redo()
	if !ok || !ch.Layout || d.Vertices[a].Size != grown {
		t.Fatalf("redo: layout stale %v, size %v, want %v", ch.Layout, d.Vertices[a].Size, grown)
	}

	// A resize below the compartments is to them; one above them is
	// kept when they shrink.
	mustApply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 60, H: 30}})
	if got := d.Vertices[a].Size; got != (gw.Size{W: 60, H: grown.H}) {
		t.Errorf("resize below: %v, want %v", got, gw.Size{W: 60, H: grown.H})
	}
	mustApply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 150, H: 120}})
	if ch := mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "fields", Value: ""}); ch.Layout || d.Vertices[a].Size != (gw.Size{W: 150, H: 120}) {
		t.Errorf("shrunk below its own size: layout stale %v, size %v", ch.Layout, d.Vertices[a].Size)
	}
	mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "fields", Value: "1\n2\n3\n4\n5\n6\n7"})
	mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: b, Key: "methods", Value: "a method with a long name"})

	var text bytes.Buffer
	if err := doc.Encode(d, "classes", &text); err != nil {
		t.Fatal(err)
	}
	again, err := doc.Decode(text.Bytes(), doc.Options{Schemas: []doc.Named{{Name: "classes", Kinds: d.Kinds()}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []gw.VertexIndex{a, b} {
		w := again.VertexByID(d.Vertices[v].ID)
		if got, want := again.Vertices[w], d.Vertices[v]; got.Pos != want.Pos || got.Size != want.Size {
			t.Errorf("%s loaded at %v %v, saved at %v %v", want.ID, got.Pos, got.Size, want.Pos, want.Size)
		}
	}
}

func mustApply(t *testing.T, d *gw.Document, c gw.Command) gw.Change {
	t.Helper()
	ch, err := d.Apply(c)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}
