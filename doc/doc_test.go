package doc_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// schemas: the built-in one and a stricter "model" with ports, a root
// and a leaf, a number attribute and a cycle rule.
func schemas() []doc.Named {
	model := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 96, H: 24}, Root: true},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36},
				Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right, Max: 2}},
				Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number, Default: "0.5"}}},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 100, H: 30}, Leaf: true},
			{Name: "zone", Size: gw.Size{W: 100, H: 60}, Container: true, Contains: []gw.KindIndex{1, 2}},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "to", From: []gw.KindIndex{0}, To: []gw.KindIndex{1}, Arrow: true},
			{Name: "outcome", From: []gw.KindIndex{1}, Route: gw.Curve, Weighted: true, Attrs: []gw.AttrSpec{{Key: "p", Kind: gw.Number}}},
		},
		Acyclic: true,
	}
	return append(doc.Builtin(), doc.Named{Name: "model", Schema: model})
}

const example = `{
  "schema": "model",
  "vertices": [
    {"id": "s", "kind": "source", "label": "storms", "x": 60, "y": 100, "placed": true},
    {"id": "a", "kind": "node", "label": "claim \"big\"", "x": 220, "y": 100, "w": 140, "h": 40, "attrs": {"p": "0.25", "note": "tab\there"}},
    {"id": "d", "kind": "leaf", "x": 420, "y": 100}
  ],
  "edges": [
    {"id": "e1", "kind": "to", "from": "s", "to": "a", "to_port": "in"},
    {"id": "e2", "kind": "outcome", "from": "a", "to": "d", "from_port": "out", "label": "pays", "weight": 0.7, "attrs": {"k": "v"}}
  ],
  "view": {"x": 0, "y": 0, "w": 600, "h": 200}
}`

func decode(t *testing.T, text string) *gw.Document {
	t.Helper()
	d, err := doc.Decode([]byte(text), doc.Options{Schemas: schemas()})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestRoundTrip: decode, encode, decode and encode again give the same
// text, and the decoded document holds what the text said.
func TestRoundTrip(t *testing.T) {
	d := decode(t, example)
	if d.LiveVertices() != 3 || d.LiveEdges() != 2 || d.View.W != 600 {
		t.Fatalf("decoded %d vertices %d edges view %+v", d.LiveVertices(), d.LiveEdges(), d.View)
	}
	a := d.Vertices[d.VertexByID("a")]
	if a.Label != `claim "big"` || a.Size != (gw.Size{W: 140, H: 40}) || a.Placed || a.Pos != (gw.Point{X: 220, Y: 100}) || len(a.Attrs) != 2 || a.Attrs[0].Value != "0.25" || a.Attrs[1].Value != "tab\there" {
		t.Errorf("a: %+v", a)
	}
	if s := d.Vertices[d.VertexByID("s")]; !s.Placed {
		t.Error("s not placed")
	}
	e := d.Edges[d.EdgeByID("e2")]
	if e.FromPort != 1 || e.ToPort != gw.None || e.Weight != 0.7 || e.Label != "pays" || len(e.Attrs) != 1 {
		t.Errorf("e2: %+v", e)
	}
	var first, second bytes.Buffer
	if err := doc.Encode(d, "model", &first); err != nil {
		t.Fatal(err)
	}
	d2 := decode(t, first.String())
	if err := doc.Encode(d2, "model", &second); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("round trip differs:\n%s\n%s", first.String(), second.String())
	}
	for _, want := range []string{`"label":"claim \"big\""`, `"note":"tab\there"`, `"placed":true`, `"w":140,"h":40`, `"from_port":"out"`, `"weight":0.7`, `"view":{"x":0,"y":0,"w":600,"h":200}`} {
		if !strings.Contains(first.String(), want) {
			t.Errorf("encoded text lacks %s:\n%s", want, first.String())
		}
	}
}

