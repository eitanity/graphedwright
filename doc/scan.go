package doc

import (
	"errors"
	"strconv"
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

// str reads a string, with escapes, up to maxS bytes.
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
			if len(raw) > s.maxS {
				return "", s.errorf("string of " + strconv.Itoa(len(raw)) + " bytes exceeds the limit of " + strconv.Itoa(s.maxS))
			}
			if plain {
				if !utf8.Valid(raw) {
					return "", s.errorf("string is not valid UTF-8")
				}
				return string(raw), nil
			}
			return unescape(raw, s)
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
			if i+4 >= len(raw) {
				return "", s.errorf("bad unicode escape")
			}
			v, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 32)
			if err != nil {
				return "", s.errorf("bad unicode escape")
			}
			out = utf8.AppendRune(out, rune(v)) //nolint:gosec // at most four hex digits
			i += 4
		default:
			return "", s.errorf("bad escape")
		}
	}
	if !utf8.Valid(out) {
		return "", s.errorf("string is not valid UTF-8")
	}
	return string(out), nil
}

// num reads a finite number.
func (s *scanner) num() (float64, error) {
	s.ws()
	start := s.pos
	// bound: the input
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			s.pos++
			continue
		}
		break
	}
	if start == s.pos {
		return 0, s.errorf("expected a number")
	}
	v, err := strconv.ParseFloat(string(s.data[start:s.pos]), 64)
	if err != nil || v != v || v-v != 0 {
		return 0, s.errorf("bad number " + strconv.Quote(string(s.data[start:s.pos])))
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
