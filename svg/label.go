package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// The boxes the labels take, at the default character width and font
// size, from the same positions the renderer draws them at: where a
// host puts the input that edits a label, and where the hit test takes
// an edge by its label.

// labelHeight is the height of a label's box: the font size and room
// either side.
const labelHeight = defaultFontSize + 8

// defaultFontSize is a label's size when the options give none.
const defaultFontSize = 12

// minLabelWidth is the least width of a label's box, so that a short or
// empty label still has room to type in.
const minLabelWidth = 80

// VertexLabelBox is the box of a vertex's label: across the vertex at
// its label's height, at an actor's foot as wide as its text, or a
// container's title at its top.
func VertexLabelBox(d *gw.Document, v gw.VertexIndex) gw.Viewport {
	x := &d.Vertices[v]
	kind := d.Schema.VertexKinds[x.Kind]
	textW := float64(float64(d.VertexLabelRunes(v)) * defaultCharWidth)
	switch {
	case kind.Container:
		top := x.Pos.Y - float64(x.Size.H/2)
		return gw.Viewport{X: x.Pos.X - float64(x.Size.W/2) + 4, Y: top + gw.ContainerTitle/2 - labelHeight/2 + 1, W: math.Max(x.Size.W-8, minLabelWidth), H: labelHeight}
	case d.ShowsCompartments(v):
		top := x.Pos.Y - float64(x.Size.H/2)
		return centred(x.Pos.X, top+gw.TitleRoom(&kind)-8-defaultFontSize/3, math.Max(x.Size.W, minLabelWidth))
	case kind.Shape == gw.Actor:
		w := math.Max(textW+16, minLabelWidth)
		return centred(x.Pos.X, x.Pos.Y+float64(x.Size.H/2)-4-defaultFontSize/3, w)
	}
	return centred(x.Pos.X, x.Pos.Y+4-defaultFontSize/3, math.Max(x.Size.W, minLabelWidth))
}

// EdgeLabelBox is the box of an edge's label, where it is drawn or
// would be when it has none: above the middle of its route, or to the
// right of it in a top-to-bottom flow, as wide as its text; below or to
// the left for a lane on that side of the others.
func EdgeLabelBox(d *gw.Document, i gw.EdgeIndex) gw.Viewport {
	b, _ := edgeLabelBox(d, i)
	return b
}

// edgeLabelBox is EdgeLabelBox and the side of its lane it is on.
func edgeLabelBox(d *gw.Document, i gw.EdgeIndex) (gw.Viewport, float64) {
	e := &d.Edges[i]
	kind := d.Schema.EdgeKinds[e.Kind]
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	f := wy.f
	mid := wy.middle(kind.Route)
	side := wy.side()
	// Wide enough for the label as drawn and as typed in.
	w := math.Max(float64(float64(max(d.EdgeLabelRunes(i), runeCount(e.Label)))*defaultCharWidth)+16, minLabelWidth)
	if f.down {
		if side < 0 {
			return gw.Viewport{X: mid.X - 2 - w, Y: mid.Y - labelHeight/2, W: w, H: labelHeight}, side
		}
		return gw.Viewport{X: mid.X + 2, Y: mid.Y - labelHeight/2, W: w, H: labelHeight}, side
	}
	base, step := mid.Y-defaultStroke/2-4, -float64(defaultFontSize)
	if side < 0 {
		base, step = mid.Y+defaultStroke/2+defaultFontSize, defaultFontSize
	}
	if kind.Stereotype != "" {
		base += step
	}
	return centred(mid.X, base-defaultFontSize/3, w), side
}

// defaultStroke is the stroke width of an unweighted edge.
const defaultStroke = 1.5

// centred is a label box of the width centred on (x, y).
func centred(x, y, w float64) gw.Viewport {
	return gw.Viewport{X: x - float64(w/2), Y: y - labelHeight/2, W: w, H: labelHeight}
}

