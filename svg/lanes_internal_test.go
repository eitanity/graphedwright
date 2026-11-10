package svg

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// textRE finds an edge's group and, in it, each label and stereotype as
// drawn: its class, position, anchor and text.
var (
	edgeRE = regexp.MustCompile(`<g class="edge [^"]*" id="([^"]+)"[^>]*>(.*?)</g>`)
	textRE = regexp.MustCompile(`<text class="(label|stereotype)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="(start|middle|end)"( transform="rotate\(-90 [-0-9. ]+\)")?>([^<]*)</text>`)
)

// textBox is the box a drawn text takes at the default metrics: as wide
// as its runes, from the font's ascent above the baseline to its
// descent below.
func textBox(x, y float64, anchor, text string) gw.Viewport {
	w := float64(runeCount(text)) * defaultCharWidth
	switch anchor {
	case "middle":
		x -= w / 2
	case "end":
		x -= w
	}
	return gw.Viewport{X: x, Y: y - 9, W: w, H: 12}
}

// turnBox is box turned a quarter, to read up, about (x, y).
func turnBox(b gw.Viewport, x, y float64) gw.Viewport {
	return gw.Viewport{X: x + b.Y - y, Y: y - (b.X + b.W - x), W: b.H, H: b.W}
}

func inBox2(b gw.Viewport, p gw.Point) bool {
	return p.X > b.X && p.X < b.X+b.W && p.Y > b.Y && p.Y < b.Y+b.H
}

// TestParallelLabelsClear: two edges between the same two vertices, run
// either way or both the same way, in either flow and by every route,
// each labelled and with a stereotype, draw each text clear of both
// edges, sampled finely along their whole length, and of the other
// edge's texts; and each label is hit where it is drawn.
func TestParallelLabelsClear(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, route := range []gw.Route{gw.Straight, gw.Curve, gw.Orthogonal} {
			for _, back := range []bool{true, false} {
				s := gw.Schema{
					VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}}},
					EdgeKinds:   []gw.EdgeKind{{Name: "go", Route: route, Arrow: true, Stereotype: "«go»"}},
				}
				d, err := makeDoc(s, gw.Caps{})
				if err != nil {
					t.Fatal(err)
				}
				d.SetFlow(flow)
				far := gw.Point{X: 240}
				if flow == gw.TopToBottom {
					far = gw.Point{Y: 160}
				}
				for _, p := range []gw.Point{{}, far} {
					if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Pos: p}); err != nil {
						t.Fatal(err)
					}
				}
				second := gw.Command{Op: gw.AddEdge, From: 1, To: 0, FromPort: gw.None, ToPort: gw.None, Text: "IgnitionOff"}
				if !back {
					second.From, second.To = 0, 1
				}
				for _, c := range []gw.Command{{Op: gw.AddEdge, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None, Text: "IgnitionOn"}, second} {
					if _, err := d.Apply(c); err != nil {
						t.Fatal(err)
					}
				}
				var buf bytes.Buffer
				if err := Render(d, Options{}, &buf); err != nil {
					t.Fatal(err)
				}
				var boxes []gw.Viewport
				var owner []string
				for _, g := range edgeRE.FindAllStringSubmatch(buf.String(), -1) {
					for _, m := range textRE.FindAllStringSubmatch(g[2], -1) {
						x, _ := strconv.ParseFloat(m[2], 64)
						y, _ := strconv.ParseFloat(m[3], 64)
						box := textBox(x, y, m[4], m[6])
						if m[5] != "" {
							box = turnBox(box, x, y)
						}
						boxes = append(boxes, box)
						owner = append(owner, g[1]+" "+m[1]+" "+m[6])
					}
				}
				if len(boxes) != 4 {
					t.Fatalf("flow %d route %d back %v: %d texts in\n%s", flow, route, back, len(boxes), buf.String())
				}
				for i := range d.EdgeSlots() {
					var lane [1]gw.Point
					w := wayOf(d, gw.EdgeIndex(i), &lane)
					for k := 0; k <= 400; k++ {
						p := along(w.f, route, w.a, w.through, w.b, float64(k)/400)
						for j, b := range boxes {
							if inBox2(b, p) {
								t.Errorf("flow %d route %d back %v: %s crosses edge %s at %v", flow, route, back, owner[j], d.Edge(gw.EdgeIndex(i)).ID(), p) //nolint:gosec // bounded by the capacity
								break
							}
						}
					}
				}
				// Each label is hit, and edited, where it is drawn.
				for j, b := range boxes {
					if owner[j][3:8] != "label" {
						continue
					}
					c := gw.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
					i := gw.EdgeIndex(0)
					if owner[j][:2] == "e2" {
						i = 1
					}
					if !inLabel(d, i, c) || !inBox2(EdgeLabelBox(d, i), c) {
						t.Errorf("flow %d route %d back %v: %s not hit at its middle %v; box %v", flow, route, back, owner[j], c, EdgeLabelBox(d, i))
					}
				}
				for j := range boxes {
					for k := j + 1; k < len(boxes); k++ {
						a, b := boxes[j], boxes[k]
						if owner[j][:2] != owner[k][:2] && a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
							t.Errorf("flow %d route %d back %v: %s overlaps %s", flow, route, back, owner[j], owner[k])
						}
					}
				}
			}
		}
	}
}

