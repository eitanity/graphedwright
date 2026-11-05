package doc

import (
	"errors"
	"strconv"
	"unicode/utf8"

	gw "code.eitanity.com/graphedwright"
)

// Codec writes and reads a host's own data on elements and on the
// document, keyed by ID, the document's being "". Each element's data
// is its "data" member, and the document's a top-level "data": one JSON
// value, which doc checks is well-formed and bounded before writing it
// or handing it over, and otherwise never reads.
//
// Encode is called at most once per element and once for the document
// on an encode, through a writer that refuses more than DataBytes; it
// runs with the document held (gw.Document.EncodeData), so a call back
// into it is refused. Decode is called at most once per member, after
// the whole document has been read and built, so never for a document
// that is then refused; raw is the member's text in the input, to be
// copied if kept. Neither may panic: TinyGo on wasm has no recover.
type Codec interface {
	gw.DataEncoder
	Decode(id gw.ID, raw []byte) error
}

// DefaultDataBytes is the longest "data" member, in bytes, when the
// options give none.
const DefaultDataBytes = 64 << 10

// MaxDataDepth is the deepest a "data" member's arrays and objects may
// nest.
const MaxDataDepth = 64

// dataBytes is the limit the options set, or the default.
func (o Options) dataBytes() int {
	if o.DataBytes <= 0 {
		return DefaultDataBytes
	}
	return min(o.DataBytes, MaxInput)
}

// span is a "data" member read and not yet handed over: the element it
// is on, the document's when both are None, its text, and where it was.
type span struct {
	v          gw.VertexIndex
	e          gw.EdgeIndex
	start, end int
	path       string
}

// data reads a "data" member at the scanner, checking it before any of
// it is kept, and returns its span with the element still to be named.
func (dec *decoder) data() (span, error) {
	s := dec.s
	if dec.opts.Codec == nil {
		return span{}, s.errorf("host data without a codec to read it")
	}
	s.ws()
	n, msg, at := jsonValue(s.data[s.pos:])
	if msg != "" {
		s.pos += at
		return span{}, s.errorf("data is not well-formed JSON: " + msg)
	}
	if limit := dec.opts.dataBytes(); n > limit {
		return span{}, s.errorf("data of " + strconv.Itoa(n) + " bytes exceeds the limit of " + strconv.Itoa(limit))
	}
	sp := span{v: gw.None, e: gw.None, start: s.pos, end: s.pos + n, path: string(s.path)}
	s.pos += n
	return sp, nil
}

// keep notes a span read on the element, for handOver.
func (dec *decoder) keep(sp span, v gw.VertexIndex, e gw.EdgeIndex) {
	sp.v, sp.e = v, e
	dec.spans = append(dec.spans, sp)
}

// handOver gives the codec every member read, in the order read, once
// the document is built.
func (dec *decoder) handOver() error {
	for _, sp := range dec.spans {
		var id gw.ID
		switch {
		case sp.v != gw.None:
			id = dec.d.Vertex(sp.v).ID()
		case sp.e != gw.None:
			id = dec.d.Edge(sp.e).ID()
		}
		if err := dec.opts.Codec.Decode(id, dec.s.data[sp.start:sp.end]); err != nil {
			return errors.New(sp.path + ": " + err.Error())
		}
	}
	return nil
}

// errDataFull is a write past the data limit.
var errDataFull = errors.New("data exceeds the limit")

// capped is the writer an Encode writes through: it keeps up to max
// bytes and refuses a write past them.
type capped struct {
	b    []byte
	max  int
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if len(c.b)+len(p) > c.max {
		c.over = true
		return 0, errDataFull
	}
	c.b = append(c.b, p...)
	return len(p), nil
}

// data writes the "data" member of the element with the ID, or of the
// document for "", when the encoder has a codec and the codec has data
// for it. what names the element in an error, never the data.
func (e *encoder) data(d *gw.Document, id gw.ID, what string) {
	if e.codec == nil || e.err != nil {
		return
	}
	if e.out == nil {
		e.out = &capped{b: make([]byte, 0, min(e.limit, 4096)), max: e.limit}
	}
	e.out.b = e.out.b[:0] // over is false: a write past the limit ends the encode
	ok, err := d.EncodeData(e.codec, id, e.out)
	switch {
	case e.out.over:
		e.err = errors.New(what + ": data exceeds the limit of " + strconv.Itoa(e.limit) + " bytes")
	case err != nil:
		e.err = errors.New(what + ": data: " + err.Error())
	}
	if e.err != nil || !ok {
		return
	}
	n, msg, at := jsonValue(e.out.b)
	switch {
	case msg != "":
		e.err = errors.New(what + ": data is not well-formed JSON: " + msg + " at byte " + strconv.Itoa(at))
	case skipWS(e.out.b, n) != len(e.out.b):
		e.err = errors.New(what + ": data is more than one JSON value")
	}
	if e.err != nil {
		return
	}
	e.field("data")
	for _, c := range e.out.b[:n] {
		e.byte(c)
	}
}

// skipWS is the position of the first byte at or after i that is not
// JSON whitespace.
func skipWS(b []byte, i int) int {
	// bound: the input
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}

// The states of jsonValue.
const (
	needValue  = iota // a value
	firstValue        // a value or ']', just after '['
	needKey           // a key, after ','
	firstKey          // a key or '}', just after '{'
	afterValue        // ',' or the closing bracket, or the end at the top
)

