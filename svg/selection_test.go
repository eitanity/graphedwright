package svg_test

import (
	"bytes"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// claims is a Selection that claims more than any document holds,
// naming the same vertex and edge each time.
type claims struct{}

func (claims) SelectedCount() (vertices, edges int) { return 1000, 1000 }
func (claims) SelectedVertex(int) gw.VertexIndex    { return 0 }
func (claims) SelectedEdge(int) gw.EdgeIndex        { return 0 }

// TestSelection: a Selected reads None out of range; the overlay
// outlines what a Selection names, and draws no more than the
// document's capacities whatever the Selection claims.
func TestSelection(t *testing.T) {
	s := svg.Selected{V: []gw.VertexIndex{1}, E: []gw.EdgeIndex{0}}
	if s.SelectedVertex(-1) != gw.None || s.SelectedVertex(1) != gw.None || s.SelectedEdge(-1) != gw.None || s.SelectedEdge(1) != gw.None || s.SelectedVertex(0) != 1 || s.SelectedEdge(0) != 0 {
		t.Error("Selected reads out of range")
	}
	d, err := gw.New(gw.Schema{VertexKinds: []gw.VertexKind{{Name: "box", Size: gw.Size{W: 40, H: 20}}}, EdgeKinds: []gw.EdgeKind{{Name: "to"}}}, gw.Caps{Vertices: 2, Edges: 1})
	if err != nil {
		t.Fatal(err)
	}
	must(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, Text: "a"})
	must(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, Text: "b", Pos: gw.Point{X: 100}})
	must(t, d, gw.Command{Op: gw.AddEdge, Kind: 0, From: 0, To: 1, FromPort: gw.None, ToPort: gw.None})
	for _, c := range []struct {
		name  string
		sel   svg.Selection
		rects int
		halos int
	}{{"none", nil, 0, 0}, {"Selected", s, 1, 1}, {"claims", claims{}, 2, 1}} {
		var buf bytes.Buffer
		if err := svg.Render(d, svg.Options{Mode: svg.Edit, Selection: c.sel}, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if r, h := strings.Count(out, `<rect class="selection"`), strings.Count(out, ` class="selection" stroke-width`); r != c.rects || h != c.halos {
			t.Errorf("%s: %d outlines and %d halos, want %d and %d", c.name, r, h, c.rects, c.halos)
		}
	}
}
