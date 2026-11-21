package graphedwright

import (
	"math"
	"strconv"
)

// Provided is the first shape a schema provides: a vertex kind with
// Shape Provided + i is drawn with the schema's i-th ShapeDef.
const Provided Shape = 64

// Outline is the edge a provided shape's edges are clipped to, and the
// band an edge starts from.
type Outline uint8

// The outlines.
const (
	OutlineBox     Outline = iota // the vertex's box
	OutlineDiamond                // the diamond touching the middle of each side
	OutlineEllipse                // the ellipse filling the box
	OutlinePolygon                // ShapeDef.Polygon, scaled as the paths are
)

// LabelPlace is where a provided shape's label is drawn.
type LabelPlace uint8

// The places.
const (
	LabelInside LabelPlace = iota // centred between the margins, cut at that width
	LabelBelow                    // under the box, as wide as it needs, the keyword over it
	LabelNone                     // not drawn
)

// The limits of the shapes a schema provides.
const (
	MaxShapes        = 32 // shapes in a schema
	MaxShapePaths    = 8  // paths in a shape
	MaxPathCommands  = 64 // commands in a path
	MaxPolygonPoints = 16 // points in an outline polygon
	maxShapeBox      = 10000
)

// ShapeDef is a shape a schema provides: paths drawn in a box of its
// own, origin at the top left, and scaled to each vertex in nine
// slices. A point within a margin (left, top, right, bottom) keeps its
// distance from that side; a point between the margins scales across
// the middle; an arc's radii scale as the region of its end point. So a
// tab, a fold or an icon in a corner keeps its size, and a circle drawn
// with no margins becomes the vertex's ellipse. Name is written as the
// class shape-<name>. A shape that turns is given for a left-to-right
// flow and turns with a top-to-bottom one.
type ShapeDef struct {
	Name    string
	Box     Size
	Margins [4]float64
	Paths   []ShapePath
	Outline Outline
	Polygon []Point // with OutlinePolygon: convex, in the box
	Label   LabelPlace
	Turns   bool
}

// ShapePath is one path of a provided shape: its data, with only M, L,
// H, V, C, Q, A and Z, absolute, every number finite and in the box and
// every arc unrotated; drawn filled solid, a mark, or as the shape is,
// a line.
type ShapePath struct {
	D    string
	Fill bool
}

// PathOp is one command of a path, as the schema parsed it.
type PathOp uint8

// The commands, with the numbers each takes: MoveTo, LineTo x y;
// HLineTo x; VLineTo y; CubicTo x1 y1 x2 y2 x y; QuadTo x1 y1 x y;
// ArcTo rx ry rotation large sweep x y; Close none.
const (
	MoveTo PathOp = iota
	LineTo
	HLineTo
	VLineTo
	CubicTo
	QuadTo
	ArcTo
	Close
)

// PathCommand is a parsed command and its numbers, in the shape's box.
type PathCommand struct {
	Op   PathOp
	Args [7]float64
}

// argCount is how many numbers a command takes.
func argCount(op PathOp) int {
	switch op {
	case MoveTo, LineTo:
		return 2
	case HLineTo, VLineTo:
		return 1
	case CubicTo:
		return 6
	case QuadTo:
		return 4
	case ArcTo:
		return 7
	case Close:
		return 0
	}
	return 0
}

// opOf is the command a letter names, if it is one of the absolute ones
// allowed.
func opOf(c byte) (PathOp, bool) {
	switch c {
	case 'M':
		return MoveTo, true
	case 'L':
		return LineTo, true
	case 'H':
		return HLineTo, true
	case 'V':
		return VLineTo, true
	case 'C':
		return CubicTo, true
	case 'Q':
		return QuadTo, true
	case 'A':
		return ArcTo, true
	case 'Z':
		return Close, true
	}
	return 0, false
}

// shape is a provided shape compiled: the definition, its lists its
// own, and each path parsed.
type shape struct {
	def   ShapeDef
	paths [][]PathCommand
}

