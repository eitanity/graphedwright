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

// belowGap is how far under a provided shape's box the baseline of a
// label drawn below it is: a line of text and a little room.
const belowGap = defaultFontSize + 2

// minLabelWidth is the least width of a label's box, so that a short or
// empty label still has room to type in.
const minLabelWidth = 80

// VertexLabelBox is the box of a vertex's label: across the vertex at
// its label's height, at an actor's foot as wide as its text, or a
// container's title at its top; a line taller for each line of the
// label after its first, about where the renderer draws them.
func VertexLabelBox(d *gw.Document, v gw.VertexIndex) gw.Viewport {
	x := d.Vertex(v)
	kind := d.Kinds().Vertex(x.Kind())
	textW := float64(float64(d.VertexLabelRunes(v)) * defaultCharWidth)
	more := gw.MoreLines(d.VertexLabelLines(v))
	switch {
	case kind.Container():
		top := x.Pos().Y - float64(x.Size().H/2)
		return gw.Viewport{X: x.Pos().X - float64(x.Size().W/2) + 4, Y: top + gw.ContainerTitle/2 - labelHeight/2 + 1, W: math.Max(x.Size().W-8, minLabelWidth), H: labelHeight + more}
	case d.ShowsCompartments(v):
		top := x.Pos().Y - float64(x.Size().H/2)
		return centred(x.Pos().X, top+d.TitleHeight(v)-8-float64(more/2)-defaultFontSize/3, math.Max(x.Size().W, minLabelWidth), more)
	case labelBelow(d, x):
		// At an actor's foot its lines go up, under a provided shape's
		// box down.
		w := math.Max(textW+16, minLabelWidth)
		foot := float64(x.Size().H/2) - 4 - float64(more/2)
		if kind.Shape() >= gw.Provided {
			foot = float64(x.Size().H/2) + belowGap + float64(more/2)
		}
		return centred(x.Pos().X, x.Pos().Y+foot-defaultFontSize/3, w, more)
	}
	if kind.Shape() >= gw.Provided {
		// Between a provided shape's margins.
		lx, ly, lw, _, _ := labelSlot(d, x)
		return centred(x.Pos().X+lx, x.Pos().Y+ly+4-defaultFontSize/3, math.Max(lw+8, minLabelWidth), more)
	}
	return centred(x.Pos().X, x.Pos().Y+4-defaultFontSize/3, math.Max(x.Size().W, minLabelWidth), more)
}

// EdgeLabelBox is the box of an edge's label, where it is drawn or
// would be when it has none: above the middle of its route, or to the
// right of it in a top-to-bottom flow, as wide as its text; below or to
// the left for a lane on that side of the others. Top to bottom, a label
// on a run through the layers, straight along the flow and long enough
// for it, is turned to read along the run, and its box stands beside
// the run as tall as the text is wide.
func EdgeLabelBox(d *gw.Document, i gw.EdgeIndex) gw.Viewport {
	b, _, _ := edgeLabelBox(d, i)
	return b
}

// EdgeLabelInputBox is where to put an input to edit an edge's label:
// its box (EdgeLabelBox), or, where the label is turned to read along
// its run, that box level about the same centre, as text is typed level.
func EdgeLabelInputBox(d *gw.Document, i gw.EdgeIndex) gw.Viewport {
	b, _, turned := edgeLabelBox(d, i)
	if !turned {
		return b
	}
	return gw.Viewport{X: b.X + float64(b.W/2) - float64(b.H/2), Y: b.Y + float64(b.H/2) - float64(b.W/2), W: b.H, H: b.W}
}

// edgeLabelBox is EdgeLabelBox, the side of its lane it is on, and
// whether it is turned.
func edgeLabelBox(d *gw.Document, i gw.EdgeIndex) (gw.Viewport, float64, bool) {
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	b, turned := labelBoxAt(d, i, &wy, wy.middle(wy.route))
	return b, wy.side(), turned
}

// labelWidth is the length of the box of edge i's label: wide enough for
// the label as drawn and as typed in.
func labelWidth(d *gw.Document, i gw.EdgeIndex) float64 {
	return math.Max(float64(float64(max(d.EdgeLabelRunes(i), runeCount(d.Edge(i).Label())))*defaultCharWidth)+16, minLabelWidth)
}

// turnedLength is the run a label turned to read along it needs: its
// text and room either side, as a layout makes the run through a dummy.
func turnedLength(d *gw.Document, i gw.EdgeIndex) float64 {
	return float64(float64(d.EdgeLabelRunes(i))*defaultCharWidth) + 16
}

