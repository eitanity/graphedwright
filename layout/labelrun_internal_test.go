package layout

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestGapRoomEdges: inside a container, the gap that a labelled edge
// without dummies crosses has gapRunRoom either side, whichever way the
// edge goes; one crossed by an edge without a label, or by a segment of
// a labelled edge with dummies, whose label is drawn in a layer, has
// none; and at the top level no gap has any.
func TestGapRoomEdges(t *testing.T) {
	d, _ := levelDoc(t)
	var es [4]gw.EdgeIndex
	for k, text := range [4]string{"on", "back", "", "long"} {
		ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		es[k] = ch.Edges[0]
	}
	l := New(d.Caps())
	l.layers, l.s = 5, 4
	// Nodes 0 to 4 in layers 0 to 4; the segments, each of one edge.
	for k := range int32(5) {
		l.layer[k] = k
	}
	for s, seg := range [4][3]int32{{0, 1, int32(es[0])}, {3, 2, int32(es[1])}, {1, 2, int32(es[2])}, {3, 4, int32(es[3])}} {
		l.segF[s], l.segT[s], l.segE[s] = seg[0], seg[1], seg[2]
	}
	l.dN[es[3]] = 1
	for _, c := range []gw.VertexIndex{0, gw.None} {
		l.gapRoom(d, c)
		want := [6]float64{0, 2 * gapRunRoom, 0, 2 * gapRunRoom, 0, 0}
		if c == gw.None {
			want = [6]float64{}
		}
		if got := [6]float64(l.rg[:6]); got != want {
			t.Errorf("container %d: room %v, want %v", c, got, want)
		}
	}
}

// TestClimbs: an edge with a label run climbs where its ends are further
// apart across the flow than a line of text, 16, either way; one without
// a run never does.
func TestClimbs(t *testing.T) {
	d, _ := levelDoc(t)
	l := New(d.Caps())
	l.eFrom[0], l.eTo[0] = 0, 1
	for _, c := range []struct {
		run, dy float64
		want    bool
	}{{100, 16, false}, {100, 17, true}, {100, -17, true}, {0, 40, false}} {
		l.runW[0], l.y[0], l.y[1] = c.run, 0, c.dy
		if got := l.climbs(0); got != c.want {
			t.Errorf("run %g, ends %g apart: climbs %v, want %v", c.run, c.dy, got, c.want)
		}
	}
}
