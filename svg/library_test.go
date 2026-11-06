package svg_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	lib "code.eitanity.com/graphedwright/shapes"
	"code.eitanity.com/graphedwright/svg"
)

// library: every shape of the library, a kind each at its box's size,
// in rows of four 220 apart, each joined to the next by a straight
// line clipped to both outlines; in flow f, where the bar turns.
func library(t testing.TB, f gw.Flow) (*gw.Document, svg.Options) {
	t.Helper()
	defs := lib.Library()
	s := gw.Schema{Shapes: defs, EdgeKinds: []gw.EdgeKind{{Name: "line", Arrow: true}}}
	for i, def := range defs {
		s.VertexKinds = append(s.VertexKinds, gw.VertexKind{Name: def.Name, Shape: gw.Provided + gw.Shape(i), Size: def.Box}) //nolint:gosec // a dozen shapes
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(f)
	prev := gw.VertexIndex(gw.None)
	for i, def := range defs {
		v := vertex(t, d, gw.KindIndex(i), def.Name, float64(220*(i%4)), float64(160*(i/4))) //nolint:gosec // a dozen shapes
		if prev != gw.None {
			edge(t, d, 0, prev, v, gw.None, gw.None, "", 0)
		}
		prev = v
	}
	return d, svg.Options{Title: "The shape library"}
}

func libraryAcross(t testing.TB) (*gw.Document, svg.Options) { return library(t, gw.LeftToRight) }
func libraryDown(t testing.TB) (*gw.Document, svg.Options)   { return library(t, gw.TopToBottom) }

// stateMachine: the demo's state machine example, as the built-in
// schema draws it.
func stateMachine(t testing.TB) (*gw.Document, svg.Options) {
	t.Helper()
	data, err := os.ReadFile("../web/examples/state-machine.json") //nolint:gosec // the demo's example, by fixed name
	if err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	return d, svg.Options{Title: "The editor's gestures"}
}

// TestLibraryRender: the library's shapes in either flow and the state
// machine example, whose drawings TestGolden freezes, render well-formed
// and without allocating.
func TestLibraryRender(t *testing.T) {
	r := svg.New()
	for _, ex := range []struct {
		name  string
		build func(testing.TB) (*gw.Document, svg.Options)
	}{{"library", libraryAcross}, {"library-down", libraryDown}, {"state-machine", stateMachine}} {
		d, opts := ex.build(t)
		var buf bytes.Buffer
		if err := r.Render(d, opts, &buf); err != nil {
			t.Fatal(err)
		}
		checkXML(t, buf.Bytes(), d)
		if n := testing.AllocsPerRun(20, func() {
			if err := r.Render(d, opts, io.Discard); err != nil {
				t.Fatal(err)
			}
		}); n > 0 {
			t.Errorf("%s: %g allocations per run", ex.name, n)
		}
	}
}