// TestOrthogonalLabelOwnLeg: of three orthogonal edges from one vertex
// to three in the next layer, which share their first leg and turn, the
// one labelled has its label along its own last leg, past the turn, and
// clear of every edge and vertex, in either flow, laid out.
func TestOrthogonalLabelOwnLeg(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		s := gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36}}},
			EdgeKinds:   []gw.EdgeKind{{Name: "wire", Route: gw.Orthogonal}},
		}
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		for range 4 {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex}); err != nil {
				t.Fatal(err)
			}
		}
		for to, text := range []string{"", "multiplicity 0..4", ""} {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: 0, To: gw.VertexIndex(to + 1), FromPort: gw.None, ToPort: gw.None, Text: text}); err != nil { //nolint:gosec // below 3
				t.Fatal(err)
			}
		}
		value := "left-to-right"
		if flow == gw.TopToBottom {
			value = "top-to-bottom"
		}
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: value}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Render(d, Options{}, &buf); err != nil {
			t.Fatal(err)
		}
		m := textRE.FindStringSubmatch(buf.String())
		if m == nil || m[6] != "multiplicity 0..4" {
			t.Fatalf("flow %d: no label in\n%s", flow, buf.String())
		}
		x, _ := strconv.ParseFloat(m[2], 64)
		y, _ := strconv.ParseFloat(m[3], 64)
		box := textBox(x, y, m[4], m[6])
		if m[5] != "" {
			box = turnBox(box, x, y)
		}
		for i := range d.EdgeSlots() {
			var lane [1]gw.Point
			w := wayOf(d, gw.EdgeIndex(i), &lane)
			for k := 0; k <= 400; k++ {
				if p := along(w.f, gw.Orthogonal, w.a, w.through, w.b, float64(k)/400); inBox2(box, p) {
					t.Errorf("flow %d: the label %v crosses edge %s at %v", flow, box, d.Edge(gw.EdgeIndex(i)).ID(), p) //nolint:gosec // bounded by the capacity
					break
				}
			}
		}
		for v := range d.VertexSlots() {
			x := d.Vertex(gw.VertexIndex(v)) //nolint:gosec // bounded by the capacity
			vb := gw.Viewport{X: x.Pos().X - x.Size().W/2, Y: x.Pos().Y - x.Size().H/2, W: x.Size().W, H: x.Size().H}
			if box.X < vb.X+vb.W && vb.X < box.X+box.W && box.Y < vb.Y+vb.H && vb.Y < box.Y+box.H {
				t.Errorf("flow %d: the label %v overlaps vertex %s %v", flow, box, x.ID(), vb)
			}
		}
		// Along its own last leg: past the turn, towards its target.
		var lane [1]gw.Point
		w := wayOf(d, 1, &lane)
		a, b := w.f.t(w.a), w.f.t(w.b)
		turn := (a.X + b.X) / 2
		lo, hi := box.X, box.X+box.W
		if flow == gw.TopToBottom {
			lo, hi = box.Y, box.Y+box.H
		}
		if lo < min(turn, b.X) || hi > max(turn, b.X) {
			t.Errorf("flow %d: the label spans %g to %g, not within its last leg, %g to %g", flow, lo, hi, turn, b.X)
		}
	}
}

// throughContainer is the state machine whose final state, laid out,
// sits below a composite state in the same layer: the transition to it
// from the state before used to be drawn across the composite state and
// the initial state inside.
const throughContainer = `{"schema": "state-machine", "vertices": [
 {"id": "i", "kind": "initial"}, {"id": "off", "kind": "state", "label": "off"},
 {"id": "on", "kind": "composite state", "label": "on"},
 {"id": "on.i", "kind": "initial", "parent": "on"},
 {"id": "idle", "kind": "state", "label": "idle", "parent": "on"},
 {"id": "moving", "kind": "state", "label": "moving", "parent": "on"},
 {"id": "end", "kind": "final"}],
 "edges": [
 {"kind": "transition", "from": "i", "to": "off"},
 {"kind": "transition", "from": "off", "to": "on"},
 {"kind": "transition", "from": "on", "to": "off"},
 {"kind": "transition", "from": "on.i", "to": "idle"},
 {"kind": "transition", "from": "idle", "to": "moving"},
 {"kind": "transition", "from": "off", "to": "end", "label": "Scrap"}]}`