// jsonValue checks that b starts with one well-formed JSON value, its
// arrays and objects nested no deeper than MaxDataDepth, and its
// strings valid UTF-8. It returns the value's length, or a message and
// the position at fault; the message never quotes the text. It walks
// the text once and never recurses.
func jsonValue(b []byte) (n int, msg string, at int) {
	w := jsonWalk{b: b, pos: skipWS(b, 0), state: needValue}
	// bound: every step consumes at least one byte of the input
	for w.state != afterValue || w.level > 0 {
		w.pos = skipWS(b, w.pos)
		if w.pos >= len(b) {
			return 0, "unexpected end", w.pos
		}
		switch w.state {
		case firstValue, needValue:
			msg = w.value()
		case firstKey, needKey:
			msg = w.key()
		default:
			msg = w.after()
		}
		if msg != "" {
			return 0, msg, w.pos
		}
	}
	return w.pos, "", 0
}

// jsonWalk is where jsonValue is: the position, the state, the nesting
// level, and a bit per level, set for an object.
type jsonWalk struct {
	b       []byte
	pos     int
	state   int
	level   int
	objects uint64
}

// value reads a value, or the ']' of an empty array, at the position.
func (w *jsonWalk) value() string {
	c := w.b[w.pos]
	if w.state == firstValue && c == ']' {
		w.pos, w.level, w.state = w.pos+1, w.level-1, afterValue
		return ""
	}
	var m string
	switch c {
	case '{', '[':
		if w.level == MaxDataDepth {
			return "nested deeper than " + strconv.Itoa(MaxDataDepth)
		}
		w.objects &^= 1 << w.level
		w.state = firstValue
		if c == '{' {
			w.objects |= 1 << w.level
			w.state = firstKey
		}
		w.pos, w.level = w.pos+1, w.level+1
		return ""
	case '"':
		w.pos, m = jsonString(w.b, w.pos)
	case 't', 'f', 'n':
		w.pos, m = jsonLiteral(w.b, w.pos)
	default:
		w.pos, m = jsonNumber(w.b, w.pos)
	}
	w.state = afterValue
	return m
}

// key reads an object's key and its colon, or the '}' of an empty
// object.
func (w *jsonWalk) key() string {
	if w.state == firstKey && w.b[w.pos] == '}' {
		w.pos, w.level, w.state = w.pos+1, w.level-1, afterValue
		return ""
	}
	if w.b[w.pos] != '"' {
		return "expected a key"
	}
	end, m := jsonString(w.b, w.pos)
	if m != "" {
		w.pos = end
		return m
	}
	w.pos = skipWS(w.b, end)
	if w.pos >= len(w.b) || w.b[w.pos] != ':' {
		return "expected ':'"
	}
	w.pos, w.state = w.pos+1, needValue
	return ""
}

// after reads the ',' or the closing bracket after a value inside an
// array or an object.
func (w *jsonWalk) after() string {
	inObject := w.objects&(1<<(w.level-1)) != 0
	closing, next := byte(']'), needValue
	if inObject {
		closing, next = '}', needKey
	}
	switch w.b[w.pos] {
	case ',':
		w.pos, w.state = w.pos+1, next
	case closing:
		w.pos, w.level = w.pos+1, w.level-1
	default:
		return "expected ',' or the closing bracket"
	}
	return ""
}

// jsonString checks the string starting at b[i], its opening quote, and
// returns the position after its closing quote.
func jsonString(b []byte, i int) (int, string) {
	start := i
	i++
	// bound: the input
	for i < len(b) {
		switch c := b[i]; {
		case c == '"':
			if !utf8.Valid(b[start+1 : i]) {
				return start, "string is not valid UTF-8"
			}
			return i + 1, ""
		case c < 0x20:
			return i, "control character in string"
		case c == '\\':
			if i+1 >= len(b) {
				return i, "unterminated string"
			}
			switch b[i+1] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i += 2
			case 'u':
				if i+6 > len(b) || !hex4(b[i+2:i+6]) {
					return i, "bad \\u escape"
				}
				i += 6
			default:
				return i, "bad escape"
			}
		default:
			i++
		}
	}
	return i, "unterminated string"
}

func hex4(h []byte) bool {
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// jsonLiteral checks true, false or null at b[i].
func jsonLiteral(b []byte, i int) (int, string) {
	for _, w := range [...]string{"true", "false", "null"} {
		if len(b)-i >= len(w) && string(b[i:i+len(w)]) == w {
			return i + len(w), ""
		}
	}
	return i, "expected a value"
}

// jsonNumber checks the number at b[i]: an optional minus, an integer
// part without a leading zero, an optional fraction and an optional
// exponent.
func jsonNumber(b []byte, i int) (int, string) {
	if i < len(b) && b[i] == '-' {
		i++
	}
	switch {
	case i < len(b) && b[i] == '0':
		i++
	case i < len(b) && b[i] >= '1' && b[i] <= '9':
		i = digits(b, i)
	default:
		return i, "expected a value"
	}
	if i < len(b) && b[i] == '.' {
		j := digits(b, i+1)
		if j == i+1 {
			return j, "expected a digit"
		}
		i = j
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		j := digits(b, i)
		if j == i {
			return j, "expected a digit"
		}
		i = j
	}
	return i, ""
}

// digits is the position after the digits at b[j].
func digits(b []byte, j int) int {
	// bound: the input
	for j < len(b) && b[j] >= '0' && b[j] <= '9' {
		j++
	}
	return j
}
