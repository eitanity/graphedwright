package graphedwright

import "strconv"

// A label format builds the label a kind's elements are drawn with from
// the label as set and the attributes. Text outside braces is drawn as
// it is. {key} is the label, when key is label, or the attribute's
// value; {before|key} and {before|key|after} add the text either side
// only when the value is not empty. So "{label}{ : |type}{ [|mult|]}"
// draws "wheels : Wheel [4]", or "wheels" when neither attribute is set.

// labelKey is the key of a format's placeholder for the label as set.
const labelKey = "label"

// NextLabelPart is the next part of the label drawn by a format, and
// the rest of the format after it: text outside braces as before, or a
// placeholder's before, value and after, all empty when the value is.
// An empty format draws the label as set.
func NextLabelPart(format, label string, attrs AttrList) (before, value, after, rest string) {
	if format == "" {
		return "", label, "", ""
	}
	open := indexByte(format, '{')
	switch {
	case open < 0:
		return format, "", "", ""
	case open > 0:
		return format[:open], "", "", format[open:]
	}
	// A schema refuses a format whose braces are not paired.
	end := indexByte(format, '}')
	before, key, after := placeholder(format[1:end])
	value = label
	if key != labelKey {
		value = attrs.Value(key)
	}
	if value == "" {
		return "", "", "", format[end+1:]
	}
	return before, value, after, format[end+1:]
}

// LabelRunes is the number of characters of the label drawn by a
// format, by which the layout makes room for it and the renderer cuts
// it.
func LabelRunes(format, label string, attrs AttrList) int {
	n := 0
	// bound: the format's parts, at most its length
	for more := true; more; {
		var before, value, after string
		before, value, after, format = NextLabelPart(format, label, attrs)
		n += runes(before) + runes(value) + runes(after)
		more = format != ""
	}
	return n
}

// AppendLabel appends the label drawn by a format.
func AppendLabel(dst []byte, format, label string, attrs AttrList) []byte {
	// bound: the format's parts, at most its length
	for more := true; more; {
		var before, value, after string
		before, value, after, format = NextLabelPart(format, label, attrs)
		dst = append(append(append(dst, before...), value...), after...)
		more = format != ""
	}
	return dst
}

// placeholder splits what is inside a placeholder's braces into the
// text before, the key and the text after.
func placeholder(s string) (before, key, after string) {
	i := indexByte(s, '|')
	if i < 0 {
		return "", s, ""
	}
	before, s = s[:i], s[i+1:]
	if j := indexByte(s, '|'); j >= 0 {
		return before, s[:j], s[j+1:]
	}
	return before, s, ""
}

// checkLabelFormat refuses a format whose braces are not paired, or
// nested, a placeholder with more than two bars, and a key that is
// neither label nor an attribute the kind declares.
func checkLabelFormat(format string, attrs []AttrSpec) error {
	if err := checkText("label format", format); err != nil {
		return err
	}
	open := -1
	// bound: the format
	for i := 0; i < len(format); i++ {
		switch format[i] {
		case '{':
			if open >= 0 {
				return refusal("label format has a brace without its pair")
			}
			open = i
		case '}':
			if open < 0 {
				return refusal("label format has a brace without its pair")
			}
			_, key, after := placeholder(format[open+1 : i])
			if indexByte(after, '|') >= 0 {
				return refusal("label format has a placeholder with more than two bars")
			}
			if _, ok := spec(attrs, key); key != labelKey && !ok {
				return refusal("label format names " + strconv.Quote(key) + ", neither label nor an attribute of the kind")
			}
			open = -1
		}
	}
	if open >= 0 {
		return refusal("label format has a brace without its pair")
	}
	return nil
}

// indexByte is the index of the first c in s, or -1.
func indexByte(s string, c byte) int {
	// bound: the string
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// VertexLabelRunes is the number of characters of a vertex's label as
// its kind's format draws it.
func (d *Document) VertexLabelRunes(v VertexIndex) int {
	x := &d.vertices[v]
	return LabelRunes(d.kinds.vertex[x.Kind].LabelFormat, x.Label, AttrList{x.Attrs})
}

// EdgeLabelRunes is the number of characters of an edge's label as its
// kind's format draws it.
func (d *Document) EdgeLabelRunes(e EdgeIndex) int {
	x := &d.edges[e]
	return LabelRunes(d.kinds.edge[x.Kind].LabelFormat, x.Label, AttrList{x.Attrs})
}
