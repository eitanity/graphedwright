package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
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
		for i := range d.Edges {
			e := &d.Edges[i]
			if !e.Live {
				continue
			}
			pts := append([]gw.Point{d.Vertices[e.From].Pos}, e.Bends...)
			pts = append(pts, d.Vertices[e.To].Pos)
			hit := false
			for k := 1; k < len(pts) && !hit; k++ {
				for j := range d.Vertices {
					v := &d.Vertices[j]
					u := gw.VertexIndex(j)
					if !v.Live || d.Within(e.From, u) || d.Within(e.To, u) || d.Within(u, e.From) || d.Within(u, e.To) {
						continue
					}
					if crosses(pts[k-1], pts[k], v) {
						t.Errorf("%s: edge %q cuts through %q", filepath.Base(f), e.Label, v.Label)
						hit = true
						break
					}
				}
			}
		}
	}
}
