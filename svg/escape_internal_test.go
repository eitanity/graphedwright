package svg

import (
	"bytes"
	"strings"
	"testing"
)

// TestEscapeDrops: text and attribute values drop the control characters
// and the noncharacters XML cannot carry, a tab in a value becomes a
// space, and a name is mapped to a token, whatever a caller hands the
// writer: the document's checks refuse all of these first, and this is
// the writer's own guard behind them.
func TestEscapeDrops(t *testing.T) {
	var b bytes.Buffer
	o := newOut()
	o.reset(&b)
	o.text("a\x01b\x1f\x00c" + strings.Repeat("\x02", 3))
	o.byte('|')
	o.text("\x0bsh￿ort￾")
	o.byte('|')
	o.attr("v\x07\t￿w")
	o.byte('|')
	o.name("weird key!")
	o.flush()
	if got, want := b.String(), "abc|short|v w|weird_key_"; got != want {
		t.Errorf("written %q, want %q", got, want)
	}
}
