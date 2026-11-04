package graphedwright_test

import (
	"math"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestSetView: a viewport is taken when its corner is within MaxCoord
// of the origin and its width and height are from 0 to MaxCoord, and
// refused otherwise, leaving the one before.
func TestSetView(t *testing.T) {
	d := containerDoc(t)
	kept := gw.Viewport{X: -5, Y: 5, W: 300, H: 200}
	if !d.SetView(kept) || d.View() != kept {
		t.Fatalf("a viewport: %v", d.View())
	}
	for _, v := range []gw.Viewport{
		{X: -gw.MaxCoord, Y: gw.MaxCoord, W: gw.MaxCoord, H: gw.MaxCoord},
		{}, // empty: the whole drawing
	} {
		if !d.SetView(v) || d.View() != v {
			t.Errorf("refused %v", v)
		}
	}
	d.SetView(kept)
	for _, v := range []gw.Viewport{
		{X: math.NaN(), W: 1, H: 1},
		{Y: math.NaN(), W: 1, H: 1},
		{X: math.Inf(1), W: 1, H: 1},
		{X: -gw.MaxCoord * 1.5, W: 1, H: 1},
		{Y: gw.MaxCoord * 1.5, W: 1, H: 1},
		{W: -1, H: 1},
		{W: 1, H: -1},
		{W: math.NaN(), H: 1},
		{W: 1, H: math.NaN()},
		{W: gw.MaxCoord * 1.5, H: 1},
		{W: 1, H: math.Inf(1)},
	} {
		if d.SetView(v) || d.View() != kept {
			t.Errorf("took %v: %v", v, d.View())
		}
	}
}

// TestSetFlow: each flow is taken; any other is refused, leaving the
// one before.
func TestSetFlow(t *testing.T) {
	d := containerDoc(t)
	for _, f := range []gw.Flow{gw.TopToBottom, gw.LeftToRight} {
		if !d.SetFlow(f) || d.Flow() != f {
			t.Errorf("refused %v", f)
		}
	}
	if d.SetFlow(gw.TopToBottom+1) || d.Flow() != gw.LeftToRight {
		t.Errorf("took an unknown flow: %v", d.Flow())
	}
}
