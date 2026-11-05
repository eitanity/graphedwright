package doc_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
)

// dist is a distribution with its parameters.
type dist struct {
	Name   string    `json:"name"`
	Params []float64 `json:"params"`
}

// recordCodec keeps a list of distributions per vertex.
type recordCodec struct {
	byID     map[gw.ID][]dist
	decoded  int
	encoded  int
	failWith error // Decode's error, if set
}

func (c *recordCodec) Encode(id gw.ID, w io.Writer) (bool, error) {
	c.encoded++
	r, ok := c.byID[id]
	if !ok {
		return false, nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return false, err
	}
	_, err = w.Write(b)
	return true, err
}

func (c *recordCodec) Decode(id gw.ID, raw []byte) error {
	c.decoded++
	if c.failWith != nil {
		return c.failWith
	}
	var r []dist
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	c.byID[id] = r
	return nil
}

// blobCodec keeps an opaque blob per edge, as base64, and settings of
// the document's own, as an object.
type blobCodec struct {
	blobs    map[gw.ID][]byte
	settings map[string]string
}

func (c *blobCodec) Encode(id gw.ID, w io.Writer) (bool, error) {
	var v any
	switch b, ok := c.blobs[id]; {
	case id == "" && len(c.settings) > 0:
		v = c.settings
	case ok:
		v = base64.StdEncoding.EncodeToString(b)
	default:
		return false, nil
	}
	return true, json.NewEncoder(w).Encode(v) // with a newline after, which is whitespace
}

