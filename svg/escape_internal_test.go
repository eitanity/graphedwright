package svg

import (
	"bytes"
	"io"
	"strconv"
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

// TestNumbers: numbers are written short, with at most two decimals and
// no trailing zeros, negative zero as 0; one longer than the room kept
// for it, and those written as the buffer fills, read whole; and writing
// them makes nothing, formatted in the buffer itself.
func TestNumbers(t *testing.T) {
	var b bytes.Buffer
	o := newOut()
	o.reset(&b)
	for _, x := range []float64{12.5, 12, 12.345, -0.001, -3.1, 1e300} {
		o.num(x)
		o.byte(' ')
	}
	o.int(-42)
	o.flush()
	long := strconv.FormatFloat(1e300, 'f', 0, 64)
	want := "12.5 12 12.35 0 -3.1 " + long + " -42"
	if got := b.String(); got != want {
		t.Errorf("written %q, want %q", got, want)
	}
	b.Reset()
	var w strings.Builder
	for i := range bufSize/6 + 10 {
		o.num(float64(i) + 0.25)
		o.byte(',')
		o.int(i)
		o.byte(';')
		w.WriteString(strconv.FormatFloat(float64(i)+0.25, 'f', 2, 64) + "," + strconv.Itoa(i) + ";")
	}
	o.flush()
	if b.String() != w.String() {
		t.Error("numbers written as the buffer fills differ")
	}
	// A long number where the buffer has room only for the 32 bytes kept.
	b.Reset()
	o.str(strings.Repeat("x", bufSize-40))
	o.num(1e300)
	o.flush()
	if got := b.String(); got != strings.Repeat("x", bufSize-40)+long {
		t.Errorf("a long number at the buffer's end written as %q", got[bufSize-40:])
	}
	o.reset(io.Discard)
	if n := testing.AllocsPerRun(100, func() { o.num(1234.56); o.int(-7); o.num(1e300) }); n > 1 {
		t.Errorf("writing numbers made %v allocations, want at most strconv's one for the long number", n)
	}
	if n := testing.AllocsPerRun(100, func() { o.num(1234.56); o.int(-7) }); n != 0 {
		t.Errorf("writing numbers made %v allocations", n)
	}
}