// routeClear reports the first vertex the drawn route of edge i passes
// through, sampled finely along its whole length, other than its ends
// and the containers that hold them, or None.
func routeClear(d *gw.Document, i gw.EdgeIndex) (gw.VertexIndex, gw.Point) {
	e := d.Edge(i)
	var lane [1]gw.Point
	w := wayOf(d, i, &lane)
	route := d.Kinds().Edge(e.Kind()).Route()
	for k := 0; k <= 1000; k++ {
		p := along(w.f, route, w.a, w.through, w.b, float64(k)/1000)
		for v := range d.VertexSlots() {
			x := d.Vertex(gw.VertexIndex(v)) //nolint:gosec // bounded by the capacity
			u := gw.VertexIndex(v)           //nolint:gosec // bounded
			if !x.Live() || d.Within(e.From(), u) || d.Within(e.To(), u) {
				continue
			}
			if p.X > x.Pos().X-x.Size().W/2+0.5 && p.X < x.Pos().X+x.Size().W/2-0.5 && p.Y > x.Pos().Y-x.Size().H/2+0.5 && p.Y < x.Pos().Y+x.Size().H/2-0.5 {
				return u, p
			}
		}
	}
	return gw.None, gw.Point{}
}

// TestRoutesClearOfContainers: laid out, no drawn route passes through
// a vertex or container that is neither one of its ends nor holds one,
// whether its ends are in one container or in different ones: the state
// machine above in either flow, and random graphs whose vertices are
// spread over containers, by every route, edges between the same two
// vertices among them.
func TestRoutesClearOfContainers(t *testing.T) {
	check := func(name string, d *gw.Document) {
		t.Helper()
		for i := range d.EdgeSlots() {
			if !d.Edge(gw.EdgeIndex(i)).Live() { //nolint:gosec // bounded by the capacity
				continue
			}
			if v, p := routeClear(d, gw.EdgeIndex(i)); v != gw.None { //nolint:gosec // bounded
				e := d.Edge(gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
				t.Errorf("%s: edge %s from %s to %s passes through %s at %v; bends %v", name, e.ID(), d.Vertex(e.From()).ID(), d.Vertex(e.To()).ID(), d.Vertex(v).ID(), p, e.Bends())
			}
		}
	}
	// An edge from a vertex in one group to one in another, below a
	// member beside its source; and a long straight edge past a group
	// beside its source.
	for _, name := range []string{"boundaries", "long-straight"} {
		data, err := os.ReadFile("testdata/routes/" + name + ".json") //nolint:gosec // fixed test data
		if err != nil {
			t.Fatal(err)
		}
		d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		check(name, d)
	}
	for _, flow := range []string{"left-to-right", "top-to-bottom"} {
		d, err := doc.Decode([]byte(throughContainer), doc.Options{Schemas: doc.Builtin(), NoLayout: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: flow}); err != nil {
			t.Fatal(err)
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		check("state machine "+flow, d)
	}
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 120, H: 36}},
			{Name: "dot", Shape: gw.Circle, Size: gw.Size{W: 28, H: 28}},
			{Name: "zone", Size: gw.Size{W: 60, H: 40}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "line"}, {Name: "curve", Route: gw.Curve}, {Name: "elbow", Route: gw.Orthogonal}},
		Acyclic:   true,
	}
	for seed := uint64(1); seed <= 24; seed++ {
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(gw.Flow(seed % 2)) //nolint:gosec // 0 or 1
		r := rand.New(rand.NewPCG(seed, 7))
		const n, zones = 30, 5
		for range n {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: gw.KindIndex(r.IntN(2))}); err != nil { //nolint:gosec // below 2
				t.Fatal(err)
			}
		}
		for range zones {
			if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 2}); err != nil {
				t.Fatal(err)
			}
		}
		for v := range n {
			if z := r.IntN(zones + 2); z < zones {
				if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: gw.VertexIndex(v), To: gw.VertexIndex(n + z)}); err != nil { //nolint:gosec // bounded
					t.Fatal(err)
				}
			}
		}
		for range 45 {
			a, b := r.IntN(n), r.IntN(n)
			if a == b {
				continue
			}
			_, _ = d.Apply(gw.Command{Op: gw.AddEdge, Kind: gw.KindIndex(r.IntN(3)), From: gw.VertexIndex(min(a, b)), To: gw.VertexIndex(max(a, b)), FromPort: gw.None, ToPort: gw.None}) //nolint:gosec // below n
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		check("seed "+strconv.FormatUint(seed, 10), d)
	}
}

// exampleNames are the demo's examples.
var exampleNames = [...]string{"shapes", "diamond", "dependencies", "use-case", "threat-model", "state-machine"}

func readExample(t *testing.T, name string) *gw.Document {
	t.Helper()
	data, err := os.ReadFile("../web/examples/" + name + ".json") //nolint:gosec // the demo's examples, by fixed name
	if err != nil {
		t.Fatal(err)
	}
	d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
	if err != nil {
		t.Fatal(err)
	}
	PlaceLabels(d) // as a host does once it has read a document
	return d
}

// TestExampleLabelsClear: in the demo's examples as they are stored, no
// edge's label or stereotype crosses another edge, sampled finely
// along its whole length, or another edge's texts: among them a wire
// that turns alone, whose label stays at its turn rather than on a last
// leg that runs beside a curve into the same vertex, under the curve's.
func TestExampleLabelsClear(t *testing.T) {
	for _, name := range exampleNames {
		edgeTextsClear(t, name, readExample(t, name))
	}
}

