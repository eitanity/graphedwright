package graphedwright

// The sizes by which a vertex holds its compartments. They are fixed
// here, not taken from the renderer's options, so that the layout, Fit
// and the renderer agree on a vertex's size with nothing measured.
const (
	// CharWidth is the estimated width of a character of a
	// compartment's line.
	CharWidth = 7
	// CompartmentLine is the height of a line of a compartment.
	CompartmentLine = 14
	// CompartmentTitle is the room for the label above the
	// compartments, and for the keyword above it when the kind has one.
	CompartmentTitle = 24
	// CompartmentPad is the room above and below a compartment's lines.
	CompartmentPad = 4
	// CompartmentInset is the room either side of a compartment's
	// lines.
	CompartmentInset = 8
)

// TitleRoom is the height of what is above a vertex's compartments: its
// label, and the keyword line above that when the kind has one.
func TitleRoom(k VertexKindView) float64 {
	if k.Keyword() != "" {
		return CompartmentTitle + CompartmentLine
	}
	return CompartmentTitle
}

// NextLine splits off the first line of a compartment's value, without
// the carriage return of a CRLF; rest is empty after the last line.
func NextLine(s string) (line, rest string, more bool) {
	// bound: the string
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line, rest = s[:i], s[i+1:]
			if n := len(line); n > 0 && line[n-1] == '\r' {
				line = line[:n-1]
			}
			return line, rest, true
		}
	}
	if n := len(s); n > 0 && s[n-1] == '\r' {
		s = s[:n-1]
	}
	return s, "", false
}

// AttrValue is the value of the attribute with the key, or empty.
func AttrValue(attrs []Attr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

// hasCompartments reports whether a live vertex is of a kind with
// compartments, whose size its attributes then set.
func (d *Document) hasCompartments() bool {
	for i, k := range d.kinds.vertex {
		if len(k.Compartments) > 0 && d.kindCount[i] > 0 {
			return true
		}
	}
	return false
}

// holdsCompartments reports whether a vertex's kind has compartments.
func (d *Document) holdsCompartments(v VertexIndex) bool {
	return len(d.kinds.vertex[d.vertices[v].Kind].Compartments) > 0
}

// refits reports whether a command, applied or undone, can change what
// the fit sizes: a move, a vertex's attributes when compartments show
// them, or a container's ports.
func (d *Document) refits(op Op) bool {
	if op == SetPorts || op == AddPort || op == DeletePort {
		// A container's ports are on its fitted border.
		return d.hasContainers()
	}
	if op == SetAttr || op == AddVertex {
		return d.hasCompartments()
	}
	return moves(op)
}

// fitted is the size of a vertex that holds compartments or ports of
// its own: its own size, grown to its ports' room (portRoom), then to
// the label's room and every compartment with a value, as wide as the
// widest line up to the kind's MaxWidth.
func (d *Document) fitted(i VertexIndex, own Size) Size {
	v := &d.vertices[i]
	kind := &d.kinds.vertex[v.Kind]
	h, w, shown := TitleRoom(VertexKindView{kind}), 0.0, false
	for _, key := range kind.Compartments {
		val := AttrValue(v.Attrs, key)
		if val == "" {
			continue
		}
		shown = true
		lines := 0
		// bound: the value's lines, at most its length
		for more := true; more; {
			var line string
			line, val, more = NextLine(val)
			lines++
			w = max(w, float64(float64(runes(line))*CharWidth)+2*CompartmentInset)
		}
		h += float64(float64(lines)*CompartmentLine) + 2*CompartmentPad
	}
	room := d.portRoom(i)
	own = Size{W: max(own.W, room.W), H: max(own.H, room.H)}
	if !shown {
		return own
	}
	limit := max(kind.MaxWidth, own.W)
	return Size{W: max(own.W, min(w, limit)), H: max(own.H, h)}
}

// fitContent sizes a vertex to its compartments and ports about its
// centre, which is where a document saves it, and names it as resized
// when that changes its size.
func (d *Document) fitContent(i VertexIndex) {
	v := &d.vertices[i]
	size := d.fitted(i, d.ownSize(i))
	if size == v.Size {
		return
	}
	v.Size = size
	d.resized = true
	d.touchVertex(i)
	d.touchEdgesAt(i)
}

// runes counts the characters of a string.
func runes(s string) int {
	n := 0
	// bound: the string
	for i := 0; i < len(s); i++ {
		if s[i]&0xC0 != 0x80 {
			n++
		}
	}
	return n
}

// ShowsCompartments reports whether a vertex draws any compartment:
// whether its kind has them and one has a value. Its label is then at
// its top, above them.
func (d *Document) ShowsCompartments(v VertexIndex) bool {
	x := &d.vertices[v]
	for _, key := range d.kinds.vertex[x.Kind].Compartments {
		if AttrValue(x.Attrs, key) != "" {
			return true
		}
	}
	return false
}
