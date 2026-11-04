package graphedwright

// The lists a document and its kinds hold are read through these
// values: each holds the list but gives no way to change it, and reads
// by position, the zero value out of range. A caller's own list is
// read the same way through PortsOf, AttrsOf and PointsOf.

// PortList is a list of ports.
type PortList struct{ s []Port }

// PortsOf reads the caller's own ports as a list.
func PortsOf(s []Port) PortList { return PortList{s} }

// Len is the number of ports.
func (l PortList) Len() int { return len(l.s) }

// At is the ith port; out of range, the zero Port.
func (l PortList) At(i int) Port {
	if i < 0 || i >= len(l.s) {
		return Port{}
	}
	return l.s[i]
}

// Rest is the list from position i on: empty past its end, all of it
// below 0.
func (l PortList) Rest(i int) PortList { return PortList{l.s[min(max(i, 0), len(l.s)):]} }

// AttrList is a list of attributes.
type AttrList struct{ s []Attr }

// AttrsOf reads the caller's own attributes as a list.
func AttrsOf(s []Attr) AttrList { return AttrList{s} }

// Len is the number of attributes.
func (l AttrList) Len() int { return len(l.s) }

// At is the ith attribute; out of range, the zero Attr.
func (l AttrList) At(i int) Attr {
	if i < 0 || i >= len(l.s) {
		return Attr{}
	}
	return l.s[i]
}

// Value is the value of the attribute with the key, or "".
func (l AttrList) Value(key string) string { return AttrValue(l.s, key) }

// PointList is a list of points.
type PointList struct{ s []Point }

// PointsOf reads the caller's own points as a list.
func PointsOf(s []Point) PointList { return PointList{s} }

// Len is the number of points.
func (l PointList) Len() int { return len(l.s) }

// At is the ith point; out of range, the zero Point.
func (l PointList) At(i int) Point {
	if i < 0 || i >= len(l.s) {
		return Point{}
	}
	return l.s[i]
}

// KindList is a list of kind indices.
type KindList struct{ s []KindIndex }

// Len is the number of kinds.
func (l KindList) Len() int { return len(l.s) }

// At is the ith kind; out of range, None.
func (l KindList) At(i int) KindIndex {
	if i < 0 || i >= len(l.s) {
		return None
	}
	return l.s[i]
}

// StringList is a list of strings.
type StringList struct{ s []string }

// Len is the number of strings.
func (l StringList) Len() int { return len(l.s) }

// At is the ith string; out of range, "".
func (l StringList) At(i int) string {
	if i < 0 || i >= len(l.s) {
		return ""
	}
	return l.s[i]
}

// AttrSpecList is a kind's attribute specifications.
type AttrSpecList struct{ s []AttrSpec }

// Len is the number of specifications.
func (l AttrSpecList) Len() int { return len(l.s) }

// At is the ith specification; out of range, every read of it is the
// zero value.
func (l AttrSpecList) At(i int) AttrSpecView {
	if i < 0 || i >= len(l.s) {
		return AttrSpecView{}
	}
	return AttrSpecView{&l.s[i]}
}
