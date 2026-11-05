package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// nine maps a provided shape's box onto a vertex's size in nine slices,
// in the vertex's own coordinates, centred on the origin: a point within
// a margin keeps its distance from that side, one between the margins
// scales across the middle. Margins wider than the vertex shrink in
// proportion, leaving no middle. Turned, the box is transposed first: x
// and y swap, and with them the margins, left with top and right with
// bottom.
type nine struct {
	bw, bh         float64 // the box, as drawn
	l0, t0, r0, b0 float64 // the margins in the box
	l, t, r, b     float64 // the margins on the vertex
	w, h           float64 // the vertex
	turned         bool
}

func nineOf(s gw.ShapeView, size gw.Size, turned bool) nine {
	box, m := s.Box(), s.Margins()
	if turned {
		box.W, box.H = box.H, box.W
		m = [4]float64{m[1], m[0], m[3], m[2]}
	}
	n := nine{bw: box.W, bh: box.H, l0: m[0], t0: m[1], r0: m[2], b0: m[3], w: size.W, h: size.H, turned: turned}
	n.l, n.r = fitMargins(m[0], m[2], size.W)
	n.t, n.b = fitMargins(m[1], m[3], size.H)
	return n
}

// turned reports whether a vertex is drawn turned: its kind's provided
// shape turns, and the flow is top to bottom.
func turned(d *gw.Document, v gw.VertexView) bool {
	return d.Flow() == gw.TopToBottom && d.Kinds().ShapeOf(d.Kinds().Vertex(v.Kind()).Shape()).Turns()
}

// fitMargins are two margins on a side of length n: as they are, or
// shrunk in proportion to fill it when they would not fit.
func fitMargins(a, b, n float64) (float64, float64) {
	if a+b <= n || a+b == 0 {
		return a, b
	}
	k := n / (a + b)
	return float64(a * k), float64(b * k)
}

// slice maps a coordinate x in [0, box] with margins a and b, scaled to
// sa and sb, onto a side of length n centred on 0, and gives the scale
// of the slice it is in.
func slice(x, box, a, b, sa, sb, n float64) (float64, float64) {
	switch {
	case x <= a && a > 0:
		k := sa / a
		return float64(-n/2) + float64(x*k), k
	case x >= box-b && b > 0:
		k := sb / b
		return float64(n/2) - float64((box-x)*k), k
	}
	mid := box - a - b
	if mid <= 0 {
		return float64(-n/2) + sa, 1
	}
	k := (n - sa - sb) / mid
	return float64(-n/2) + sa + float64((x-a)*k), k
}

// x and y map a coordinate of the box, with the scale of its slice.
func (n nine) x(x float64) (float64, float64) { return slice(x, n.bw, n.l0, n.r0, n.l, n.r, n.w) }
func (n nine) y(y float64) (float64, float64) { return slice(y, n.bh, n.t0, n.b0, n.t, n.b, n.h) }

// pt maps a point of the shape's own box, transposed when turned.
func (n nine) pt(x, y float64) (float64, float64) {
	if n.turned {
		x, y = y, x
	}
	px, _ := n.x(x)
	py, _ := n.y(y)
	return px, py
}

// provided writes a provided shape: with the shape's class, its group
// and paths; with the overlay's ring, its outline.
func (r *Renderer) provided(class string, s gw.ShapeView, size gw.Size, turned bool) {
	o := r.o
	if class != shapeClass {
		r.outline(class, s, size, turned)
		return
	}
	n := nineOf(s, size, turned)
	o.str(`<g class="shape shape-`)
	o.str(s.Name()) // a word of lowercase letters, digits and '-', as the schema checked it
	o.str(`">`)
	for i := range s.Paths() {
		p := s.Path(i)
		o.str(`<path class="`)
		if p.Fill() {
			o.str(`mark`)
		} else {
			o.str(`line`)
		}
		o.str(`" d="`)
		for j := range p.Len() {
			if j > 0 {
				o.byte(' ')
			}
			n.command(o, p.At(j))
		}
		o.str(`"/>`)
	}
	o.str(`</g>`)
}

