package doc

import (
	"io"
	"strconv"

	gw "code.eitanity.com/graphedwright"
)

// Encode writes the document's live content as JSON under the schema
// name, in index order, so that the same document gives the same text.
func Encode(d *gw.Document, schema string, w io.Writer) error {
	e := &encoder{w: w, buf: make([]byte, 0, 4096)}
	e.str(`{"schema":`)
	e.quote(schema)
	if d.Flow != gw.LeftToRight {
		e.field("flow")
		e.quote(d.Flow.String())
	}
	e.str(`,"vertices":[`)
	first := true
	for i := range d.Vertices {
		if v := &d.Vertices[i]; v.Live {
			if !first {
				e.byte(',')
			}
			first = false
			e.vertex(d, v)
		}
	}
	e.str(`],"edges":[`)
	first = true
	for i := range d.Edges {
		if ed := &d.Edges[i]; ed.Live {
			if !first {
				e.byte(',')
			}
			first = false
			e.edge(d, ed)
		}
	}
	e.str(`],"view":{"x":`)
	e.num(d.View.X)
	e.str(`,"y":`)
	e.num(d.View.Y)
	e.str(`,"w":`)
	e.num(d.View.W)
	e.str(`,"h":`)
	e.num(d.View.H)
	e.str("}}\n")
	return e.flush()
}

// encoder buffers the text and writes it through.
type encoder struct {
	w   io.Writer
	buf []byte
	err error
}

func (e *encoder) flush() error {
	if e.err == nil && len(e.buf) > 0 {
		_, e.err = e.w.Write(e.buf)
	}
	e.buf = e.buf[:0]
	return e.err
}

func (e *encoder) byte(c byte) {
	if len(e.buf) == cap(e.buf) {
		_ = e.flush() //nolint:errcheck // kept in e.err and returned by Encode
	}
	e.buf = append(e.buf, c)
}

func (e *encoder) str(s string) {
	for i := 0; i < len(s); i++ {
		e.byte(s[i])
	}
}

// quote writes a JSON string.
func (e *encoder) quote(s string) {
	e.byte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			e.str(`\"`)
		case c == '\\':
			e.str(`\\`)
		case c == '\n':
			e.str(`\n`)
		case c == '\r':
			e.str(`\r`)
		case c == '\t':
			e.str(`\t`)
		case c < 0x20:
			e.str(`\u00`)
			e.byte("0123456789abcdef"[c>>4])
			e.byte("0123456789abcdef"[c&0xf])
		default:
			e.byte(c)
		}
	}
	e.byte('"')
}

// num writes a number in the shortest form that reads back the same.
func (e *encoder) num(x float64) {
	var tmp [32]byte
	for _, c := range strconv.AppendFloat(tmp[:0], x, 'g', -1, 64) {
		e.byte(c)
	}
}

func (e *encoder) field(name string) {
	e.byte(',')
	e.quote(name)
	e.byte(':')
}

func (e *encoder) vertex(d *gw.Document, v *gw.Vertex) {
	e.str(`{"id":`)
	e.quote(string(v.ID))
	e.field("kind")
	e.quote(d.Schema.VertexKinds[v.Kind].Name)
	if v.Label != "" {
		e.field("label")
		e.quote(v.Label)
	}
	e.field("x")
	e.num(v.Pos.X)
	e.field("y")
	e.num(v.Pos.Y)
	// A container's size is its own, which its fit grows from; the
	// size it is drawn at follows from what it holds.
	size, kind := v.Size, d.Schema.VertexKinds[v.Kind]
	if kind.Container {
		size = v.Min
	}
	if size != kind.Size && size != (gw.Size{}) {
		e.field("w")
		e.num(size.W)
		e.field("h")
		e.num(size.H)
	}
	if v.Placed {
		e.str(`,"placed":true`)
	}
	if v.Parent != gw.None {
		e.field("parent")
		e.quote(string(d.Vertices[v.Parent].ID))
	}
	e.attrs(v.Attrs)
	e.byte('}')
}

func (e *encoder) edge(d *gw.Document, ed *gw.Edge) {
	e.str(`{"id":`)
	e.quote(string(ed.ID))
	e.field("kind")
	e.quote(d.Schema.EdgeKinds[ed.Kind].Name)
	e.field("from")
	e.quote(string(d.Vertices[ed.From].ID))
	e.field("to")
	e.quote(string(d.Vertices[ed.To].ID))
	if ed.FromPort != gw.None {
		e.field("from_port")
		e.quote(d.Vertices[ed.From].Ports[ed.FromPort].Name)
	}
	if ed.ToPort != gw.None {
		e.field("to_port")
		e.quote(d.Vertices[ed.To].Ports[ed.ToPort].Name)
	}
	if ed.Label != "" {
		e.field("label")
		e.quote(ed.Label)
	}
	if ed.Weight != 0 {
		e.field("weight")
		e.num(ed.Weight)
	}
	if len(ed.Bends) > 0 {
		e.field("bends")
		e.byte('[')
		for i, p := range ed.Bends {
			if i > 0 {
				e.byte(',')
			}
			e.byte('[')
			e.num(p.X)
			e.byte(',')
			e.num(p.Y)
			e.byte(']')
		}
		e.byte(']')
	}
	e.attrs(ed.Attrs)
	e.byte('}')
}

func (e *encoder) attrs(attrs []gw.Attr) {
	if len(attrs) == 0 {
		return
	}
	e.str(`,"attrs":{`)
	for i, a := range attrs {
		if i > 0 {
			e.byte(',')
		}
		e.quote(a.Key)
		e.byte(':')
		e.quote(a.Value)
	}
	e.byte('}')
}
