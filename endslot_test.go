package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// The kinds of the end slot schema.
const (
	esBox gw.KindIndex = iota
	esPorted
)

const (
	esCurve gw.KindIndex = iota
	esWire
	esLine
)

// endDoc is a target box t of height h at the origin and sources at the
// given positions, in flow f, with none of the edges yet.
func endDoc(t *testing.T, f gw.Flow, h float64, at ...gw.Point) (*gw.Document, gw.VertexIndex, []gw.VertexIndex) {
	t.Helper()
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Shape: gw.Rect, Size: gw.Size{W: h, H: h}},
			{Name: "ported", Shape: gw.Rect, Size: gw.Size{W: h, H: h}, Ports: []gw.Port{{Name: "in", Side: gw.Left}}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "curve", Route: gw.Curve}, {Name: "wire", Route: gw.Orthogonal}, {Name: "line"}},
	}
	d, err := makeDoc(s, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(f)
	tv := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: esBox, Text: "t"}).Vertices[0]
	vs := make([]gw.VertexIndex, 0, len(at))
	for _, p := range at {
		v := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: esBox}).Vertices[0]
		mustApply(t, d, gw.Command{Op: gw.Move, Vertex: v, Pos: p})
		vs = append(vs, v)
	}
	return d, tv, vs
}

func edgeTo(t *testing.T, d *gw.Document, kind gw.KindIndex, from, to gw.VertexIndex, toPort gw.PortIndex) gw.EdgeIndex {
	t.Helper()
	return mustApply(t, d, gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: gw.None, ToPort: toPort}).Edges[0]
}

// TestEndSlot: the curved and orthogonal edges into one side of a
// vertex take its slots in order of where their sources stand across
// the flow, and by index where two stand level; each is offset from the
// side's middle by its place, EndGap apart, or a share of a short side.
// Straight edges, an edge leaving the other side, a removed edge and
// edges to ports take no slot and leave the others' as they are. An edge
// run backward arrives at the far side, among those leaving it, and
// leaves its source by the side the edges into it arrive at, where two
// from the same vertex go by index.
func TestEndSlot(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		across := func(a float64) gw.Point {
			if f == gw.TopToBottom {
				return gw.Point{X: a, Y: -200}
			}
			return gw.Point{X: -200, Y: a}
		}
		ahead := across(0)
		ahead.X, ahead.Y = -ahead.X, -ahead.Y
		d, tv, s := endDoc(t, f, 100, across(50), across(-50), across(0), across(0), ahead, across(10))
		hi := edgeTo(t, d, esCurve, s[0], tv, gw.None)
		lo := edgeTo(t, d, esWire, s[1], tv, gw.None)
		mid := edgeTo(t, d, esCurve, s[2], tv, gw.None)
		level := edgeTo(t, d, esWire, s[3], tv, gw.None)
		edgeTo(t, d, esLine, s[5], tv, gw.None)
		pv := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: esPorted, Text: "ported"}).Vertices[0]
		mustApply(t, d, gw.Command{Op: gw.Move, Vertex: pv, Pos: across(400)})
		toPort := edgeTo(t, d, esCurve, s[0], pv, 0)
		edgeTo(t, d, esWire, s[1], pv, 0)
		// From a port further on than s4, into s4: back along the flow,
		// but drawn forward as an edge with a port is, so arriving at
		// s4's back side with out and back.
		far := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: esPorted}).Vertices[0]
		mustApply(t, d, gw.Command{Op: gw.Move, Vertex: far, Pos: gw.Point{X: 2 * ahead.X, Y: 2 * ahead.Y}})
		ported := mustApply(t, d, gw.Command{Op: gw.AddEdge, Kind: esWire, From: far, To: s[4], FromPort: 0, ToPort: gw.None}).Edges[0]
		out := edgeTo(t, d, esCurve, tv, s[4], gw.None)
		back := edgeTo(t, d, esWire, s[4], tv, gw.None)
		gone := edgeTo(t, d, esCurve, s[5], tv, gw.None)
		mustApply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: gone})
		for _, c := range []struct {
			e      gw.EdgeIndex
			target bool
			k, n   int
			off    float64
		}{
			{lo, true, 0, 4, -30}, {mid, true, 1, 4, -10}, {level, true, 2, 4, 10}, {hi, true, 3, 4, 30},
			{out, false, 0, 2, -10}, {back, true, 1, 2, 10}, {gone, true, 0, 1, 0}, {toPort, true, 0, 1, 0},
			{ported, true, 0, 3, -20}, {out, true, 1, 3, 0}, {back, false, 2, 3, 20},
		} {
			k, n := d.EndSlot(c.e, c.target)
			if off := d.EndOffset(c.e, c.target); k != c.k || n != c.n || off != c.off {
				t.Errorf("flow %d: edge %d, target %v: slot %d of %d, offset %v; want %d of %d, %v", f, c.e, c.target, k, n, off, c.k, c.n, c.off)
			}
		}
	}
}

// TestEndOffsetShortSide: on a side too short for EndGap between ends,
// the ends share it evenly and stay on it.
func TestEndOffsetShortSide(t *testing.T) {
	d, tv, s := endDoc(t, gw.LeftToRight, 40, gw.Point{X: -200, Y: -10}, gw.Point{X: -200, Y: 10})
	a := edgeTo(t, d, esCurve, s[0], tv, gw.None)
	b := edgeTo(t, d, esCurve, s[1], tv, gw.None)
	if oa, ob := d.EndOffset(a, true), d.EndOffset(b, true); oa != -40.0/6 || ob != 40.0/6 {
		t.Errorf("offsets %v, %v; want ±%v", oa, ob, 40.0/6)
	}
}

// TestEndSlotsBound: past MaxEndSlots edges at a side, every end stays
// at its middle; at the bound they are still spread.
func TestEndSlotsBound(t *testing.T) {
	for _, n := range []int{gw.MaxEndSlots, gw.MaxEndSlots + 1} {
		at := make([]gw.Point, n)
		for i := range at {
			at[i] = gw.Point{X: -200, Y: float64(10 * i)}
		}
		d, tv, s := endDoc(t, gw.LeftToRight, 400, at...)
		var e gw.EdgeIndex
		for _, v := range s {
			e = edgeTo(t, d, esCurve, v, tv, gw.None)
		}
		k, got := d.EndSlot(e, true)
		if want := n; n > gw.MaxEndSlots {
			if k != 0 || got != 1 {
				t.Errorf("%d edges: slot %d of %d, want alone", n, k, got)
			}
		} else if k != n-1 || got != want {
			t.Errorf("%d edges: slot %d of %d, want %d of %d", n, k, got, n-1, want)
		}
	}
	d, _, _ := endDoc(t, gw.LeftToRight, 40)
	if k, n := d.EndSlot(-1, true); k != 0 || n != 1 || d.EndOffset(-1, true) != 0 {
		t.Errorf("an edge out of range: slot %d of %d, offset %v", k, n, d.EndOffset(-1, true))
	}
	if k, n := d.EndSlot(5, true); k != 0 || n != 1 {
		t.Errorf("an edge past the slots: slot %d of %d", k, n)
	}
}
