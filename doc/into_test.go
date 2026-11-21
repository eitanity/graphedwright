package doc_test

import (
	"errors"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
)

// small are the capacities the refusals are measured against.
var small = gw.Caps{Vertices: 4, Edges: 3, PortsPerVertex: 2, AttrsPerElement: 2, LabelBytes: 8, BendsPerEdge: 2}

// boxes is a basic document of n boxes, each with the extra members
// given, and the edges given.
func boxes(n int, extra, edges string) string {
	var b strings.Builder
	b.WriteString(`{"schema":"basic","vertices":[`)
	for i := range n {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"v` + strconv.Itoa(i) + `","kind":"box","x":` + strconv.Itoa(100*i) + `,"y":0` + extra + `}`)
	}
	b.WriteString(`],"edges":[` + edges + `]}`)
	return b.String()
}

// TestDecodeIntoOverCapacity: a document past any capacity of the held
// document is refused before it is touched, which is left as it was;
// one at every capacity is taken.
func TestDecodeIntoOverCapacity(t *testing.T) {
	opts := doc.Options{Schemas: doc.Builtin()}
	edge := `{"id":"e","kind":"arrow","from":"v0","to":"v1"}`
	full := boxes(4, `,"attrs":{"a":"12345678","b":"x"},"ports":[{"name":"p","side":"left"},{"name":"q","side":"right"}]`,
		edge+`,{"kind":"arrow","from":"v1","to":"v2","bends":[[1,2],[3,4]]},{"kind":"arrow","from":"v2","to":"v3","label":"\n\n\n\n\n\n\n\n"}`)
	for _, c := range []struct{ name, text, want string }{
		{"vertices", boxes(5, "", ""), "more vertices than the capacity of 4"},
		{"edges", boxes(4, "", strings.Repeat(`{"kind":"arrow","from":"v0","to":"v1"},`, 3)+edge), "more edges than the capacity of 3"},
		{"attributes", boxes(1, `,"attrs":{"a":"1","b":"2","c":"3"}`, ""), "more attributes than the capacity of 2"},
		{"ports", boxes(1, `,"ports":[{"name":"a","side":"left"},{"name":"b","side":"left"},{"name":"c","side":"left"}]`, ""), "more ports than the capacity of 2"},
		{"bends", boxes(2, "", `{"kind":"arrow","from":"v0","to":"v1","bends":[[1,1],[2,2],[3,3],[4,4],[5,5],[6,6],[7,7]]}`), "more bend points than the capacity of 6"},
		{"label", boxes(1, `,"label":"123456789"`, ""), "a string of 9 bytes exceeds the limit of 8"},
		{"escaped label", boxes(1, `,"label":"ééééé"`, ""), "a string of 10 bytes exceeds the limit of 8"},
		{"attribute value", boxes(1, `,"attrs":{"a":"123456789"}`, ""), "a string of 9 bytes exceeds the limit of 8"},
		{"key", `{"schema":"basic","vertices":[{"kind":"box","attrs":{"123456789":"x"}}]}`, "a string of 9 bytes exceeds the limit of 8 at byte 53"},
	} {
		held, err := doc.Decode([]byte(full), doc.Options{Schemas: doc.Builtin(), Caps: small})
		if err != nil {
			t.Fatalf("at every capacity: %v", err)
		}
		before := encoded(t, held, "basic")
		_, err = doc.DecodeInto(held, []byte(c.text), opts)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if got := encoded(t, held, "basic"); got != before {
			t.Errorf("%s: the held document changed:\n%s", c.name, got)
		}
		// The same document fits a document of room enough.
		if _, err := doc.Decode([]byte(c.text), opts); err != nil {
			t.Errorf("%s: refused at the default capacities too: %v", c.name, err)
		}
	}
	held, err := doc.Decode([]byte(boxes(1, "", "")), doc.Options{Schemas: doc.Builtin(), Caps: small})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.DecodeInto(held, []byte(full), opts); err != nil || held.LiveVertices() != 4 || held.LiveEdges() != 3 {
		t.Fatalf("a document at every capacity: %v", err)
	}
}

// TestDecodeIntoRefusals: a locked document and a missing one are
// refused, the locked one left as it was; a document refused for
// anything but its size leaves the held document empty, of its kinds
// and capacities, and ready to read into again.
func TestDecodeIntoRefusals(t *testing.T) {
	opts := doc.Options{Schemas: doc.Builtin()}
	held, err := doc.Decode([]byte(boxes(3, "", "")), doc.Options{Schemas: opts.Schemas, Caps: small})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.DecodeInto(nil, []byte(boxes(1, "", "")), opts); err == nil {
		t.Error("no document taken")
	}
	held.Lock()
	if _, err := doc.DecodeInto(held, []byte(boxes(1, "", "")), opts); !errors.Is(err, gw.ErrLocked) || held.LiveVertices() != 3 {
		t.Errorf("locked: %v, %d vertices", err, held.LiveVertices())
	}
	held.Unlock()
	kinds, caps := held.Kinds(), held.Caps()
	if _, err := doc.DecodeInto(held, []byte(boxes(2, `,"kind":"nope"`, "")), opts); err == nil || held.LiveVertices() != 0 || held.Kinds() != kinds || held.Caps() != caps {
		t.Fatalf("a bad document: %v, %d vertices", err, held.LiveVertices())
	}
	if _, err := doc.DecodeInto(held, []byte(boxes(2, "", "")), opts); err != nil || held.LiveVertices() != 2 {
		t.Fatalf("read again: %v", err)
	}
}

// TestDecodeIntoAllocations: reading a document into one held at its
// capacities allocates the same at small capacities as at the default,
// and nothing for them: what it allocates is the document's own, its
// texts and the decoder's working, not the room.
func TestDecodeIntoAllocations(t *testing.T) {
	text := []byte(boxes(4, `,"label":"a box","attrs":{"k":"v"}`, `{"kind":"arrow","from":"v0","to":"v1"},{"kind":"arrow","from":"v1","to":"v2","bends":[[1,2]]}`))
	opts := doc.Options{Schemas: doc.Builtin()}
	measure := func(caps gw.Caps) (allocs, bytes uint64) {
		d, err := doc.Decode(text, doc.Options{Schemas: doc.Builtin(), Caps: caps})
		if err != nil {
			t.Fatal(err)
		}
		// The least of five, as the counts are the process's, which
		// the runtime's own work may add to.
		allocs, bytes = math.MaxUint64, math.MaxUint64
		for range 5 {
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)
			for range 10 {
				if _, err := doc.DecodeInto(d, text, opts); err != nil {
					t.Fatal(err)
				}
			}
			runtime.ReadMemStats(&m1)
			allocs, bytes = min(allocs, (m1.Mallocs-m0.Mallocs)/10), min(bytes, (m1.TotalAlloc-m0.TotalAlloc)/10)
		}
		return allocs, bytes
	}
	sa, sb := measure(gw.Caps{Vertices: 64, Edges: 256})
	da, db := measure(gw.Caps{})
	t.Logf("64 vertices: %d allocations, %d bytes; default: %d allocations, %d bytes", sa, sb, da, db)
	if sa != da || sb != db || db > 8<<10 {
		t.Errorf("DecodeInto allocates %d times, %d bytes, at 64 vertices and %d times, %d bytes, at the default capacities", sa, sb, da, db)
	}
}

// TestDecodeIntoMalformed: text that is not a well-formed document, or
// is too long, is refused by DecodeInto as Decode refuses it, the check
// of the capacities leaving it to the decoder; host data, nested and
// holding text of any length, is the host's and passes the check.
func TestDecodeIntoMalformed(t *testing.T) {
	opts := doc.Options{Schemas: doc.Builtin(), Codec: &rawCodec{}}
	long := strings.Repeat("y", 300)
	for _, text := range []string{
		`{"schema":"basic","vertices":[`,
		`{"schema":"basic","vertices":[{"kind":"box","label":"a` + "\x01" + `"}]}`,
		`{"schema":"basic", 5}`,
		`{"schema":"basic","a` + "\x01" + `":1}`,
		`{"schema" "basic"}`,
		`{"schema":"basic" "x"}`,
		`{"schema":"basic","vertices":[{"kind":"box","attrs":{}}],"unknown":{}}`,
		string(make([]byte, doc.MaxInput+1)),
	} {
		held, err := doc.Decode([]byte(boxes(1, "", "")), doc.Options{Schemas: opts.Schemas})
		if err != nil {
			t.Fatal(err)
		}
		_, want := doc.Decode([]byte(text), opts)
		if _, err := doc.DecodeInto(held, []byte(text), opts); want == nil || err == nil || err.Error() != want.Error() {
			t.Errorf("%.60q: DecodeInto %v, Decode %v", text, err, want)
		}
	}
	data := `{"schema":"basic","data":{"k":["` + long + `",{"n":"` + long + `"}]},"vertices":[{"kind":"box","data":{"v":"` + long + `"}}]}`
	held, err := doc.Decode([]byte(boxes(1, "", "")), doc.Options{Schemas: opts.Schemas})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.DecodeInto(held, []byte(data), opts); err != nil || held.LiveVertices() != 1 {
		t.Fatalf("host data: %v", err)
	}
	// A schema of more vertex kinds than the held document was made with
	// is refused, and the document left as it was.
	more := gw.Schema{VertexKinds: []gw.VertexKind{}}
	for i := range 9 {
		more.VertexKinds = append(more.VertexKinds, gw.VertexKind{Name: "k" + strconv.Itoa(i), Shape: gw.Rect, Size: gw.Size{W: 10, H: 10}})
	}
	k, err := gw.Compile(more)
	if err != nil {
		t.Fatal(err)
	}
	before := encoded(t, held, "basic")
	if _, err := doc.DecodeInto(held, []byte(`{"schema":"more"}`), doc.Options{Schemas: []doc.Named{{Name: "more", Kinds: k}}}); err == nil || encoded(t, held, "basic") != before {
		t.Fatalf("more kinds: %v", err)
	}
}