// TestContainersRoundTrip: a vertex names its container by id, which
// may come later in the array; the container is fitted around what it
// holds on load, and the parent is written back, but not the size it
// was fitted to; a container's w and h are its own size, which it is
// fitted to at least and which is written back; a document without
// positions is laid out with its containers.
func TestContainersRoundTrip(t *testing.T) {
	text := `{"schema":"model","vertices":[
		{"id":"a","kind":"node","x":220,"y":100,"parent":"z"},
		{"id":"d","kind":"leaf","x":420,"y":100,"parent":"z"},
		{"id":"z","kind":"zone","x":0,"y":0}]}`
	d := decode(t, text)
	z := d.VertexByID("z")
	if d.Vertices[d.VertexByID("a")].Parent != z || d.Vertices[z].Size.W != 470-160+2*gw.ContainerPad || d.CanUndo() {
		t.Errorf("zone %v %v, undo %v", d.Vertices[z].Pos, d.Vertices[z].Size, d.CanUndo())
	}
	var first, second bytes.Buffer
	if err := doc.Encode(d, "model", &first); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.String(), `"parent":"z"`) || !strings.Contains(first.String(), `"kind":"zone","x":315,"y":90}`) {
		t.Errorf("no parent written, or the fitted size:\n%s", first.String())
	}
	if err := doc.Encode(decode(t, first.String()), "model", &second); err != nil || first.String() != second.String() {
		t.Errorf("round trip differs:\n%s\n%s", first.String(), second.String())
	}
	// Its own size, 600 by 200, is wider than what it holds and higher:
	// the zone grows right and down from the fit's corner (144,46).
	d = decode(t, strings.Replace(text, `"x":0,"y":0`, `"x":0,"y":0,"w":600,"h":200`, 1))
	z = d.VertexByID("z")
	if v := d.Vertices[z]; v.Size != (gw.Size{W: 600, H: 200}) || v.Pos != (gw.Point{X: 444, Y: 146}) || v.Min != v.Size {
		t.Errorf("own size: zone %v %v %v", v.Pos, v.Size, v.Min)
	}
	first.Reset()
	if err := doc.Encode(d, "model", &first); err != nil || !strings.Contains(first.String(), `"w":600,"h":200`) {
		t.Errorf("own size not written back: %v\n%s", err, first.String())
	}
	d = decode(t, `{"schema":"model","vertices":[{"id":"a","kind":"node","parent":"z"},{"id":"d","kind":"leaf"},{"id":"z","kind":"zone"}],
		"edges":[{"kind":"outcome","from":"a","to":"d","from_port":"out"}]}`)
	if a, zv := d.Vertices[d.VertexByID("a")], d.Vertices[d.VertexByID("z")]; a.Pos.X-a.Size.W/2 < zv.Pos.X-zv.Size.W/2 || a.Pos.X+a.Size.W/2 > zv.Pos.X+zv.Size.W/2 {
		t.Errorf("laid out on load: a %v outside z %v %v", a.Pos, zv.Pos, zv.Size)
	}
}

// TestLayoutOnLoad: a document without positions is laid out, unless
// the host says not to; one with them is left alone.
func TestLayoutOnLoad(t *testing.T) {
	text := `{"schema":"basic","vertices":[{"id":"a","kind":"box"},{"id":"b","kind":"box"}],"edges":[{"kind":"arrow","from":"a","to":"b"}]}`
	d := decode(t, text)
	if d.Vertices[1].Pos.X <= d.Vertices[0].Pos.X {
		t.Errorf("not laid out: %v %v", d.Vertices[0].Pos, d.Vertices[1].Pos)
	}
	d, err := doc.Decode([]byte(text), doc.Options{Schemas: schemas(), NoLayout: true})
	if err != nil || d.Vertices[1].Pos != (gw.Point{}) {
		t.Errorf("NoLayout: %v %v", err, d.Vertices[1].Pos)
	}
	d = decode(t, example)
	if d.Vertices[2].Pos != (gw.Point{X: 420, Y: 100}) {
		t.Errorf("positioned document moved: %v", d.Vertices[2].Pos)
	}
	if d.CanUndo() {
		t.Error("a decoded document can undo what built it")
	}
}

