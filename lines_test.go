package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestLabelLines: a label's size is its widest line's characters and its
// number of lines, a line ending at a line feed, a carriage return, or
// the two together, wherever in the format's parts it falls.
func TestLabelLines(t *testing.T) {
	for _, tc := range []struct {
		format, label string
		attrs         []gw.Attr
		widest, lines int
	}{
		{"", "", nil, 0, 1},
		{"", "ab", nil, 2, 1},
		{"", "ab\ncde", nil, 3, 2},
		{"", "abc\r\nde", nil, 3, 2},
		{"", "ab\rcde", nil, 3, 2},
		{"", "a\r\r\nb", nil, 1, 3},
		{"", "\n", nil, 0, 2},
		{"", "ünï\nx", nil, 3, 2},
		{"{label}\n{|type}", "abc", []gw.Attr{{Key: "type", Value: "de"}}, 3, 2},
		// A return at the end of the label and a feed at the start of
		// the text after it are one break.
		{"{label}\nb", "a\r", nil, 1, 2},
	} {
		widest, lines := gw.LabelLines(tc.format, tc.label, gw.AttrsOf(tc.attrs))
		if widest != tc.widest || lines != tc.lines {
			t.Errorf("%q with %q: %d wide, %d lines; want %d, %d", tc.format, tc.label, widest, lines, tc.widest, tc.lines)
		}
		if n := gw.LabelRunes(tc.format, tc.label, gw.AttrsOf(tc.attrs)); n != tc.widest {
			t.Errorf("%q with %q: %d characters, want the widest line's %d", tc.format, tc.label, n, tc.widest)
		}
	}
}

// TestNextLine: a line ends at a line feed, a carriage return, or the
// two together.
func TestNextLine(t *testing.T) {
	for _, tc := range []struct {
		s, line, rest string
		more          bool
	}{
		{"ab", "ab", "", false},
		{"a\nb", "a", "b", true},
		{"a\r\nb", "a", "b", true},
		{"a\rb", "a", "b", true},
		{"a\r", "a", "", true},
		{"a\r\r\nb", "a", "\r\nb", true},
	} {
		line, rest, more := gw.NextLine(tc.s)
		if line != tc.line || rest != tc.rest || more != tc.more {
			t.Errorf("%q: %q %q %v, want %q %q %v", tc.s, line, rest, more, tc.line, tc.rest, tc.more)
		}
	}
}

// TestTitleLines: a container's title, and the label above a vertex's
// compartments, have a line's more room for each line after their
// first, which a relabel fits them to, and gives back.
func TestTitleLines(t *testing.T) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 40, H: 20}},
			{Name: "zone", Size: gw.Size{W: 40, H: 20}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "line"}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex) gw.VertexIndex {
		t.Helper()
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, Text: "a"})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	apply := func(c gw.Command) {
		t.Helper()
		if _, err := d.Apply(c); err != nil {
			t.Fatal(err)
		}
	}
	z, m := add(1), add(0)
	apply(gw.Command{Op: gw.SetParent, Vertex: m, To: z})
	// The class in a document of its own, with no container, which a
	// relabel would refit anyway.
	cd, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "class", Size: gw.Size{W: 100, H: 40}, Attrs: []gw.AttrSpec{{Key: "fields"}}, Compartments: []string{"fields"}}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := cd.Apply(gw.Command{Op: gw.AddVertex, Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	c := ch.Vertices[0]
	capply := func(cmd gw.Command) {
		t.Helper()
		if _, err := cd.Apply(cmd); err != nil {
			t.Fatal(err)
		}
	}
	capply(gw.Command{Op: gw.SetAttr, Vertex: c, Key: "fields", Value: "id"})
	zone, class := d.Vertex(z).Size(), cd.Vertex(c).Size()
	if want := 20 + 2*gw.ContainerPad + float64(gw.ContainerTitle); zone.H != want || d.TitleHeight(z) != gw.ContainerTitle {
		t.Fatalf("one line: zone %v, title %v; want %v high", zone, d.TitleHeight(z), want)
	}
	for _, text := range []string{"a\nb", "a\nb\r\nc"} {
		apply(gw.Command{Op: gw.Relabel, Vertex: z, Edge: gw.None, Text: text})
		capply(gw.Command{Op: gw.Relabel, Vertex: c, Edge: gw.None, Text: text})
		more := gw.MoreLines(d.VertexLabelLines(z))
		if got := d.Vertex(z).Size(); got.H != zone.H+more || got.W != zone.W {
			t.Errorf("%q: zone %v, want %v high", text, got, zone.H+more)
		}
		if got := cd.TitleHeight(c); got != gw.CompartmentTitle+more {
			t.Errorf("%q: class's title %v, want %v", text, got, gw.CompartmentTitle+more)
		}
		if got := cd.Vertex(c).Size(); got.H != class.H+more {
			t.Errorf("%q: class %v, want %v high", text, got, class.H+more)
		}
		// What the zone holds stays where it was: the zone grows up.
		if got := d.Vertex(m).Pos(); got != (gw.Point{}) {
			t.Errorf("%q: the member moved to %v", text, got)
		}
	}
	apply(gw.Command{Op: gw.Relabel, Vertex: z, Edge: gw.None, Text: "a"})
	capply(gw.Command{Op: gw.Relabel, Vertex: c, Edge: gw.None, Text: "a"})
	if d.Vertex(z).Size() != zone || cd.Vertex(c).Size() != class {
		t.Errorf("back to one line: %v %v, want %v %v", d.Vertex(z).Size(), cd.Vertex(c).Size(), zone, class)
	}
}
