package doc

import (
	"bytes"
	"testing"
)

// TestQuoteControls: a control character in a string is written as an
// escape, so that the text stays valid JSON whatever a caller hands the
// encoder: the document's checks refuse it first, and the reader refuses
// it back.
func TestQuoteControls(t *testing.T) {
	var b bytes.Buffer
	e := &encoder{w: &b, buf: make([]byte, 0, 64)}
	e.quote("a\x01\x1f\"\\")
	if err := e.flush(); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), `"a\u0001\u001f\"\\"`; got != want {
		t.Errorf("quoted %s, want %s", got, want)
	}
}
