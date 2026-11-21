package graphedwright

import (
	"unicode"
	"unicode/utf8"
)

// Find is the next live vertex after the one given, in index order and
// round to the start, whose label or ID holds the text, compared with
// its case folded; the vertex given is the last one tried, and None
// starts from the first. A hidden vertex is found too. It is None for
// empty text and when no vertex matches. It allocates nothing.
func (d *Document) Find(text string, after VertexIndex) VertexIndex {
	n := len(d.vertices)
	if text == "" || n == 0 {
		return None
	}
	start := 0
	if after >= 0 && int(after) < n {
		start = int(after) + 1
	}
	// bound: every vertex once
	for k := range n {
		i := (start + k) % n
		if d.matches(i, text) {
			return VertexIndex(i) //nolint:gosec // bounded by the capacity
		}
	}
	return None
}

// FindCount is how many live vertices Find would find for the text.
func (d *Document) FindCount(text string) int {
	if text == "" {
		return 0
	}
	n := 0
	for i := range d.vertices {
		if d.matches(i, text) {
			n++
		}
	}
	return n
}

// matches reports whether vertex slot i is live and its label or ID
// holds the text.
func (d *Document) matches(i int, text string) bool {
	v := &d.vertices[i]
	return v.Live && (holdsFolded(v.Label, text) || holdsFolded(string(v.ID), text))
}

// holdsFolded reports whether s holds sub, their cases folded, as
// strings.EqualFold compares.
func holdsFolded(s, sub string) bool {
	// bound: every rune start of s
	for i := 0; i < len(s); {
		if prefixFolded(s[i:], sub) {
			return true
		}
		_, w := utf8.DecodeRuneInString(s[i:])
		i += w
	}
	return false
}

// prefixFolded reports whether s starts with p, their cases folded.
func prefixFolded(s, p string) bool {
	// bound: the runes of p
	for p != "" {
		if s == "" {
			return false
		}
		a, wa := utf8.DecodeRuneInString(s)
		b, wb := utf8.DecodeRuneInString(p)
		if !sameFolded(a, b) {
			return false
		}
		s, p = s[wa:], p[wb:]
	}
	return true
}

// sameFolded reports whether two runes are the same under simple case
// folding.
func sameFolded(a, b rune) bool {
	if a == b {
		return true
	}
	if a < utf8.RuneSelf && b < utf8.RuneSelf {
		return 'A' <= a && a <= 'Z' && a+'a'-'A' == b || 'A' <= b && b <= 'Z' && b+'a'-'A' == a
	}
	// bound: a rune's fold orbit, at most four runes long
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}
