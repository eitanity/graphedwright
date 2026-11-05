package doc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// jsonCases are values jsonValue accepts and refuses, with the length
// it gives an accepted one.
var jsonCases = []struct {
	in  string
	n   int // the value's length; -1 for refused
	msg string
}{
	{`0`, 1, ""}, {`-0`, 2, ""}, {`12.5e-3`, 7, ""}, {`1E+2`, 4, ""}, {` 7 `, 2, ""},
	{`true`, 4, ""}, {`false`, 5, ""}, {`null`, 4, ""},
	{`""`, 2, ""}, {`"a\"\\\/\b\f\n\r\tz"`, 20, ""}, {`"\u00e9\uD83D"`, 14, ""}, {`"é"`, 4, ""},
	{`[]`, 2, ""}, {`{}`, 2, ""}, {`[1,[2,{}],{"a":[]}]`, 19, ""}, {`{"a":1,"b":{"c":null}}`, 22, ""},
	{` [ 1 , 2 ] `, 10, ""}, {`[{"a":1},[1,2]]`, 15, ""}, {`{"a":[1],"b":{"c":2}}`, 21, ""}, {`{ "a" : 1 }`, 11, ""}, {`[1]]`, 3, ""}, {`1 2`, 1, ""},
	{``, -1, "unexpected end"}, {`   `, -1, "unexpected end"}, {`[`, -1, "unexpected end"}, {`{"a"`, -1, "expected ':'"},
	{`{"a" 1}`, -1, "expected ':'"}, {`{"a":}`, -1, "expected a value"}, {`[1,]`, -1, "expected a value"}, {`{,}`, -1, "expected a key"}, {`{"a":1,}`, -1, "expected a key"},
	{`{1:2}`, -1, "expected a key"}, {`[1 2]`, -1, "expected ',' or the closing bracket"}, {`{"a":1]`, -1, "expected ',' or the closing bracket"},
	{`[1}`, -1, "expected ',' or the closing bracket"},
	{`01`, 1, ""}, {`-`, -1, "expected a value"}, {`1.`, -1, "expected a digit"}, {`1.e2`, -1, "expected a digit"}, {`1e`, -1, "expected a digit"}, {`1e+`, -1, "expected a digit"},
	{`+1`, -1, "expected a value"}, {`.5`, -1, "expected a value"}, {`tru`, -1, "expected a value"}, {`nul`, -1, "expected a value"}, {`True`, -1, "expected a value"},
	{`"abc`, -1, "unterminated string"}, {`"a\`, -1, "unterminated string"}, {`"\x"`, -1, "bad escape"}, {`"\u12"`, -1, "bad \\u escape"}, {`"\u12G4"`, -1, "bad \\u escape"},
	{"\"a\x1fb\"", -1, "control character in string"}, {"\"\xff\"", -1, "string is not valid UTF-8"}, {"{\"\xc3\":1}", -1, "string is not valid UTF-8"},
	{strings.Repeat("[", 64) + strings.Repeat("]", 64), 128, ""},
	{strings.Repeat("[", 65) + strings.Repeat("]", 65), -1, "nested deeper than 64"},
	{strings.Repeat(`{"a":`, 64) + "1" + strings.Repeat("}", 64), 64*5 + 1 + 64, ""},
	{strings.Repeat(`{"a":`, 65) + "1" + strings.Repeat("}", 65), -1, "nested deeper than 64"},
	{strings.Repeat("[", 63) + `{"a":[1]}` + strings.Repeat("]", 63), -1, "nested deeper than 64"},
	{strings.Repeat("[", 62) + `{"a":[1]}` + strings.Repeat("]", 62), 62*2 + 9, ""},
}

// TestJSONValue: jsonValue accepts exactly the values above, with their
// length, and refuses the rest with the message.
func TestJSONValue(t *testing.T) {
	for _, c := range jsonCases {
		n, msg, _ := jsonValue([]byte(c.in))
		switch {
		case c.n >= 0 && (msg != "" || n != c.n):
			t.Errorf("%q: got %d %q, want %d", c.in, n, msg, c.n)
		case c.n < 0 && msg != c.msg:
			t.Errorf("%q: got %d %q, want %q", c.in, n, msg, c.msg)
		}
	}
}

// TestJSONValueAt: the position of the fault.
func TestJSONValueAt(t *testing.T) {
	for _, c := range []struct {
		in string
		at int
	}{{"{\"a\x01\":1}", 3}, {`{"a" 1}`, 5}, {`[1,]`, 3}, {"[\"\xff\"]", 1}, {`[1 2]`, 3}, {`[1.e]`, 3}} {
		if _, msg, at := jsonValue([]byte(c.in)); msg == "" || at != c.at {
			t.Errorf("%q: %q at %d, want %d", c.in, msg, at, c.at)
		}
	}
}

// FuzzJSONValue: on any text, jsonValue agrees with encoding/json on
// whether the text up to the value's end, with nothing but whitespace
// after, is one valid value, within the nesting limit.
func FuzzJSONValue(f *testing.F) {
	for _, c := range jsonCases {
		f.Add([]byte(c.in))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		n, msg, at := jsonValue(b)
		if msg != "" {
			if at < 0 || at > len(b) {
				t.Fatalf("%q: position %d out of range", b, at)
			}
			if json.Valid(b) && depth(b) <= MaxDataDepth && !strings.Contains(msg, "UTF-8") {
				t.Fatalf("%q: refused (%s at %d), encoding/json accepts it", b, msg, at)
			}
			return
		}
		if n <= 0 || n > len(b) {
			t.Fatalf("%q: length %d", b, n)
		}
		if !json.Valid(b[:n]) {
			t.Fatalf("%q: accepted %q, encoding/json refuses it", b, b[:n])
		}
		if rest := bytes.TrimLeft(b[n:], " \t\r\n"); len(rest) == 0 != json.Valid(b) {
			t.Fatalf("%q: nothing after the value is %v, encoding/json says %v", b, len(rest) == 0, json.Valid(b))
		}
	})
}

// depth is the deepest nesting of brackets outside strings.
func depth(b []byte) int {
	d, most, in := 0, 0, false
	for i := 0; i < len(b); i++ {
		switch c := b[i]; {
		case in && c == '\\':
			i++
		case c == '"':
			in = !in
		case !in && (c == '[' || c == '{'):
			d++
			most = max(most, d)
		case !in && (c == ']' || c == '}'):
			d--
		}
	}
	return most
}