// TestDecodeRefusals: every refusal names the path and the rule, and
// the engine's own refusals come through with it.
func TestDecodeRefusals(t *testing.T) {
	v := func(body string) string { return `{"schema":"model","vertices":[` + body + `]}` }
	e := func(body string) string {
		return `{"schema":"model","vertices":[{"id":"s","kind":"source"},{"id":"a","kind":"node"},{"id":"b","kind":"node"}],"edges":[` + body + `]}`
	}
	for _, tc := range []struct{ name, text, want string }{
		{"empty", ``, "document: unexpected end of input, expected '{'"},
		{"not an object", `[]`, "expected '{', found '['"},
		{"unknown root key", `{"colour": 1}`, `document: unknown key "colour"`},
		{"schema twice", `{"schema":"basic","schema":"basic"}`, `"schema" given twice`},
		{"vertices twice", `{"schema":"basic","vertices":[],"vertices":[]}`, `"vertices" given twice`},
		{"edges twice", `{"schema":"basic","edges":[],"edges":[]}`, `"edges" given twice`},
		{"view twice", `{"schema":"basic","view":{},"view":{}}`, `"view" given twice`},
		{"view negative width", `{"schema":"basic","view":{"w":-5}}`, `document.view: w must not be negative and at most 1e9`},
		{"view huge height", `{"schema":"basic","view":{"h":1e308}}`, `document.view: h must not be negative and at most 1e9`},
		{"view far corner", `{"schema":"basic","view":{"x":-2e9}}`, `document.view: x must be within 1e9 of the origin`},
		{"view far y", `{"schema":"basic","view":{"y":1e10}}`, `y must be within 1e9`},
		{"vertex far", v(`{"kind":"node","x":1e308}`), `document.vertices[0]: position must be finite and within 1e9 of the origin`},
		{"vertex negative size", v(`{"kind":"node","w":-500,"h":-500}`), `size must be positive and finite, at most 1e9`},
		{"no schema", `{}`, "document has no schema"},
		{"unknown schema", `{"schema":"nope"}`, `document.schema: schema "nope" not supplied`},
		{"vertices before schema", `{"vertices":[]}`, `"schema" must come first`},
		{"edges before schema", `{"edges":[]}`, `"schema" must come first`},
		{"view before schema", `{"view":{}}`, `"schema" must come first`},
		{"text after", `{"schema":"basic"} x`, "text after the document"},
		{"bad separator", `{"schema":"basic" "x":1}`, "expected ',' or '}', found '\"'"},
		{"truncated", `{"schema":"basic"`, "unexpected end of input"},
		{"vertex not object", v(`1`), "document.vertices[0]: expected '{', found '1'"},
		{"vertex unknown key", v(`{"kind":"node","colour":"red"}`), `document.vertices[0].colour: unknown key "colour"`},
		{"vertex kind missing", v(`{"id":"a"}`), `document.vertices[0]: "kind" missing`},
		{"vertex kind unknown", v(`{"kind":"blob"}`), `document.vertices[0].kind: unknown vertex kind "blob"`},
		{"vertex kind not a string", v(`{"kind":3}`), `document.vertices[0].kind: expected '"', found '3'`},
		{"attr keys that would collide", v(`{"kind":"node","attrs":{"a b":"1","a_b":"2"}}`), `attribute key "a b" may hold only lowercase letters, digits, '-' and '_'`},
		{"attr key in capitals", v(`{"kind":"node","attrs":{"Owner":"x"}}`), `attribute key "Owner" may hold only`},
		{"bad number", v(`{"kind":"node","x":"a"}`), "document.vertices[0].x: expected a number"},
		{"nan number", v(`{"kind":"node","x":1e999}`), `bad number "1e999"`},
		{"placed not a bool", v(`{"kind":"node","placed":1}`), "document.vertices[0].placed: expected true or false"},
		{"attrs not object", v(`{"kind":"node","attrs":[]}`), "document.vertices[0].attrs: expected '{', found '['"},
		{"attr not a string", v(`{"kind":"node","attrs":{"p":1}}`), "document.vertices[0].attrs: expected '\"', found '1'"},
		{"attr refused by the schema", v(`{"kind":"node","attrs":{"p":"x"}}`), `document.vertices[0]: attribute "p" of vertex "v1" must be a number: "x"`},
		{"too many attrs", v(`{"kind":"node","attrs":{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","h":"8","i":"9","j":"10","k":"11","l":"12","m":"13","n":"14","o":"15","p":"16","q":"17"}}`), "more than 16 attributes"},
		{"duplicate id", v(`{"id":"a","kind":"node"},{"id":"a","kind":"node"}`), `document.vertices[1]: id "a" already used`},
		{"long string", v(`{"kind":"node","label":"` + strings.Repeat("x", 300) + `"}`), "string of 300 bytes exceeds the limit of 256"},
		{"control char", v(`{"kind":"node","label":"a` + "\x01" + `"}`), "control character in string"},
		{"bad escape", v(`{"kind":"node","label":"\q"}`), "bad escape"},
		{"bad unicode escape", v(`{"kind":"node","label":"\u12"}`), "bad unicode escape"},
		{"bad utf8", v(`{"kind":"node","label":"` + "\xff" + `"}`), "string is not valid UTF-8"},
		{"unterminated", v(`{"kind":"node","label":"abc`), "unterminated string"},
		{"edge not object", e(`"x"`), "document.edges[0]: expected '{', found '\"'"},
		{"edge unknown key", e(`{"kind":"to","from":"s","to":"a","colour":"red"}`), `document.edges[0].colour: unknown key "colour"`},
		{"edge required", e(`{"kind":"to","from":"s"}`), `document.edges[0]: "kind", "from" and "to" are required`},
		{"edge kind unknown", e(`{"kind":"blob","from":"s","to":"a"}`), `unknown edge kind "blob"`},
		{"edge from unknown", e(`{"kind":"to","from":"x","to":"a"}`), `unknown vertex "x"`},
		{"edge to unknown", e(`{"kind":"to","from":"s","to":"x"}`), `unknown vertex "x"`},
		{"edge port unknown", e(`{"kind":"to","from":"s","to":"a","to_port":"side"}`), `vertex "a" has no port "side"`},
		{"edge from port unknown", e(`{"kind":"outcome","from":"a","to":"b","from_port":"side","to_port":"in"}`), `vertex "a" has no port "side"`},
		{"edge refused by the schema", e(`{"kind":"to","from":"s","to":"a"}`), `document.edges[0]: vertex "a" has ports; the edge must name one`},
		{"edge attr refused", e(`{"kind":"outcome","from":"a","to":"b","from_port":"out","to_port":"in","attrs":{"p":"x"}}`), `document.edges[0]: attribute "p" of edge "e1" must be a number: "x"`},
		{"view unknown key", `{"schema":"basic","view":{"z":1}}`, `document.view: unknown key "z"`},
		{"view bad number", `{"schema":"basic","view":{"x":"a"}}`, "document.view: expected a number"},
		{"view bad separator", `{"schema":"basic","view":{"x":1 "y":2}}`, "expected ',' or '}'"},
		{"root key not a string", `{1:2}`, "document: expected '\"', found '1'"},
		{"schema not a string", `{"schema":1}`, "document.schema: expected '\"'"},
		{"vertices not an array", `{"schema":"basic","vertices":{}}`, "document.vertices: expected '[', found '{'"},
		{"vertex separator", v(`{"kind":"node"} {"kind":"node"}`), "document.vertices: expected ',' or ']', found '{'"},
		{"vertex key not a string", v(`{1:2}`), "document.vertices[0]: expected '\"', found '1'"},
		{"vertex object separator", v(`{"kind":"node" "x":1}`), "document.vertices[0]: expected ',' or '}'"},
		{"attr key not a string", v(`{"kind":"node","attrs":{1:"a"}}`), "document.vertices[0].attrs: expected '\"', found '1'"},
		{"attr separator", v(`{"kind":"node","attrs":{"a":"1" "b":"2"}}`), "document.vertices[0].attrs: expected ',' or '}'"},
		{"edges not an array", `{"schema":"model","edges":{}}`, "document.edges: expected '[', found '{'"},
		{"view not an object", `{"schema":"model","view":[]}`, "document.view: expected '{', found '['"},
		{"edge separator", e(`{"kind":"to","from":"s","to":"a","to_port":"in"} {}`), "document.edges: expected ',' or ']', found '{'"},
		{"edge key not a string", e(`{1:2}`), "document.edges[0]: expected '\"', found '1'"},
		{"edge object separator", e(`{"kind":"to" "from":"s"}`), "document.edges[0]: expected ',' or '}'"},
		{"view key not a string", `{"schema":"basic","view":{1:2}}`, "document.view: expected '\"', found '1'"},
		{"bad hex escape", v(`{"kind":"node","label":"\uZZZZ"}`), "bad unicode escape"},
		{"bad utf8 in an escaped string", v(`{"kind":"node","label":"\n` + "\xff" + `"}`), "string is not valid UTF-8"},
		{"placed false", v(`{"kind":"node","placed":false}`), ""},
		{"parent unknown", v(`{"kind":"node","parent":"x"}`), `document.vertices[0].parent: unknown vertex "x"`},
		{"parent not a string", v(`{"kind":"node","parent":1}`), `document.vertices[0].parent: expected '"'`},
		{"parent refused", v(`{"id":"a","kind":"node","parent":"s"},{"id":"s","kind":"source"}`), `document.vertices[0]: vertex "s" of kind "source" is not a container`},
		{"parent kind refused", v(`{"id":"s","kind":"source","parent":"z"},{"id":"z","kind":"zone"}`), `document.vertices[0]: container kind "zone" cannot hold kind "source"`},
		{"bends not an array", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":{}}`), "document.edges[0].bends: expected '[', found '{'"},
		{"bend not a pair", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[1]}`), "document.edges[0].bends: expected '[', found '1'"},
		{"bend x not a number", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[["a",1]]}`), "document.edges[0].bends: expected a number"},
		{"bend without y", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[[1]]}`), "document.edges[0].bends: expected ',', found ']'"},
		{"bend y not a number", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[[1,"a"]]}`), "document.edges[0].bends: expected a number"},
		{"bend of three", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[[1,2,3]]}`), "document.edges[0].bends: expected ']', found ','"},
		{"bends separator", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[[1,2] [3,4]]}`), "document.edges[0].bends: expected ',' or ']', found '['"},
		{"no bends", e(`{"kind":"to","from":"s","to":"a","to_port":"in","bends":[]}`), ""},
		{"flow twice", `{"schema":"basic","flow":"left-to-right","flow":"left-to-right"}`, `"flow" given twice`},
		{"flow before schema", `{"flow":"top-to-bottom"}`, `"schema" must come first`},
		{"unknown flow", `{"schema":"basic","flow":"sideways"}`, `document.flow: unknown flow "sideways"`},
		{"flow not a string", `{"schema":"basic","flow":1}`, `document.flow: expected '"', found '1'`},
		{"too deep", `{"schema":"basic","vertices":[{"kind":"box","attrs":{"a":"[[[[[[[[[[[[[[[[["}}]}`, ""},
	} {
		_, err := doc.Decode([]byte(tc.text), doc.Options{Schemas: schemas()})
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, err := doc.Decode(make([]byte, doc.MaxInput+1), doc.Options{}); err == nil || !strings.Contains(err.Error(), "exceeds the limit") {
		t.Errorf("over the input limit: %v", err)
	}
	if _, err := doc.Decode([]byte(`{"schema":"broken"}`), doc.Options{Schemas: []doc.Named{{Name: "broken"}}}); err == nil || !strings.Contains(err.Error(), "document.schema: schema: no vertex kinds") {
		t.Errorf("broken schema: %v", err)
	}
	// A layout on load can fail on the dummy budget: a placed edge
	// spanning ten layers with room for eight dummies per edge capacity.
	var sb strings.Builder
	sb.WriteString(`{"schema":"basic","vertices":[{"id":"z","kind":"box"}`)
	for i := 0; i < 11; i++ {
		sb.WriteString(`,{"id":"n` + strconv.Itoa(i) + `","kind":"box","x":` + strconv.Itoa(i*160) + `,"y":0,"placed":true}`)
	}
	sb.WriteString(`],"edges":[{"kind":"line","from":"n0","to":"n10"}]}`)
	if _, err := doc.Decode([]byte(sb.String()), doc.Options{Schemas: schemas(), Caps: gw.Caps{Vertices: 16, Edges: 1}}); err == nil || !strings.Contains(err.Error(), "dummy vertices, over the budget") {
		t.Errorf("layout on load: %v", err)
	}
	if _, err := doc.Decode([]byte(`{"schema":"model","vertices":[{"kind":"node"},{"kind":"node"}]}`), doc.Options{Schemas: schemas(), Caps: gw.Caps{Vertices: 1}}); err == nil || !strings.Contains(err.Error(), "vertex limit 1 reached") {
		t.Errorf("capacity: %v", err)
	}
	// Bends past the slab: more than it holds in one edge, then more
	// than is left after another.
	two := `{"schema":"model","vertices":[{"id":"s","kind":"source"},{"id":"a","kind":"node"},{"id":"b","kind":"node"}],"edges":[
		{"kind":"to","from":"s","to":"a","to_port":"in","bends":[[1,2],[3,4]]},
		{"kind":"outcome","from":"a","to":"b","from_port":"out","to_port":"in","bends":[[5,6]]}]}`
	if _, err := doc.Decode([]byte(two), doc.Options{Schemas: schemas(), Caps: gw.Caps{Edges: 1, BendsPerEdge: 1}}); err == nil || !strings.Contains(err.Error(), "document.edges[0].bends: bends exceed the limit of 1") {
		t.Errorf("bends over the limit: %v", err)
	}
	if _, err := doc.Decode([]byte(two), doc.Options{Schemas: schemas(), Caps: gw.Caps{Edges: 2, BendsPerEdge: 1}}); err == nil || !strings.Contains(err.Error(), "document.edges[1]: bends exceed the room left for them") {
		t.Errorf("bends over the room left: %v", err)
	}
}

