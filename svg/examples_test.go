package svg_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// The kinds of the test schema.
const (
	kSource gw.KindIndex = iota
	kNode
	kDecision
	kLeaf
	kRound
)

// The edge kinds of the test schema.
const (
	eTo gw.KindIndex = iota
	eOutcome
	eWire
)

// testSchema has one of every shape and route: a pill source, a round
// node with ports, a diamond decision, a plain rect leaf and a circle;
// straight arrows from a source, weighted curves between nodes, and
// orthogonal wires between anything.
func testSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 96, H: 24}, Root: true},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36},
				Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right}, {Name: "aux", Side: gw.Right}, {Name: "top", Side: gw.Top}},
				Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number, Default: "0.5"}, {Key: "mode", Kind: gw.Text}}},
			{Name: "decision", Shape: gw.Diamond, Size: gw.Size{W: 80, H: 48}},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 100, H: 30}, Leaf: true},
			{Name: "round thing", Shape: gw.Circle, Size: gw.Size{W: 40, H: 40}},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "to", From: []gw.KindIndex{kSource}, Arrow: true},
			{Name: "outcome", Route: gw.Curve, Weighted: true, Arrow: true, Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number}}},
			{Name: "wire", Route: gw.Orthogonal},
		},
		Acyclic: true,
	}
}

func newDoc(t testing.TB, caps gw.Caps) *gw.Document {
	t.Helper()
	d, err := makeDoc(testSchema(), caps)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func must(t testing.TB, d *gw.Document, c gw.Command) gw.Change {
	t.Helper()
	ch, err := d.Apply(c)
	if err != nil {
		t.Fatalf("%+v: %v", c, err)
	}
	return ch
}

func vertex(t testing.TB, d *gw.Document, kind gw.KindIndex, id string, x, y float64) gw.VertexIndex {
	t.Helper()
	v := must(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: id}).Vertices[0]
	must(t, d, gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: x, Y: y}})
	return v
}

func edge(t testing.TB, d *gw.Document, kind gw.KindIndex, from, to gw.VertexIndex, fp, tp gw.PortIndex, label string, weight float64) gw.EdgeIndex {
	t.Helper()
	return must(t, d, gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: fp, ToPort: tp, Text: label, Weight: weight}).Edges[0]
}

// diamond: a source into node a, which splits to b and c, which join at
// leaf d; with annotations on everything.
func diamond(t testing.TB) (*gw.Document, svg.Options) {
	d := newDoc(t, gw.Caps{})
	s := vertex(t, d, kSource, "s", 60, 100)
	a := vertex(t, d, kNode, "a", 220, 100)
	b := vertex(t, d, kNode, "b", 420, 40)
	c := vertex(t, d, kNode, "c", 420, 160)
	l := vertex(t, d, kLeaf, "d", 620, 100)
	edge(t, d, eTo, s, a, gw.None, 0, "", 0)
	edge(t, d, eOutcome, a, b, 1, 0, "fail", 0.3)
	edge(t, d, eOutcome, a, c, 1, 0, "pass", 0.7)
	edge(t, d, eOutcome, b, l, 1, gw.None, "", 0.3)
	edge(t, d, eOutcome, c, l, 1, gw.None, "", 0.7)
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: a, Key: "mode", Value: "sequential"})
	opts := svg.Options{Title: "Diamond", Desc: "a source, a split and a join",
		VertexNotes: []svg.Annotation{{}, {Text: "2.1/period"}, {Text: "0.63/period", Class: "hot"}, {Text: "1.47/period"}, {Text: "2.1/period"}},
		EdgeNotes:   []svg.Annotation{{}, {Text: "30%"}, {Text: "70%"}, {}, {}}}
	return d, opts
}

