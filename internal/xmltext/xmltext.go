// Package xmltext writes XML text through a fixed buffer: element
// content and attribute values escaped, names mapped to tokens, and
// numbers in a short form that is the same on every target. The graph
// renderer (svg) and the charts (chart) both write with it, so that
// neither imports the other.
package xmltext

import (
	"io"
	"strconv"
)

// BufSize is the writer's buffer: the text is written through
// it in pieces, so a render of any size needs no more memory than this.
const BufSize = 64 << 10

// Writer is a fixed buffer in front of an io.Writer, with the escaping and
// number formatting every element needs. A write past the buffer flushes
// it; the first error stops further writes and is returned at the end.
type Writer struct {
	buf []byte
	n   int
	w   io.Writer
	err error
}

func (o *Writer) Reset(w io.Writer) { o.w, o.n, o.err = w, 0, nil }

// Flush writes the buffer out.
func (o *Writer) Flush() {
	if o.n == 0 || o.err != nil {
		o.n = 0
		return
	}
	_, o.err = o.w.Write(o.buf[:o.n])
	o.n = 0
}

func (o *Writer) Byte(c byte) {
	if o.n == len(o.buf) {
		o.Flush()
	}
	o.buf[o.n] = c
	o.n++
}

// Str writes a string as it is.
func (o *Writer) Str(s string) {
	for i := 0; i < len(s); i++ {
		o.Byte(s[i])
	}
}

// Bytes writes bytes as they are: text known to need no escaping, such
// as a number formatted.
func (o *Writer) Bytes(b []byte) {
	for _, c := range b {
		o.Byte(c)
	}
}

// Text writes a string escaped for element content.
func (o *Writer) Text(s string) {
	for i := 0; i < len(s); i++ {
		if Nonchar(s, i) {
			i += 2
			continue
		}
		o.TextByte(s[i])
	}
}

// Nonchar reports whether U+FFFE or U+FFFF starts at byte i, which XML
// cannot carry; like the controls, the document refuses them and the
// writer drops any that get past.
func Nonchar(s string, i int) bool {
	return s[i] == 0xef && i+2 < len(s) && s[i+1] == 0xbf && s[i+2] >= 0xbe
}

// TextByte writes one byte of element content escaped. A control
// character XML cannot carry is dropped; the document refuses them, so
// this guards only a document built around its checks.
func (o *Writer) TextByte(c byte) {
	switch c {
	case '&':
		o.Str("&amp;")
	case '<':
		o.Str("&lt;")
	case '>':
		o.Str("&gt;")
	case '\t', '\n', '\r':
		o.Byte(c)
	default:
		if c >= 0x20 {
			o.Byte(c)
		}
	}
}

// Attr writes a string escaped for a double-quoted attribute value.
func (o *Writer) Attr(s string) {
	for i := 0; i < len(s); i++ {
		if Nonchar(s, i) {
			i += 2
			continue
		}
		switch c := s[i]; c {
		case '&':
			o.Str("&amp;")
		case '<':
			o.Str("&lt;")
		case '>':
			o.Str("&gt;")
		case '"':
			o.Str("&quot;")
		case '\n', '\r', '\t':
			o.Byte(' ')
		default:
			if c >= 0x20 {
				o.Byte(c)
			}
		}
	}
}

// Name writes a string as a token for a class or an attribute name:
// letters, digits, '-' and '_' pass, anything else becomes '_', so a
// host's kind name cannot break the markup or split into two classes.
func (o *Writer) Name(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			o.Byte(c)
		} else {
			o.Byte('_')
		}
	}
}

// Num writes a number with at most two decimals, trailing zeros cut,
// so that geometry reads short and the same everywhere. It formats into
// the buffer's free end, which holds 32 bytes once flushed, and any
// number longer is written through from where strconv copied it: a scratch
// array of its own would be on the heap under TinyGo, whose strconv
// keeps the slice it is given.
func (o *Writer) Num(x float64) {
	if o.n+32 > len(o.buf) {
		o.Flush()
	}
	free := o.buf[o.n:o.n]
	b := strconv.AppendFloat(free, x, 'f', 2, 64)
	// Cut "0" and then "." from the end: 12.50 → 12.5, 12.00 → 12.
	if len(b) > 0 && b[len(b)-1] == '0' {
		b = b[:len(b)-1]
	}
	if len(b) > 0 && b[len(b)-1] == '0' {
		b = b[:len(b)-1]
	}
	if len(b) > 0 && b[len(b)-1] == '.' {
		b = b[:len(b)-1]
	}
	if len(b) == 2 && b[0] == '-' && b[1] == '0' {
		b[0] = '0'
		b = b[:1]
	}
	o.put(free, b)
}

// put ends a number formatted by appending to free, the buffer's free
// end: it is there already, unless it was too long and was copied out.
func (o *Writer) put(free, b []byte) {
	if cap(b) == cap(free) {
		o.n += len(b)
		return
	}
	for _, c := range b {
		o.Byte(c)
	}
}

// Int writes an integer, formatted into the buffer's free end as Num.
func (o *Writer) Int(v int) {
	if o.n+24 > len(o.buf) {
		o.Flush()
	}
	free := o.buf[o.n:o.n]
	o.put(free, strconv.AppendInt(free, int64(v), 10))
}

// NumAttr writes ` name="x"`.
func (o *Writer) NumAttr(name string, x float64) {
	o.Byte(' ')
	o.Str(name)
	o.Str(`="`)
	o.Num(x)
	o.Byte('"')
}

// Point writes "x y".
func (o *Writer) Point(x, y float64) {
	o.Num(x)
	o.Byte(' ')
	o.Num(y)
}

// Err is the first error a write met since Reset.
func (o *Writer) Err() error { return o.err }