// TestBendsRoundTrip: an edge's bends read in, are drawn from, and are
// written back as they were.
func TestBendsRoundTrip(t *testing.T) {
	text := `{"schema":"basic","vertices":[{"id":"a","kind":"box","x":0,"y":0},{"id":"b","kind":"box","x":400,"y":0}],
		"edges":[{"id":"e1","kind":"line","from":"a","to":"b","bends":[[100,80], [300.5,80]]}]}`
	d := decode(t, text)
	if b := d.Edges[0].Bends; len(b) != 2 || b[1] != (gw.Point{X: 300.5, Y: 80}) {
		t.Fatalf("bends %v", b)
	}
	var first, second bytes.Buffer
	if err := doc.Encode(d, "basic", &first); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.String(), `"bends":[[100,80],[300.5,80]]`) {
		t.Errorf("bends not written:\n%s", first.String())
	}
	if err := doc.Encode(decode(t, first.String()), "basic", &second); err != nil || first.String() != second.String() {
		t.Errorf("round trip differs:\n%s\n%s", first.String(), second.String())
	}
}

// TestEscapes: every escape JSON allows reads back, and the encoder
// writes what reads back the same; an escape for a control character
// XML cannot carry is refused.
func TestEscapes(t *testing.T) {
	for _, esc := range []string{`\b`, `\f`, `\u0001`, `\u001f`} {
		_, err := doc.Decode([]byte(`{"schema":"basic","vertices":[{"kind":"box","label":"a`+esc+`"}]}`), doc.Options{Schemas: doc.Builtin()})
		if err == nil || !strings.Contains(err.Error(), "control character") {
			t.Errorf("%s: got %v", esc, err)
		}
	}
	d := decode(t, `{"schema":"basic","vertices":[{"kind":"box","label":"a\"b\\c\/d\n\r\t\u00e9\u007f"}]}`)
	want := "a\"b\\c/d\n\r\té\x7f"
	if got := d.Vertices[0].Label; got != want {
		t.Errorf("label %q, want %q", got, want)
	}
	var buf bytes.Buffer
	if err := doc.Encode(d, "basic", &buf); err != nil {
		t.Fatal(err)
	}
	d2 := decode(t, buf.String())
	if d2.Vertices[0].Label != want {
		t.Errorf("after encode: %q", d2.Vertices[0].Label)
	}
	// A control character written into the document past its checks is
	// still written as valid JSON, which the reader then refuses.
	d2.Vertices[0].Label = "a\x01\x1f"
	buf.Reset()
	if err := doc.Encode(d2, "basic", &buf); err != nil || !strings.Contains(buf.String(), `"label":"a\u0001\u001f"`) {
		t.Errorf("control characters written as %v\n%s", err, buf.String())
	}
}