// shapes: one vertex of every kind, with every route and the hostile
// text a label or an attribute can carry, in edit mode with a selection.
func shapes(t testing.TB) (*gw.Document, svg.Options) {
	d := newDoc(t, gw.Caps{})
	s := vertex(t, d, kSource, `s "quoted" & <tagged>`, 60, 60)
	n := vertex(t, d, kNode, "a label far too long to fit in the box it is drawn in", 240, 60)
	dc := vertex(t, d, kDecision, "decide?", 240, 180)
	l := vertex(t, d, kLeaf, "leaf", 440, 180)
	r := vertex(t, d, kRound, "ö", 440, 60)
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: n, Key: "mode", Value: `x="1" <b>&`})
	// A key the document would refuse, written past its checks, to see
	// the renderer's own mapping of a name to a token.
	d.Vertices[n].Attrs = append(d.Vertices[n].Attrs, gw.Attr{Key: "weird key!", Value: "v"})
	edge(t, d, eTo, s, n, gw.None, 0, "", 0)
	edge(t, d, eWire, n, dc, 3, gw.None, "wire <&>", 0)
	edge(t, d, eOutcome, dc, l, gw.None, gw.None, "yes", 0.9)
	edge(t, d, eOutcome, n, r, 2, gw.None, "", 0.1)
	// One vertex left unplaced, to show the class.
	must(t, d, gw.Command{Op: gw.AddVertex, Kind: kLeaf, ID: "free", Text: "unplaced"})
	opts := svg.Options{Mode: svg.Edit, Title: "Shapes & routes", Selection: svg.Selected{V: []gw.VertexIndex{n}, E: []gw.EdgeIndex{2}}}
	return d, opts
}

// big is a layered graph of n vertices and about m edges with the
// positions a layered layout would give, for the budget.
func big(t testing.TB, n, m int) *gw.Document {
	d := newDoc(t, gw.Caps{Vertices: n, Edges: m})
	const cols = 40
	for i := 0; i < n; i++ {
		kind := kNode
		if i < cols {
			kind = kSource
		}
		v := must(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, Text: "vertex"}).Vertices[0]
		must(t, d, gw.Command{Op: gw.Move, Vertex: v, Pos: gw.Point{X: float64(i/cols) * 160, Y: float64(i%cols) * 48}})
	}
	for i := 0; d.LiveEdges() < m && i < 4*m; i++ {
		from := (i * 7919) % n
		to := from + cols + (i*104729)%(2*cols)
		if to >= n {
			continue
		}
		kind, fp, tp := eOutcome, gw.PortIndex(1), gw.PortIndex(0)
		if from < cols {
			kind, fp = eTo, gw.None
		}
		cmd := gw.Command{Op: gw.AddEdge, Kind: kind, From: gw.VertexIndex(from), To: gw.VertexIndex(to), FromPort: fp, ToPort: tp, Weight: float64(i%5 + 1)} //nolint:gosec // below n
		if _, err := d.Apply(cmd); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// notation: the figures of a use case diagram and a data flow diagram:
// an actor, ellipses in a system boundary, a store and a dashed box; an
// association, dashed
// stereotyped edges with open heads, a generalisation with a hollow
// head; and straight edges between shapes without ports at every
// angle, each clipped where it leaves its shape.
func notation(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 76}},
			{Name: "use case", Shape: gw.Circle, Size: gw.Size{W: 140, H: 44}},
			{Name: "store", Shape: gw.DataStore, Size: gw.Size{W: 110, H: 36}},
			{Name: "zone", Shape: gw.RoundRect, Size: gw.Size{W: 100, H: 40}, Dashed: true},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Contains: []gw.KindIndex{1}},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "association"},
			{Name: "include", Arrow: true, Head: gw.OpenHead, Dashed: true, Stereotype: "«include»"},
			{Name: "generalisation", Arrow: true, Head: gw.HollowHead},
			{Name: "flow", Arrow: true},
		},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	user := vertex(t, d, 0, "user", 60, 120)
	admin := vertex(t, d, 0, "admin", 60, 260)
	order := vertex(t, d, 1, "place order", 280, 80)
	pay := vertex(t, d, 1, "pay", 300, 200)
	db := vertex(t, d, 2, "orders", 520, 80)
	zone := vertex(t, d, 3, "dmz", 520, 220)
	edge(t, d, 0, user, order, gw.None, gw.None, "", 0)
	edge(t, d, 1, order, pay, gw.None, gw.None, "", 0)
	edge(t, d, 2, admin, user, gw.None, gw.None, "", 0)
	edge(t, d, 3, order, db, gw.None, gw.None, "write", 0)
	edge(t, d, 3, zone, pay, gw.None, gw.None, "", 0)
	sys := must(t, d, gw.Command{Op: gw.AddVertex, Kind: 4, ID: "shop", Text: "shop"}).Vertices[0]
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: order, To: sys})
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: pay, To: sys})
	return d, svg.Options{Title: "Notation", Mode: svg.View}
}

