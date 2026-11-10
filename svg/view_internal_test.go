package svg

import (
	"math"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestUsable: a viewport is drawn when its corner is within MaxCoord of
// the origin, either way on either axis, and its width and height are
// positive and at most MaxCoord; any other gives way to the drawing's
// bounds.
func TestUsable(t *testing.T) {
	const m = gw.MaxCoord
	for _, c := range []struct {
		v    gw.Viewport
		want bool
	}{
		{gw.Viewport{X: 0, Y: 0, W: 100, H: 50}, true},
		{gw.Viewport{X: -m, Y: m, W: m, H: m}, true},
		{gw.Viewport{X: m, Y: -m, W: 1, H: 1}, true},
		{gw.Viewport{X: -m * 2, Y: 0, W: 1, H: 1}, false},
		{gw.Viewport{X: m * 2, Y: 0, W: 1, H: 1}, false},
		{gw.Viewport{X: 0, Y: -m * 2, W: 1, H: 1}, false},
		{gw.Viewport{X: 0, Y: m * 2, W: 1, H: 1}, false},
		{gw.Viewport{X: math.NaN(), Y: 0, W: 1, H: 1}, false},
		{gw.Viewport{X: 0, Y: 0, W: 0, H: 1}, false},
		{gw.Viewport{X: 0, Y: 0, W: 1, H: 0}, false},
		{gw.Viewport{X: 0, Y: 0, W: m * 2, H: 1}, false},
		{gw.Viewport{X: 0, Y: 0, W: 1, H: m * 2}, false},
		{gw.Viewport{}, false},
	} {
		if got := usable(c.v); got != c.want {
			t.Errorf("%+v: usable %v, want %v", c.v, got, c.want)
		}
	}
}
