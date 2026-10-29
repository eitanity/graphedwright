package graphedwright

// ID is a stable, host-visible name of a vertex or an edge. The document
// mints one as v1, v2, … or e1, e2, … when the host gives none, and a
// minted ID never repeats within a document.
type ID string

// VertexIndex, EdgeIndex, KindIndex and PortIndex number the elements of
// a document and its schema; None is the absent one.
type (
	VertexIndex int32
	EdgeIndex   int32
	KindIndex   int32
	PortIndex   int32
)

// None is the absent index of any kind.
const None = -1

// Point is a position in document units.
type Point struct{ X, Y float64 }

// Size is a width and a height in document units.
type Size struct{ W, H float64 }

// Viewport is the part of the document shown: the SVG viewBox.
type Viewport struct{ X, Y, W, H float64 }

// Flow is the direction a document's layers run in: the way a layout
// places them and the way a curved or orthogonal edge leaves and enters
// a vertex without ports.
type Flow uint8

// The flows.
const (
	LeftToRight Flow = iota // layers left to right; edges leave the right side and enter the left
	TopToBottom             // layers top to bottom, as a dependency tree; edges leave the bottom and enter the top
)

// String is the flow's name, as a Layout command and the document
// format name it.
func (f Flow) String() string {
	switch f {
	case LeftToRight:
		return "left-to-right"
	case TopToBottom:
		return "top-to-bottom"
	}
	return "flow " + itoa(int(f))
}

// FlowNamed is the flow of a name, or ok = false.
func FlowNamed(name string) (Flow, bool) {
	for _, f := range [...]Flow{LeftToRight, TopToBottom} {
		if f.String() == name {
			return f, true
		}
	}
	return LeftToRight, false
}

// Side is the side of a vertex a port sits on.
type Side uint8

// The sides of a vertex.
const (
	Left Side = iota
	Right
	Top
	Bottom
)

// Port is a place on a vertex's side where edges attach. Max is the
// number of edges it takes; 0 is unbounded.
type Port struct {
	Name string
	Side Side
	Max  int
}

// Attr is one key and value on a vertex or an edge, shown as data-* on
// its SVG element.
type Attr struct{ Key, Value string }

// Vertex is one vertex of a document.
type Vertex struct {
	ID     ID
	Kind   KindIndex // into Schema.VertexKinds
	Label  string
	Pos    Point
	Size   Size   // from the kind, overridable
	Min    Size   // a container's own size: what it is when it holds nothing, and least when it holds something; zero for its kind's size and no least
	Ports  []Port // from the kind
	Attrs  []Attr
	Placed bool        // false until a layout or a drag gives it a position
	Live   bool        // false for a tombstone
	Parent VertexIndex // the container it is in, or None
	diedBy uint64
}

// Edge is one edge of a document.
type Edge struct {
	ID       ID
	Kind     KindIndex // into Schema.EdgeKinds
	From, To VertexIndex
	FromPort PortIndex
	ToPort   PortIndex
	Label    string
	Attrs    []Attr
	Weight   float64 // drives stroke width when the kind says so; 0 = none
	Live     bool
	Bends    []Point // the points it is drawn through between its ends, or none; see SetBends
	diedBy   uint64
	route    []Point // the bends it was given, in the slab
	routed   ends    // where its ends were then
}

// Caps are the capacities fixed when a document is made. Everything the
// document holds is allocated then; an edit past a capacity is refused.
type Caps struct {
	Vertices        int
	Edges           int
	PortsPerVertex  int
	AttrsPerElement int
	LabelBytes      int
	UndoDepth       int
	UndoLayouts     int // layouts the undo log can reach back across
	BendsPerEdge    int // bend points per edge capacity, shared among the edges
}

// DefaultCaps are the capacities a document has when the host gives none.
func DefaultCaps() Caps {
	return Caps{Vertices: 4096, Edges: 16384, PortsPerVertex: 8, AttrsPerElement: 16, LabelBytes: 256, UndoDepth: 256, UndoLayouts: 8, BendsPerEdge: 16}
}

// Document is a directed graph with typed vertices and edges, a schema
// that says what is allowed, positions for drawing, and a viewport.
// Vertices and Edges are by index; a deleted entry is a tombstone until
// Compact. Every change goes through Apply.
type Document struct {
	Schema   Schema
	Vertices []Vertex
	Edges    []Edge
	View     Viewport
	Flow     Flow // the way the layers run: set by a Layout command, or by a loader
	caps     Caps

	portSlab  []Port
	vattrSlab []Attr
	eattrSlab []Attr
	bendSlab  []Point
	bendN     int // bend points handed out since ClearBends

	// Adjacency as intrusive lists over the edge indices; tombstones
	// stay in the lists and are skipped.
	outHead, inHead []int32
	nextOut, nextIn []int32

	kindCount    []int32
	nextVertexID uint64
	nextEdgeID   uint64
	seq          uint64
	log          undoLog

	// Scratch for the cycle search, the change report and Compact.
	mark    []uint32
	gen     uint32
	stack   []int32
	parent  []int32
	changeV []VertexIndex
	changeE []EdgeIndex
	vmap    []VertexIndex
	emap    []EdgeIndex
	vmapN   int
	emapN   int

	// Every vertex slot's position before and after each layout the log
	// still holds: a ring of UndoLayouts pairs of snapshots, each
	// covering the slots that existed when it was taken.
	layoutPos    []Point
	layoutPlaced []bool
	layoutN      []int
	layouts      uint64 // layouts applied, numbering each

	// A change names each element once: the stamp of the change that
	// last named it, per slot.
	vstamp, estamp []uint32
	stamp          uint32

	// Scratch for fitting the containers: each vertex's depth and the
	// box around what each holds, x0 y0 x1 y1.
	depth []int32
	box   []float64

	// The live containers in drawing order, and per container the
	// first container it holds and the next beside it, for building it.
	order    []VertexIndex
	kid, sib []VertexIndex
}