// bends: a graph laid out in four layers whose long edges, one of each
// route, are drawn through the bends the layout gave them, around the
// vertices of the layers they cross.
func bends(t testing.TB) (*gw.Document, svg.Options) {
	return bent(t, gw.LeftToRight)
}

// tree: the same graph flowing top to bottom, as a dependency tree: its
// curves and elbows leave the bottom of a vertex and enter the top, and
// an edge's label and annotation sit either side of it.
func tree(t testing.TB) (*gw.Document, svg.Options) {
	d, opts := bent(t, gw.TopToBottom)
	opts.Title = "Tree"
	opts.EdgeNotes = []svg.Annotation{{}, {Text: "40%"}}
	must(t, d, gw.Command{Op: gw.Relabel, Vertex: gw.None, Edge: 1, Text: "maybe"})
	return d, opts
}

// bent builds the graph of bends in a flow and lays it out.
func bent(t testing.TB, flow gw.Flow) (*gw.Document, svg.Options) {
	d := newDoc(t, gw.Caps{})
	d.SetFlow(flow)
	add := func(kind gw.KindIndex, id string) gw.VertexIndex {
		return must(t, d, gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: id}).Vertices[0]
	}
	s := add(kSource, "s")
	r := add(kRound, "r")
	d1 := add(kDecision, "d1")
	d2 := add(kDecision, "d2")
	l := add(kLeaf, "l")
	edge(t, d, eTo, s, d1, gw.None, gw.None, "", 0)
	edge(t, d, eOutcome, d1, d2, gw.None, gw.None, "", 0)
	edge(t, d, eOutcome, d2, l, gw.None, gw.None, "", 0)
	edge(t, d, eTo, s, l, gw.None, gw.None, "skip", 0)
	edge(t, d, eOutcome, s, d2, gw.None, gw.None, "", 0)
	edge(t, d, eWire, r, d2, gw.None, gw.None, "", 0)
	edge(t, d, eWire, d1, l, gw.None, gw.None, "", 0)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	return d, svg.Options{Title: "Bends"}
}

// keywords: vertex kinds with a keyword line above the label: a box,
// one whose label is cut, an actor, and a container, whose keyword
// comes before its title; beside a kind without one.
func keywords(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "block", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part def»"},
			{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 76}, Keyword: "«actor»"},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Keyword: "«system»"},
			{Name: "plain", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	user := vertex(t, d, 1, "driver", 60, 100)
	car := vertex(t, d, 0, "vehicle", 240, 60)
	long := vertex(t, d, 0, "an engine with a long name", 240, 160)
	plain := vertex(t, d, 3, "plain", 440, 60)
	edge(t, d, 0, user, car, gw.None, gw.None, "", 0)
	edge(t, d, 0, car, plain, gw.None, gw.None, "", 0)
	sys := must(t, d, gw.Command{Op: gw.AddVertex, Kind: 2, ID: "vehicle system", Text: "vehicle system"}).Vertices[0]
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: car, To: sys})
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: long, To: sys})
	return d, svg.Options{Title: "Keywords", Mode: svg.View}
}

// decorationSchema has an edge kind for each tail and each marked head,
// on each route.
func decorationSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "block", Shape: gw.Rect, Size: gw.Size{W: 100, H: 36}}},
		EdgeKinds: []gw.EdgeKind{
			{Name: "composition", Tail: gw.FilledDiamond},
			{Name: "aggregation", Route: gw.Curve, Tail: gw.HollowDiamond, Arrow: true, Head: gw.OpenHead},
			{Name: "containment", Route: gw.Orthogonal, Tail: gw.CircledPlus},
			{Name: "typing", Arrow: true, Head: gw.TypedHead},
			{Name: "redefinition", Route: gw.Curve, Arrow: true, Head: gw.BarHead},
			{Name: "subsetting", Route: gw.Orthogonal, Arrow: true, Head: gw.TypedHead, Tail: gw.HollowDiamond},
		},
	}
}

