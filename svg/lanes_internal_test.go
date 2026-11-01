package svg

import (
	"bytes"
	"math/rand/v2"
	"os"
	"regexp"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// textRE finds an edge's group and, in it, each label and stereotype as
// drawn: its class, position, anchor and text.
var (
	edgeRE = regexp.MustCompile(`<g class="edge [^"]*" id="([^"]+)"[^>]*>(.*?)</g>`)
	textRE = regexp.MustCompile(`<text class="(label|stereotype)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="(start|middle|end)">([^<]*)</text>`)
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
				d, err := gw.New(s, gw.Caps{})
				if err != nil {
					t.Fatal(err)
				}
				d.Flow = flow
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
						boxes = append(boxes, textBox(x, y, m[4], m[5]))
						owner = append(owner, g[1]+" "+m[1]+" "+m[5])
					}
				}
				if len(boxes) != 4 {
					t.Fatalf("flow %d route %d back %v: %d texts in\n%s", flow, route, back, len(boxes), buf.String())
				}
				for i := range d.Edges {
					var lane [1]gw.Point
					w := wayOf(d, gw.EdgeIndex(i), &lane)
					for k := 0; k <= 400; k++ {
						p := along(w.f, route, w.a, w.through, w.b, float64(k)/400)
						for j, b := range boxes {
							if inBox2(b, p) {
								t.Errorf("flow %d route %d back %v: %s crosses edge %s at %v", flow, route, back, owner[j], d.Edges[i].ID, p)
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
		d, err := gw.New(s, gw.Caps{})
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
		if m == nil || m[5] != "multiplicity 0..4" {
			t.Fatalf("flow %d: no label in\n%s", flow, buf.String())
		}
		x, _ := strconv.ParseFloat(m[2], 64)
		y, _ := strconv.ParseFloat(m[3], 64)
		box := textBox(x, y, m[4], m[5])
		for i := range d.Edges {
			var lane [1]gw.Point
			w := wayOf(d, gw.EdgeIndex(i), &lane)
			for k := 0; k <= 400; k++ {
				if p := along(w.f, gw.Orthogonal, w.a, w.through, w.b, float64(k)/400); inBox2(box, p) {
					t.Errorf("flow %d: the label %v crosses edge %s at %v", flow, box, d.Edges[i].ID, p)
					break
				}
			}
		}
		for v := range d.Vertices {
			x := &d.Vertices[v]
			vb := gw.Viewport{X: x.Pos.X - x.Size.W/2, Y: x.Pos.Y - x.Size.H/2, W: x.Size.W, H: x.Size.H}
			if box.X < vb.X+vb.W && vb.X < box.X+box.W && box.Y < vb.Y+vb.H && vb.Y < box.Y+box.H {
				t.Errorf("flow %d: the label %v overlaps vertex %s %v", flow, box, x.ID, vb)
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
	e := &d.Edges[i]
	var lane [1]gw.Point
	w := wayOf(d, i, &lane)
	route := d.Schema.EdgeKinds[e.Kind].Route
	for k := 0; k <= 1000; k++ {
		p := along(w.f, route, w.a, w.through, w.b, float64(k)/1000)
		for v := range d.Vertices {
			x := &d.Vertices[v]
			u := gw.VertexIndex(v) //nolint:gosec // bounded
			if !x.Live || d.Within(e.From, u) || d.Within(e.To, u) {
				continue
			}
			if p.X > x.Pos.X-x.Size.W/2+0.5 && p.X < x.Pos.X+x.Size.W/2-0.5 && p.Y > x.Pos.Y-x.Size.H/2+0.5 && p.Y < x.Pos.Y+x.Size.H/2-0.5 {
				return u, p
			}
		}
	}
	return gw.None, gw.Point{}
}

// TestRoutesClearOfContainers: laid out, no drawn route of an edge whose
// ends are in the same container, or both in none, passes through a
// vertex or container that is not one of its ends: the state machine
// above in either flow, and random graphs whose vertices are spread over
// containers, by every route.
func TestRoutesClearOfContainers(t *testing.T) {
	check := func(name string, d *gw.Document) {
		t.Helper()
		for i := range d.Edges {
			// An edge from one container to another is routed through
			// runs inside each, which can still pass the others that a
			// container holds; it is not checked here.
			if e := &d.Edges[i]; !e.Live || d.Vertices[e.From].Parent != d.Vertices[e.To].Parent {
				continue
			}
			if v, p := routeClear(d, gw.EdgeIndex(i)); v != gw.None { //nolint:gosec // bounded
				e := &d.Edges[i]
				t.Errorf("%s: edge %s from %s to %s passes through %s at %v; bends %v", name, e.ID, d.Vertices[e.From].ID, d.Vertices[e.To].ID, d.Vertices[v].ID, p, e.Bends)
			}
		}
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
		d, err := gw.New(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.Flow = gw.Flow(seed % 2) //nolint:gosec // 0 or 1
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

// TestExampleLabelsClear: in the demo's examples as they are laid out,
// no edge's label or stereotype crosses another edge, sampled finely
// along its whole length, or another edge's texts: among them a wire
// that turns alone, whose label stays at its turn rather than on a last
// leg that runs beside a curve into the same vertex, under the curve's.
func TestExampleLabelsClear(t *testing.T) {
	for _, name := range []string{"shapes", "diamond", "dependencies", "use-case", "threat-model"} {
		data, err := os.ReadFile("../web/examples/" + name + ".json") //nolint:gosec // the demo's examples, by fixed name
		if err != nil {
			t.Fatal(err)
		}
		d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
		if err != nil {
			t.Fatal(err)
		}
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
				box := textBox(x, y, m[4], m[5])
				for j, b := range boxes {
					if owners[j] != g[1] && box.X < b.X+b.W && b.X < box.X+box.W && box.Y < b.Y+b.H && b.Y < box.Y+box.H {
						t.Errorf("%s: the %s %q of edge %s overlaps a text of edge %s", name, m[1], m[5], g[1], owners[j])
					}
				}
				boxes = append(boxes, box)
				owners = append(owners, g[1])
				for i := range d.Edges {
					if !d.Edges[i].Live || string(d.Edges[i].ID) == g[1] {
						continue
					}
					var lane [1]gw.Point
					w := wayOf(d, gw.EdgeIndex(i), &lane)
					route := d.Schema.EdgeKinds[d.Edges[i].Kind].Route
					for k := 0; k <= 400; k++ {
						if p := along(w.f, route, w.a, w.through, w.b, float64(k)/400); inBox2(box, p) {
							t.Errorf("%s: the %s %q of edge %s crosses edge %s at %v", name, m[1], m[5], g[1], d.Edges[i].ID, p)
							break
						}
					}
				}
			}
		}
	}
}
