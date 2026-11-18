package doc

import (
	"errors"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// MaxInput bounds the bytes a decoder reads: a document past it is
// refused before anything is parsed.
const MaxInput = 16 << 20

// scanner walks JSON text one token at a time. It keeps the position
// for error messages and the nesting depth, and reads strings and
// numbers into Go values bounded by the caller's limits. It never
// recurses: the decoder drives it with an explicit expectation of what
// comes next.
type scanner struct {
	data []byte
	pos  int
	path []byte // the key path for messages, e.g. vertices[3].attrs
	maxS int    // the longest string accepted
}

func (s *scanner) errorf(msg string) error {
	return errors.New(string(s.path) + ": " + msg + " at byte " + strconv.Itoa(s.pos))
}

// ws skips whitespace.
func (s *scanner) ws() {
	// bound: the input
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

// peek is the next byte after whitespace, or 0 at the end.
func (s *scanner) peek() byte {
	s.ws()
	if s.pos >= len(s.data) {
		return 0
	}
	return s.data[s.pos]
}

// expect consumes one byte or fails naming it.
func (s *scanner) expect(c byte) error {
	if s.peek() != c {
		if s.pos >= len(s.data) {
			return s.errorf("unexpected end of input, expected " + strconv.QuoteRune(rune(c)))
		}
		return s.errorf("expected " + strconv.QuoteRune(rune(c)) + ", found " + strconv.QuoteRune(rune(s.data[s.pos])))
	}
	s.pos++
	return nil
}

// open consumes an opening bracket. The format nests five levels at
// most, and the decoder walks them with one function per level, so
// there is no stack to bound.
func (s *scanner) open(c byte) error { return s.expect(c) }

// close consumes the closing bracket that more has just seen.
func (s *scanner) close() { s.pos++ }

// more reports whether another element follows in an array or object:
// after a comma yes, before the closing bracket no.
func (s *scanner) more(closing byte) (bool, error) {
	switch s.peek() {
	case ',':
		s.pos++
		return true, nil
	case closing:
		return false, nil
	case 0:
		return false, s.errorf("unexpected end of input")
	}
	return false, s.errorf("expected ',' or " + strconv.QuoteRune(rune(closing)) + ", found " + strconv.QuoteRune(rune(s.data[s.pos])))
}

// str reads a string, with escapes, of up to maxS bytes once they are
// resolved.
func (s *scanner) str() (string, error) {
	if err := s.expect('"'); err != nil {
		return "", err
	}
	start := s.pos
	plain := true
	// bound: the input
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c == '"' {
			raw := s.data[start:s.pos]
			s.pos++
			return s.text(raw, plain)
		}
		if c < 0x20 {
			return "", s.errorf("control character in string")
		}
		if c == '\\' {
			plain = false
			s.pos++
		}
		s.pos++
	}
	return "", s.errorf("unterminated string")
}

// text is a string's text from its body, raw, which holds an escape
// unless plain.
func (s *scanner) text(raw []byte, plain bool) (string, error) {
	if plain {
		if len(raw) > s.maxS {
			return "", s.tooLong(len(raw))
		}
		if !utf8.Valid(raw) {
			return "", s.errorf("string is not valid UTF-8")
		}
		return string(raw), nil
	}
	// The limit is on the text, as an edit counts it, not on the text as
	// written: an escape writes a byte in two or more. Six is the most
	// (\u0041), so a body longer than six times the limit holds too much
	// text and is refused unread.
	if len(raw) > maxEscaped(s.maxS) {
		return "", s.errorf("string exceeds the limit of " + strconv.Itoa(s.maxS) + " bytes")
	}
	text, err := unescape(raw, s)
	if err == nil && len(text) > s.maxS {
		return "", s.tooLong(len(text))
	}
	return text, err
}

// tooLong refuses a string of n bytes of text.
func (s *scanner) tooLong(n int) error {
	return s.errorf("string of " + strconv.Itoa(n) + " bytes exceeds the limit of " + strconv.Itoa(s.maxS))
}

// escapeGrowth is the most bytes a string's body takes, written with
// escapes, for one byte of its text: \u0041 is six for one. A surrogate
// pair is twelve for four, and every other escape two for one.
const escapeGrowth = 6

// maxEscaped is the longest body a string of at most limit bytes of text
// can be written in; the input's own limit beyond that.
func maxEscaped(limit int) int {
	if limit > MaxInput/escapeGrowth {
		return MaxInput
	}
	return limit * escapeGrowth
}

// unescape resolves the escapes of a raw string body.
func unescape(raw []byte, s *scanner) (string, error) {
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		// A backslash is never last: it would have escaped the closing quote.
		i++
		switch raw[i] {
		case '"', '\\', '/':
			out = append(out, raw[i])
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			r, end, msg := uEscape(raw, i)
			if msg != "" {
				return "", s.errorf(msg)
			}
			out = utf8.AppendRune(out, r)
			i = end
		default:
			return "", s.errorf("bad escape")
		}
	}
	if !utf8.Valid(out) {
		return "", s.errorf("string is not valid UTF-8")
	}
	return string(out), nil
}