// New makes an empty document for the schema with the capacities,
// allocating everything it will ever hold. A zero capacity takes its
// default.
func New(schema Schema, caps Caps) (*Document, error) {
	caps = caps.filled()
	if err := caps.check(); err != nil {
		return nil, err
	}
	if err := checkSchema(schema, caps); err != nil {
		return nil, err
	}
	d := &Document{Schema: schema, caps: caps}
	d.Vertices = make([]Vertex, 0, caps.Vertices)
	d.Edges = make([]Edge, 0, caps.Edges)
	d.portSlab = make([]Port, caps.Vertices*caps.PortsPerVertex)
	d.vattrSlab = make([]Attr, caps.Vertices*caps.AttrsPerElement)
	d.eattrSlab = make([]Attr, caps.Edges*caps.AttrsPerElement)
	d.bendSlab = make([]Point, caps.Edges*caps.BendsPerEdge)
	d.outHead = make([]int32, caps.Vertices)
	d.inHead = make([]int32, caps.Vertices)
	d.nextOut = make([]int32, caps.Edges)
	d.nextIn = make([]int32, caps.Edges)
	d.kindCount = make([]int32, len(schema.VertexKinds))
	d.mark = make([]uint32, caps.Vertices)
	d.stack = make([]int32, caps.Vertices)
	d.parent = make([]int32, caps.Vertices)
	d.changeV = make([]VertexIndex, 0, caps.Vertices)
	d.changeE = make([]EdgeIndex, 0, caps.Edges)
	d.vmap = make([]VertexIndex, caps.Vertices)
	d.emap = make([]EdgeIndex, caps.Edges)
	d.log = newUndoLog(caps.UndoDepth, caps.PortsPerVertex)
	d.layoutPos = make([]Point, 2*caps.UndoLayouts*caps.Vertices)
	d.layoutPlaced = make([]bool, 2*caps.UndoLayouts*caps.Vertices)
	d.layoutN = make([]int, 2*caps.UndoLayouts)
	d.vstamp = make([]uint32, caps.Vertices)
	d.estamp = make([]uint32, caps.Edges)
	d.depth = make([]int32, caps.Vertices)
	d.box = make([]float64, 4*caps.Vertices)
	d.order = make([]VertexIndex, 0, caps.Vertices)
	d.kid = make([]VertexIndex, caps.Vertices)
	d.sib = make([]VertexIndex, caps.Vertices)
	return d, nil
}

// Caps are the document's capacities.
func (d *Document) Caps() Caps { return d.caps }

// filled puts the default in every zero capacity.
func (c Caps) filled() Caps {
	def := DefaultCaps()
	if c.Vertices == 0 {
		c.Vertices = def.Vertices
	}
	if c.Edges == 0 {
		c.Edges = def.Edges
	}
	if c.PortsPerVertex == 0 {
		c.PortsPerVertex = def.PortsPerVertex
	}
	if c.AttrsPerElement == 0 {
		c.AttrsPerElement = def.AttrsPerElement
	}
	if c.LabelBytes == 0 {
		c.LabelBytes = def.LabelBytes
	}
	if c.UndoDepth == 0 {
		c.UndoDepth = def.UndoDepth
	}
	if c.UndoLayouts == 0 {
		c.UndoLayouts = def.UndoLayouts
	}
	if c.BendsPerEdge == 0 {
		c.BendsPerEdge = def.BendsPerEdge
	}
	return c
}

// MaxCap bounds every capacity so that indices fit an int32 and the
// slabs stay within reason.
const MaxCap = 1 << 24

// check refuses a negative capacity or one beyond MaxCap.
func (c Caps) check() error {
	for _, v := range [...]struct {
		name string
		n    int
	}{{"vertices", c.Vertices}, {"edges", c.Edges}, {"ports per vertex", c.PortsPerVertex}, {"attributes per element", c.AttrsPerElement}, {"label bytes", c.LabelBytes}, {"undo depth", c.UndoDepth}, {"undo layouts", c.UndoLayouts}, {"bends per edge", c.BendsPerEdge}} {
		if v.n < 0 || v.n > MaxCap {
			return capError(v.name, v.n)
		}
	}
	return c.checkBytes()
}