func (c *blobCodec) Decode(id gw.ID, raw []byte) error {
	if id == "" {
		return json.Unmarshal(raw, &c.settings)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	b, err := base64.StdEncoding.DecodeString(s)
	c.blobs[id] = b
	return err
}

func distsEqual(a, b []dist) bool {
	return slices.EqualFunc(a, b, func(x, y dist) bool { return x.Name == y.Name && slices.Equal(x.Params, y.Params) })
}

func encodeWith(t *testing.T, d *gw.Document, c doc.Codec) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := doc.EncodeWith(d, "model", doc.Options{Codec: c}, &b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestDataRoundTrip: records per vertex, and a blob per edge with the
// document's settings, are written as "data" members and read back the
// same, and the text encodes again to the same bytes.
func TestDataRoundTrip(t *testing.T) {
	d := decode(t, example)
	rec := &recordCodec{byID: map[gw.ID][]dist{
		"a": {{Name: "poisson", Params: []float64{0.2}}, {Name: "lognormal", Params: []float64{10, 2.5}}},
		"d": {},
	}}
	text := encodeWith(t, d, rec)
	if !bytes.Contains(text, []byte(`"data":[{"name":"poisson"`)) || bytes.Count(text, []byte(`"data"`)) != 2 {
		t.Fatalf("records encoded as\n%s", text)
	}
	if rec.encoded != d.LiveVertices()+d.LiveEdges()+1 {
		t.Fatalf("Encode called %d times, want once per element and once for the document", rec.encoded)
	}
	back := &recordCodec{byID: map[gw.ID][]dist{}}
	d2, err := doc.Decode(text, doc.Options{Schemas: schemas(), Codec: back})
	if err != nil {
		t.Fatal(err)
	}
	if !maps.EqualFunc(back.byID, rec.byID, distsEqual) || back.decoded != 2 {
		t.Fatalf("decoded %v in %d calls, want %v", back.byID, back.decoded, rec.byID)
	}
	if again := encodeWith(t, d2, back); !bytes.Equal(again, text) {
		t.Fatalf("encoded again differently:\n%s\n%s", again, text)
	}

	blob := &blobCodec{blobs: map[gw.ID][]byte{"e1": {0, 1, 2, 0xff}, "e2": []byte("<b>&\"")}, settings: map[string]string{"seed": "42", "runs": "1000"}}
	text = encodeWith(t, d, blob)
	if !bytes.HasSuffix(text, []byte(`,"data":{"runs":"1000","seed":"42"}}`+"\n")) {
		t.Fatalf("the document's data encoded as\n%s", text)
	}
	bback := &blobCodec{blobs: map[gw.ID][]byte{}}
	if _, err := doc.Decode(text, doc.Options{Schemas: schemas(), Codec: bback}); err != nil {
		t.Fatal(err)
	}
	if !maps.EqualFunc(bback.blobs, blob.blobs, bytes.Equal) || !maps.Equal(bback.settings, blob.settings) {
		t.Fatalf("decoded blobs %v settings %v", bback.blobs, bback.settings)
	}

	// Without a codec, Encode writes the same text as before, and
	// EncodeWith without one writes no data.
	var plain, none bytes.Buffer
	if err := doc.Encode(d, "model", &plain); err != nil {
		t.Fatal(err)
	}
	if err := doc.EncodeWith(d, "model", doc.Options{}, &none); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain.Bytes(), []byte(`"data"`)) || !bytes.Equal(plain.Bytes(), none.Bytes()) {
		t.Fatalf("without a codec:\n%s\n%s", plain.Bytes(), none.Bytes())
	}
}

// rawCodec writes what it is given, for the checks on what a codec
// writes.
type rawCodec struct {
	out     string
	ignore  bool // ignore the writer's error and report success
	d       *gw.Document
	reenter error // what a call back into d gave
	locked  bool  // whether d was locked during the call
}

func (c *rawCodec) Encode(id gw.ID, w io.Writer) (bool, error) {
	if c.d != nil && id == "a" {
		_, c.reenter = c.d.Apply(gw.Command{Op: gw.Move, Vertex: c.d.VertexByID("a"), Pos: gw.Point{X: 1, Y: 1}})
		c.locked = c.d.Locked()
		c.d.Unlock()
	}
	if id != "a" {
		return false, nil
	}
	_, err := io.WriteString(w, c.out)
	if c.ignore {
		err = nil
	}
	return true, err
}

func (c *rawCodec) Decode(gw.ID, []byte) error { return nil }

// TestEncodeDataRefused: what a codec writes is refused, failing the
// encode and naming the element but never the data, when it is past
// DataBytes, not well-formed, or more than one value.
func TestEncodeDataRefused(t *testing.T) {
	d := decode(t, example)
	for _, c := range []struct {
		out    string
		ignore bool
		want   string
	}{
		{strings.Repeat("7", 101), false, `vertex "a": data exceeds the limit of 100 bytes`},
		{`"` + strings.Repeat("x", 99) + `"`, true, `vertex "a": data exceeds the limit of 100 bytes`},
		{`{"secret":`, false, `vertex "a": data is not well-formed JSON: unexpected end at byte 10`},
		{`[1,2]]`, false, `vertex "a": data is more than one JSON value`},
		{`1 2`, false, `vertex "a": data is more than one JSON value`},
		{``, false, `vertex "a": data is not well-formed JSON: unexpected end at byte 0`},
		{`"secret` + "\x01" + `"`, false, `vertex "a": data is not well-formed JSON: control character in string at byte 7`},
	} {
		var b bytes.Buffer
		err := doc.EncodeWith(d, "model", doc.Options{Codec: &rawCodec{out: c.out, ignore: c.ignore}, DataBytes: 100}, &b)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: got %v, want %q", c.out, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("%q: the error holds the data: %v", c.out, err)
		}
	}
	// Exactly at the limit is accepted.
	text := encodeWith2(t, d, &rawCodec{out: strings.Repeat("7", 100)}, 100)
	if !bytes.Contains(text, []byte(`"data":`+strings.Repeat("7", 100)+`}`)) {
		t.Fatalf("data at the limit encoded as\n%s", text)
	}
	// A limit past MaxInput is MaxInput: no more than a decode reads.
	var big bytes.Buffer
	err := doc.EncodeWith(d, "model", doc.Options{Codec: &rawCodec{out: strings.Repeat("7", doc.MaxInput+1)}, DataBytes: doc.MaxInput + 10}, &big)
	if want := `vertex "a": data exceeds the limit of ` + strconv.Itoa(doc.MaxInput) + " bytes"; err == nil || err.Error() != want {
		t.Fatalf("past MaxInput: %v", err)
	}

	// A codec's own error names the element.
	var b bytes.Buffer
	err = doc.EncodeWith(d, "model", doc.Options{Codec: failCodec{}}, &b)
	if err == nil || err.Error() != `vertex "s": data: the store is closed` {
		t.Fatalf("a codec's error: %v", err)
	}
}

