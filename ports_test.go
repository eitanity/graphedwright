package graphedwright_test

import (
	"bytes"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
)

// portSchema has a part kind with no ports of its own kind, a block kind
// with one, and a container.
func portSchema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "part", Shape: gw.Rect, Size: gw.Size{W: 100, H: 40}},
			{Name: "block", Shape: gw.Rect, Size: gw.Size{W: 100, H: 40}, Ports: []gw.Port{{Name: "in", Side: gw.Left}}},
			{Name: "system", Size: gw.Size{W: 160, H: 100}, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow"}},
	}
}

func port(name string, side gw.Side) gw.Port { return gw.Port{Name: name, Side: side} }

// TestAddPort: a vertex takes ports of its own after its kind's, each
// named once on it, within the capacity, and undo and redo take them
// away and give them back.
func TestAddPort(t *testing.T) {
	d, err := makeDoc(portSchema(), gw.Caps{PortsPerVertex: 3})
	if err != nil {
		t.Fatal(err)
	}
	b := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 1, ID: "b"}).Vertices[0]
	ch := mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: b, Port: port("out", gw.Right)})
	if got := d.Vertex(b).Ports(); got.Len() != 2 || got.At(1) != port("out", gw.Right) || d.OwnPorts(b) != 1 {
		t.Fatalf("ports %v, own %d", got, d.OwnPorts(b))
	}
	if len(ch.Vertices) != 1 || ch.Vertices[0] != b {
		t.Errorf("change %+v", ch)
	}
	for _, tc := range []struct {
		p    gw.Port
		want string
	}{
		{port("in", gw.Top), `vertex "b" has a port "in"`},
		{port("", gw.Top), "a port needs a name, a side and a non-negative max"},
		{gw.Port{Name: "x", Side: gw.Bottom + 1}, "a port needs a name, a side and a non-negative max"},
		{gw.Port{Name: "x", Max: -1}, "a port needs a name, a side and a non-negative max"},
		{port("a\tb", gw.Top), `port "a\tb" holds the control character`},
	} {
		if _, err := d.Apply(gw.Command{Op: gw.AddPort, Vertex: b, Port: tc.p}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: got %v, want %q", tc.p, err, tc.want)
		}
	}
	mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: b, Port: port("ctl", gw.Top)})
	if _, err := d.Apply(gw.Command{Op: gw.AddPort, Vertex: b, Port: port("more", gw.Top)}); err == nil || !strings.Contains(err.Error(), "4 ports exceed the limit of 3") {
		t.Errorf("past the capacity: %v", err)
	}
	d.Undo()
	d.Undo()
	if n := d.Vertex(b).Ports().Len(); n != 1 {
		t.Fatalf("undone: %d ports, want the kind's 1", n)
	}
	d.Redo()
	if got := d.Vertex(b).Ports(); got.Len() != 2 || got.At(1).Name != "out" {
		t.Errorf("redone: %v", got)
	}
}

