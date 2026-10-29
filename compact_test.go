package graphedwright_test

import (
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// TestCompact: tombstones go, live elements keep their order and their
// ports and attributes, edges are renumbered to the new vertices, the
// maps say where everything went, and the adjacency still answers.
func TestCompact(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	s, a, b, c := chain(t, d)
	apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: b, Key: "note", Value: "keep"})
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: a})
	before := snapshot(d)
	vmap, emap := d.Compact()
	if len(vmap) != 4 || vmap[s] != 0 || vmap[a] != gw.None || vmap[b] != 1 || vmap[c] != 2 {
		t.Errorf("vmap %v", vmap)
	}
	if len(emap) != 3 || emap[0] != gw.None || emap[1] != gw.None || emap[2] != 0 {
		t.Errorf("emap %v", emap)
	}
	if len(d.Vertices) != 3 || len(d.Edges) != 1 || d.Edges[0].From != 1 || d.Edges[0].To != 2 {
		t.Errorf("after compact: %d vertices %d edges, edge %+v", len(d.Vertices), len(d.Edges), d.Edges[0])
	}
	if d.Vertices[1].ID != "b" || d.Vertices[1].Attrs[2].Value != "keep" || d.Vertices[1].Ports[1].Name != "out" {
		t.Errorf("b after compact: %+v", d.Vertices[1])
	}
	if got := snapshot(d); got == before {
		t.Error("snapshot unchanged though indices moved")
	}
	// The port limit and the cycle check read the rebuilt lists.
	b, c = 1, 2
	addEdge(t, d, eOutcome, b, c, 1, gw.None)
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: b, To: c, FromPort: 1, ToPort: gw.None}, `port "out" of vertex "b" is full`)
	x := addVertex(t, d, kNode, "x")
	addEdge(t, d, eOutcome, x, b, 1, 0)
	refuse(t, d, gw.Command{Op: gw.AddEdge, Kind: eOutcome, From: b, To: x, FromPort: 0, ToPort: 0}, "edge would make a cycle: b → x → b")
	// Compacting a clean document changes nothing but the log.
	vmap, _ = d.Compact()
	for i, v := range vmap {
		if int(v) != i {
			t.Errorf("clean compact moved %d to %d", i, v)
		}
	}
}

// TestUnreachable: the vertices no root reaches, in index order, and
// none once they are joined.
func TestUnreachable(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, _, _ := chain(t, d)
	x := addVertex(t, d, kNode, "x")
	y := addVertex(t, d, kLeaf, "y")
	addEdge(t, d, eOutcome, x, y, 1, gw.None)
	got := d.Unreachable(make([]gw.VertexIndex, 0, 8))
	if len(got) != 2 || got[0] != x || got[1] != y {
		t.Errorf("unreachable %v, want [%d %d]", got, x, y)
	}
	addEdge(t, d, eOutcome, a, x, 1, 0)
	if got = d.Unreachable(got); len(got) != 0 {
		t.Errorf("unreachable after joining: %v", got)
	}
	if got = d.Unreachable(nil); len(got) != 0 {
		t.Errorf("no room: %v", got)
	}
}