// compileShapes checks the schema's shapes and parses their paths.
func compileShapes(defs []ShapeDef) ([]shape, error) {
	if len(defs) > MaxShapes {
		return nil, schemaError("", "more than "+itoa(MaxShapes)+" shapes")
	}
	if len(defs) == 0 {
		return nil, nil
	}
	out := make([]shape, len(defs))
	for i, def := range defs {
		where := "shape " + strconv.Quote(def.Name)
		if err := checkShapeName(defs, i); err != nil {
			return nil, schemaError(where, err.Error())
		}
		if err := checkShapeBox(def); err != nil {
			return nil, schemaError(where, err.Error())
		}
		if err := checkOutline(def); err != nil {
			return nil, schemaError(where, err.Error())
		}
		if len(def.Paths) > MaxShapePaths {
			return nil, schemaError(where, "more than "+itoa(MaxShapePaths)+" paths")
		}
		s := shape{def: def, paths: make([][]PathCommand, len(def.Paths))}
		s.def.Paths = append([]ShapePath(nil), def.Paths...)
		s.def.Polygon = append([]Point(nil), def.Polygon...)
		for j, p := range def.Paths {
			cmds, err := parsePath(p.D, def.Box)
			if err != nil {
				return nil, schemaError(where, "path "+itoa(j)+": "+err.Error())
			}
			s.paths[j] = cmds
		}
		out[i] = s
	}
	return out, nil
}

// checkShapeRef refuses a shape that is neither built in nor one of the
// schema's.
func checkShapeRef(s Schema, sh Shape) error {
	if sh > DataStore && (sh < Provided || int(sh-Provided) >= len(s.Shapes)) {
		return refusal("shape " + itoa(int(sh)) + " is neither built in nor one the schema provides")
	}
	return nil
}

// builtinShapeName reports whether a name is one the browser's schema
// objects give a built-in shape, which a provided shape may not take.
func builtinShapeName(name string) bool {
	switch name {
	case "rect", "round", "pill", "diamond", "circle", "actor", "datastore":
		return true
	}
	return false
}

// checkShapeName refuses an empty name, one that is not a lowercase
// word of letters, digits and '-' starting with a letter, a built-in's,
// and a name an earlier shape has.
func checkShapeName(defs []ShapeDef, i int) error {
	name := defs[i].Name
	if name == "" || name[0] < 'a' || name[0] > 'z' || len(name) > 64 {
		return refusal("the name must start with a lowercase letter and be at most 64 bytes")
	}
	// bound: the name, at most 64 bytes
	for k := 0; k < len(name); k++ {
		if c := name[k]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return refusal("the name holds only lowercase letters, digits and '-'")
		}
	}
	if builtinShapeName(name) {
		return refusal("the name is a built-in shape's")
	}
	for j := range i {
		if defs[j].Name == name {
			return refusal("duplicate name")
		}
	}
	return nil
}

// checkShapeBox refuses a box that is not positive and at most
// maxShapeBox, and margins that are negative, not finite, or that meet
// across the box.
func checkShapeBox(def ShapeDef) error {
	b := def.Box
	if !(b.W > 0) || !(b.H > 0) || b.W > maxShapeBox || b.H > maxShapeBox {
		return refusal("the box must be positive and at most " + itoa(maxShapeBox))
	}
	for _, m := range def.Margins {
		if !(m >= 0) || math.IsInf(m, 1) {
			return refusal("a margin must be finite and not negative")
		}
	}
	if def.Margins[0]+def.Margins[2] > b.W || def.Margins[1]+def.Margins[3] > b.H {
		return refusal("the margins are wider or higher than the box")
	}
	if def.Label > LabelNone {
		return refusal("the label place is out of range")
	}
	return nil
}

// checkOutline refuses an outline out of range, a polygon without
// OutlinePolygon, and a polygon that is not convex, with fewer than 3
// points or more than MaxPolygonPoints, or outside the box.
func checkOutline(def ShapeDef) error {
	switch def.Outline {
	case OutlineBox, OutlineDiamond, OutlineEllipse:
		if len(def.Polygon) > 0 {
			return refusal("a polygon without the polygon outline")
		}
		return nil
	case OutlinePolygon:
	default:
		return refusal("the outline is out of range")
	}
	ps := def.Polygon
	if len(ps) < 3 || len(ps) > MaxPolygonPoints {
		return refusal("the polygon has from 3 to " + itoa(MaxPolygonPoints) + " points")
	}
	for k, p := range ps {
		if !inShapeBox(p.X, p.Y, def.Box) {
			return refusal("polygon point " + itoa(k) + " is outside the box")
		}
	}
	if !convex(ps) {
		return refusal("the polygon is not convex")
	}
	return nil
}