// TestLaidOutExamplesClear: the demo's examples laid out afresh in
// either flow keep their texts and edges apart. The name at an actor's
// foot, wider than the actor, clears the names and boxes of the
// vertices beside it, which stood a gap apart by their boxes alone and
// so overlapped in a top-to-bottom flow. No edge's text crosses another
// edge, and no two edges run along each other: an edge into a use case
// inside the system came in along the gap before the use case's layer,
// through the stereotype of the edge from that layer and then on top of
// that edge, at the middle of the side they shared; and a curved and an
// orthogonal edge drawn to one side of a vertex both ended at its
// middle, where the last leg of one lay on the other.
func TestLaidOutExamplesClear(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		for _, name := range exampleNames {
			d := readExample(t, name)
			d.SetFlow(flow)
			if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
				t.Fatal(err)
			}
			PlaceLabels(d)
			label := name + " " + strconv.Itoa(int(flow))
			footLabelsClear(t, label, d)
			edgeTextsClear(t, label, d)
			edgesApart(t, label, d, knownAlong[label]...)
		}
	}
}

// knownAlong are the pairs of edges in the examples laid out afresh that
// run along each other where the order of the ends at a side, found from
// the bends alone, cannot tell a route that runs on along the side's
// middle from one that turns away: left to right, t16 runs on along
// EditingText's middle from its back side, and t20 turns down across it
// at once; top to bottom, t4 runs down from press level with its end to
// an upright far into the gap, and t6 curves across it.
var knownAlong = map[string][][2]gw.ID{
	"state-machine 0": {{"t20", "t16"}},
	"state-machine 1": {{"t6", "t4"}},
}

// footLabelsClear checks that each label drawn at an actor's foot
// overlaps neither another such label nor another vertex's box.
func footLabelsClear(t *testing.T, name string, d *gw.Document) {
	t.Helper()
	box := func(i int) (gw.Viewport, bool) {
		v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if !v.Live() || d.Kinds().Vertex(v.Kind()).Shape() != gw.Actor {
			return gw.Viewport{}, false
		}
		return textBox(v.Pos().X, v.Pos().Y+v.Size().H/2-4, "middle", v.Label()), true
	}
	for i := range d.VertexSlots() {
		a, ok := box(i)
		if !ok {
			continue
		}
		for j := range d.VertexSlots() {
			v := d.Vertex(gw.VertexIndex(j)) //nolint:gosec // bounded by the capacity
			if j == i || !v.Live() || d.Kinds().Vertex(v.Kind()).Container() {
				continue
			}
			b, below := box(j)
			if !below {
				b = gw.Viewport{X: v.Pos().X - v.Size().W/2, Y: v.Pos().Y - v.Size().H/2, W: v.Size().W, H: v.Size().H}
			}
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				t.Errorf("%s: the label of %q overlaps %q", name, d.Vertex(gw.VertexIndex(i)).Label(), v.Label()) //nolint:gosec // bounded by the capacity
			}
		}
	}
}

// TestUprightsUntangled: in the state machine laid out top to bottom,
// the leg from "under it?" to Resizing leaves level from where the one
// from the composite state back to Idle arrives, its upright in the
// same gap; with their uprights the other way round, the two ran on
// each other there.
func TestUprightsUntangled(t *testing.T) {
	d := readExample(t, "state-machine")
	d.SetFlow(gw.TopToBottom)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	a, b := d.EdgeByID("t4"), d.EdgeByID("t11")
	if a == gw.None || b == gw.None {
		t.Fatal("no edges t4 and t11")
	}
	pa, pb := drawnPoints(d, a), drawnPoints(d, b)
	if run := max(sharedRun(d, pa, pb), sharedRun(d, pb, pa)); run > 8 {
		t.Errorf("t4 and t11 run on each other for %v", run)
	}
}

// drawnPoints is edge e's line as drawn, at apartSamples steps.
func drawnPoints(d *gw.Document, e gw.EdgeIndex) []gw.Point {
	var lane [1]gw.Point
	w := wayOf(d, e, &lane)
	route := d.Kinds().Edge(d.Edge(e).Kind()).Route()
	pts := make([]gw.Point, 0, apartSamples+1)
	for k := 0; k <= apartSamples; k++ {
		pts = append(pts, along(w.f, route, w.a, w.through, w.b, float64(k)/apartSamples))
	}
	return pts
}

// apartSamples is how finely edgesApart samples an edge.
const apartSamples = 400

