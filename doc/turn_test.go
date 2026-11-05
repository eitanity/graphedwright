package doc_test

import (
	"bytes"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
)

// TestTurnedRoundTrip: a document in a top-to-bottom flow with vertices
// of a kind that turns, one at its own size, is written with its sizes
// as drawn and read back the same, whether the flow comes before the
// vertices, as Encode writes it, or after them; a Layout to the other
// flow and its undo write the document byte for byte as it was.
func TestTurnedRoundTrip(t *testing.T) {
	k, err := gw.Compile(gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "fork", Shape: gw.Provided, Size: gw.Size{W: 8, H: 80}}},
		Shapes:      []gw.ShapeDef{{Name: "bar", Box: gw.Size{W: 8, H: 80}, Turns: true, Paths: []gw.ShapePath{{D: "M 0 0 H 8 V 80 H 0 Z", Fill: true}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := doc.Options{Schemas: []doc.Named{{Name: "forks", Kinds: k}}}
	in := `{"schema":"forks","flow":"top-to-bottom","vertices":[{"id":"a","kind":"fork","x":100,"y":50,"placed":true},{"id":"b","kind":"fork","x":300,"y":50,"w":120,"h":10,"placed":true}],"edges":[]}`
	d, err := doc.Decode([]byte(in), opts)
	if err != nil {
		t.Fatal(err)
	}
	if d.Vertex(0).Size() != (gw.Size{W: 80, H: 8}) || d.Vertex(1).Size() != (gw.Size{W: 120, H: 10}) {
		t.Fatalf("read: %v %v", d.Vertex(0).Size(), d.Vertex(1).Size())
	}
	var out bytes.Buffer
	if err := doc.Encode(d, "forks", &out); err != nil {
		t.Fatal(err)
	}
	if want := in[:len(in)-1] + `,"view":{"x":0,"y":0,"w":0,"h":0}}`; strings.TrimSpace(out.String()) != want {
		t.Errorf("written:\n%s\nwant\n%s", out.String(), want)
	}
	late := `{"schema":"forks","vertices":[{"id":"a","kind":"fork","x":100,"y":50,"placed":true}],"edges":[],"flow":"top-to-bottom"}`
	if d2, err := doc.Decode([]byte(late), opts); err != nil || d2.Vertex(0).Size() != (gw.Size{W: 80, H: 8}) {
		t.Errorf("the flow after the vertices: %v %v", d2.Vertex(0).Size(), err)
	}
	if _, err := d.Apply(gw.Command{Op: gw.Layout, Value: "left-to-right"}); err != nil {
		t.Fatal(err)
	}
	if d.Vertex(0).Size() != (gw.Size{W: 8, H: 80}) {
		t.Errorf("laid out left to right: %v", d.Vertex(0).Size())
	}
	d.Undo()
	var again bytes.Buffer
	if err := doc.Encode(d, "forks", &again); err != nil {
		t.Fatal(err)
	}
	if again.String() != out.String() {
		t.Errorf("after the layout and its undo:\n%s\nwant\n%s", again.String(), out.String())
	}
}
