package svg_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// markStore keeps a string per token that would show if it were drawn.
type markStore struct{ data map[uint64]string }

func (s *markStore) Do(tok uint64) error { s.data[tok] = "HOST-DATA-<b>&"; return nil }
func (s *markStore) Undo(tok uint64)     { delete(s.data, tok) }
func (s *markStore) Redo(tok uint64)     { s.data[tok] = "HOST-DATA-<b>&" }
func (s *markStore) Forget(uint64)       {}
func (s *markStore) Released(gw.IDList)  {}

// TestHostDataNotDrawn: with a store holding data on every vertex, every
// edge and the document, each example renders byte for byte to its
// golden file, and each vertex and edge alone as it does without the
// store.
func TestHostDataNotDrawn(t *testing.T) {
	for _, ex := range []struct {
		name  string
		build func(testing.TB) (*gw.Document, svg.Options)
	}{{"diamond", diamond}, {"shapes", shapes}, {"compartments", compartments}, {"own-ports", ownPorts}} {
		d, opts := ex.build(t)
		r := svg.New()
		plain := make([][]byte, 0, d.VertexSlots()+d.EdgeSlots())
		for i := range d.VertexSlots() {
			var b bytes.Buffer
			if err := r.RenderVertex(d, opts, gw.VertexIndex(i), &b); err != nil { //nolint:gosec // bounded by the capacity
				t.Fatal(err)
			}
			plain = append(plain, b.Bytes())
		}
		for i := range d.EdgeSlots() {
			var b bytes.Buffer
			if err := r.RenderEdge(d, opts, gw.EdgeIndex(i), &b); err != nil { //nolint:gosec // bounded by the capacity
				t.Fatal(err)
			}
			plain = append(plain, b.Bytes())
		}
		d.ClearUndo()
		s := &markStore{data: map[uint64]string{}}
		if err := d.SetStore(s); err != nil {
			t.Fatal(err)
		}
		tok := uint64(0)
		host := func(v gw.VertexIndex, e gw.EdgeIndex) {
			tok++
			if _, err := d.Apply(gw.Command{Op: gw.HostChange, Vertex: v, Edge: e, Token: tok}); err != nil {
				t.Fatal(err)
			}
		}
		host(gw.None, gw.None)
		for i := range d.VertexSlots() {
			host(gw.VertexIndex(i), gw.None) //nolint:gosec // bounded by the capacity
		}
		for i := range d.EdgeSlots() {
			host(gw.None, gw.EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		}
		var got bytes.Buffer
		if err := r.Render(d, opts, &got); err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("testdata", "golden", ex.name+".svg")) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("%s: with host data, the render differs from the golden file\n%s", ex.name, firstDiff(got.Bytes(), want))
		}
		k := 0
		for i := range d.VertexSlots() {
			var b bytes.Buffer
			if err := r.RenderVertex(d, opts, gw.VertexIndex(i), &b); err != nil { //nolint:gosec // bounded by the capacity
				t.Fatal(err)
			}
			if !bytes.Equal(b.Bytes(), plain[k]) {
				t.Errorf("%s: vertex %d with host data:\n%s\nwant\n%s", ex.name, i, b.Bytes(), plain[k])
			}
			k++
		}
		for i := range d.EdgeSlots() {
			var b bytes.Buffer
			if err := r.RenderEdge(d, opts, gw.EdgeIndex(i), &b); err != nil { //nolint:gosec // bounded by the capacity
				t.Fatal(err)
			}
			if !bytes.Equal(b.Bytes(), plain[k]) {
				t.Errorf("%s: edge %d with host data:\n%s\nwant\n%s", ex.name, i, b.Bytes(), plain[k])
			}
			k++
		}
		if len(s.data) == 0 {
			t.Fatalf("%s: the store holds nothing", ex.name)
		}
	}
}
