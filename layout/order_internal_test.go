package layout

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestInversionsWide: the inversions of a descending run of 100 000
// values, n(n-1)/2 = 4 999 950 000, are counted exactly: past 2^32, so
// the count would wrap where an int has 32 bits.
func TestInversionsWide(t *testing.T) {
	const n = 100_000
	l := New(gw.Caps{Vertices: n})
	a := make([]int32, n)
	for i := range a {
		a[i] = int32(n - 1 - i)
	}
	if got, want := l.inversions(a, n), int64(n)*(n-1)/2; got != want {
		t.Errorf("inversions %d, want %d", got, want)
	}
}
