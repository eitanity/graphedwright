package svg_test

import (
	"io"
	"os"
	"testing"
	"time"

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
}

// TestRenderBudget: a 2 000-vertex, 5 000-edge document renders in
// under 50 ms. The check is skipped under the race detector, whose
// instrumentation is not what the budget is about.
func TestRenderBudget(t *testing.T) {
	if os.Getenv("GRAPHEDWRIGHT_RACE") == "1" {
		t.Skip("timing under the race detector")
	}
	d := big(t, 2000, 5000)
	if d.LiveEdges() < 5000 {
		t.Fatalf("built %d edges", d.LiveEdges())
	}
	r := svg.New()
	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := r.Render(d, svg.Options{}, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
	per := time.Duration(res.NsPerOp())
	t.Logf("render of %d vertices and %d edges: %v", d.LiveVertices(), d.LiveEdges(), per)
	if per > 50*time.Millisecond {
		t.Errorf("render took %v, over the 50 ms budget", per)
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