func encodeWith2(t *testing.T, d *gw.Document, c doc.Codec, limit int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := doc.EncodeWith(d, "model", doc.Options{Codec: c, DataBytes: limit}, &b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type failCodec struct{}

func (failCodec) Encode(gw.ID, io.Writer) (bool, error) {
	return false, errors.New("the store is closed")
}
func (failCodec) Decode(gw.ID, []byte) error { return errors.New("the store is closed") }

// TestEncodeDataHeld: a codec's call back into the document while it
// encodes is refused, with the document locked for the call and
// unchanged; a document locked before the encode stays locked.
func TestEncodeDataHeld(t *testing.T) {
	d := decode(t, example)
	before := encodeWith(t, d, &rawCodec{out: "1"})
	c := &rawCodec{out: "1", d: d}
	if got := encodeWith(t, d, c); !bytes.Equal(got, before) {
		t.Fatalf("a codec that called back changed the text:\n%s\n%s", got, before)
	}
	if !errors.Is(c.reenter, gw.ErrReentered) || !c.locked {
		t.Fatalf("the call back gave %v, locked %v", c.reenter, c.locked)
	}
	if d.Locked() {
		t.Fatal("the document stayed locked after the encode")
	}
	if p := d.Vertex(d.VertexByID("a")).Pos(); p != (gw.Point{X: 220, Y: 100}) {
		t.Fatalf("the call back moved a to %v", p)
	}
	d.Lock()
	if got := encodeWith(t, d, &rawCodec{out: "1"}); !bytes.Equal(got, before) || !d.Locked() {
		t.Fatalf("a locked document encoded as\n%s\nlocked %v", got, d.Locked())
	}
}

// withData is the example with data on vertex a, edge e2 and the
// document.
func withData(a, e2, top string) string {
	text := strings.Replace(example, `"attrs": {"p": "0.25", "note": "tab\there"}}`, `"attrs": {"p": "0.25", "note": "tab\there"}, "data": `+a+`}`, 1)
	text = strings.Replace(text, `"attrs": {"k": "v"}}`, `"attrs": {"k": "v"}, "data": `+e2+`}`, 1)
	return strings.Replace(text, `"view":`, `"data": `+top+`, "view":`, 1)
}

// TestDecodeDataRefused: a "data" member is refused before the codec
// sees it when there is no codec, when it is past DataBytes, nested
// past MaxDataDepth, or not well-formed, and when it is given twice;
// the message names where, never the content. The codec is not called
// for a document refused after its data was read.
func TestDecodeDataRefused(t *testing.T) {
	deep := strings.Repeat("[", doc.MaxDataDepth+1) + strings.Repeat("]", doc.MaxDataDepth+1)
	ok := strings.Repeat("[", doc.MaxDataDepth) + strings.Repeat("]", doc.MaxDataDepth)
	for _, c := range []struct {
		text  string
		codec bool
		want  string
	}{
		{withData("1", "2", "3"), false, "document.vertices[1].data: host data without a codec to read it"},
		{withData(`"secret`+strings.Repeat("x", 100)+`"`, "2", "3"), true, "document.vertices[1].data: data of 108 bytes exceeds the limit of 100"},
		{withData(`"secret`+strings.Repeat("x", 93)+`"`, "2", "3"), true, "document.vertices[1].data: data of 101 bytes exceeds the limit of 100"},
		{withData(deep, "2", "3"), true, "document.vertices[1].data: data is not well-formed JSON: nested deeper than 64"},
		{withData("1", `{"secret": tru}`, "3"), true, "document.edges[1].data: data is not well-formed JSON: expected a value"},
		{withData("1", "2", `["secret"`), true, "document.data: data is not well-formed JSON: expected ',' or the closing bracket"},
		{strings.Replace(withData("1", "2", "3"), `"data": 1}`, `"data": 1, "data": 1}`, 1), true, `document.vertices[1].data: "data" given twice`},
		{strings.Replace(withData("1", "2", "3"), `"data": 2}`, `"data": 2, "data": 2}`, 1), true, `document.edges[1].data: "data" given twice`},
		{strings.Replace(withData("1", "2", "3"), `"data": 3,`, `"data": 3, "data": 3,`, 1), true, `document: "data" given twice`},
	} {
		rec := &recordCodec{byID: map[gw.ID][]dist{}}
		opts := doc.Options{Schemas: schemas(), DataBytes: 100}
		if c.codec {
			opts.Codec = rec
		}
		_, err := doc.Decode([]byte(c.text), opts)
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("got %v, want %q", err, c.want)
			continue
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("the error holds the data: %v", err)
		}
		if rec.decoded != 0 {
			t.Errorf("%q: Decode called %d times", c.want, rec.decoded)
		}
	}
	// At the nesting and byte limits, the member is handed over.
	rec := &recordCodec{byID: map[gw.ID][]dist{}, failWith: errors.New("not a list of distributions")}
	if _, err := doc.Decode([]byte(withData(ok, "2", "3")), doc.Options{Schemas: schemas(), Codec: rec, DataBytes: len(ok)}); err == nil ||
		err.Error() != "document.vertices[1].data: not a list of distributions" || rec.decoded != 1 {
		t.Fatalf("at the limits: %v, %d calls", err, rec.decoded)
	}
	// Refused after the data was read: an unknown vertex in an edge.
	rec = &recordCodec{byID: map[gw.ID][]dist{}}
	bad := strings.Replace(withData("[]", "[]", "[]"), `"to": "d"`, `"to": "nowhere"`, 1)
	if _, err := doc.Decode([]byte(bad), doc.Options{Schemas: schemas(), Codec: rec}); err == nil || rec.decoded != 0 {
		t.Fatalf("a refused document: %v, Decode called %d times", err, rec.decoded)
	}
	// Each member once, with the element's ID, the document's "".
	ids := &idCodec{}
	if _, err := doc.Decode([]byte(withData("1", "2", "3")), doc.Options{Schemas: schemas(), Codec: ids}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a=1", "e2=2", "=3"}; !slices.Equal(ids.got, want) {
		t.Fatalf("handed over %v, want %v", ids.got, want)
	}
}

type idCodec struct{ got []string }

func (*idCodec) Encode(gw.ID, io.Writer) (bool, error) { return false, nil }
func (c *idCodec) Decode(id gw.ID, raw []byte) error {
	c.got = append(c.got, string(id)+"="+string(raw))
	return nil
}
