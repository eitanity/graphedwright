package graphedwright_test

import (
	"slices"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestLabelFormat: a format draws its text, the label and attributes,
// and the text either side of a placeholder only when its value is set.
func TestLabelFormat(t *testing.T) {
	const usage = "{label}{ : |type}{ [|mult|]}"
	const transition = "{trigger}{ [|guard|]}{ / |effect}"
	for _, tc := range []struct {
		format, label string
		attrs         []gw.Attr
		want          string
	}{
		{"", "wheels", nil, "wheels"},
		{usage, "wheels", []gw.Attr{{Key: "type", Value: "Wheel"}, {Key: "mult", Value: "4"}}, "wheels : Wheel [4]"},
		{usage, "wheels", []gw.Attr{{Key: "mult", Value: "4"}}, "wheels [4]"},
		{usage, "wheels", []gw.Attr{{Key: "type", Value: ""}}, "wheels"},
		{usage, "", []gw.Attr{{Key: "type", Value: "Wheel"}}, " : Wheel"},
		{usage, "", nil, ""},
		{transition, "ignored", []gw.Attr{{Key: "trigger", Value: "start"}, {Key: "effect", Value: "run()"}}, "start / run()"},
		{transition, "", []gw.Attr{{Key: "guard", Value: "ready"}}, " [ready]"},
		{"part {label} of {owner}", "wheel", []gw.Attr{{Key: "owner", Value: "car"}}, "part wheel of car"},
		{"part {label} of {owner}", "wheel", nil, "part wheel of "},
		{"«{label}»", "ünïcode", nil, "«ünïcode»"},
	} {
		got := string(gw.AppendLabel(nil, tc.format, tc.label, tc.attrs))
		if got != tc.want {
			t.Errorf("%q with %q %v: %q, want %q", tc.format, tc.label, tc.attrs, got, tc.want)
		}
		if n := gw.LabelRunes(tc.format, tc.label, tc.attrs); n != len([]rune(tc.want)) {
			t.Errorf("%q with %q %v: %d characters, want %d", tc.format, tc.label, tc.attrs, n, len([]rune(tc.want)))
		}
	}
}

// TestLabelFormatRefusals: a schema refuses a format with braces
// unpaired or nested, more than two bars, or a key that is neither the
// label nor an attribute of the kind, on vertex and edge kinds alike.
func TestLabelFormatRefusals(t *testing.T) {
	for _, tc := range []struct{ format, want string }{
		{"{label", "brace without its pair"},
		{"label}", "brace without its pair"},
		{"}{label}", "brace without its pair"},
		{"{a{label}}", "brace without its pair"},
		{"{a|label|b|c}", "more than two bars"},
		{"{weight}", `names "weight", neither label nor an attribute of the kind`},
		{"{ : |}", `names "", neither label nor an attribute of the kind`},
		{"{label}\x01", "label format holds the control character U+0001"},
	} {
		for _, edge := range []bool{false, true} {
			s := gw.Schema{
				VertexKinds: []gw.VertexKind{{Name: "v", Size: gw.Size{W: 80, H: 30}, Attrs: []gw.AttrSpec{{Key: "type"}}}},
				EdgeKinds:   []gw.EdgeKind{{Name: "e", Attrs: []gw.AttrSpec{{Key: "type"}}}},
			}
			if edge {
				s.EdgeKinds[0].LabelFormat = tc.format
			} else {
				s.VertexKinds[0].LabelFormat = tc.format
			}
			if _, err := makeDoc(s, gw.Caps{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%q on edge %v: got %v, want %q", tc.format, edge, err, tc.want)
			}
		}
	}
	ok := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "v", Size: gw.Size{W: 80, H: 30}, Attrs: []gw.AttrSpec{{Key: "type"}}, LabelFormat: "{label}{ : |type}"}},
		EdgeKinds:   []gw.EdgeKind{{Name: "e", Attrs: []gw.AttrSpec{{Key: "type"}}, LabelFormat: "«flow» {type|label|}"}},
	}
	if _, err := makeDoc(ok, gw.Caps{}); err != nil {
		t.Errorf("formats refused: %v", err)
	}
}

// TestLabelFormatChange: a SetAttr on a key a format names reports the
// element, and the label drawn grows with it.
func TestLabelFormatChange(t *testing.T) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "usage", Size: gw.Size{W: 120, H: 36}, Attrs: []gw.AttrSpec{{Key: "type"}}, LabelFormat: "{label}{ : |type}"}},
		EdgeKinds:   []gw.EdgeKind{{Name: "transition", Attrs: []gw.AttrSpec{{Key: "guard"}}, LabelFormat: "{label}{ [|guard|]}"}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	a := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, Text: "wheels"}).Vertices[0]
	b := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, Text: "engine"}).Vertices[0]
	e := mustApply(t, d, gw.Command{Op: gw.AddEdge, Kind: 0, From: a, To: b, FromPort: gw.None, ToPort: gw.None, Text: "go"}).Edges[0]
	if n := d.VertexLabelRunes(a); n != 6 {
		t.Errorf("vertex label %d characters, want 6", n)
	}
	if ch := mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "type", Value: "Wheel"}); !slices.Contains(ch.Vertices, a) || d.VertexLabelRunes(a) != len("wheels : Wheel") {
		t.Errorf("vertex: change %+v, %d characters", ch, d.VertexLabelRunes(a))
	}
	if ch := mustApply(t, d, gw.Command{Op: gw.SetAttr, Vertex: gw.None, Edge: e, Key: "guard", Value: "ready"}); !slices.Contains(ch.Edges, e) || d.EdgeLabelRunes(e) != len("go [ready]") {
		t.Errorf("edge: change %+v, %d characters", ch, d.EdgeLabelRunes(e))
	}
}
