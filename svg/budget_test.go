package svg_test

import (
	"io"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// TestAllocRender: a renderer renders without allocating, and a hit
// test allocates nothing.
func TestAllocRender(t *testing.T) {
	d, opts := diamond(t)
	r := svg.New()
	if n := testing.AllocsPerRun(20, func() {
		if err := r.Render(d, opts, io.Discard); err != nil {
			t.Fatal(err)
		}
	}); n > 0 {
		t.Errorf("render: %g allocations per run", n)
	}
	if n := testing.AllocsPerRun(20, func() { svg.HitTest(d, gw.Point{X: 220, Y: 100}) }); n > 0 {
		t.Errorf("hit test: %g allocations per run", n)
	}
	for name, example := range map[string]func(testing.TB) (*gw.Document, svg.Options){"compartments": compartments, "label formats": formats, "own ports": ownPorts, "cut texts": cutTexts, "a hovered vertex": cutTextsHover} {
		d, opts := example(t)
		if n := testing.AllocsPerRun(20, func() {
			if err := r.Render(d, opts, io.Discard); err != nil {
				t.Fatal(err)
			}
		}); n > 0 {
			t.Errorf("render with %s: %g allocations per run", name, n)
		}
	}
}

func BenchmarkRender(b *testing.B) {
	d := big(b, 2000, 5000)
	r := svg.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.Render(d, svg.Options{}, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHitTest(b *testing.B) {
	d := big(b, 2000, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svg.HitTest(d, gw.Point{X: 3, Y: 3})
	}
}