// edgesApart checks that no edge runs within a unit of another for more
// than 8 units outside the vertices: they may cross, and meet at a
// vertex, but not share a stretch of line. A pair known to (one edge's ID,
// then the other's) is let be, and must still be found, so that the
// list goes when the overlap does.
func edgesApart(t *testing.T, name string, d *gw.Document, known ...[2]gw.ID) {
	t.Helper()
	pts := make([][]gw.Point, d.EdgeSlots())
	for i := range d.EdgeSlots() {
		if !d.Edge(gw.EdgeIndex(i)).Live() { //nolint:gosec // bounded by the capacity
			continue
		}
		pts[i] = drawnPoints(d, gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
	}
	found := make([]bool, len(known))
	for i, a := range pts {
		for j, b := range pts {
			if i == j || len(a) == 0 || len(b) == 0 || sharedRun(d, a, b) <= 8 {
				continue
			}
			pair := [2]gw.ID{d.Edge(gw.EdgeIndex(i)).ID(), d.Edge(gw.EdgeIndex(j)).ID()} //nolint:gosec // bounded by the capacity
			if k := slices.Index(known, pair); k >= 0 {
				found[k] = true
				continue
			}
			t.Errorf("%s: edge %s runs along edge %s", name, pair[0], pair[1])
		}
	}
	for k, ok := range found {
		if !ok {
			t.Errorf("%s: edge %s no longer runs along edge %s; drop it from the known pairs", name, known[k][0], known[k][1])
		}
	}
}

// sharedRun is the longest stretch of a, outside every vertex but the
// containers, that lies within a unit of b.
func sharedRun(d *gw.Document, a, b []gw.Point) float64 {
	run, longest := 0.0, 0.0
	// A leg counts where both its ends do: one that only ends within a
	// unit of b crosses it there.
	for k := 1; k < len(a); k++ {
		if !near(a[k], b) || nearVertex(d, a[k]) || !near(a[k-1], b) {
			run = 0
			continue
		}
		run += math.Hypot(a[k].X-a[k-1].X, a[k].Y-a[k-1].Y)
		longest = math.Max(longest, run)
	}
	return longest
}

func near(p gw.Point, b []gw.Point) bool {
	for k := 1; k < len(b); k++ {
		if segmentDistance(p, b[k-1], b[k]) < 1 {
			return true
		}
	}
	return false
}

// nearVertex reports whether p is within 2 units of a vertex's box that
// holds nothing.
func nearVertex(d *gw.Document, p gw.Point) bool {
	for i := range d.VertexSlots() {
		v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
		if !v.Live() || d.Kinds().Vertex(v.Kind()).Container() {
			continue
		}
		if math.Abs(p.X-v.Pos().X) < v.Size().W/2+2 && math.Abs(p.Y-v.Pos().Y) < v.Size().H/2+2 {
			return true
		}
	}
	return false
}

// edgeTextsClear checks that no edge's label or stereotype, as drawn,
// crosses another edge or another edge's texts, or meets the box of a
// vertex other than a container, reporting through t.
func edgeTextsClear(t reporter, name string, d *gw.Document) {
	t.Helper()
	{
		var buf bytes.Buffer
		if err := Render(d, Options{}, &buf); err != nil {
			t.Fatal(err)
		}
		var boxes []gw.Viewport
		var owners []string
		for _, g := range edgeRE.FindAllStringSubmatch(buf.String(), -1) {
			for _, m := range textRE.FindAllStringSubmatch(g[2], -1) {
				x, _ := strconv.ParseFloat(m[2], 64)
				y, _ := strconv.ParseFloat(m[3], 64)
				box := textBox(x, y, m[4], m[6])
				if m[5] != "" {
					box = turnBox(box, x, y)
				}
				for j, b := range boxes {
					if owners[j] != g[1] && box.X < b.X+b.W && b.X < box.X+box.W && box.Y < b.Y+b.H && b.Y < box.Y+box.H {
						t.Errorf("%s: the %s %q of edge %s overlaps a text of edge %s", name, m[1], m[6], g[1], owners[j])
					}
				}
				boxes = append(boxes, box)
				owners = append(owners, g[1])
				for i := range d.VertexSlots() {
					v := d.Vertex(gw.VertexIndex(i)) //nolint:gosec // bounded by the capacity
					switch {
					case !v.Live():
					case !d.Kinds().Vertex(v.Kind()).Container():
						if boxesMeet(box, vertexBox(v)) {
							t.Errorf("%s: the %s %q of edge %s overlaps vertex %s %+v %+v", name, m[1], m[6], g[1], v.ID(), box, vertexBox(v))
						}
					case boxesMeet(box, titleBox(d, v)):
						t.Errorf("%s: the %s %q of edge %s overlaps the title of %s", name, m[1], m[6], g[1], v.ID())
					case acrossSide(box, vertexBox(v)):
						t.Errorf("%s: the %s %q of edge %s lies across a side of %s", name, m[1], m[6], g[1], v.ID())
					}
				}
				for i := range d.EdgeSlots() {
					if !d.Edge(gw.EdgeIndex(i)).Live() || string(d.Edge(gw.EdgeIndex(i)).ID()) == g[1] { //nolint:gosec // bounded by the capacity
						continue
					}
					var lane [1]gw.Point
					w := wayOf(d, gw.EdgeIndex(i), &lane)
					route := d.Kinds().Edge(d.Edge(gw.EdgeIndex(i)).Kind()).Route() //nolint:gosec // bounded by the capacity
					for k := 0; k <= 400; k++ {
						if p := along(w.f, route, w.a, w.through, w.b, float64(k)/400); inBox2(box, p) {
							t.Errorf("%s: the %s %q of edge %s crosses edge %s at %v", name, m[1], m[6], g[1], d.Edge(gw.EdgeIndex(i)).ID(), p) //nolint:gosec // bounded by the capacity
							break
						}
					}
				}
			}
		}
	}
}

// reporter is what the checks report through: a test, or a record of
// what they report (reports).
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatal(args ...any)
}

