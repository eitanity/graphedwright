package layout

import "testing"

// TestStraighten: the dummies of a long edge are brought into one row
// where they fit, the gap clear of their neighbours in their layers: the
// row of the first of them that the most of them fit at. One that does
// not fit does once another edge's dummy has moved out of its way.
func TestStraighten(t *testing.T) {
	const c, a0, a1, b0, b1 = 0, 1, 2, 3, 4
	for _, tc := range []struct {
		name  string
		order [2][]int32
		y     []float64
		edges int
		want  []float64
	}{
		{
			// Edge a's dummies fit neither row, c holding 0 in the second
			// layer and b0 48 in the first, until edge b's move to 96
			// makes room at 48.
			name:  "after another",
			order: [2][]int32{{a0, b0}, {c, a1, b1}},
			y:     []float64{c: 0, a0: 0, a1: 48, b0: 48, b1: 96},
			edges: 2,
			want:  []float64{c: 0, a0: 48, a1: 48, b0: 96, b1: 96},
		},
		{
			// At 40, a1 would be clear of c but within the gap of it.
			name:  "the gap",
			order: [2][]int32{{a0}, {c, a1}},
			y:     []float64{c: 0, a0: 40, a1: 64},
			edges: 1,
			want:  []float64{c: 0, a0: 64, a1: 64},
		},
		{
			name:  "the first",
			order: [2][]int32{{a0}, {a1}},
			y:     []float64{a0: 0, a1: 48},
			edges: 1,
			want:  []float64{a0: 0, a1: 0},
		},
	} {
		d, _ := levelDoc(t)
		l := New(d.Caps())
		l.layers, l.edges = 2, tc.edges
		l.keys[0], l.keys[1], l.keys[2] = 0, 0, 0
		for k, vs := range tc.order {
			for _, v := range vs {
				l.order[l.keys[2]] = v
				l.keys[2]++
			}
			l.keys[k+1] = l.keys[2]
		}
		for k, vs := range tc.order {
			for _, v := range vs {
				l.layer[v], l.h[v] = int32(k), 24 //nolint:gosec // two layers
			}
		}
		copy(l.y, tc.y)
		copy(l.dFirst, []int32{a0, b0})
		copy(l.dN, []int32{2, 2})
		l.straighten(24)
		for v, want := range tc.want {
			if l.y[v] != want {
				t.Errorf("%s: node %d at %v, want %v", tc.name, v, l.y[v], want)
			}
		}
	}
}