// failing is a writer that fails, to see the error come back.
type failing struct{}

type writeError struct{}

func (*writeError) Error() string { return "write failed" }

func (failing) Write([]byte) (int, error) { return 0, &writeError{} }

func TestEncodeError(t *testing.T) {
	d := decode(t, example)
	if err := doc.Encode(d, "model", failing{}); err == nil || err.Error() != "write failed" {
		t.Errorf("got %v", err)
	}
	// A document past the encoder's buffer meets the error mid-way.
	for i := 0; i < 80; i++ {
		if _, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 1, Text: strings.Repeat("label ", 20)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := doc.Encode(d, "model", failing{}); err == nil || err.Error() != "write failed" {
		t.Errorf("big: got %v", err)
	}
}

// FuzzDecode: the decoder returns an error or a document, never
// panics, and never makes more than the capacities allow; a document it
// returns renders to well-formed XML with no attribute twice on an
// element, whatever its labels, IDs and attributes hold.
func FuzzDecode(f *testing.F) {
	f.Add([]byte(example))
	f.Add([]byte(`{"schema":"basic","vertices":[{"kind":"box"}],"edges":[]}`))
	f.Add([]byte(`{"schema":"model","vertices":[{"id":"a","kind":"node","attrs":{"p":"1"}}],"edges":[{"kind":"outcome","from":"a","to":"a","from_port":"out","to_port":"in"}]}`))
	f.Add([]byte(`{"schema":"basic","vertices":[{"kind":"box","label":"\u00e9\n"}],"view":{"x":-1e3}}`))
	for _, hostile := range []string{
		`<img src=x onerror=alert(1)>`, `</text></svg><script>alert(1)</script>`, `\"><svg onload=alert(1)>`,
		`<![CDATA[x]]>`, `&amp;&#0;&lt;`, `\u0001\u001f\b\f`, `\ud800`, `\u007f\t\r\n`,
	} {
		// One field at a time, so that a refusal of one does not hide
		// another.
		f.Add([]byte(`{"schema":"basic","vertices":[{"id":"` + hostile + `","kind":"box"},{"id":"b","kind":"group"}],"edges":[{"kind":"arrow","from":"b","to":"` + hostile + `"}]}`))
		f.Add([]byte(`{"schema":"basic","vertices":[{"id":"a","kind":"box","label":"` + hostile + `"}]}`))
		f.Add([]byte(`{"schema":"basic","vertices":[{"id":"a","kind":"group","label":"` + hostile + `"}]}`))
		f.Add([]byte(`{"schema":"basic","vertices":[{"id":"a","kind":"box","attrs":{"k":"` + hostile + `"}}]}`))
		f.Add([]byte(`{"schema":"basic","vertices":[{"id":"a","kind":"box"},{"id":"b","kind":"box"}],"edges":[{"kind":"arrow","from":"a","to":"b","label":"` + hostile + `"}]}`))
	}
	f.Add([]byte(`{"schema":"basic","vertices":[{"kind":"box","attrs":{"a b":"1","a_b":"2","A":"3","a":"4"}}]}`))
	r := svg.New()
	var buf bytes.Buffer
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := doc.Decode(data, doc.Options{Schemas: schemas(), Caps: gw.Caps{Vertices: 64, Edges: 64}, NoLayout: true})
		if err != nil {
			return
		}
		if d.LiveVertices() > 64 || d.LiveEdges() > 64 {
			t.Fatalf("over the capacities: %d %d", d.LiveVertices(), d.LiveEdges())
		}
		buf.Reset()
		if err := r.Render(d, svg.Options{Mode: svg.Edit, Title: "fuzz"}, &buf); err != nil {
			t.Fatal(err)
		}
		if err := wellFormed(buf.Bytes()); err != nil {
			t.Fatalf("%v\n%s", err, buf.Bytes())
		}
	})
}

