package svg

import (
	"io"
	"strconv"
)

// bufSize is the renderer's output buffer: the text is written through
// it in pieces, so a render of any size needs no more memory than this.
const bufSize = 64 << 10

// out is a fixed buffer in front of an io.Writer, with the escaping and
// number formatting every element needs. A write past the buffer flushes
// it; the first error stops further writes and is returned at the end.
type out struct {
	buf []byte
	n   int
	w   io.Writer
	err error
}

func (o *out) reset(w io.Writer) { o.w, o.n, o.err = w, 0, nil }

// flush writes the buffer out.
func (o *out) flush() {
	if o.n == 0 || o.err != nil {
		o.n = 0
		return
	}
	_, o.err = o.w.Write(o.buf[:o.n])
	o.n = 0
}

func (o *out) byte(c byte) {
	if o.n == len(o.buf) {
		o.flush()
	}
	o.buf[o.n] = c
	o.n++
}

// str writes a string as it is.
func (o *out) str(s string) {
	for i := 0; i < len(s); i++ {
		o.byte(s[i])
	}
}

// text writes a string escaped for element content.
func (o *out) text(s string) {
	for i := 0; i < len(s); i++ {
		if nonchar(s, i) {
			i += 2
			continue
		}
		o.textByte(s[i])
	}
}

// nonchar reports whether U+FFFE or U+FFFF starts at byte i, which XML
// cannot carry; like the controls, the document refuses them and the
// writer drops any that get past.
func nonchar(s string, i int) bool {
	return s[i] == 0xef && i+2 < len(s) && s[i+1] == 0xbf && s[i+2] >= 0xbe
}

// textByte writes one byte of element content escaped. A control
// character XML cannot carry is dropped; the document refuses them, so
// this guards only a document built around its checks.
func (o *out) textByte(c byte) {
	switch c {
	case '&':
		o.str("&amp;")
	case '<':
		o.str("&lt;")
	case '>':
		o.str("&gt;")
	case '\t', '\n', '\r':
		o.byte(c)
	default:
		if c >= 0x20 {
			o.byte(c)
		}
	}
}

// attr writes a string escaped for a double-quoted attribute value.
func (o *out) attr(s string) {
	for i := 0; i < len(s); i++ {
		if nonchar(s, i) {
			i += 2
			continue
		}
		switch c := s[i]; c {
		case '&':
			o.str("&amp;")
		case '<':
			o.str("&lt;")
		case '>':
			o.str("&gt;")
		case '"':
			o.str("&quot;")
		case '\n', '\r', '\t':
			o.byte(' ')
		default:
			if c >= 0x20 {
				o.byte(c)
			}
		}
	}
}

// name writes a string as a token for a class or an attribute name:
// letters, digits, '-' and '_' pass, anything else becomes '_', so a
// host's kind name cannot break the markup or split into two classes.
func (o *out) name(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			o.byte(c)
		} else {
			o.byte('_')
		}
	}
}

// num writes a number with at most two decimals, trailing zeros cut,
// so that geometry reads short and the same everywhere. It formats into
// the buffer's free end, which holds 32 bytes once flushed, and any
// number longer is written through from where strconv copied it: a scratch
// array of its own would be on the heap under TinyGo, whose strconv
// keeps the slice it is given.
func (o *out) num(x float64) {
	if o.n+32 > len(o.buf) {
		o.flush()
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
func (o *out) put(free, b []byte) {
	if cap(b) == cap(free) {
		o.n += len(b)
		return
	}
	for _, c := range b {
		o.byte(c)
	}
}

// int writes an integer, formatted into the buffer's free end as num.
func (o *out) int(v int) {
	if o.n+24 > len(o.buf) {
		o.flush()
	}
	free := o.buf[o.n:o.n]
	o.put(free, strconv.AppendInt(free, int64(v), 10))
}

// numAttr writes ` name="x"`.
func (o *out) numAttr(name string, x float64) {
	o.byte(' ')
	o.str(name)
	o.str(`="`)
	o.num(x)
	o.byte('"')
}

// point writes "x y".
func (o *out) point(x, y float64) {
	o.num(x)
	o.byte(' ')
	o.num(y)
}