// reports records what a check reports, and fails the test it is given
// on a fatal one.
type reports struct {
	t    *testing.T
	errs []string
}

func (r *reports) Helper() {}

func (r *reports) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func (r *reports) Fatal(args ...any) { r.t.Fatal(args...) }

// TestEdgeTextsMeetVertices: edgeTextsClear reports an edge text drawn
// over a vertex's box: with the use case Pay put back where the example
// once had it, 58 from Place order, the «include» between them, 63 wide,
// goes into both boxes; where the example has it, 82 apart, it clears
// them.
func TestEdgeTextsMeetVertices(t *testing.T) {
	for _, c := range []struct {
		x    float64
		want []string
	}{{528, []string{"overlaps vertex order", "overlaps vertex pay"}}, {552, nil}} {
		d := readExample(t, "use-case")
		pay := d.VertexByID("pay")
		if pay == gw.None {
			t.Fatal("no vertex pay")
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: pay, Pos: gw.Point{X: c.x, Y: d.Vertex(pay).Pos().Y}}); err != nil {
			t.Fatal(err)
		}
		r := &reports{t: t}
		edgeTextsClear(r, "use-case", d)
		got := strings.Join(r.errs, "\n")
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("pay at %g: no report that the stereotype %s, in %q", c.x, w, got)
			}
		}
		if c.want == nil && got != "" {
			t.Errorf("pay at %g: reports %q, want none", c.x, got)
		}
	}
}

// TestEdgeTextsMeetContainers: the check reports an edge's text that
// lies across a container's side or over its title, and not one inside
// the container clear of the title. The zone holds a box at 300, 0; the
// edge checked goes from a box at from to one at to, or to the zone's
// box with into, and from a second box in the zone with member.
func TestEdgeTextsMeetContainers(t *testing.T) {
	for _, c := range []struct {
		name         string
		from, to     gw.Point
		into, member bool
		want         string
	}{
		{"into the zone from 200", gw.Point{X: 200, Y: 0}, gw.Point{}, true, false, "lies across a side of"},
		{"over the title", gw.Point{X: 150, Y: -40}, gw.Point{X: 450, Y: -40}, false, false, "overlaps the title of"},
		{"between the zone's boxes", gw.Point{X: 500, Y: 0}, gw.Point{}, true, true, ""},
	} {
		d, z, e := zoneDoc(t, gw.Point{X: -200, Y: 0}) // its own edge far off, unlabelled
		if _, err := d.Apply(gw.Command{Op: gw.Relabel, Edge: e, Vertex: gw.None, Text: ""}); err != nil {
			t.Fatal(err)
		}
		a := placeVertexAt(t, d, c.from)
		if c.member {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: a, To: z}); err != nil {
				t.Fatal(err)
			}
		}
		b := d.Edge(e).To()
		if !c.into {
			b = placeVertexAt(t, d, c.to)
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: a, To: b, FromPort: gw.None, ToPort: gw.None, Text: "label"}); err != nil {
			t.Fatal(err)
		}
		r := &reports{t: t}
		edgeTextsClear(r, c.name, d)
		got := strings.Join(r.errs, "\n")
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: reports %q, want %q", c.name, got, c.want)
		}
	}
}

// TestInLineLabelsInZone: top to bottom, two boxes in a zone, one above
// the other, joined both ways by curves labelled wider than the zone
// would be around them, have their labels turned along runs inside the
// zone, clear of each other's edges: each edge runs straight down from
// one end, level with it, and has a distinct pair of bends level with
// the other end, which its spread puts across from the run. Left to
// right the labels, level over their edges, are in the zone with no
// run.
func TestInLineLabelsInZone(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := makeDoc(gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}}, {Name: "zone", Size: gw.Size{W: 80, H: 60}, Container: true}},
			EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
		}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: placeZone, Text: "Zone"})
		if err != nil {
			t.Fatal(err)
		}
		z := ch.Vertices[0]
		a, b := placeVertexAt(t, d, gw.Point{}), placeVertexAt(t, d, gw.Point{})
		for _, v := range [2]gw.VertexIndex{a, b} {
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: v, To: z}); err != nil {
				t.Fatal(err)
			}
		}
		for _, e := range [2][2]gw.VertexIndex{{a, b}, {b, a}} {
			if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e[0], To: e[1], FromPort: gw.None, ToPort: gw.None, Text: "Press [middle or Alt]"}); err != nil {
				t.Fatal(err)
			}
		}
		d.SetFlow(f)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		if f == gw.TopToBottom && d.Vertex(a).Pos().X != d.Vertex(b).Pos().X {
			t.Fatalf("the boxes are not in line: %v %v", d.Vertex(a).Pos(), d.Vertex(b).Pos())
		}
		for e := range gw.EdgeIndex(2) {
			inLineRun(t, d, f, e)
		}
		PlaceLabels(d)
		edgeTextsClear(t, "flow "+strconv.Itoa(int(f)), d)
	}
}