// FuzzLayout: a document decoded and laid out, as the reader does when
// a vertex is unplaced, then laid out again with every vertex free in
// either flow, is refused or placed: never a panic, every live vertex at
// a coordinate, and the drawing well-formed. The seeds place vertices
// far apart, so that the layers and the dummies between them run large;
// run it with GOARCH=386 too, where an int has 32 bits as in TinyGo's
// wasm.
func FuzzLayout(f *testing.F) {
	f.Add([]byte(example))
	f.Add([]byte(`{"schema":"basic","vertices":[{"id":"u","kind":"box"},{"id":"a","kind":"box","x":0,"y":0,"placed":true},{"id":"b","kind":"box","x":1e9,"y":0,"placed":true}],"edges":[{"kind":"arrow","from":"a","to":"b"},{"kind":"arrow","from":"u","to":"b"}]}`))
	f.Add([]byte(`{"schema":"basic","vertices":[{"id":"u","kind":"box"},{"id":"a","kind":"box","x":0,"y":0,"placed":true},{"id":"b","kind":"box","x":2000,"y":0,"placed":true}],"edges":[{"kind":"arrow","from":"a","to":"b"},{"kind":"arrow","from":"u","to":"b"}]}`))
	f.Add([]byte(`{"schema":"basic","vertices":[{"id":"a","kind":"box","x":-1e9,"y":-1e9,"placed":true},{"id":"g","kind":"group"},{"id":"c","kind":"box","parent":"g"}],"edges":[{"kind":"arrow","from":"a","to":"c"},{"kind":"arrow","from":"c","to":"a"}]}`))
	f.Add([]byte(`{"schema":"model","vertices":[{"id":"s","kind":"source"},{"id":"n","kind":"node","x":5e8,"placed":true},{"id":"z","kind":"zone"},{"id":"l","kind":"leaf","parent":"z"}],"edges":[{"kind":"to","from":"s","to":"n","to_port":"in"},{"kind":"outcome","from":"n","to":"l","from_port":"out"}]}`))
	r := svg.New()
	var buf bytes.Buffer
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := doc.Decode(data, doc.Options{Schemas: schemas(), Caps: gw.Caps{Vertices: 64, Edges: 64}})
		if err != nil {
			return
		}
		l := layout.New(d.Caps())
		for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
			d.Flow = flow
			if _, err := l.Run(d, layout.Options{All: true}); err != nil {
				continue
			}
			for i := range d.Vertices {
				if v := &d.Vertices[i]; v.Live && !(math.Abs(v.Pos.X) <= gw.MaxCoord && math.Abs(v.Pos.Y) <= gw.MaxCoord) {
					t.Fatalf("flow %v: vertex %s at %v", flow, v.ID, v.Pos)
				}
			}
			buf.Reset()
			if err := r.Render(d, svg.Options{Mode: svg.Edit, Title: "fuzz"}, &buf); err != nil {
				t.Fatal(err)
			}
			if err := wellFormed(buf.Bytes()); err != nil {
				t.Fatalf("%v\n%s", err, buf.Bytes())
			}
		}
	})
}