// command writes one command of a path, its numbers scaled. An arc's
// radii scale as the slices its end point is in. Turned, a horizontal
// line is vertical and the other way round, an arc's radii swap, and
// its sweep, mirrored, flips.
func (n nine) command(o *out, c gw.PathCommand) {
	a := c.Args
	op := c.Op
	if n.turned && op == gw.HLineTo {
		op = gw.VLineTo
	} else if n.turned && op == gw.VLineTo {
		op = gw.HLineTo
	}
	switch op {
	case gw.MoveTo, gw.LineTo:
		o.str(opLetter(c.Op))
		x, y := n.pt(a[0], a[1])
		o.point(x, y)
	case gw.HLineTo:
		o.str(`H `)
		x, _ := n.x(a[0])
		o.num(x)
	case gw.VLineTo:
		o.str(`V `)
		y, _ := n.y(a[0])
		o.num(y)
	case gw.CubicTo, gw.QuadTo:
		o.str(opLetter(c.Op))
		x, y := n.pt(a[0], a[1])
		o.point(x, y)
		o.byte(' ')
		x, y = n.pt(a[2], a[3])
		o.point(x, y)
		if c.Op == gw.CubicTo {
			o.byte(' ')
			x, y = n.pt(a[4], a[5])
			o.point(x, y)
		}
	case gw.ArcTo:
		rx, ry, ex, ey, sweep := a[0], a[1], a[5], a[6], a[4]
		if n.turned {
			rx, ry, ex, ey, sweep = ry, rx, ey, ex, 1-sweep
		}
		x, kx := n.x(ex)
		y, ky := n.y(ey)
		o.str(`A `)
		o.point(float64(rx*kx), float64(ry*ky))
		o.str(` 0 `)
		o.num(a[3])
		o.byte(' ')
		o.num(sweep)
		o.byte(' ')
		o.point(x, y)
	case gw.Close:
		o.byte('Z')
	}
}

// opLetter is a command's letter and the space after it.
func opLetter(op gw.PathOp) string {
	switch op {
	case gw.MoveTo:
		return "M "
	case gw.LineTo:
		return "L "
	case gw.CubicTo:
		return "C "
	case gw.QuadTo:
		return "Q "
	case gw.HLineTo, gw.VLineTo, gw.ArcTo, gw.Close:
	}
	return ""
}

// outline writes the outline of a provided shape, for the overlay's
// ring: its box, diamond, ellipse or polygon.
func (r *Renderer) outline(class string, s gw.ShapeView, size gw.Size, turned bool) {
	if s.Outline() != gw.OutlinePolygon {
		r.builtin(class, builtinOutline(s.Outline()), size)
		return
	}
	o := r.o
	n := nineOf(s, size, turned)
	poly := s.Polygon()
	o.str(`<polygon class="`)
	o.str(class)
	o.str(`" points="`)
	for i := range poly.Len() {
		if i > 0 {
			o.byte(' ')
		}
		x, y := n.pt(poly.At(i).X, poly.At(i).Y)
		o.point(x, y)
	}
	o.str(`"/>`)
}

// builtinOutline is the built-in shape whose outline a provided
// outline other than a polygon is.
func builtinOutline(ol gw.Outline) gw.Shape {
	switch ol {
	case gw.OutlineDiamond:
		return gw.Diamond
	case gw.OutlineEllipse:
		return gw.Circle
	case gw.OutlineBox, gw.OutlinePolygon:
	}
	return gw.Rect
}

// outlineOf is what a vertex's edges are clipped to: a built-in shape's
// outline, or, for a provided shape, the built-in its outline is, or its
// polygon when ok is false for a built-in.
func outlineOf(d *gw.Document, v gw.VertexView) (gw.Shape, gw.ShapeView) {
	shape := d.Kinds().Vertex(v.Kind()).Shape()
	if shape < gw.Provided {
		return shape, gw.ShapeView{}
	}
	s := d.Kinds().ShapeOf(shape)
	if s.Outline() == gw.OutlinePolygon {
		return gw.Provided, s
	}
	return builtinOutline(s.Outline()), s
}

// polygon is a provided shape's outline polygon on a vertex, in
// document coordinates, in the array, and how many points it has.
func polygon(d *gw.Document, s gw.ShapeView, v gw.VertexView, ps *[gw.MaxPolygonPoints]gw.Point) int {
	n := nineOf(s, v.Size(), turned(d, v))
	poly := s.Polygon()
	k := min(poly.Len(), len(ps))
	for i := range k {
		x, y := n.pt(poly.At(i).X, poly.At(i).Y)
		ps[i] = gw.Point{X: v.Pos().X + x, Y: v.Pos().Y + y}
	}
	return k
}

// vertexOutlineDistance is how far p lies from the vertex's outline,
// negative inside.
func vertexOutlineDistance(d *gw.Document, v gw.VertexView, p gw.Point) float64 {
	shape, s := outlineOf(d, v)
	if shape != gw.Provided {
		return outlineDistance(shape, v, p)
	}
	var ps [gw.MaxPolygonPoints]gw.Point
	n := polygon(d, s, v, &ps)
	dist, inside := math.Inf(1), true
	for i := range n {
		a, b := ps[i], ps[(i+1)%n]
		dist = math.Min(dist, segmentDistance(p, a, b))
		if (cross(a, b, p) < 0) != (orientation(ps[:n]) < 0) {
			inside = false
		}
	}
	if inside {
		return -dist
	}
	return dist
}