// inLineRun checks edge e of TestInLineLabelsInZone: left to right it
// has no bends; top to bottom, four, the first pair level with its
// source's end and the second, two apart, with its target's.
func inLineRun(t *testing.T, d *gw.Document, f gw.Flow, e gw.EdgeIndex) {
	t.Helper()
	b := d.Edge(e).Bends()
	if f == gw.LeftToRight {
		if b.Len() != 0 {
			t.Errorf("flow %d edge %d: %d bends, want none", f, e, b.Len())
		}
		return
	}
	if b.Len() != 4 {
		t.Fatalf("flow %d edge %d: %d bends, want a run and a pair", f, e, b.Len())
	}
	from := d.Vertex(d.Edge(e).From()).Pos().X + d.EndOffset(e, false)
	to := d.Vertex(d.Edge(e).To()).Pos().X + d.EndOffset(e, true)
	if b.At(0).X != from || b.At(1).X != from || b.At(2).X != to || b.At(3).X != to || b.At(2) == b.At(3) {
		t.Errorf("flow %d edge %d: bends %v %v %v %v, the ends at %v and %v", f, e, b.At(0), b.At(1), b.At(2), b.At(3), from, to)
	}
}

// TestEnclosedEdgeLaidOut: a zone holds three boxes in a row, m, n and
// r, an edge from a box outside comes in to m, and edges run in from the
// zone to m, to n, behind m, and to r, behind both, and out to the zone
// from n, with r behind it, and from m, the first two labelled, the edge
// in to m's label the longest, so that n stands past the zone's middle. Laid out in either flow, each
// edge in starts on the zone's side facing back and each out ends on its
// side facing on, level with the bend next to it, or its box's end, and
// running with the flow;
// the edge in to m, with nothing in its way, has no detour, and the
// others go round the boxes in their way inside the zone, whose padding is
// narrower than a detour's gap; and every label is clear of the
// other edges and of the zone's sides.
func TestEnclosedEdgeLaidOut(t *testing.T) {
	for _, f := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := makeDoc(gw.Schema{
			VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}}, {Name: "zone", Size: gw.Size{W: 80, H: 60}, Container: true}},
			EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
		}, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: placeZone, Text: "Zone"})
		if err != nil {
			t.Fatal(err)
		}
		z := ch.Vertices[0]
		x := placeVertexAt(t, d, gw.Point{})
		var mnr [3]gw.VertexIndex
		for k := range mnr {
			mnr[k] = placeVertexAt(t, d, gw.Point{})
			if _, err := d.Apply(gw.Command{Op: gw.SetParent, Vertex: mnr[k], To: z}); err != nil {
				t.Fatal(err)
			}
		}
		m, n, r := mnr[0], mnr[1], mnr[2]
		var es [8]gw.EdgeIndex
		for k, e := range [8]struct {
			from, to gw.VertexIndex
			text     string
		}{{z, m, "PaletteVertex, PaletteEdge"}, {n, z, "PaletteVertex"}, {z, n, ""}, {z, r, ""}, {m, z, ""}, {x, m, "PaletteVertex"}, {m, n, ""}, {n, r, ""}} {
			ch, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e.from, To: e.to, FromPort: gw.None, ToPort: gw.None, Text: e.text})
			if err != nil {
				t.Fatal(err)
			}
			es[k] = ch.Edges[0]
		}
		d.SetFlow(f)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		zb := vertexBox(d.Vertex(z))
		for k, way := range [5]float64{-1, 1, -1, -1, 1} {
			e := d.Edge(es[k])
			a, b := ends(d, es[k], e.Bends())
			c, o := a, b // the zone's end, and the box's or the bend next to the zone's
			if bs := e.Bends(); way < 0 && bs.Len() > 0 {
				o = bs.At(0)
			}
			if way > 0 {
				c, o = b, a
				if bs := e.Bends(); bs.Len() > 0 {
					o = bs.At(bs.Len() - 1)
				}
			}
			side, at, lv := zb.X+float64(float64(way+1)/2)*zb.W, c.X, c.Y == o.Y
			if f == gw.TopToBottom {
				side, at, lv = zb.Y+float64(float64(way+1)/2)*zb.H, c.Y, c.X == o.X
			}
			if at != side || !lv || frameOf(d, e).back || k == 0 && e.Bends().Len() != 0 {
				t.Errorf("flow %d edge %d: ends %v %v, the zone's side at %v, back %v, %d bends", f, k, a, b, side, frameOf(d, e).back, e.Bends().Len())
			}
		}
		for _, e := range es {
			if enc := d.Enclosing(d.Edge(e).From(), d.Edge(e).To()); enc != 0 {
				b := d.Edge(e).Bends()
				for k := range b.Len() {
					if p := b.At(k); p.X < zb.X || p.X > zb.X+zb.W || p.Y < zb.Y || p.Y > zb.Y+zb.H {
						t.Errorf("flow %d: edge %d has a bend outside the zone %+v: %v", f, e, zb, p)
					}
				}
			}
			if v, p := routeClear(d, e); v != gw.None {
				t.Errorf("flow %d: edge %d passes through %s at %v", f, e, d.Vertex(v).ID(), p)
			}
		}
		PlaceLabels(d)
		edgeTextsClear(t, "flow "+strconv.Itoa(int(f)), d)
	}
}

