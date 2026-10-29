package graphedwright

import "strconv"

// setAttr sets or removes an attribute on a vertex, or on an edge when
// Vertex is None, as the kind's specification allows.
func (d *Document) setAttr(c Command) error {
	if c.Key == "" {
		return refusal("attribute key must not be empty")
	}
	if len(c.Value) > d.caps.LabelBytes {
		return refusal("attribute value of " + itoa(len(c.Value)) + " bytes exceeds the limit of " + d.limitLabel())
	}
	if len(c.Key) > d.caps.LabelBytes {
		return refusal("attribute key of " + itoa(len(c.Key)) + " bytes exceeds the limit of " + d.limitLabel())
	}
	if err := checkKey(c.Key); err != nil {
		return err
	}
	if err := checkText("attribute value", c.Value); err != nil {
		return err
	}
	if c.Vertex != None {
		if !d.vertexLive(c.Vertex) {
			return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
		}
		v := &d.Vertices[c.Vertex]
		if err := d.checkAttr(c, d.Schema.VertexKinds[v.Kind].Attrs); err != nil {
			return err
		}
		attrs, en, err := d.writeAttr(v.Attrs, c)
		if err != nil {
			return err
		}
		en.vertex, en.edge = c.Vertex, None
		v.Attrs = attrs
		d.seq++
		en.seq = d.seq
		d.log.push(en)
		d.touchVertex(c.Vertex)
		return nil
	}
	if !d.edgeLive(c.Edge) {
		return refusal("edge " + itoa(int(c.Edge)) + " is not live")
	}
	e := &d.Edges[c.Edge]
	if err := d.checkAttr(c, d.Schema.EdgeKinds[e.Kind].Attrs); err != nil {
		return err
	}
	attrs, en, err := d.writeAttr(e.Attrs, c)
	if err != nil {
		return err
	}
	en.vertex, en.edge = None, c.Edge
	e.Attrs = attrs
	d.seq++
	en.seq = d.seq
	d.log.push(en)
	d.touchEdge(c.Edge)
	return nil
}

func (d *Document) limitLabel() string { return itoa(d.caps.LabelBytes) }

// attrWhere names the element a SetAttr command addresses, for a
// refusal; it is built only when one is needed.
func (d *Document) attrWhere(c Command) string {
	if c.Vertex != None {
		return "vertex " + d.vertexName(c.Vertex)
	}
	return "edge " + strconv.Quote(string(d.Edges[c.Edge].ID))
}

// checkAttr applies the kind's specification to a value: a number must
// be one, a choice one of the choices, and a required attribute cannot
// be removed.
func (d *Document) checkAttr(c Command, specs []AttrSpec) error {
	s, ok := spec(specs, c.Key)
	if !ok {
		return nil
	}
	if c.Value == "" {
		if s.Required {
			return refusal(d.attrName(c) + " is required")
		}
		return nil
	}
	if !s.accepts(c.Value) {
		switch s.Kind {
		case Number:
			return refusal(d.attrName(c) + " must be a number: " + strconv.Quote(c.Value))
		case Choice:
			return refusal(d.attrName(c) + " must be one of " + joinChoices(s.Choices))
		case Text:
		}
	}
	return nil
}

func (d *Document) attrName(c Command) string {
	return "attribute " + strconv.Quote(c.Key) + " of " + d.attrWhere(c)
}

// joinChoices lists choices for a refusal.
func joinChoices(choices []string) string {
	out := ""
	for i, c := range choices {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// writeAttr sets, adds or removes the attribute in the element's slice
// and returns the new slice with the log entry recording what was
// there. An empty value removes.
func (d *Document) writeAttr(attrs []Attr, c Command) ([]Attr, entry, error) {
	en := entry{cmd: c}
	at := -1
	for i := range attrs {
		if attrs[i].Key == c.Key {
			at = i
			break
		}
	}
	if at >= 0 {
		en.hadAttr, en.at = true, at
		en.prev = Command{Value: attrs[at].Value}
		if c.Value == "" {
			copy(attrs[at:], attrs[at+1:])
			return attrs[:len(attrs)-1], en, nil
		}
		attrs[at].Value = c.Value
		return attrs, en, nil
	}
	if c.Value == "" {
		return attrs, en, nil
	}
	if len(attrs) == d.caps.AttrsPerElement {
		return attrs, en, refusal("attribute limit " + itoa(d.caps.AttrsPerElement) + " reached on " + d.attrWhere(c))
	}
	en.at = len(attrs)
	attrs = attrs[:len(attrs)+1]
	attrs[len(attrs)-1] = Attr{Key: c.Key, Value: c.Value}
	return attrs, en, nil
}

// undoAttr puts an attribute back as the entry recorded it.
func (d *Document) undoAttr(en entry) {
	var attrs *[]Attr
	if en.vertex != None {
		attrs = &d.Vertices[en.vertex].Attrs
		d.touchVertex(en.vertex)
	} else {
		attrs = &d.Edges[en.edge].Attrs
		d.touchEdge(en.edge)
	}
	switch {
	case en.hadAttr && en.cmd.Value != "":
		(*attrs)[en.at].Value = en.prev.Value
	case en.hadAttr:
		*attrs = (*attrs)[:len(*attrs)+1]
		copy((*attrs)[en.at+1:], (*attrs)[en.at:])
		(*attrs)[en.at] = Attr{Key: en.cmd.Key, Value: en.prev.Value}
	case en.cmd.Value != "":
		copy((*attrs)[en.at:], (*attrs)[en.at+1:])
		*attrs = (*attrs)[:len(*attrs)-1]
	}
}

// redoAttr applies the entry's command again without logging.
func (d *Document) redoAttr(en entry) {
	var attrs *[]Attr
	if en.vertex != None {
		attrs = &d.Vertices[en.vertex].Attrs
		d.touchVertex(en.vertex)
	} else {
		attrs = &d.Edges[en.edge].Attrs
		d.touchEdge(en.edge)
	}
	next, _, _ := d.writeAttr(*attrs, en.cmd)
	*attrs = next
}
