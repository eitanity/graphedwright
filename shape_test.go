package graphedwright_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// tabbed is a package's shape: a tab 40 by 12 above a body, the tab in
// the left and top margins so that it keeps its size.
func tabbed() gw.ShapeDef {
	return gw.ShapeDef{
		Name: "tabbed", Box: gw.Size{W: 100, H: 60}, Margins: [4]float64{40, 12, 0, 0},
		Paths: []gw.ShapePath{{D: "M 0 12 H 100 V 60 H 0 Z"}, {D: "M 0 0 H 40 V 12 H 0 Z", Fill: true}},
	}
}

// shaped is a schema with one kind drawn as the shape.
func shaped(defs ...gw.ShapeDef) gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "pkg", Shape: gw.Provided, Size: gw.Size{W: 100, H: 60}}},
		Shapes:      defs,
	}
}

// TestShapeRefusals: each refusal names the shape, and in a path the
// path and the byte at fault.
func TestShapeRefusals(t *testing.T) {
	path := func(d string) func(*gw.ShapeDef) { return func(s *gw.ShapeDef) { s.Paths[0].D = d } }
	for _, c := range []struct {
		name   string
		change func(*gw.ShapeDef)
		want   string
	}{
		{"command outside the list", path("M 0 0 S 1 1 2 2"), `shape "tabbed": path 0: at byte 6: command 'S' is not M, L, H, V, C, Q, A or Z, absolute`},
		{"relative command", path("M 0 0 l 5 5"), "at byte 6: command 'l' is not"},
		{"x outside the box", path("M 0 0 L 101 5"), "path 0: at byte 8: x 101 is outside the box"},
		{"y outside the box", path("M 0 0 L 5 61"), "at byte 10: y 61 is outside the box"},
		{"negative", path("M -1 0"), "at byte 2: x -1 is outside the box"},
		{"vertical outside", path("M 0 0 V 70"), "at byte 8: y 70 is outside the box"},
		{"horizontal outside", path("M 0 0 H 170"), "at byte 8: x 170 is outside the box"},
		{"control point outside", path("M 0 0 C 0 0 0 99 10 10"), "at byte 14: y 99 is outside the box"},
		{"quad outside", path("M 0 0 Q 120 0 10 10"), "at byte 8: x 120 is outside the box"},
		{"rotated arc", path("M 0 0 A 5 5 30 0 1 10 10"), "at byte 12: an arc is rotated"},
		{"arc flag", path("M 0 0 A 5 5 0 2 1 10 10"), "at byte 14: an arc's flag is not 0 or 1"},
		{"sweep flag", path("M 0 0 A 5 5 0 0 3 10 10"), "at byte 16: an arc's flag is not 0 or 1"},
		{"arc radius", path("M 0 0 A 200 5 0 0 1 10 10"), "at byte 8: a radius 200 is outside the box"},
		{"arc height", path("M 0 0 A 5 70 0 0 1 10 10"), "at byte 10: a radius 70 is outside the box"},
		{"arc end", path("M 0 0 A 5 5 0 0 1 10 90"), "at byte 21: y 90 is outside the box"},
		{"empty path", path(""), "path 0: the path is empty"},
		{"blank path", path(" , "), "path 0: the path is empty"},
		{"not from M", path("L 1 1"), "at byte 0: the path must start with M"},
		{"closed first", path(" Z"), "at byte 1: the path must start with M"},
		{"a number alone", path("1 2"), "at byte 0: a number without a command"},
		{"a number after Z", path("M 0 0 Z 1"), "at byte 8: a number without a command"},
		{"cut short", path("M 0"), "at byte 3: expected a number"},
		{"infinite", path("M 1e999 0"), "at byte 2: not a finite number"},
		{"two points", path("M 1.2.3 0"), "at byte 2: not a finite number"},
		{"markup", path(`M 0 0"/><script>`), "at byte 5: expected a number"},
		{"ampersand", path("M 0 0 & 1"), "at byte 6: expected a number"},
		{"a name for a number", path("M Infinity 0"), "at byte 2: expected a number"},
		{"a letter outside", path("M 0 0 X 1"), "at byte 6: command 'X' is not"},
		{"too long", path(strings.Repeat("M 0 0 ", 900)), "the data is longer than 5120 bytes"},
		{"too many commands", path("M 0 0" + strings.Repeat(" L 1 1", gw.MaxPathCommands)), "more than 64 commands"},
		{"too many closes", path("M 0 0" + strings.Repeat(" Z", gw.MaxPathCommands)), "more than 64 commands"},
		{"too many paths", func(s *gw.ShapeDef) {
			for len(s.Paths) <= gw.MaxShapePaths {
				s.Paths = append(s.Paths, gw.ShapePath{D: "M 0 0"})
			}
		}, `shape "tabbed": more than 8 paths`},
		{"empty name", func(s *gw.ShapeDef) { s.Name = "" }, "the name must start with a lowercase letter"},
		{"capital", func(s *gw.ShapeDef) { s.Name = "Tab" }, "the name must start with a lowercase letter"},
		{"space in the name", func(s *gw.ShapeDef) { s.Name = "a b" }, "only lowercase letters, digits and '-'"},
		{"quote in the name", func(s *gw.ShapeDef) { s.Name = `a"` }, "only lowercase letters, digits and '-'"},
		{"long name", func(s *gw.ShapeDef) { s.Name = strings.Repeat("a", 65) }, "at most 64 bytes"},
		{"built-in name", func(s *gw.ShapeDef) { s.Name = "circle" }, `shape "circle": the name is a built-in shape's`},
		{"store's name", func(s *gw.ShapeDef) { s.Name = "datastore" }, "the name is a built-in shape's"},
		{"empty box", func(s *gw.ShapeDef) { s.Box.W = 0 }, "the box must be positive and at most 10000"},
		{"huge box", func(s *gw.ShapeDef) { s.Box.H = 10001 }, "the box must be positive"},
		{"negative margin", func(s *gw.ShapeDef) { s.Margins[3] = -1 }, "a margin must be finite and not negative"},
		{"margins across", func(s *gw.ShapeDef) { s.Margins = [4]float64{60, 0, 41, 0} }, "the margins are wider or higher than the box"},
		{"margins down", func(s *gw.ShapeDef) { s.Margins = [4]float64{0, 30, 0, 31} }, "the margins are wider or higher than the box"},
		{"label place", func(s *gw.ShapeDef) { s.Label = gw.LabelNone + 1 }, "the label place is out of range"},
		{"outline", func(s *gw.ShapeDef) { s.Outline = gw.OutlinePolygon + 1 }, "the outline is out of range"},
		{"polygon without its outline", func(s *gw.ShapeDef) { s.Polygon = []gw.Point{{X: 0, Y: 0}, {X: 9, Y: 0}, {X: 0, Y: 9}} }, "a polygon without the polygon outline"},
		{"polygon of two", polygonOf(gw.Point{X: 0, Y: 0}, gw.Point{X: 9, Y: 9}), "the polygon has from 3 to 16 points"},
		{"polygon outside", polygonOf(gw.Point{X: 0, Y: 0}, gw.Point{X: 120, Y: 0}, gw.Point{X: 0, Y: 9}), "polygon point 1 is outside the box"},
		{"concave", polygonOf(gw.Point{X: 0, Y: 0}, gw.Point{X: 100, Y: 0}, gw.Point{X: 50, Y: 20}, gw.Point{X: 100, Y: 60}, gw.Point{X: 0, Y: 60}), "the polygon is not convex"},
		{"straight", polygonOf(gw.Point{X: 0, Y: 0}, gw.Point{X: 50, Y: 0}, gw.Point{X: 100, Y: 0}, gw.Point{X: 50, Y: 60}), "the polygon is not convex"},
		{"star", polygonOf(gw.Point{X: 50, Y: 0}, gw.Point{X: 80, Y: 60}, gw.Point{X: 0, Y: 22}, gw.Point{X: 100, Y: 22}, gw.Point{X: 20, Y: 60}), "the polygon is not convex"},
		{"too many points", func(s *gw.ShapeDef) {
			s.Outline = gw.OutlinePolygon
			for k := range gw.MaxPolygonPoints + 1 {
				s.Polygon = append(s.Polygon, gw.Point{X: float64(k), Y: 0})
			}
		}, "the polygon has from 3 to 16 points"},
	} {
		t.Run(c.name, func(t *testing.T) {
			def := tabbed()
			c.change(&def)
			_, err := gw.Compile(shaped(def))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want %q", err, c.want)
			}
		})
	}
}

