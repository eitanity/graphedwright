package graphedwright

import (
	"strconv"
	"unicode/utf8"
)

// checkText refuses text a document cannot hold: invalid UTF-8, a
// control character other than tab, newline and carriage return (C0,
// DEL and C1 alike), or U+FFFE or U+FFFF. XML cannot carry most of them,
// and the rest draw nothing. It applies to labels, attribute values and
// the schema's free text.
func checkText(what, s string) error {
	if !utf8.ValidString(s) {
		return refusal(what + " is not valid UTF-8")
	}
	// bound: the string
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 && c != '\t' && c != '\n' && c != '\r' || c == 0x7f || c1(s, i) {
			return controlError(what, s, i)
		}
		if nonchar(s, i) {
			return noncharError(what, s, i)
		}
	}
	return nil
}

// checkName refuses a name with any control character, tab and newline
// included, DEL and C1 too, or U+FFFE or U+FFFF: IDs and the schema's kind and port
// names, which end up in markup and selectors. The refusal quotes the
// name after what it is.
func checkName(what, s string) error {
	if !utf8.ValidString(s) {
		return refusal(what + " " + strconv.Quote(s) + " is not valid UTF-8")
	}
	// bound: the string
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c == 0x7f || c1(s, i) {
			return controlError(what+" "+strconv.Quote(s), s, i)
		}
		if nonchar(s, i) {
			return noncharError(what+" "+strconv.Quote(s), s, i)
		}
	}
	return nil
}

// checkKey refuses an attribute key other than lowercase letters,
// digits, '-' and '_', or one the drawing uses. A key is written as a
// data- attribute, so any other character would be mapped to '_' and
// two keys could become one, and the HTML parser folds case, which
// would make two more.
func checkKey(k string) error {
	// bound: the key
	for i := 0; i < len(k); i++ {
		if !keyByte(k[i]) {
			return refusal("attribute key " + strconv.Quote(k) + " may hold only lowercase letters, digits, '-' and '_'")
		}
	}
	// The drawing writes its own data-kind, data-from and data-to on the
	// same element as the attributes.
	if k == "kind" || k == "from" || k == "to" {
		return refusal("attribute key " + strconv.Quote(k) + " is reserved for the drawing")
	}
	return nil
}

func keyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// nonchar reports whether U+FFFE or U+FFFF starts at byte i of valid
// UTF-8: characters XML cannot carry, as it cannot the controls.
func nonchar(s string, i int) bool {
	return s[i] == 0xef && i+2 < len(s) && s[i+1] == 0xbf && s[i+2] >= 0xbe
}

func noncharError(what, s string, at int) error {
	last := "E"
	if s[at+2] == 0xbf {
		last = "F"
	}
	return refusal(what + " holds the noncharacter U+FFF" + last + " at byte " + itoa(at))
}

func controlError(what, s string, at int) error {
	c := s[at] // C0 and DEL are one byte, and a C1 character is its second
	if c1(s, at) {
		c = s[at+1]
	}
	hex := strconv.FormatUint(uint64(c), 16)
	if len(hex) < 2 {
		hex = "0" + hex
	}
	return refusal(what + " holds the control character U+00" + hex + " at byte " + itoa(at))
}

// c1 reports whether s holds a C1 control character, U+0080 to U+009F,
// at byte i: in UTF-8, 0xC2 and then 0x80 to 0x9F. s is valid UTF-8, so
// a 0xC2 has a byte after it.
func c1(s string, i int) bool {
	return s[i] == 0xc2 && s[i+1] >= 0x80 && s[i+1] <= 0x9f
}

// checkID refuses an ID past the label limit or holding a control
// character. An empty ID is minted and needs no check.
func (d *Document) checkID(id ID) error {
	if len(id) > d.caps.LabelBytes {
		return refusal("id of " + itoa(len(id)) + " bytes exceeds the limit of " + itoa(d.caps.LabelBytes))
	}
	return checkName("id", string(id))
}
