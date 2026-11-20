package doc

import (
	"errors"
	"strconv"
	"unicode/utf8"

	gw "code.eitanity.com/graphedwright"
)

// What a container in the text is, to the capacity check: what its
// members count against.
const (
	inOther    uint8 = iota // nothing counted
	inRoot                  // the document
	inVertices              // the vertex array
	inEdges                 // the edge array
	inVertex                // a vertex object
	inEdge                  // an edge object
	inAttrs                 // an element's attributes
	inPorts                 // a vertex's ports
	inBends                 // an edge's bends
	inData                  // a "data" member, whose text is the host's
)

// fitDepth is the deepest the check follows: a document's own members
// are no deeper than five, and a "data" member's as deep as
// MaxDataDepth below an element's.
const fitDepth = MaxDataDepth + 8

// fitWalk is where fits is in the text: the position, the state, the
// nesting level, and per level whether it is an object, what it is,
// and how many members it has.
type fitWalk struct {
	b        []byte
	caps     gw.Caps
	pos      int
	state    int
	level    int
	pending  uint8 // what a container opened as the value of the last key would be
	object   [fitDepth]bool
	in       [fitDepth]uint8
	members  [fitDepth]int
	vertices int
	edges    int
	bends    int
}

// fits refuses a document that does not fit the capacities: more
// vertices or edges, more attributes on an element or ports on a vertex,
// more bend points than the edges have room for, or a text, a key or a
// value, longer than a label may be. It reads the text once, keeps
// nothing and never recurses; text that is not well-formed it leaves to
// the decoder, which says what is wrong with it.
func fits(data []byte, caps gw.Caps) error {
	w := fitWalk{b: data, caps: caps, pos: skipWS(data, 0), state: needValue}
	// bound: every step consumes at least one byte of the input
	for w.state != afterValue || w.level > 0 {
		w.pos = skipWS(data, w.pos)
		if w.pos >= len(data) || w.level >= fitDepth {
			return nil
		}
		var err error
		ok := true
		switch w.state {
		case firstValue, needValue:
			ok, err = w.value()
		case firstKey, needKey:
			ok, err = w.key()
		default:
			ok = w.after()
		}
		if err != nil {
			return errors.New(err.Error() + " at byte " + strconv.Itoa(w.pos))
		}
		if !ok {
			return nil
		}
	}
	return nil
}

// within is what the innermost open container is.
func (w *fitWalk) within() uint8 {
	if w.level == 0 {
		return inOther
	}
	return w.in[w.level-1]
}

// value reads a value, or the ']' of an empty array, counting it as a
// member of its array; ok is false for text the decoder is to refuse.
func (w *fitWalk) value() (bool, error) {
	c := w.b[w.pos]
	if w.state == firstValue && c == ']' {
		w.pos, w.level, w.state = w.pos+1, w.level-1, afterValue
		return true, nil
	}
	if err := w.count(); err != nil {
		return false, err
	}
	var m string
	switch c {
	case '{', '[':
		w.open(c == '{')
		return true, nil
	case '"':
		end, msg := jsonString(w.b, w.pos)
		if msg != "" {
			return false, nil
		}
		if err := w.text(w.pos, end); err != nil {
			return false, err
		}
		w.pos = end
	case 't', 'f', 'n':
		w.pos, m = jsonLiteral(w.b, w.pos)
	default:
		w.pos, m = jsonNumber(w.b, w.pos)
	}
	w.state = afterValue
	return m == "", nil
}

// count counts a value against what its array holds.
func (w *fitWalk) count() error {
	c := w.caps
	switch w.within() {
	case inVertices:
		if w.vertices++; w.vertices > c.Vertices {
			return errors.New("more vertices than the capacity of " + strconv.Itoa(c.Vertices))
		}
	case inEdges:
		if w.edges++; w.edges > c.Edges {
			return errors.New("more edges than the capacity of " + strconv.Itoa(c.Edges))
		}
	case inPorts:
		if w.members[w.level-1]++; w.members[w.level-1] > c.PortsPerVertex {
			return errors.New("a vertex has more ports than the capacity of " + strconv.Itoa(c.PortsPerVertex))
		}
	case inBends:
		if w.bends++; w.bends > c.Edges*c.BendsPerEdge {
			return errors.New("more bend points than the capacity of " + strconv.Itoa(c.Edges*c.BendsPerEdge))
		}
	}
	return nil
}