// uEscape reads the \u escape whose u is raw[i]: the rune it names and
// the index of its last byte, or why it names none. A character past
// U+FFFF is escaped as a pair of halves, high then low; either alone is
// no character. A backslash is never last, so one after the first half
// has a byte after it.
func uEscape(raw []byte, i int) (r rune, end int, msg string) {
	r, ok := escaped(raw, i+1)
	if !ok {
		return 0, i, "bad unicode escape"
	}
	if !utf16.IsSurrogate(r) {
		return r, i + 4, ""
	}
	lo := rune(0)
	if i+5 < len(raw) && raw[i+5] == '\\' && raw[i+6] == 'u' {
		lo, _ = escaped(raw, i+7)
	}
	if r = utf16.DecodeRune(r, lo); r == utf8.RuneError {
		return 0, i, "unpaired surrogate in unicode escape"
	}
	return r, i + 10, ""
}

// escaped is the rune that the four hex digits of a \u escape, in raw
// from j, name, and whether there are four.
func escaped(raw []byte, j int) (rune, bool) {
	if j+4 > len(raw) {
		return 0, false
	}
	v, err := strconv.ParseUint(string(raw[j:j+4]), 16, 32)
	return rune(v), err == nil //nolint:gosec // at most four hex digits
}

// num reads a finite number.
func (s *scanner) num() (float64, error) {
	s.ws()
	start := s.pos
	// JSON's grammar: no plus, no leading zero, a digit either side of a
	// point (jsonNumber).
	end, msg := jsonNumber(s.data, start)
	s.pos = end
	if end == start {
		return 0, s.errorf("expected a number")
	}
	v, err := strconv.ParseFloat(string(s.data[start:end]), 64)
	if msg != "" || err != nil || v != v || v-v != 0 {
		return 0, s.errorf("bad number " + strconv.Quote(string(s.data[start:end])))
	}
	return v, nil
}

// boolean reads true or false.
func (s *scanner) boolean() (bool, error) {
	s.ws()
	if s.lit("true") {
		return true, nil
	}
	if s.lit("false") {
		return false, nil
	}
	return false, s.errorf("expected true or false")
}

// lit consumes a literal if it is next.
func (s *scanner) lit(word string) bool {
	if len(s.data)-s.pos < len(word) || string(s.data[s.pos:s.pos+len(word)]) != word {
		return false
	}
	s.pos += len(word)
	return true
}

// key reads an object key and its colon.
func (s *scanner) key() (string, error) {
	k, err := s.str()
	if err != nil {
		return "", err
	}
	return k, s.expect(':')
}

// push and pop keep the path for messages.
func (s *scanner) push(seg string) { s.path = append(s.path, seg...) }

func (s *scanner) pop(n int) { s.path = s.path[:len(s.path)-n] }