// convex reports whether the points, in order, go once round a convex
// polygon: every turn the same way and not straight, and the turns
// adding to one full turn.
func convex(ps []Point) bool {
	n := len(ps)
	sign, turn := 0.0, 0.0
	for k := range n {
		a, b, c := ps[k], ps[(k+1)%n], ps[(k+2)%n]
		ux, uy, vx, vy := b.X-a.X, b.Y-a.Y, c.X-b.X, c.Y-b.Y
		cross := float64(ux*vy) - float64(uy*vx)
		if cross == 0 || sign != 0 && (cross > 0) != (sign > 0) {
			return false
		}
		sign = cross
		turn += math.Atan2(cross, float64(ux*vx)+float64(uy*vy))
	}
	return math.Abs(math.Abs(turn)-2*math.Pi) < 1e-6
}

// inShapeBox reports whether a point is within the box, edges included.
func inShapeBox(x, y float64, b Size) bool {
	return x >= 0 && x <= b.W && y >= 0 && y <= b.H
}

// pathScan is where parsePath is in a path's text: the position, the
// command whose numbers come next, if any, and what it has parsed.
type pathScan struct {
	s    string
	pos  int
	box  Size
	op   PathOp
	have bool
	cmds []PathCommand
}

// parsePath parses a path's data into commands, refusing, at the byte
// it reaches, a letter outside M, L, H, V, C, Q, A and Z, a relative
// command, anything but a plain number where one is due, a number not
// finite or outside the box, a rotated arc or a flag that is not 0 or
// 1, a path that does not start with M, an empty one, and more than
// MaxPathCommands commands. Numbers after a command's own repeat it, an
// M's as L.
func parsePath(d string, box Size) ([]PathCommand, error) {
	if len(d) > MaxPathCommands*80 {
		return nil, refusal("the data is longer than " + itoa(MaxPathCommands*80) + " bytes")
	}
	sc := pathScan{s: d, box: box, cmds: make([]PathCommand, 0, 8)}
	// bound: every step consumes at least one byte, or ends
	for {
		sc.space()
		if sc.pos >= len(d) {
			break
		}
		if err := sc.step(); err != nil {
			return nil, err
		}
		if len(sc.cmds) > MaxPathCommands {
			return nil, sc.fail("more than " + itoa(MaxPathCommands) + " commands")
		}
	}
	if len(sc.cmds) == 0 {
		return nil, refusal("the path is empty")
	}
	return sc.cmds, nil
}