// TestDeletePort: a vertex's own port goes when no edge uses it, with a
// refusal naming the edges that do; the kind's ports stay; edges at the
// ports after it follow them, and undo puts all back, a deleted edge's
// end at the port included.
func TestDeletePort(t *testing.T) {
	d, err := makeDoc(portSchema(), gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	a := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "a"}).Vertices[0]
	b := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 1, ID: "b"}).Vertices[0]
	for _, name := range []string{"p", "q", "r"} {
		mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: a, Port: port(name, gw.Right)})
	}
	// An edge to a vertex with ports only of its own need not name one.
	plain := mustApply(t, d, gw.Command{Op: gw.AddEdge, ID: "plain", From: b, To: a, FromPort: 0, ToPort: gw.None}).Edges[0]
	ep := mustApply(t, d, gw.Command{Op: gw.AddEdge, ID: "ep", From: a, To: b, FromPort: 0, ToPort: 0}).Edges[0]
	er := mustApply(t, d, gw.Command{Op: gw.AddEdge, ID: "er", From: a, To: b, FromPort: 2, ToPort: 0}).Edges[0]
	eq := mustApply(t, d, gw.Command{Op: gw.AddEdge, ID: "eq", From: a, To: b, FromPort: 1, ToPort: 0}).Edges[0]

	for _, tc := range []struct {
		v    gw.VertexIndex
		name string
		want string
	}{
		{a, "p", `port "p" is used by "ep"`},
		{b, "in", `port "in" is the kind's, not the vertex's own`},
		{a, "z", `vertex "a" has no port "z"`},
	} {
		if _, err := d.Apply(gw.Command{Op: gw.DeletePort, Vertex: tc.v, Text: tc.name}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("delete %q: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, err := d.Apply(gw.Command{Op: gw.DeletePort, Vertex: b, Text: "in"}); err == nil {
		t.Error("the kind's port was deleted")
	}

	// Port q goes once its edge has: r's edge follows r down, and undo
	// gives q back and puts the edge's end at it.
	mustApply(t, d, gw.Command{Op: gw.DeleteEdge, Edge: eq})
	mustApply(t, d, gw.Command{Op: gw.DeletePort, Vertex: a, Text: "q"})
	if got := d.Vertex(a).Ports(); got.Len() != 2 || got.At(1).Name != "r" || d.Edge(er).FromPort() != 1 || d.Edge(ep).FromPort() != 0 {
		t.Fatalf("deleted q: ports %v, er at %d, ep at %d", got, d.Edge(er).FromPort(), d.Edge(ep).FromPort())
	}
	d.Undo()
	if got := d.Vertex(a).Ports(); got.Len() != 3 || got.At(1).Name != "q" || d.Edge(er).FromPort() != 2 {
		t.Fatalf("undone: ports %v, er at %d", got, d.Edge(er).FromPort())
	}
	d.Undo()
	if e := d.Edge(eq); !e.Live() || e.FromPort() != 1 {
		t.Fatalf("edge back: live %v at port %d, want 1", e.Live(), e.FromPort())
	}
	d.Redo()
	d.Redo()
	if d.Edge(er).FromPort() != 1 || d.Edge(plain).ToPort() != gw.None || d.Vertex(a).Ports().Len() != 2 {
		t.Errorf("redone: er at %d, plain at %d, %d ports", d.Edge(er).FromPort(), d.Edge(plain).ToPort(), d.Vertex(a).Ports().Len())
	}
}

// TestPortRoom: a vertex grows so that the ports on each side are
// PortGap apart, and back when they go, which marks the layout stale; a
// container keeps that room around what it holds; and a saved document
// loads with the ports, the edges at them and the sizes it had.
func TestPortRoom(t *testing.T) {
	d, err := makeDoc(portSchema(), gw.Caps{PortsPerVertex: 16})
	if err != nil {
		t.Fatal(err)
	}
	a := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "a", Pos: gw.Point{X: 100, Y: 100}}).Vertices[0]
	for i, name := range []string{"l1", "l2", "l3"} {
		ch := mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: a, Port: port(name, gw.Left)})
		want := max(40, float64((i+2)*gw.PortGap))
		if got := d.Vertex(a).Size(); got.H != want || got.W != 100 {
			t.Errorf("%d left ports: size %v, want height %g", i+1, got, want)
		}
		if ch.Layout != (want > 40) {
			t.Errorf("%d left ports: layout stale %v", i+1, ch.Layout)
		}
	}
	for _, name := range []string{"t1", "t2", "t3", "t4", "t5", "t6"} {
		mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: a, Port: port(name, gw.Top)})
	}
	// Each top port's slot is its two-character label either side
	// of it and a gap.
	slot := float64(max(gw.PortGap, 2*2*gw.PortLabelChar+16))
	if got := d.Vertex(a).Size(); got != (gw.Size{W: 7 * slot, H: 4 * gw.PortGap}) {
		t.Errorf("six on top: %v, want %v", got, gw.Size{W: 7 * slot, H: 4 * gw.PortGap})
	}
	if got := d.Vertex(a).Pos(); got != (gw.Point{X: 100, Y: 100}) {
		t.Errorf("moved to %v", got)
	}
	mustApply(t, d, gw.Command{Op: gw.Resize, Vertex: a, Size: gw.Size{W: 50, H: 300}})
	if got := d.Vertex(a).Size(); got != (gw.Size{W: 7 * slot, H: 300}) {
		t.Errorf("resized below its ports: %v", got)
	}
	d.Undo()

	sys := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 2, ID: "sys"}).Vertices[0]
	in := mustApply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: "in", Pos: gw.Point{X: 400, Y: 100}}).Vertices[0]
	mustApply(t, d, gw.Command{Op: gw.SetParent, Vertex: in, To: sys})
	for _, name := range []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		mustApply(t, d, gw.Command{Op: gw.AddPort, Vertex: sys, Port: port(name, gw.Right)})
	}
	if got := d.Vertex(sys).Size().H; got != 8*gw.PortGap {
		t.Errorf("container with seven on the right: height %g, want %d", got, 8*gw.PortGap)
	}
	mustApply(t, d, gw.Command{Op: gw.AddEdge, ID: "e", From: a, To: sys, FromPort: 0, ToPort: 3})

	var text bytes.Buffer
	if err := doc.Encode(d, "ports", &text); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), `"ports":[{"name":"l1","side":"left"}`) {
		t.Errorf("no ports saved: %s", text.String())
	}
	again, err := doc.Decode(text.Bytes(), doc.Options{Schemas: []doc.Named{{Name: "ports", Kinds: d.Kinds()}}, Caps: gw.Caps{PortsPerVertex: 16}})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []gw.VertexIndex{a, sys, in} {
		w := again.VertexByID(d.Vertex(v).ID())
		got, want := again.Vertex(w), d.Vertex(v)
		if got.Pos() != want.Pos() || got.Size() != want.Size() || got.Ports().Len() != want.Ports().Len() {
			t.Errorf("%s loaded at %v %v with %d ports, saved at %v %v with %d", want.ID(), got.Pos(), got.Size(), got.Ports().Len(), want.Pos(), want.Size(), want.Ports().Len())
		}
	}
	if e := again.Edge(again.EdgeByID("e")); e.FromPort() != 0 || e.ToPort() != 3 {
		t.Errorf("edge loaded at ports %d and %d, want 0 and 3", e.FromPort(), e.ToPort())
	}
}
