package graphedwright_test

import (
	"math"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestBoundedNumber: a bounded number takes a value from its least to
// its greatest, both included, and refuses one outside, saying the
// bounds, with the document left as it was; a bound may be infinite,
// for a number bounded on one side, or on neither.
func TestBoundedNumber(t *testing.T) {
	s := testSchema()
	inf := math.Inf(1)
	s.VertexKinds[1].Attrs = append(s.VertexKinds[1].Attrs,
		gw.AttrSpec{Key: "share", Kind: gw.Number, Bounded: true, Min: 0, Max: 1},
		gw.AttrSpec{Key: "rate", Kind: gw.Number, Bounded: true, Min: 0, Max: inf},
		gw.AttrSpec{Key: "loss", Kind: gw.Number, Bounded: true, Min: -inf, Max: 10},
		gw.AttrSpec{Key: "any", Kind: gw.Number, Bounded: true, Min: -inf, Max: inf},
	)
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kNode, Text: "n"})
	if err != nil {
		t.Fatal(err)
	}
	v := ch.Vertices[0]
	set := func(key, value string) error {
		_, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: v, Key: key, Value: value})
		return err
	}
	for _, tc := range []struct{ key, value string }{
		{"share", "0"}, {"share", "1"}, {"share", "0.25"}, {"rate", "0"}, {"rate", "1e300"},
		{"loss", "10"}, {"loss", "-1e300"}, {"any", "-1e300"}, {"any", "1e300"},
	} {
		if err := set(tc.key, tc.value); err != nil {
			t.Errorf("%s = %s: %v", tc.key, tc.value, err)
		}
	}
	for _, tc := range []struct{ key, value, want string }{
		{"share", "1.5", `attribute "share" of vertex "v1" must be a number from 0 to 1: "1.5"`},
		{"share", "-0.1", "must be a number from 0 to 1"},
		{"share", "x", "must be a number from 0 to 1"},
		{"rate", "-1", "must be a number at least 0"},
		{"loss", "10.5", "must be a number at most 10"},
		{"any", "Inf", `must be a number: "Inf"`},
	} {
		err := set(tc.key, tc.value)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s = %s: got %v, want %q", tc.key, tc.value, err, tc.want)
		}
	}
	if got := gw.AttrValue(d.Vertices[v].Attrs, "share"); got != "0.25" {
		t.Errorf("share after the refusals: %q, want the last accepted, 0.25", got)
	}
}