// decorated: owners joined to their parts by each tail, and features
// joined by the marked heads, laid out in the flow. Each tail leaves a
// vertex of its own, since edges that leave one side of a vertex share
// their start.
func decorated(t testing.TB, flow gw.Flow) (*gw.Document, svg.Options) {
	d, err := makeDoc(decorationSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetFlow(flow)
	add := func(id string) gw.VertexIndex {
		return must(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: gw.ID(id), Text: id}).Vertices[0]
	}
	owner, fleet, depot := add("vehicle"), add("fleet"), add("depot")
	a, b, c := add("engine"), add("wheels"), add("cargo")
	typed, x, y := add("mass"), add("Real"), add("Mass")
	edge(t, d, 0, owner, a, gw.None, gw.None, "", 0)
	edge(t, d, 1, fleet, b, gw.None, gw.None, "", 0)
	edge(t, d, 2, depot, c, gw.None, gw.None, "", 0)
	edge(t, d, 3, typed, x, gw.None, gw.None, "", 0)
	edge(t, d, 4, typed, y, gw.None, gw.None, "", 0)
	edge(t, d, 5, a, x, gw.None, gw.None, "", 0)
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	return d, svg.Options{Title: "Decorations"}
}

// tails: decorations at an edge's start and the marked hollow heads,
// left to right; tailsDown, top to bottom.
func tails(t testing.TB) (*gw.Document, svg.Options) { return decorated(t, gw.LeftToRight) }

func tailsDown(t testing.TB) (*gw.Document, svg.Options) { return decorated(t, gw.TopToBottom) }

// compartmentSchema has a part kind that shows three of its attributes
// as compartments, and a plain kind beside it.
func compartmentSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{
				Name: "part", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part def»",
				Attrs:        []gw.AttrSpec{{Key: "attributes"}, {Key: "ports"}, {Key: "doc"}},
				Compartments: []string{"attributes", "ports", "doc"}, MaxWidth: 200,
			},
			{Name: "state", Shape: gw.RoundRect, Size: gw.Size{W: 100, H: 36}, Attrs: []gw.AttrSpec{{Key: "do"}}, Compartments: []string{"do"}},
			{Name: "plain", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}
}

// compartments draws a part with three lines in one compartment, an
// empty one left out, and a line too long for its widest; a part with
// none, drawn as a plain box; a state with one line; and a plain kind.
func compartments(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(compartmentSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	vehicle := vertex(t, d, 0, "vehicle", 120, 110)
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: vehicle, Key: "attributes", Value: "mass : Real\nwheels : Integer = 4\ncolour : String"})
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: vehicle, Key: "doc", Value: "A vehicle is anything with wheels that carries people or goods on a road."})
	empty := vertex(t, d, 0, "engine", 360, 60)
	idle := vertex(t, d, 1, "idle", 360, 160)
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: idle, Key: "do", Value: "wait"})
	plain := vertex(t, d, 2, "plain", 560, 60)
	edge(t, d, 0, vehicle, empty, gw.None, gw.None, "", 0)
	edge(t, d, 0, vehicle, idle, gw.None, gw.None, "", 0)
	edge(t, d, 0, empty, plain, gw.None, gw.None, "", 0)
	return d, svg.Options{Title: "Compartments", Mode: svg.View}
}

// formats draws usages as name : Type [m], with each part set and not,
// one too long for its box and cut, and transitions as
// trigger [guard] / effect.
func formats(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{{
			Name: "usage", Shape: gw.Rect, Size: gw.Size{W: 150, H: 36},
			Attrs: []gw.AttrSpec{{Key: "type"}, {Key: "mult"}}, LabelFormat: "{label}{ : |type}{ [|mult|]}",
		}},
		EdgeKinds: []gw.EdgeKind{{
			Name: "transition", Route: gw.Curve, Arrow: true,
			Attrs: []gw.AttrSpec{{Key: "trigger"}, {Key: "guard"}, {Key: "effect"}}, LabelFormat: "{trigger}{ [|guard|]}{ / |effect}",
		}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	set := func(v gw.VertexIndex, e gw.EdgeIndex, key, value string) {
		must(t, d, gw.Command{Op: gw.SetAttr, Vertex: v, Edge: e, Key: key, Value: value})
	}
	wheels := vertex(t, d, 0, "wheels", 100, 60)
	set(wheels, gw.None, "type", "Wheel")
	set(wheels, gw.None, "mult", "4")
	engine := vertex(t, d, 0, "engine", 100, 160)
	set(engine, gw.None, "type", "Engine")
	frame := vertex(t, d, 0, "frame", 440, 60)
	long := vertex(t, d, 0, "suspension", 440, 160)
	set(long, gw.None, "type", "IndependentSuspension")
	set(long, gw.None, "mult", "0..2")
	mount := edge(t, d, 0, wheels, frame, gw.None, gw.None, "ignored", 0)
	set(gw.None, mount, "trigger", "mount")
	set(gw.None, mount, "guard", "aligned")
	set(gw.None, mount, "effect", "bolt()")
	stop := edge(t, d, 0, engine, long, gw.None, gw.None, "", 0)
	set(gw.None, stop, "guard", "idle")
	return d, svg.Options{Title: "Label formats", Mode: svg.View}
}

// ownPortSchema has a part kind and a container kind with no ports, a
// block kind with the kind's own, and a routed edge kind.
func ownPortSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "part", Shape: gw.Rect, Size: gw.Size{W: 100, H: 40}},
			{Name: "system", Size: gw.Size{W: 160, H: 100}, Container: true},
			{Name: "block", Shape: gw.RoundRect, Size: gw.Size{W: 100, H: 40}, Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right}}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "connect", Route: gw.Orthogonal, Arrow: true}},
	}
}