// wellFormed parses an SVG back and refuses one that is not well-formed
// XML or that repeats an attribute on an element, which encoding/xml
// lets through and a browser's XML parser does not.
func wellFormed(text []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(text))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for i, a := range el.Attr {
			for _, b := range el.Attr[:i] {
				if a.Name == b.Name {
					return errors.New("attribute " + a.Name.Local + " twice on " + el.Name.Local)
				}
			}
		}
	}
}

// TestExamples: every example document of the demo page decodes against
// the built-in schemas and encodes back to a document that decodes the
// same.
func TestExamples(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "web", "examples", "*.json"))
	if err != nil || len(files) < 4 {
		t.Fatalf("examples: %v %v", files, err)
	}
	for _, f := range files {
		text, err := os.ReadFile(f) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		d, schema, err := doc.DecodeNamed(text, doc.Options{Schemas: doc.Builtin()})
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if !strings.Contains(string(text), `"schema": "`+schema+`"`) {
			t.Errorf("%s: schema named %q", f, schema)
		}
		var out bytes.Buffer
		if err := doc.Encode(d, schema, &out); err != nil {
			t.Fatal(err)
		}
		if _, err := doc.Decode(out.Bytes(), doc.Options{Schemas: doc.Builtin()}); err != nil {
			t.Errorf("%s encoded: %v", f, err)
		}
	}
}

