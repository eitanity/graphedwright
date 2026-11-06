package dom

import (
	"errors"
	"math"
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/shapes"
)

// shapesFromJS reads the shapes a schema object defines under shapes:
//
//	[{name, w, h, margins: [left, top, right, bottom], paths: [{d, fill}],
//	  outline, polygon: [[x, y]], label, turns}]
//
// Outlines are "box" (the default), "diamond", "ellipse" and "polygon";
// labels "inside" (the default), "below" and "none". Past the schema's
// limits it reads one more of each list, which the schema refuses.
func shapesFromJS(v js.Value) ([]gw.ShapeDef, error) {
	if v.Type() != js.TypeObject {
		return nil, nil
	}
	n := min(v.Length(), gw.MaxShapes+1)
	out := make([]gw.ShapeDef, 0, n)
	// bound: at most MaxShapes+1 shapes
	for i := range n {
		def, err := shapeFromJS(v.Index(i))
		if err != nil {
			return nil, err
		}
		out = append(out, def)
	}
	return out, nil
}

func shapeFromJS(v js.Value) (gw.ShapeDef, error) {
	def := gw.ShapeDef{Name: str(v, "name"), Box: gw.Size{W: float(v, "w"), H: float(v, "h")}, Turns: get(v, "turns").Truthy()}
	where := "shape " + strconv.Quote(def.Name)
	if m := get(v, "margins"); m.Type() == js.TypeObject {
		// bound: the four margins
		for j := range min(m.Length(), 4) {
			def.Margins[j] = number(m.Index(j))
		}
	}
	if ps := get(v, "paths"); ps.Type() == js.TypeObject {
		n := min(ps.Length(), gw.MaxShapePaths+1)
		def.Paths = make([]gw.ShapePath, 0, n)
		// bound: at most MaxShapePaths+1 paths
		for j := range n {
			p := ps.Index(j)
			def.Paths = append(def.Paths, gw.ShapePath{D: str(p, "d"), Fill: get(p, "fill").Truthy()})
		}
	}
	if ps := get(v, "polygon"); ps.Type() == js.TypeObject {
		n := min(ps.Length(), gw.MaxPolygonPoints+1)
		def.Polygon = make([]gw.Point, 0, n)
		// bound: at most MaxPolygonPoints+1 points
		for j := range n {
			p := ps.Index(j)
			if p.Type() != js.TypeObject {
				return def, errors.New(where + ": polygon point " + strconv.Itoa(j) + " is not [x, y]")
			}
			def.Polygon = append(def.Polygon, gw.Point{X: number(p.Index(0)), Y: number(p.Index(1))})
		}
	}
	switch o := str(v, "outline"); o {
	case "", "box":
		def.Outline = gw.OutlineBox
	case "diamond":
		def.Outline = gw.OutlineDiamond
	case "ellipse":
		def.Outline = gw.OutlineEllipse
	case "polygon":
		def.Outline = gw.OutlinePolygon
	default:
		return def, errors.New(where + ": outline " + strconv.Quote(o) + " is not box, diamond, ellipse or polygon")
	}
	switch l := str(v, "label"); l {
	case "", "inside":
		def.Label = gw.LabelInside
	case "below":
		def.Label = gw.LabelBelow
	case "none":
		def.Label = gw.LabelNone
	default:
		return def, errors.New(where + ": label " + strconv.Quote(l) + " is not inside, below or none")
	}
	return def, nil
}

// number is a JavaScript number, NaN when it is not one, which the
// schema refuses where a number must be finite.
func number(v js.Value) float64 {
	if v.Type() != js.TypeNumber {
		return math.NaN()
	}
	return v.Float()
}

// providedShape is the shape a vertex kind names that is not built in:
// one the schema object defines, or the library's of that name, added
// to the schema's shapes the first time a kind names it.
func providedShape(s *gw.Schema, name string) (gw.Shape, bool) {
	// bound: the schema's shapes, at most MaxShapes+1 as read
	for i := range s.Shapes {
		if s.Shapes[i].Name == name {
			return gw.Provided + gw.Shape(i), true //nolint:gosec // bounded by the shapes read
		}
	}
	def, ok := shapes.Named(name)
	if !ok || len(s.Shapes) > gw.MaxShapes {
		return gw.Rect, false
	}
	s.Shapes = append(s.Shapes, def)
	return gw.Provided + gw.Shape(len(s.Shapes)-1), true //nolint:gosec // at most MaxShapes+1
}