// maxBytes bounds the memory the capacities may ask for: each is within
// MaxCap, but their products need not be.
const maxBytes = 1 << 30

// checkBytes refuses capacities whose document and layout would take
// more than maxBytes, naming the largest part. The sizes are those of
// the structures on a 64-bit target, rounded up, and the layout's share
// counts its eight placeholder nodes per edge (layout.dummiesPerEdge);
// the estimate needs to be of the right size, not exact.
func (c Caps) checkBytes() error {
	v, e := uint64(c.Vertices), uint64(c.Edges) //nolint:gosec // within MaxCap, checked above
	parts := [...]struct {
		name  string
		bytes uint64
	}{
		{"vertices", v * 400},
		{"edges", e * 1200},
		{"vertices × ports per vertex", v * uint64(c.PortsPerVertex) * 32},                           //nolint:gosec // within MaxCap
		{"vertices × attributes per element", v * uint64(c.AttrsPerElement) * 32},                    //nolint:gosec // within MaxCap
		{"edges × attributes per element", e * uint64(c.AttrsPerElement) * 32},                       //nolint:gosec // within MaxCap
		{"edges × bends per edge", e * uint64(c.BendsPerEdge) * 16},                                  //nolint:gosec // within MaxCap
		{"undo depth × ports per vertex", uint64(c.UndoDepth) * (400 + uint64(c.PortsPerVertex)*32)}, //nolint:gosec // within MaxCap
		{"undo layouts × vertices", 2 * uint64(c.UndoLayouts) * v * 17},                              //nolint:gosec // within MaxCap
	}
	var total uint64
	largest := 0
	for i, p := range parts {
		total += p.bytes
		if p.bytes > parts[largest].bytes {
			largest = i
		}
	}
	if total > maxBytes {
		return refusal("capacities would take about " + utoa(total>>20) + " MiB, over the limit of " + utoa(maxBytes>>20) + " MiB; the largest part is " + parts[largest].name)
	}
	return nil
}

// Live reports whether vertex v is in range and not a tombstone.
func (d *Document) vertexLive(v VertexIndex) bool {
	return v >= 0 && int(v) < len(d.Vertices) && d.Vertices[v].Live
}

// edgeLive reports whether edge e is in range and not a tombstone.
func (d *Document) edgeLive(e EdgeIndex) bool {
	return e >= 0 && int(e) < len(d.Edges) && d.Edges[e].Live
}

// VertexByID finds a live vertex by its ID, or None.
func (d *Document) VertexByID(id ID) VertexIndex {
	for i := range d.Vertices {
		if d.Vertices[i].Live && d.Vertices[i].ID == id {
			return VertexIndex(i) //nolint:gosec // bounded by the capacity
		}
	}
	return None
}

// EdgeByID finds a live edge by its ID, or None.
func (d *Document) EdgeByID(id ID) EdgeIndex {
	for i := range d.Edges {
		if d.Edges[i].Live && d.Edges[i].ID == id {
			return EdgeIndex(i) //nolint:gosec // bounded by the capacity
		}
	}
	return None
}

// idUsed reports whether any vertex or edge, live or not, carries the ID.
// A tombstone keeps its ID so that an undo can bring it back unchanged.
func (d *Document) idUsed(id ID) bool {
	for i := range d.Vertices {
		if d.Vertices[i].ID == id {
			return true
		}
	}
	for i := range d.Edges {
		if d.Edges[i].ID == id {
			return true
		}
	}
	return false
}

// mintVertexID is the next unused v<n>.
func (d *Document) mintVertexID() ID {
	// bound: every used ID is distinct, so at most len(Vertices)+len(Edges)+1 tries
	for {
		d.nextVertexID++
		id := ID("v" + utoa(d.nextVertexID))
		if !d.idUsed(id) {
			return id
		}
	}
}

// mintEdgeID is the next unused e<n>.
func (d *Document) mintEdgeID() ID {
	// bound: every used ID is distinct, so at most len(Vertices)+len(Edges)+1 tries
	for {
		d.nextEdgeID++
		id := ID("e" + utoa(d.nextEdgeID))
		if !d.idUsed(id) {
			return id
		}
	}
}

// LiveVertices counts the vertices that are not tombstones.
func (d *Document) LiveVertices() int {
	n := 0
	for i := range d.Vertices {
		if d.Vertices[i].Live {
			n++
		}
	}
	return n
}

// LiveEdges counts the edges that are not tombstones.
func (d *Document) LiveEdges() int {
	n := 0
	for i := range d.Edges {
		if d.Edges[i].Live {
			n++
		}
	}
	return n
}

// Place sets a vertex's position and its Placed flag outside the
// command log: for a layout engine after a Layout command, whose undo
// keeps the positions it sets, which leaves the flag false so the vertex
// is laid out again next time, and for a loader restoring a pinned
// vertex. It reports
// whether the vertex was live.
func (d *Document) Place(v VertexIndex, p Point, placed bool) bool {
	if !d.vertexLive(v) || !point(p) {
		return false
	}
	d.Vertices[v].Pos, d.Vertices[v].Placed = p, placed
	return true
}
