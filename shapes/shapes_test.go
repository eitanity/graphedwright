package shapes_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/shapes"
)

// TestLibraryCompiles: the whole library is one schema's shapes, each
// named as Names says and drawn by a kind, and every path parses.
func TestLibraryCompiles(t *testing.T) {
	lib := shapes.Library()
	names := shapes.Names()
	if len(lib) != shapes.Count {
		t.Fatalf("%d shapes, want %d", len(lib), shapes.Count)
	}
	s := gw.Schema{Shapes: lib}
	for i, def := range lib {
		if def.Name != names[i] {
			t.Errorf("shape %d is %q, want %q", i, def.Name, names[i])
		}
		s.VertexKinds = append(s.VertexKinds, gw.VertexKind{Name: def.Name, Shape: gw.Provided + gw.Shape(i), Size: def.Box}) //nolint:gosec // a dozen shapes
	}
	k, err := gw.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	for i := range lib {
		v := k.ShapeOf(gw.Provided + gw.Shape(i)) //nolint:gosec // a dozen shapes
		if !v.Valid() || v.Name() != names[i] || v.Paths() != len(lib[i].Paths) {
			t.Errorf("shape %d compiled as %q with %d paths", i, v.Name(), v.Paths())
		}
	}
}

// TestNamed: each name gives its shape alone, an unknown name or a
// built-in's none, and each call a shape of its own, so that changing
// one leaves the next as the library has it.
func TestNamed(t *testing.T) {
	for _, name := range shapes.Names() {
		def, ok := shapes.Named(name)
		if !ok || def.Name != name {
			t.Errorf("%q: got %q, %v", name, def.Name, ok)
			continue
		}
		if _, err := gw.Compile(gw.Schema{Shapes: []gw.ShapeDef{def}, VertexKinds: []gw.VertexKind{{Name: "k", Shape: gw.Provided}}}); err != nil {
			t.Errorf("%q alone: %v", name, err)
		}
	}
	for _, name := range []string{"", "rect", "circle", "Bullseye", "bullseye "} {
		if _, ok := shapes.Named(name); ok {
			t.Errorf("%q is a library shape", name)
		}
	}
	a, _ := shapes.Named(shapes.Hexagon)
	a.Polygon[0].X, a.Paths[0].D = 99, "M 0 0 Z"
	b, _ := shapes.Named(shapes.Hexagon)
	if b.Polygon[0].X != 20 || b.Paths[0].D == "M 0 0 Z" {
		t.Errorf("a change to one hexagon reached the next: %v, %q", b.Polygon[0], b.Paths[0].D)
	}
}

// TestBarTurns: the bar alone turns with the flow, standing across a
// left-to-right one; the round shapes put their names below, the bar
// none, and the rest inside.
func TestBarTurns(t *testing.T) {
	for _, def := range shapes.Library() {
		turns := def.Name == shapes.Bar
		if def.Turns != turns {
			t.Errorf("%q turns %v", def.Name, def.Turns)
		}
		want := gw.LabelInside
		switch def.Name {
		case shapes.FilledCircle, shapes.Bullseye, shapes.CrossedCircle, shapes.DoubleCircle:
			want = gw.LabelBelow
		case shapes.Bar:
			want = gw.LabelNone
		}
		if def.Label != want {
			t.Errorf("%q: label %d, want %d", def.Name, def.Label, want)
		}
	}
	bar, _ := shapes.Named(shapes.Bar)
	if bar.Box.H <= bar.Box.W {
		t.Errorf("the bar is %v, not across a left-to-right flow", bar.Box)
	}
}
