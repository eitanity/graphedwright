// Package shapes is a library of common shapes for a schema to provide
// (graphedwright.Schema.Shapes): UML's initial, final and flow final
// nodes and fork bars, BPMN's double-ringed events, packages, notes, and
// the flowchart's document, store, input and output, preparation and
// manual step. Each is a graphedwright.ShapeDef written as a host would
// write its own, so a host that does not import this package does not
// link it.
package shapes

import (
	"strconv"

	gw "code.eitanity.com/graphedwright"
)

// The names of the library's shapes.
const (
	FilledCircle    = "filled-circle"    // a solid disc: an initial state or node
	Bullseye        = "bullseye"         // a ring round a disc: a final state or node
	CrossedCircle   = "crossed-circle"   // a ring with a cross: a flow final node
	DoubleCircle    = "double-circle"    // two rings: an intermediate event
	Bar             = "bar"              // a solid bar across the flow, turning with it: a fork or a join
	TabbedRectangle = "tabbed-rectangle" // a box with a tab at its top left: a package or a folder
	FoldedNote      = "folded-note"      // a box with its top right corner folded: a note or a data object
	Document        = "document"         // a box with a wavy foot
	Cylinder        = "cylinder"         // a drum: a database or a store
	Parallelogram   = "parallelogram"    // a box slanted: input or output
	Hexagon         = "hexagon"          // preparation
	Trapezoid       = "trapezoid"        // wider at the top: a manual operation
)

// Count is the number of the library's shapes.
const Count = 12

// Names are the library's shapes' names, in the order Library gives
// them.
func Names() [Count]string {
	return [Count]string{FilledCircle, Bullseye, CrossedCircle, DoubleCircle, Bar, TabbedRectangle, FoldedNote, Document, Cylinder, Parallelogram, Hexagon, Trapezoid}
}

// Library is every shape of the library, in the order of Names, made
// afresh for the caller to keep or change.
func Library() []gw.ShapeDef {
	names := Names()
	out := make([]gw.ShapeDef, 0, Count)
	for _, name := range names {
		def, _ := Named(name)
		out = append(out, def)
	}
	return out
}

// Named is the library's shape of that name, made afresh, and whether
// there is one.
func Named(name string) (gw.ShapeDef, bool) {
	switch name {
	case FilledCircle:
		return round(name, 24, gw.ShapePath{D: disc(12, 12), Fill: true}), true
	case Bullseye:
		return round(name, 28, gw.ShapePath{D: disc(14, 14)}, gw.ShapePath{D: disc(14, 8), Fill: true}), true
	case CrossedCircle:
		// The cross meets the ring at 45°: 14 ∓ 14/√2.
		return round(name, 28, gw.ShapePath{D: disc(14, 14)}, gw.ShapePath{D: "M 4.1 4.1 L 23.9 23.9 M 23.9 4.1 L 4.1 23.9"}), true
	case DoubleCircle:
		return round(name, 28, gw.ShapePath{D: disc(14, 14)}, gw.ShapePath{D: disc(14, 11)}), true
	case Bar:
		return gw.ShapeDef{Name: name, Box: gw.Size{W: 8, H: 60}, Label: gw.LabelNone, Turns: true,
			Paths: []gw.ShapePath{{D: "M 0 0 H 8 V 60 H 0 Z", Fill: true}}}, true
	case TabbedRectangle:
		// The tab, 40 by 12, keeps its size in the left and top margins;
		// the right margin as wide keeps the name centred on the body.
		return gw.ShapeDef{Name: name, Box: gw.Size{W: 180, H: 80}, Margins: [4]float64{40, 12, 40, 0},
			Outline: gw.OutlinePolygon, Polygon: []gw.Point{{X: 0, Y: 12}, {X: 180, Y: 12}, {X: 180, Y: 80}, {X: 0, Y: 80}},
			Paths: []gw.ShapePath{{D: "M 0 12 H 180 V 80 H 0 Z"}, {D: "M 0 12 V 0 H 40 V 12 Z"}}}, true
	case FoldedNote:
		return gw.ShapeDef{Name: name, Box: gw.Size{W: 120, H: 60}, Margins: [4]float64{12, 12, 12, 12},
			Outline: gw.OutlinePolygon, Polygon: []gw.Point{{X: 0, Y: 0}, {X: 108, Y: 0}, {X: 120, Y: 12}, {X: 120, Y: 60}, {X: 0, Y: 60}},
			Paths: []gw.ShapePath{{D: "M 0 0 H 108 L 120 12 V 60 H 0 Z"}, {D: "M 108 0 V 12 H 120"}}}, true
	case Document:
		return gw.ShapeDef{Name: name, Box: gw.Size{W: 120, H: 60}, Margins: [4]float64{0, 0, 0, 12},
			Paths: []gw.ShapePath{{D: "M 0 0 H 120 V 52 C 90 40 30 60 0 52 Z"}}}, true
	case Cylinder:
		// The lid's ellipse, 12 high, keeps its height in the top and
		// bottom margins and stretches across the width.
		return gw.ShapeDef{Name: name, Box: gw.Size{W: 80, H: 60}, Margins: [4]float64{0, 12, 0, 12},
			Paths: []gw.ShapePath{{D: "M 0 6 A 40 6 0 0 1 80 6 V 54 A 40 6 0 0 1 0 54 Z"}, {D: "M 0 6 A 40 6 0 0 0 80 6"}}}, true
	case Parallelogram:
		return slanted(name, "M 20 0 H 120 L 100 60 H 0 Z", gw.Point{X: 20}, gw.Point{X: 120}, gw.Point{X: 100, Y: 60}, gw.Point{Y: 60}), true
	case Hexagon:
		return slanted(name, "M 20 0 H 100 L 120 30 L 100 60 H 20 L 0 30 Z",
			gw.Point{X: 20}, gw.Point{X: 100}, gw.Point{X: 120, Y: 30}, gw.Point{X: 100, Y: 60}, gw.Point{X: 20, Y: 60}, gw.Point{Y: 30}), true
	case Trapezoid:
		return slanted(name, "M 0 0 H 120 L 100 60 H 20 Z", gw.Point{}, gw.Point{X: 120}, gw.Point{X: 100, Y: 60}, gw.Point{X: 20, Y: 60}), true
	}
	return gw.ShapeDef{}, false
}

// round is a shape of circles in a square box of side n, clipped to its
// ellipse, its label below as a pseudostate's or an event's name is.
func round(name string, n float64, paths ...gw.ShapePath) gw.ShapeDef {
	return gw.ShapeDef{Name: name, Box: gw.Size{W: n, H: n}, Outline: gw.OutlineEllipse, Label: gw.LabelBelow, Paths: paths}
}

// disc is the path of a circle of radius r about (c, c).
func disc(c, r int) string {
	l, rt, y, rs := strconv.Itoa(c-r), strconv.Itoa(c+r), strconv.Itoa(c), strconv.Itoa(r)
	return "M " + l + " " + y + " A " + rs + " " + rs + " 0 0 1 " + rt + " " + y + " A " + rs + " " + rs + " 0 0 1 " + l + " " + y + " Z"
}

// slanted is a convex polygon in a box 120 by 60, drawn by d through
// the corners given, which are its outline; the 20 a corner stands in
// from a side keeps its size in the left and right margins.
func slanted(name, d string, corners ...gw.Point) gw.ShapeDef {
	return gw.ShapeDef{Name: name, Box: gw.Size{W: 120, H: 60}, Margins: [4]float64{20, 0, 20, 0},
		Outline: gw.OutlinePolygon, Polygon: corners, Paths: []gw.ShapePath{{D: d}}}
}