// open opens an object or an array as the value at the position, and
// notes what it is from what holds it and the key it is the value of.
func (w *fitWalk) open(object bool) {
	in := inOther
	switch w.within() {
	case inOther:
		if w.level == 0 && object {
			in = inRoot
		}
	case inRoot, inVertex, inEdge:
		in = w.pending
	case inVertices:
		in = inVertex
	case inEdges:
		in = inEdge
	case inData:
		in = inData
	case inAttrs, inPorts, inBends:
	}
	w.object[w.level], w.in[w.level], w.members[w.level] = object, in, 0
	w.state = firstValue
	if object {
		w.state = firstKey
	}
	w.pos, w.level = w.pos+1, w.level+1
}

// key reads an object's key and its colon, or the '}' of an empty
// object, counting an attribute, and notes what a container as its
// value would be.
func (w *fitWalk) key() (bool, error) {
	if w.state == firstKey && w.b[w.pos] == '}' {
		w.pos, w.level, w.state = w.pos+1, w.level-1, afterValue
		return true, nil
	}
	if w.b[w.pos] != '"' {
		return false, nil
	}
	end, m := jsonString(w.b, w.pos)
	if m != "" {
		return false, nil
	}
	if err := w.text(w.pos, end); err != nil {
		return false, err
	}
	in := w.within()
	if in == inAttrs {
		if w.members[w.level-1]++; w.members[w.level-1] > w.caps.AttrsPerElement {
			return false, errors.New("an element has more attributes than the capacity of " + strconv.Itoa(w.caps.AttrsPerElement))
		}
	}
	w.pending = keyHolds(in, w.b[w.pos+1:end-1])
	w.pos = skipWS(w.b, end)
	if w.pos >= len(w.b) || w.b[w.pos] != ':' {
		return false, nil
	}
	w.pos, w.state = w.pos+1, needValue
	return true, nil
}

// keyHolds is what a container that is the value of the key, in an
// object of the kind given, is. The key is as written: one with an
// escape is none the decoder knows.
func keyHolds(in uint8, k []byte) uint8 {
	switch {
	case same(k, dataKey) && in != inOther && in != inData:
		return inData
	case in == inRoot && same(k, "vertices"):
		return inVertices
	case in == inRoot && same(k, "edges"):
		return inEdges
	case (in == inVertex || in == inEdge) && same(k, "attrs"):
		return inAttrs
	case in == inVertex && same(k, "ports"):
		return inPorts
	case in == inEdge && same(k, "bends"):
		return inBends
	}
	return inOther
}

// same reports whether the bytes are the string, without making one of
// them.
func same(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	// bound: the string
	for i := 0; i < len(s); i++ {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}

// after reads the ',' or the closing bracket after a value; ok is false
// for anything else.
func (w *fitWalk) after() bool {
	object := w.object[w.level-1]
	closing, next := byte(']'), needValue
	if object {
		closing, next = '}', needKey
	}
	switch w.b[w.pos] {
	case ',':
		w.pos, w.state = w.pos+1, next
	case closing:
		w.pos, w.level = w.pos+1, w.level-1
	default:
		return false
	}
	return true
}

// text refuses the string from start, its opening quote, to end, after
// its closing one, when its text, its escapes resolved, is longer than a
// label may be; a "data" member's strings are the host's.
func (w *fitWalk) text(start, end int) error {
	if w.within() == inData {
		return nil
	}
	if n := textBytes(w.b[start+1 : end-1]); n > w.caps.LabelBytes {
		return errors.New("a string of " + strconv.Itoa(n) + " bytes exceeds the limit of " + strconv.Itoa(w.caps.LabelBytes))
	}
	return nil
}

// textBytes is the length of a string's text, its escapes resolved as
// unescape resolves them, of a body jsonString has checked.
func textBytes(raw []byte) int {
	n := 0
	// bound: the body
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			n++
			continue
		}
		i++
		if raw[i] != 'u' {
			n++
			continue
		}
		r, end, msg := uEscape(raw, i)
		if msg != "" {
			return n
		}
		n += utf8.RuneLen(r)
		i = end
	}
	return n
}