func polygonOf(ps ...gw.Point) func(*gw.ShapeDef) {
	return func(s *gw.ShapeDef) { s.Outline, s.Polygon = gw.OutlinePolygon, ps }
}

// TestShapesInSchema: two shapes of one name, more than MaxShapes, and a
// kind naming a shape past the schema's are refused; a schema within
// every limit compiles.
func TestShapesInSchema(t *testing.T) {
	if _, err := gw.Compile(shaped(tabbed(), tabbed())); err == nil || !strings.Contains(err.Error(), `shape "tabbed": duplicate name`) {
		t.Errorf("two of a name: %v", err)
	}
	defs := make([]gw.ShapeDef, gw.MaxShapes+1)
	for i := range defs {
		defs[i] = tabbed()
		defs[i].Name = "s" + strconv.Itoa(i)
	}
	if _, err := gw.Compile(shaped(defs...)); err == nil || !strings.Contains(err.Error(), "schema: more than 32 shapes") {
		t.Errorf("33 shapes: %v", err)
	}
	s := shaped(defs[:gw.MaxShapes]...)
	if _, err := gw.Compile(s); err != nil {
		t.Errorf("32 shapes: %v", err)
	}
	s.VertexKinds[0].Shape = gw.Provided + gw.MaxShapes
	if _, err := gw.Compile(s); err == nil || !strings.Contains(err.Error(), "shape 96 is neither built in nor one the schema provides") {
		t.Errorf("past the shapes: %v", err)
	}
	full := tabbed()
	full.Paths = nil
	for range gw.MaxShapePaths {
		full.Paths = append(full.Paths, gw.ShapePath{D: "M 0 0" + strings.Repeat(" L 1 1", gw.MaxPathCommands-1)})
	}
	full.Outline = gw.OutlinePolygon
	for k := range gw.MaxPolygonPoints {
		a := 2 * 3.141592653589793 * float64(k) / gw.MaxPolygonPoints
		full.Polygon = append(full.Polygon, gw.Point{X: 50 + 40*cos(a), Y: 30 + 25*sin(a)})
	}
	if _, err := gw.Compile(shaped(full)); err != nil {
		t.Errorf("a shape at every limit: %v", err)
	}
}

