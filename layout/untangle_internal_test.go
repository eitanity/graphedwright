package layout

import (
	"slices"
	"testing"
)

// leg is one leg given an upright: its gap, the y across the flow it
// leaves the earlier layer at and reaches the later one at, and whether
// it stacks back from the later layer.
type leg struct {
	gap    int32
	y0, y1 float64
	hi     bool
}

// setLegs puts legs into a layouter's leg arrays, each indexed by its
// place in the list.
func setLegs(t *testing.T, legs []leg) *Layouter {
	t.Helper()
	d, _ := levelDoc(t)
	l := New(d.Caps())
	for i, g := range legs {
		l.zIdx[i], l.zGap[i], l.zY0[i], l.zY1[i], l.zHi[i] = int32(i), g.gap, g.y0, g.y1, g.hi //nolint:gosec // a few
	}
	return l
}

// TestSortLegs: the legs are sorted by gap; in one gap, those going
// lower come first, by the y they leave and then the y they reach, then
// those going higher, the highest first.
func TestSortLegs(t *testing.T) {
	legs := []leg{
		{gap: 2, y0: 10, y1: 5},
		{gap: 2, y0: 10, y1: 0},
		{gap: 2, y0: 0, y1: 20},
		{gap: 2, y0: 0, y1: 30},
		{gap: 2, y0: 5, y1: 0},
		{gap: 1, y0: 0, y1: 40},
	}
	l := setLegs(t, legs)
	l.sortLegs(len(legs))
	if got, want := l.zIdx[:len(legs)], []int32{5, 4, 1, 0, 3, 2}; !slices.Equal(got, want) {
		t.Errorf("sorted %v, want %v", got, want)
	}
}

// TestUntangle: a leg that leaves where one before it in its stack
// reaches moves to just before the first such; one in the other stack
// stays.
func TestUntangle(t *testing.T) {
	for _, c := range []struct {
		name string
		legs []leg
		want []int32
	}{
		{
			name: "after",
			legs: []leg{{y0: 0, y1: 50}, {y0: 50, y1: 80}},
			want: []int32{1, 0},
		},
		{
			name: "the first",
			legs: []leg{{y0: 90, y1: 70}, {y0: 0, y1: 50}, {y0: 10, y1: 50}, {y0: 50.5, y1: 80}},
			want: []int32{0, 3, 1, 2},
		},
		{
			name: "the other stack",
			legs: []leg{{y0: 0, y1: 50, hi: true}, {y0: 50, y1: 80}},
			want: []int32{0, 1},
		},
		{
			name: "apart",
			legs: []leg{{y0: 0, y1: 50}, {y0: 51, y1: 80}},
			want: []int32{0, 1},
		},
	} {
		l := setLegs(t, c.legs)
		l.untangle(0, len(c.legs))
		if got := l.zIdx[:len(c.legs)]; !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