// step parses one command: a letter and its numbers, or numbers that
// repeat the last command.
func (sc *pathScan) step() error {
	if c := sc.s[sc.pos]; isLetter(c) {
		o, ok := opOf(c)
		switch {
		case !ok:
			return sc.fail("command " + strconv.QuoteRune(rune(c)) + " is not M, L, H, V, C, Q, A or Z, absolute")
		case len(sc.cmds) == 0 && o != MoveTo:
			return sc.fail("the path must start with M")
		}
		sc.pos++
		if o == Close {
			sc.cmds = append(sc.cmds, PathCommand{Op: Close})
			sc.have = false
			return nil
		}
		sc.op, sc.have = o, true
	} else if !sc.have {
		return sc.fail("a number without a command")
	}
	cmd, err := sc.args(sc.op, sc.box)
	if err != nil {
		return err
	}
	sc.cmds = append(sc.cmds, cmd)
	if sc.op == MoveTo {
		sc.op = LineTo
	}
	return nil
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// space skips spaces and commas.
func (sc *pathScan) space() {
	// bound: the text
	for sc.pos < len(sc.s) && (sc.s[sc.pos] == ' ' || sc.s[sc.pos] == ',' || sc.s[sc.pos] == '\t' || sc.s[sc.pos] == '\n' || sc.s[sc.pos] == '\r') {
		sc.pos++
	}
}

// fail is a refusal at the scan's byte.
func (sc *pathScan) fail(msg string) error {
	return refusal("at byte " + itoa(sc.pos) + ": " + msg)
}

// args reads a command's numbers and checks them against the box.
func (sc *pathScan) args(op PathOp, box Size) (PathCommand, error) {
	cmd := PathCommand{Op: op}
	n := argCount(op)
	for k := range n {
		sc.space()
		at := sc.pos
		x, err := sc.number()
		if err != nil {
			return cmd, err
		}
		cmd.Args[k] = x
		if err := checkArg(op, k, x, box); err != nil {
			return cmd, refusal("at byte " + itoa(at) + ": " + err.Error())
		}
	}
	return cmd, nil
}

// checkArg checks a command's kth number: an x in the box's width, a y
// in its height, an arc's radii likewise, its rotation 0 and its flags
// 0 or 1.
func checkArg(op PathOp, k int, x float64, box Size) error {
	limit, what := box.W, "x"
	switch {
	case op == VLineTo, op != HLineTo && op != ArcTo && k%2 == 1:
		limit, what = box.H, "y"
	case op == ArcTo && k == 1:
		limit, what = box.H, "a radius"
	case op == ArcTo && k == 0:
		what = "a radius"
	case op == ArcTo && k == 2:
		if x != 0 {
			return refusal("an arc is rotated")
		}
		return nil
	case op == ArcTo && (k == 3 || k == 4):
		if x != 0 && x != 1 {
			return refusal("an arc's flag is not 0 or 1")
		}
		return nil
	case op == ArcTo && k == 6:
		limit, what = box.H, "y"
	}
	if x < 0 || x > limit {
		return refusal(what + " " + strconv.FormatFloat(x, 'g', -1, 64) + " is outside the box")
	}
	return nil
}

// exponent is the position after an exponent at s[i], if there is one.
func exponent(s string, i int) int {
	if i >= len(s) || s[i] != 'e' && s[i] != 'E' {
		return i
	}
	i++
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	// bound: the text
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// number reads a plain decimal number: an optional sign, digits with an
// optional point, and an optional exponent. Anything else, such as a
// name, a letter or markup, is refused.
func (sc *pathScan) number() (float64, error) {
	s, start := sc.s, sc.pos
	i := start
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	digits := 0
	// bound: the text
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		if s[i] != '.' {
			digits++
		}
		i++
	}
	if digits > 0 {
		i = exponent(s, i)
	}
	if digits == 0 {
		return 0, sc.fail("expected a number")
	}
	x, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil || math.IsInf(x, 0) || math.IsNaN(x) {
		return 0, sc.fail("not a finite number")
	}
	sc.pos = i
	return x, nil
}

// ShapeOf is the provided shape a vertex kind's Shape names; it is not
// Valid for a built-in shape.
func (k *Kinds) ShapeOf(s Shape) ShapeView {
	if k == nil || s < Provided || int(s-Provided) >= len(k.shapes) {
		return ShapeView{}
	}
	return ShapeView{&k.shapes[s-Provided]}
}

// Shapes is the number of shapes the schema provides.
func (k *Kinds) Shapes() int {
	if k == nil {
		return 0
	}
	return len(k.shapes)
}

// ShapeView reads a provided shape. The zero ShapeView is not Valid and
// reads as empty.
type ShapeView struct{ s *shape }

// Valid reports whether the view names a provided shape.
func (v ShapeView) Valid() bool { return v.s != nil }

// Name is the shape's name.
func (v ShapeView) Name() string {
	if v.s == nil {
		return ""
	}
	return v.s.def.Name
}

// Box is the size its paths are drawn in.
func (v ShapeView) Box() Size {
	if v.s == nil {
		return Size{}
	}
	return v.s.def.Box
}

// Margins are its left, top, right and bottom margins.
func (v ShapeView) Margins() [4]float64 {
	if v.s == nil {
		return [4]float64{}
	}
	return v.s.def.Margins
}

// Outline is what its edges are clipped to.
func (v ShapeView) Outline() Outline {
	if v.s == nil {
		return OutlineBox
	}
	return v.s.def.Outline
}

// Polygon is its outline polygon, with OutlinePolygon.
func (v ShapeView) Polygon() PointList {
	if v.s == nil {
		return PointList{}
	}
	return PointList{s: v.s.def.Polygon}
}

// Label is where its label is drawn.
func (v ShapeView) Label() LabelPlace {
	if v.s == nil {
		return LabelInside
	}
	return v.s.def.Label
}

// Turns reports whether it turns with the flow.
func (v ShapeView) Turns() bool { return v.s != nil && v.s.def.Turns }

// Paths is the number of its paths.
func (v ShapeView) Paths() int {
	if v.s == nil {
		return 0
	}
	return len(v.s.paths)
}

// Path is its ith path; it is not Valid out of range.
func (v ShapeView) Path(i int) PathView {
	if v.s == nil || i < 0 || i >= len(v.s.paths) {
		return PathView{}
	}
	return PathView{cmds: v.s.paths[i], fill: v.s.def.Paths[i].Fill}
}

// PathView reads one parsed path of a provided shape.
type PathView struct {
	cmds []PathCommand
	fill bool
}

// Valid reports whether the view names a path.
func (p PathView) Valid() bool { return p.cmds != nil }

// Fill reports whether the path is filled solid, a mark.
func (p PathView) Fill() bool { return p.fill }

// Len is the number of its commands.
func (p PathView) Len() int { return len(p.cmds) }

// At is its ith command, or Close out of range.
func (p PathView) At(i int) PathCommand {
	if i < 0 || i >= len(p.cmds) {
		return PathCommand{Op: Close}
	}
	return p.cmds[i]
}

// turns reports whether a vertex kind's shape turns with the flow.
func (k *Kinds) turns(kind KindIndex) bool {
	if k == nil || kind < 0 || int(kind) >= len(k.vertex) {
		return false
	}
	s := k.vertex[kind].Shape
	return s >= Provided && int(s-Provided) < len(k.shapes) && k.shapes[s-Provided].def.Turns
}

// LabelBelow reports whether a vertex of the kind draws its label at
// its foot, under the figure and as wide as the text needs, rather than
// cut to its width: an actor's, or a provided shape's whose label goes
// below. A layout makes room for it.
func (k *Kinds) LabelBelow(kind KindIndex) bool {
	if k == nil || kind < 0 || int(kind) >= len(k.vertex) {
		return false
	}
	s := k.vertex[kind].Shape
	return s == Actor || s >= Provided && int(s-Provided) < len(k.shapes) && k.shapes[s-Provided].def.Label == LabelBelow
}

// Turned reports whether a vertex is drawn turned: its kind's shape
// turns with the flow, and the flow is top to bottom. Its size is then
// its kind's with the width and height swapped, and its ports' sides
// are turned, left to top and right to bottom.
func (d *Document) Turned(v VertexIndex) bool {
	return d.flow == TopToBottom && v >= 0 && int(v) < len(d.vertices) && d.kinds.turns(d.vertices[v].Kind)
}

// NewSize is the size a new vertex of the kind takes: the kind's, with
// the width and height swapped for a kind that turns in a top-to-bottom
// flow.
func (d *Document) NewSize(kind KindIndex) Size {
	if kind < 0 || int(kind) >= len(d.kinds.vertex) {
		return Size{}
	}
	s := d.kinds.vertex[kind].Size
	if d.flow == TopToBottom && d.kinds.turns(kind) {
		s.W, s.H = s.H, s.W
	}
	return s
}

// turnSizes swaps the width and height, and those of the least size, of
// every vertex of a kind that turns, live or not, as the flow changes:
// so a turning vertex keeps the size for its flow, and undoing the
// change swaps them back.
func (d *Document) turnSizes() {
	if !d.kinds.anyTurns {
		return
	}
	for i := range d.vertices {
		if v := &d.vertices[i]; d.kinds.turns(v.Kind) {
			v.Size.W, v.Size.H = v.Size.H, v.Size.W
			v.Min.W, v.Min.H = v.Min.H, v.Min.W
		}
	}
}
