package layout_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// TestExamplesNoEdgeCutsThrough: in every example as laid out, no edge,
// its legs to its ends included, passes through a vertex that holds
// neither end and that neither end holds. An edge into a container was
// drawn straight from the container's side to its end, through the
// members in the way.
func TestExamplesNoEdgeCutsThrough(t *testing.T) {
	files, err := filepath.Glob("../web/examples/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for i := range d.EdgeSlots() {
			e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
			if !e.Live() {
				continue
			}
			pts := make([]gw.Point, 0, e.Bends().Len()+2)
			pts = append(pts, d.Vertex(e.From()).Pos())
			for k := range e.Bends().Len() {
				pts = append(pts, e.Bends().At(k))
			}
			pts = append(pts, d.Vertex(e.To()).Pos())
			hit := false
			for k := 1; k < len(pts) && !hit; k++ {
				for j := range d.VertexSlots() {
					v := d.Vertex(gw.VertexIndex(j)) //nolint:gosec // bounded by the capacity
					u := gw.VertexIndex(j)
					if !v.Live() || d.Within(e.From(), u) || d.Within(e.To(), u) || d.Within(u, e.From()) || d.Within(u, e.To()) {
						continue
					}
					if crosses(pts[k-1], pts[k], v) {
						t.Errorf("%s: edge %q cuts through %q", filepath.Base(f), e.Label(), v.Label())
						hit = true
						break
					}
				}
			}
		}
	}
}

// TestExamplesAcrossRunsApart: in every example as laid out, no two
// edges' legs between bends that run straight across the flow lie on
// each other: two runs along the same gap before a container's member,
// one to the layer after the gap and one back to the layer before it,
// took the gap's middle, and in the state machine example the
// transitions into Panning and back into Placing ran down it together.
func TestExamplesAcrossRunsApart(t *testing.T) {
	files, err := filepath.Glob("../web/examples/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	type run struct {
		e         gw.ID
		x, y0, y1 float64
	}
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var runs []run
		for i := range d.EdgeSlots() {
			e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
			if !e.Live() {
				continue
			}
			for k := 1; k < e.Bends().Len(); k++ {
				p, q := e.Bends().At(k-1), e.Bends().At(k)
				x, y0, y1 := p.X, p.Y, q.Y // across a left-to-right flow
				if d.Flow() == gw.TopToBottom {
					x, y0, y1 = p.Y, p.X, q.X
				}
				if p != q && (d.Flow() == gw.TopToBottom && p.Y == q.Y || d.Flow() != gw.TopToBottom && p.X == q.X) {
					runs = append(runs, run{e.ID(), x, math.Min(y0, y1), math.Max(y0, y1)})
				}
			}
		}
		for i, a := range runs {
			for _, b := range runs[i+1:] {
				if a.e != b.e && a.x == b.x && math.Min(a.y1, b.y1)-math.Max(a.y0, b.y0) > 1 {
					t.Errorf("%s: edges %q and %q run on each other across the flow at %v", filepath.Base(f), a.e, b.e, a.x)
				}
			}
		}
	}
}

// TestExamplesEndsMeetTheirBends: in every example laid out afresh in
// either flow, and in the state machine with its transitions reversed
// but the one from its initial state, which none may enter, so that
// edges leave the members of its composite state, the bend next to an
// end spread along its side, where it stands within the spread of the
// side's ends, is level with the end, so that the route arrives level;
// or it stands at the side's middle, where the edge's dummies run
// straight on from the vertex. The order of the ends at a side follows
// where their routes head, which is known only once every route is
// found and changes with a detour: the bends set level with an end
// before were left where it fell then.
func TestExamplesEndsMeetTheirBends(t *testing.T) {
	files, err := filepath.Glob("../web/examples/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	type example struct {
		name string
		data []byte
	}
	examples := make([]example, 0, len(files)+1)
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		examples = append(examples, example{filepath.Base(f), data})
		if filepath.Base(f) == "state-machine.json" {
			examples = append(examples, example{"state-machine.json reversed", turned(t, data, "initial")})
		}
	}
	for _, ex := range examples {
		for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
			d, err := doc.Decode(ex.data, doc.Options{Schemas: doc.Builtin()})
			if err != nil {
				t.Fatalf("%s: %v", ex.name, err)
			}
			d.SetFlow(flow)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			for i := range d.EdgeSlots() {
				for _, target := range []bool{false, true} {
					if msg := endMeetsBend(d, gw.EdgeIndex(i), target); msg != "" { //nolint:gosec // bounded by the capacity
						t.Errorf("%s %d: edge %s: %s", ex.name, flow, d.Edge(gw.EdgeIndex(i)).ID(), msg) //nolint:gosec // bounded by the capacity
					}
				}
			}
		}
	}
}

// turned is the document data with every edge reversed but those from
// a vertex of kind keep.
func turned(t *testing.T, data []byte, keep string) []byte {
	t.Helper()
	var raw struct {
		Schema   string           `json:"schema"`
		Vertices []map[string]any `json:"vertices"`
		Edges    []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	kept := map[any]bool{}
	for _, v := range raw.Vertices {
		kept[v["id"]] = v["kind"] == keep
	}
	for _, e := range raw.Edges {
		if !kept[e["from"]] {
			e["from"], e["to"] = e["to"], e["from"]
		}
	}
	out, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// endMeetsBend checks edge e's end at its target, or with target false
// at its source, against the bend next to it, and says how it fails.
func endMeetsBend(d *gw.Document, e gw.EdgeIndex, target bool) string {
	ed := d.Edge(e)
	b := ed.Bends()
	if !ed.Live() || b.Len() == 0 {
		return ""
	}
	k, n := d.EndSlot(e, target)
	v, p := ed.From(), b.At(0)
	if target {
		v, p = ed.To(), b.At(b.Len()-1)
	}
	vx := d.Vertex(v)
	if n < 2 || d.Kinds().Vertex(vx.Kind()).Container() {
		return ""
	}
	mid, at, side := vx.Pos().Y, p.Y, vx.Size().H
	if d.Flow() == gw.TopToBottom {
		mid, at, side = vx.Pos().X, p.X, vx.Size().W
	}
	step := math.Min(gw.EndGap, side/float64(n+1))
	end := mid + d.EndOffset(e, target)
	if math.Abs(at-mid) > float64(float64(n-1)/2)*step || at == end || at == mid {
		return ""
	}
	return fmt.Sprintf("its end, slot %d of %d, is drawn at %v across, its bend next to it at %v", k, n, end, at)
}