// TestShapeParsed: the paths are parsed into commands, numbers after a
// command's own repeating it and an M's as L; the views read the shape,
// and a built-in shape has none; the compiled shape is the schema's own
// copy.
func TestShapeParsed(t *testing.T) {
	def := tabbed()
	def.Paths = append(def.Paths, gw.ShapePath{D: "M0,0 10 10 C 1 2 3 4 5 6 Q 7 8 9 10 A 4 5 0 1 0 20 30 V 40Z"})
	def.Outline, def.Polygon, def.Label, def.Turns = gw.OutlinePolygon, []gw.Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 50, Y: 60}}, gw.LabelBelow, true
	k, err := gw.Compile(shaped(def))
	if err != nil {
		t.Fatal(err)
	}
	def.Paths[0].D, def.Polygon[0].X, def.Name = "M 1 1", 9, "changed"
	s := k.ShapeOf(k.Vertex(0).Shape())
	if !s.Valid() || s.Name() != "tabbed" || s.Box() != (gw.Size{W: 100, H: 60}) || s.Margins() != [4]float64{40, 12, 0, 0} ||
		s.Outline() != gw.OutlinePolygon || s.Polygon().Len() != 3 || s.Polygon().At(0) != (gw.Point{}) || s.Label() != gw.LabelBelow || !s.Turns() || s.Paths() != 3 || k.Shapes() != 1 {
		t.Fatalf("view: %q %v %v %v %v %d", s.Name(), s.Box(), s.Margins(), s.Outline(), s.Label(), s.Paths())
	}
	if p := s.Path(0); p.Len() != 5 || p.Fill() || p.At(0) != (gw.PathCommand{Op: gw.MoveTo, Args: [7]float64{0, 12}}) || p.At(1) != (gw.PathCommand{Op: gw.HLineTo, Args: [7]float64{100}}) || p.At(4).Op != gw.Close {
		t.Errorf("path 0: %d commands, %+v", p.Len(), p.At(0))
	}
	if !s.Path(1).Fill() {
		t.Error("path 1 is filled")
	}
	want := []gw.PathCommand{
		{Op: gw.MoveTo}, {Op: gw.LineTo, Args: [7]float64{10, 10}}, {Op: gw.CubicTo, Args: [7]float64{1, 2, 3, 4, 5, 6}},
		{Op: gw.QuadTo, Args: [7]float64{7, 8, 9, 10}}, {Op: gw.ArcTo, Args: [7]float64{4, 5, 0, 1, 0, 20, 30}},
		{Op: gw.VLineTo, Args: [7]float64{40}}, {Op: gw.Close},
	}
	p := s.Path(2)
	if p.Len() != len(want) {
		t.Fatalf("path 2: %d commands", p.Len())
	}
	for i, c := range want {
		if p.At(i) != c {
			t.Errorf("path 2 command %d: %+v, want %+v", i, p.At(i), c)
		}
	}
	if p.At(-1).Op != gw.Close || p.At(len(want)).Op != gw.Close || s.Path(3).Valid() || s.Path(-1).Valid() {
		t.Error("out of range")
	}
	for _, sh := range []gw.Shape{gw.Rect, gw.DataStore, gw.Provided + 1} {
		if b := k.ShapeOf(sh); b.Valid() || b.Name() != "" || b.Paths() != 0 || b.Box() != (gw.Size{}) || b.Polygon().Len() != 0 || b.Turns() || b.Outline() != gw.OutlineBox || b.Label() != gw.LabelInside || b.Margins() != [4]float64{} {
			t.Errorf("shape %d has a view", sh)
		}
	}
	var none *gw.Kinds
	if none.Shapes() != 0 || none.ShapeOf(gw.Provided).Valid() {
		t.Error("nil kinds")
	}
}

func cos(a float64) float64 { return math.Cos(a) }
func sin(a float64) float64 { return math.Sin(a) }