// fanDoc is a box with edges out to n boxes, curved, the middle one
// labelled text, laid out in the flow; it returns the document and the
// gap between the box's layer and the next, from centre to centre.
func fanDoc(t *testing.T, f gw.Flow, n int, text string) (*gw.Document, float64) {
	t.Helper()
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	u := placeVertexAt(t, d, gw.Point{})
	var v gw.VertexIndex
	for k := range n {
		v = placeVertexAt(t, d, gw.Point{})
		label := ""
		if k == n/2 {
			label = text
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: u, To: v, FromPort: gw.None, ToPort: gw.None, Text: label}); err != nil {
			t.Fatal(err)
		}
	}
	d.SetFlow(f)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	gap := d.Vertex(v).Pos().X - d.Vertex(u).Pos().X
	if f == gw.TopToBottom {
		gap = d.Vertex(v).Pos().Y - d.Vertex(u).Pos().Y
	}
	return d, gap
}

// TestCrowdedGapLabelClear: left to right, a box with edges out to three
// boxes in the next layer, the middle one labelled: the others climb
// across the gap at the label's height, in the middle where it is drawn,
// so the gap is twice the label's room, and the label is placed clear of
// them. A labelled edge alone, which nothing climbs past, keeps the
// label's room, as do one that climbs itself past one level, one in a
// lane, one before a fan in the next gap, and ones above and below a
// fan; and so does the fan top to bottom, whose labels are beside their
// edges.
func TestCrowdedGapLabelClear(t *testing.T) {
	const text = "DoubleClick [element]"
	room := float64(len(text))*defaultCharWidth + 16
	d, gap := fanDoc(t, gw.LeftToRight, 3, text)
	if gap < 2*room+120 {
		t.Errorf("the fan's gap %v, want at least twice the label's room %v and a box", gap, room)
	}
	PlaceLabels(d)
	edgeTextsClear(t, "the fan", d)
	if _, alone := fanDoc(t, gw.LeftToRight, 1, text); alone >= 2*room+120 {
		t.Errorf("a labelled edge alone: gap %v, want less than twice the room", alone)
	}
	// Nor does a labelled edge that climbs itself past one level, one in
	// a lane beside another, one with a fan in the next gap at its height,
	// or one beside a fan whose climbs stay above or below it.
	for _, c := range []struct {
		name  string
		boxes int
		edges [][2]gw.VertexIndex // the first labelled
	}{
		{"climbing", 3, [][2]gw.VertexIndex{{2, 1}, {0, 1}}},
		{"in a lane", 2, [][2]gw.VertexIndex{{0, 1}, {0, 1}}},
		{"before a fan", 5, [][2]gw.VertexIndex{{0, 1}, {1, 2}, {1, 3}, {1, 4}}},
		{"above a fan", 6, [][2]gw.VertexIndex{{0, 1}, {2, 3}, {2, 4}, {2, 5}}},
		{"below a fan", 6, [][2]gw.VertexIndex{{4, 5}, {0, 1}, {0, 2}, {0, 3}}},
	} {
		if gap := labelledGap(t, c.boxes, c.edges, text); gap >= 2*room+120 {
			t.Errorf("%s: gap %v, want less than twice the label's room", c.name, gap)
		}
	}
	_, down := fanDoc(t, gw.TopToBottom, 3, text)
	if _, plain := fanDoc(t, gw.TopToBottom, 3, ""); down != plain {
		t.Errorf("top to bottom: the labelled fan's gap %v, the unlabelled one's %v", down, plain)
	}
}

// labelledGap lays out, left to right, the boxes and the curves between
// them given, the first labelled text, and returns how far apart along
// the flow the first's ends are.
func labelledGap(t *testing.T, boxes int, edges [][2]gw.VertexIndex, text string) float64 {
	t.Helper()
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "curve", Route: gw.Curve}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	for range boxes {
		placeVertexAt(t, d, gw.Point{})
	}
	for k, e := range edges {
		label := ""
		if k == 0 {
			label = text
		}
		if _, err := d.Apply(gw.Command{Op: gw.AddEdge, From: e[0], To: e[1], FromPort: gw.None, ToPort: gw.None, Text: label}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
		t.Fatal(err)
	}
	return d.Vertex(edges[0][1]).Pos().X - d.Vertex(edges[0][0]).Pos().X
}