// cross is the cross product of ab and ap: positive when p is to the
// left of ab, going round anticlockwise in y-down coordinates.
func cross(a, b, p gw.Point) float64 {
	return float64((b.X-a.X)*(p.Y-a.Y)) - float64((b.Y-a.Y)*(p.X-a.X))
}

// orientation is the sign of a convex polygon's turn, from its first
// three points.
func orientation(ps []gw.Point) float64 { return cross(ps[0], ps[1], ps[2]) }

// vertexClip is where the line from the vertex's centre towards p
// leaves its outline: the last crossing of its polygon, for a provided
// shape with one.
func vertexClip(d *gw.Document, v gw.VertexView, p gw.Point) gw.Point {
	shape, s := outlineOf(d, v)
	if shape != gw.Provided {
		return clip(shape, v, p)
	}
	var ps [gw.MaxPolygonPoints]gw.Point
	n := polygon(d, s, v, &ps)
	c := v.Pos()
	ux, uy := p.X-c.X, p.Y-c.Y
	best := 0.0
	for i := range n {
		a, b := ps[i], ps[(i+1)%n]
		ex, ey := b.X-a.X, b.Y-a.Y
		den := float64(ux*ey) - float64(uy*ex)
		if den == 0 {
			continue
		}
		t := (float64((a.X-c.X)*ey) - float64((a.Y-c.Y)*ex)) / den
		s := (float64((a.X-c.X)*uy) - float64((a.Y-c.Y)*ux)) / den
		if s >= 0 && s <= 1 && t > best {
			best = t
		}
	}
	if best == 0 {
		return c
	}
	return gw.Point{X: c.X + float64(ux*best), Y: c.Y + float64(uy*best)}
}

// inVertex reports whether p hits the vertex's body: its box, or for a
// provided shape, inside its outline.
func inVertex(d *gw.Document, v gw.VertexView, p gw.Point) bool {
	if d.Kinds().Vertex(v.Kind()).Shape() < gw.Provided {
		return inBox(v, p)
	}
	return inBox(v, p) && vertexOutlineDistance(d, v, p) < 0
}

// PortOn is the absolute position of port p of vertex v, as drawn: on
// its box's side, or for a provided shape, where its outline meets the
// line in from there.
func PortOn(d *gw.Document, v gw.VertexIndex, p gw.PortIndex) gw.Point { return portOn(d, v, p) }

// outlineSteps bisect the line in from a port's place on the box to
// the outline: the box's size over 2^40 at most.
const outlineSteps = 40

func portOn(d *gw.Document, i gw.VertexIndex, p gw.PortIndex) gw.Point {
	v := d.Vertex(i)
	turn := turned(d, v)
	q := portAt(v, p, turn)
	if d.Kinds().Vertex(v.Kind()).Shape() < gw.Provided || vertexOutlineDistance(d, v, q) <= 0 {
		return q
	}
	// The point across the vertex from the port, on the line in: inside
	// unless the line misses the shape, when the port stays on the box.
	in := q
	switch turnSide(v.Ports().At(int(p)).Side, turn) {
	case gw.Left, gw.Right:
		in.X = v.Pos().X
	case gw.Top, gw.Bottom:
		in.Y = v.Pos().Y
	}
	return toOutline(d, v, q, in)
}

// labelSlot is where a vertex's label goes on its shape: the centre's
// offset and the width it is cut at, and whether it is below, as an
// actor's, or not drawn.
func labelSlot(d *gw.Document, v gw.VertexView) (x, y, w float64, below, none bool) {
	kind := d.Kinds().Vertex(v.Kind())
	shape := kind.Shape()
	switch {
	case shape == gw.Actor:
		return 0, 0, 0, true, false
	case shape < gw.Provided:
		return 0, 0, v.Size().W - 8, false, false
	}
	s := d.Kinds().ShapeOf(shape)
	switch s.Label() {
	case gw.LabelBelow:
		return 0, 0, 0, true, false
	case gw.LabelNone:
		return 0, 0, 0, false, true
	case gw.LabelInside:
	}
	n := nineOf(s, v.Size(), turned(d, v))
	return float64((n.l - n.r) / 2), float64((n.t - n.b) / 2), math.Max(0, v.Size().W-n.l-n.r-8), false, false
}

// labelBelow reports whether a vertex's label is drawn at its foot, as
// an actor's is, as wide as it needs.
func labelBelow(d *gw.Document, v gw.VertexView) bool {
	return d.Kinds().LabelBelow(v.Kind())
}