// labelBoxAt is the box of edge i's label drawn at mid on its way w, and
// whether it is turned: a line taller, or turned wider, for each line
// of the label after its first.
func labelBoxAt(d *gw.Document, i gw.EdgeIndex, w *way, mid gw.Point) (gw.Viewport, bool) {
	kind := d.Kinds().Edge(d.Edge(i).Kind())
	lw, side := labelWidth(d, i), w.side()
	more := gw.MoreLines(d.EdgeLabelLines(i))
	if c, ok := w.turnedAt(mid, turnedLength(d, i)); ok {
		x := mid.X + 2
		if side < 0 {
			x = mid.X - 2 - labelHeight - more
		}
		return gw.Viewport{X: x, Y: c - float64(lw/2), W: labelHeight + more, H: lw}, true
	}
	return levelBoxAt(kind, w.f, mid, side, lw, more), false
}

// levelBoxAt is the box of a label of width w drawn level at mid, on
// side of its lane, in frame f, with more room for its lines after its
// first: beside the stroke they are centred on mid, above it the last
// is the nearest, and below it the first.
func levelBoxAt(kind gw.EdgeKindView, f frame, mid gw.Point, side, w, more float64) gw.Viewport {
	if f.down {
		y := mid.Y - float64((labelHeight+more)/2)
		if side < 0 {
			return gw.Viewport{X: mid.X - 2 - w, Y: y, W: w, H: labelHeight + more}
		}
		return gw.Viewport{X: mid.X + 2, Y: y, W: w, H: labelHeight + more}
	}
	base, step := mid.Y-defaultStroke/2-4, -float64(defaultFontSize)
	if side < 0 {
		base, step = mid.Y+defaultStroke/2+defaultFontSize, defaultFontSize
	}
	if kind.Stereotype() != "" {
		base += step
	}
	if side < 0 {
		return centred(mid.X, base+float64(more/2)-defaultFontSize/3, w, more)
	}
	return centred(mid.X, base-float64(more/2)-defaultFontSize/3, w, more)
}

// defaultStroke is the stroke width of an unweighted edge.
const defaultStroke = 1.5

// centred is a label box of the width centred on (x, y), with more
// room for the lines of a label after its first.
func centred(x, y, w, more float64) gw.Viewport {
	h := labelHeight + more
	return gw.Viewport{X: x - float64(w/2), Y: y - float64(h/2), W: w, H: h}
}

// inLabel reports whether p is in the box of an edge's label, when it
// has one.
func inLabel(d *gw.Document, i gw.EdgeIndex, p gw.Point) bool {
	if d.EdgeLabelRunes(i) == 0 {
		return false
	}
	b, side, turned := edgeLabelBox(d, i)
	b = drawnBox(d, i, b, side, turned)
	return p.X >= b.X && p.X <= b.X+b.W && p.Y >= b.Y && p.Y <= b.Y+b.H
}

// drawnBox is the part of edge i's label box b, on side of its lane and
// turned or not, that its text takes as drawn: the box for typing in is
// wider than the text, and the text's own length is what is drawn, hit
// and kept clear of other edges.
func drawnBox(d *gw.Document, i gw.EdgeIndex, b gw.Viewport, side float64, turned bool) gw.Viewport {
	w := float64(float64(d.EdgeLabelRunes(i))*defaultCharWidth) + 8
	switch {
	case turned:
		b.Y, b.H = b.Y+float64(b.H/2)-float64(w/2), w
	case d.Flow() != gw.TopToBottom:
		b.X, b.W = b.X+float64(b.W/2)-float64(w/2), w
	case side < 0:
		b.X, b.W = b.X+b.W-w, w
	default:
		b.W = w
	}
	return b
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
	v := d.Vertex(i)
	at := portOn(d, i, p)
	w := float64(float64(runeCount(v.Ports().At(int(p)).Name)) * gw.PortLabelChar)
	side := turnSide(v.Ports().At(int(p)).Side, turned(d, v))
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
	for k := range d.EdgeSlots() {
		e := d.Edge(gw.EdgeIndex(k)) //nolint:gosec // bounded by the capacity
		from := e.From() == v && e.FromPort() == p
		if !e.Live() || (!from && (e.To() != v || e.ToPort() != p)) {
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
	route := wy.route
	legs := wy.through.Len() + 1
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