// ownPorts draws a part with ports of its own on every side, a
// container with ports on its border, and a block with only its kind's,
// whose ports are not named, joined at the ports.
func ownPorts(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(ownPortSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	addPort := func(v gw.VertexIndex, name string, side gw.Side) {
		must(t, d, gw.Command{Op: gw.AddPort, Vertex: v, Port: gw.Port{Name: name, Side: side}})
	}
	engine := vertex(t, d, 0, "engine", 260, 140)
	addPort(engine, "fuel", gw.Left)
	addPort(engine, "air", gw.Left)
	addPort(engine, "torque", gw.Right)
	addPort(engine, "ecu", gw.Top)
	addPort(engine, "mount", gw.Bottom)
	tank := vertex(t, d, 2, "tank", 60, 60)
	sys := must(t, d, gw.Command{Op: gw.AddVertex, Kind: 1, ID: "drive", Text: "drive"}).Vertices[0]
	gearbox := vertex(t, d, 0, "gearbox", 520, 140)
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: gearbox, To: sys})
	addPort(sys, "power", gw.Left)
	addPort(sys, "wheels", gw.Right)
	edge(t, d, 0, tank, engine, 1, 0, "", 0)
	edge(t, d, 0, engine, sys, 2, 0, "", 0)
	axle := vertex(t, d, 0, "axle", 760, 140)
	edge(t, d, 0, sys, axle, 1, gw.None, "", 0)
	return d, svg.Options{Title: "Ports of their own", Mode: svg.View}
}

// cutTexts draws a text cut to fit of each kind: a part whose keyword,
// label and compartment line are cut, a container whose title is, a
// vertex whose annotation is, and one with nothing cut.
func cutTexts(t testing.TB) (*gw.Document, svg.Options) {
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{
				Name: "part", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part definition of a kind»",
				Attrs: []gw.AttrSpec{{Key: "doc"}}, Compartments: []string{"doc"}, MaxWidth: 160,
			},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Keyword: "«system»"},
			{Name: "plain", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	part := vertex(t, d, 0, "a part with a name too long for it", 120, 70)
	must(t, d, gw.Command{Op: gw.SetAttr, Vertex: part, Key: "doc", Value: "short\nA line of documentation too long to fit."})
	inner := vertex(t, d, 2, "inside", 400, 90)
	sys := must(t, d, gw.Command{Op: gw.AddVertex, Kind: 1, ID: "sys", Text: "a system whose title is too long"}).Vertices[0]
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: inner, To: sys})
	noted := vertex(t, d, 2, "noted", 120, 220)
	plain := vertex(t, d, 2, "plain", 400, 220)
	edge(t, d, 0, part, inner, gw.None, gw.None, "", 0)
	edge(t, d, 0, noted, plain, gw.None, gw.None, "", 0)
	notes := make([]svg.Annotation, d.LiveVertices()+1)
	notes[noted] = svg.Annotation{Text: "an annotation far too long to fit"}
	return d, svg.Options{Title: "Cut texts", Mode: svg.View, VertexNotes: notes}
}

// cutTextsHover is cutTexts with the part hovered, its cut texts drawn
// in full in the overlay.
func cutTextsHover(t testing.TB) (*gw.Document, svg.Options) {
	d, opts := cutTexts(t)
	opts.Hover, opts.HasHover = 0, true
	return d, opts
}