// TestFlowRoundTrip: a document's flow reads in, a document without
// positions is laid out in it, and it is written back; the default flow
// is not written.
func TestFlowRoundTrip(t *testing.T) {
	text := `{"schema":"basic","flow":"top-to-bottom","vertices":[{"id":"a","kind":"box"},{"id":"b","kind":"box"}],"edges":[{"kind":"arrow","from":"a","to":"b"}]}`
	d := decode(t, text)
	a, b := d.Vertices[d.VertexByID("a")].Pos, d.Vertices[d.VertexByID("b")].Pos
	if d.Flow != gw.TopToBottom || b.Y <= a.Y || b.X != a.X {
		t.Errorf("flow %v: a at %v, b at %v, not below it", d.Flow, a, b)
	}
	var first, second bytes.Buffer
	if err := doc.Encode(d, "basic", &first); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.String(), `{"schema":"basic","flow":"top-to-bottom",`) {
		t.Errorf("flow not written:\n%s", first.String())
	}
	if err := doc.Encode(decode(t, first.String()), "basic", &second); err != nil || first.String() != second.String() {
		t.Errorf("round trip differs:\n%s\n%s", first.String(), second.String())
	}
	second.Reset()
	if err := doc.Encode(decode(t, example), "model", &second); err != nil || strings.Contains(second.String(), "flow") {
		t.Errorf("the default flow was written: %v\n%s", err, second.String())
	}
}
