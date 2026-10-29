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
	textW := float64(float64(runeCount(x.Label)) * defaultCharWidth)
	switch {
	case kind.Container:
		top := x.Pos.Y - float64(x.Size.H/2)
		return gw.Viewport{X: x.Pos.X - float64(x.Size.W/2) + 4, Y: top + gw.ContainerTitle/2 - labelHeight/2 + 1, W: math.Max(x.Size.W-8, minLabelWidth), H: labelHeight}
	case kind.Shape == gw.Actor:
		w := math.Max(textW+16, minLabelWidth)
		return centred(x.Pos.X, x.Pos.Y+float64(x.Size.H/2)-4-defaultFontSize/3, w)
	}
	return centred(x.Pos.X, x.Pos.Y+4-defaultFontSize/3, math.Max(x.Size.W, minLabelWidth))
}

// EdgeLabelBox is the box of an edge's label, where it is drawn or
// would be when it has none: above the middle of its route, or to the
// right of it in a top-to-bottom flow, as wide as its text.
func EdgeLabelBox(d *gw.Document, i gw.EdgeIndex) gw.Viewport {
	e := &d.Edges[i]
	kind := d.Schema.EdgeKinds[e.Kind]
	var lane [1]gw.Point
	wy := wayOf(d, i, &lane)
	f := wy.f
	mid := wy.middle(kind.Route)
	w := math.Max(float64(float64(runeCount(e.Label))*defaultCharWidth)+16, minLabelWidth)
	if f.down {
		return gw.Viewport{X: mid.X + 2, Y: mid.Y - labelHeight/2, W: w, H: labelHeight}
	}
	base := mid.Y - defaultStroke/2 - 4
	if kind.Stereotype != "" {
		base -= defaultFontSize
	}
	return centred(mid.X, base-defaultFontSize/3, w)
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
	if d.Edges[i].Label == "" {
		return false
	}
	b := EdgeLabelBox(d, i)
	// The box for typing in is wider than the text; the text's own
	// width is what is drawn and hit.
	w := float64(float64(runeCount(d.Edges[i].Label))*defaultCharWidth) + 8
	if d.Flow != gw.TopToBottom {
		b.X, b.W = b.X+float64(b.W/2)-float64(w/2), w
	} else {
		b.W = w
	}
	return p.X >= b.X && p.X <= b.X+b.W && p.Y >= b.Y && p.Y <= b.Y+b.H
}