// inLabel reports whether p is in the box of an edge's label, when it
// has one.
func inLabel(d *gw.Document, i gw.EdgeIndex, p gw.Point) bool {
	drawn := d.EdgeLabelRunes(i)
	if drawn == 0 {
		return false
	}
	b, side := edgeLabelBox(d, i)
	// The box for typing in is wider than the text; the text's own
	// width is what is drawn and hit.
	w := float64(float64(drawn)*defaultCharWidth) + 8
	switch {
	case d.Flow != gw.TopToBottom:
		b.X, b.W = b.X+float64(b.W/2)-float64(w/2), w
	case side < 0:
		b.X, b.W = b.X+b.W-w, w
	default:
		b.W = w
	}
	return p.X >= b.X && p.X <= b.X+b.W && p.Y >= b.Y && p.Y <= b.Y+b.H
}

// anchorEnd anchors a text at its end.
const anchorEnd = "end"

// portLabel is where a port's label is drawn, outside the shape beside
// the port: on a left or right side above the line an edge leaves along,
// outwards, or below it when an edge at the port rises under the label;
// on the top or bottom to the right of the port, or to its left when an
// edge runs right beside the label, above or below the shape. The box
// is the text's estimated extent.
func portLabel(d *gw.Document, i gw.VertexIndex, p gw.PortIndex) (x, y float64, anchor string, box gw.Viewport) {
	v := &d.Vertices[i]
	at := portAt(v, p)
	w := float64(float64(runeCount(v.Ports[p].Name)) * gw.PortLabelChar)
	side := v.Ports[p].Side
	// An edge is followed as far as the label reaches from the port.
	reach := w + portRadius + 4
	if side == gw.Top || side == gw.Bottom {
		reach = portRadius + 15
	}
	up, right := portHeads(d, i, p, at, reach)
	x, y, anchor = at.X+portRadius+2, at.Y-portRadius-2, "start"
	if side == gw.Left {
		x, anchor = at.X-portRadius-2, anchorEnd
	}
	switch side {
	case gw.Left, gw.Right:
		if up {
			y = at.Y + portRadius + 11
		}
	case gw.Top, gw.Bottom:
		if side == gw.Bottom {
			y = at.Y + portRadius + 11
		}
		if right {
			x, anchor = at.X-portRadius-2, anchorEnd
		}
	}
	box = gw.Viewport{X: x, Y: y - 9, W: w, H: 11}
	if anchor == anchorEnd {
		box.X -= w
	}
	return x, y, anchor, box
}

// portHeads reports whether an edge at a port rises above it, and
// whether one runs right of it, within reach of it as drawn.
func portHeads(d *gw.Document, v gw.VertexIndex, p gw.PortIndex, at gw.Point, reach float64) (up, right bool) {
	for k := range d.Edges {
		e := &d.Edges[k]
		from := e.From == v && e.FromPort == p
		if !e.Live || (!from && (e.To != v || e.ToPort != p)) {
			continue
		}
		u, r := edgeHeads(d, gw.EdgeIndex(k), from, at, reach) //nolint:gosec // bounded by the capacity
		up, right = up || u, right || r
	}
	return up, right
}

// edgeHeads reports whether an edge's route as drawn, followed leg by
// leg from its start or from its end until it is reach from the port,
// rises above the port or runs right of it on the way.
func edgeHeads(d *gw.Document, i gw.EdgeIndex, from bool, at gw.Point, reach float64) (up, right bool) {
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	f := wy.f
	route := d.Schema.EdgeKinds[d.Edges[i].Kind].Route
	legs := len(wy.through) + 1
	// bound: the legs, at most the bends per edge and one
	for n := range legs {
		k := n
		if !from {
			k = legs - 1 - n
		}
		u, w := leg(wy.a, wy.through, wy.b, k)
		u, w = f.t(u), f.t(w)
		r := legRoute(route, legs > 1, u, w)
		// bound: edgeSamples
		for s := 1; s <= edgeSamples; s++ {
			t := float64(s) / edgeSamples
			if !from {
				t = float64(edgeSamples-s) / edgeSamples
			}
			q := f.t(f.at(r, u, w, t))
			up, right = up || q.Y < at.Y-1, right || q.X > at.X+1
			if math.Hypot(q.X-at.X, q.Y-at.Y) >= reach {
				return up, right
			}
		}
	}
	return up, right
}
