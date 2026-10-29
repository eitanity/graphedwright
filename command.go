package graphedwright

import "strconv"

// Op is what a command does.
type Op uint8

// The operations a command can carry.
const (
	AddVertex    Op = iota // Kind, ID (minted if empty), Text as the label, Pos, Size (the kind's if zero)
	DeleteVertex           // Vertex; its edges go with it
	AddEdge                // Kind, ID, From, To, FromPort, ToPort, Text as the label, Weight
	DeleteEdge             // Edge
	Move                   // Vertex, Pos; the vertex becomes placed
	Resize                 // Vertex, Size; a container's becomes its own (Vertex.Min)
	Relabel                // Vertex or Edge (Vertex None for an edge), Text
	SetAttr                // Vertex or Edge, Key, Value; an empty Value removes the attribute
	SetPorts               // Vertex: the ports go back to the kind's
	Layout                 // Text "all" unplaces every vertex; Value names a flow to change to, or is empty; the layout is stale either way
	Compact                // renumbers; see Document.Compact for the index maps
	SetParent              // Vertex, To: the container it goes in, or None
)

// Command is one change to a document. Which fields matter depends on
// Op; the rest are ignored.
type Command struct {
	Op       Op
	Vertex   VertexIndex
	Edge     EdgeIndex
	Kind     KindIndex
	From, To VertexIndex
	FromPort PortIndex
	ToPort   PortIndex
	ID       ID
	Pos      Point
	Size     Size
	Text     string
	Key      string
	Value    string
	Weight   float64
}

// Change names what a command moved: the vertices and edges touched,
// and whether the layout is stale. The slices are the document's
// scratch and are valid until the next Apply, Undo or Redo.
type Change struct {
	Vertices []VertexIndex
	Edges    []EdgeIndex
	Layout   bool
}

// Apply validates the command against the schema and the capacities and
// either leaves the document untouched, returning an error that names
// the rule, or applies it and records its inverse for Undo.
func (d *Document) Apply(c Command) (Change, error) {
	d.resetChange()
	var err error
	switch c.Op {
	case AddVertex:
		err = d.addVertex(c)
	case DeleteVertex:
		err = d.deleteVertex(c)
	case AddEdge:
		err = d.addEdge(c)
	case DeleteEdge:
		err = d.deleteEdge(c)
	case Move:
		err = d.move(c)
	case Resize:
		err = d.resize(c)
	case Relabel:
		err = d.relabel(c)
	case SetAttr:
		err = d.setAttr(c)
	case SetPorts:
		err = d.setPorts(c)
	case Layout:
		err = d.layout(c)
	case Compact:
		d.compact()
	case SetParent:
		err = d.setParent(c)
	default:
		err = refusal("unknown op " + itoa(int(c.Op)))
	}
	if err != nil {
		return Change{}, err
	}
	if structural(c.Op) {
		d.reorder()
	}
	if moves(c.Op) {
		d.fit()
		d.rebend()
	}
	return d.change(structural(c.Op)), nil
}

// resetChange starts a new change report, which names each element
// once.
func (d *Document) resetChange() {
	d.changeV = d.changeV[:0]
	d.changeE = d.changeE[:0]
	d.stamp++
	if d.stamp == 0 {
		clear(d.vstamp)
		clear(d.estamp)
		d.stamp = 1
	}
}

// change is the report of the touched indices gathered so far.
func (d *Document) change(layoutStale bool) Change {
	return Change{Vertices: d.changeV, Edges: d.changeE, Layout: layoutStale}
}

func (d *Document) touchVertex(v VertexIndex) {
	if d.vstamp[v] == d.stamp {
		return
	}
	d.vstamp[v] = d.stamp
	d.changeV = d.changeV[:len(d.changeV)+1]
	d.changeV[len(d.changeV)-1] = v
}

// touchEdgesAt records every live edge at a vertex as changed, since its
// drawing follows the vertex.
func (d *Document) touchEdgesAt(v VertexIndex) {
	// bound: the out list visits each edge of v once
	for e := d.outHead[v]; e != None; e = d.nextOut[e] {
		if d.Edges[e].Live {
			d.touchEdge(EdgeIndex(e))
		}
	}
	// bound: the in list visits each edge of v once
	for e := d.inHead[v]; e != None; e = d.nextIn[e] {
		if d.Edges[e].Live && d.Edges[e].From != v {
			d.touchEdge(EdgeIndex(e))
		}
	}
}

// EdgesAt appends the live edges at a vertex to dst, within its capacity.
func (d *Document) EdgesAt(v VertexIndex, dst []EdgeIndex) []EdgeIndex {
	if !d.vertexLive(v) {
		return dst
	}
	// bound: the out list visits each edge of v once
	for e := d.outHead[v]; e != None; e = d.nextOut[e] {
		if d.Edges[e].Live && len(dst) < cap(dst) {
			dst = append(dst, EdgeIndex(e))
		}
	}
	// bound: the in list visits each edge of v once
	for e := d.inHead[v]; e != None; e = d.nextIn[e] {
		if d.Edges[e].Live && d.Edges[e].From != v && len(dst) < cap(dst) {
			dst = append(dst, EdgeIndex(e))
		}
	}
	return dst
}

// touchEdge records an edge as changed, and with it the others between
// the same two vertices, whose lanes follow from it (Parallel).
func (d *Document) touchEdge(e EdgeIndex) {
	if d.estamp[e] == d.stamp {
		return
	}
	d.addChangeE(e)
	ed := &d.Edges[e]
	// bound: the out list visits each edge of the vertex once
	for o := d.outHead[ed.From]; o != None; o = d.nextOut[o] {
		if d.parallel(e, EdgeIndex(o)) && d.estamp[o] != d.stamp {
			d.addChangeE(EdgeIndex(o))
		}
	}
	// bound: the in list visits each edge of the vertex once
	for o := d.inHead[ed.From]; o != None; o = d.nextIn[o] {
		if d.parallel(e, EdgeIndex(o)) && d.estamp[o] != d.stamp {
			d.addChangeE(EdgeIndex(o))
		}
	}
}

func (d *Document) addChangeE(e EdgeIndex) {
	d.estamp[e] = d.stamp
	d.changeE = d.changeE[:len(d.changeE)+1]
	d.changeE[len(d.changeE)-1] = e
}

// parallel reports whether o is a live edge joining the same two
// vertices as e, either way, e itself included.
func (d *Document) parallel(e, o EdgeIndex) bool {
	a, b := &d.Edges[e], &d.Edges[o]
	return b.Live && (a.From == b.From && a.To == b.To || a.From == b.To && a.To == b.From)
}

// Parallel is where a live edge falls among the edges drawn straight
// between the same two vertices, which a renderer runs in lanes apart:
// the live edges joining them, either way, with no bends and no ports.
// k of them come before e by index, of n in all, e included. A
// self-loop, or an edge with bends or a port, is alone: 0 of 1.
func (d *Document) Parallel(e EdgeIndex) (k, n int) {
	ed := &d.Edges[e]
	if !d.lane(ed) || ed.From == ed.To {
		return 0, 1
	}
	// bound: the out list visits each edge of the vertex once
	for o := d.outHead[ed.From]; o != None; o = d.nextOut[o] {
		if d.parallel(e, EdgeIndex(o)) && d.lane(&d.Edges[o]) {
			n++
			if EdgeIndex(o) < e {
				k++
			}
		}
	}
	// bound: the in list visits each edge of the vertex once
	for o := d.inHead[ed.From]; o != None; o = d.nextIn[o] {
		if d.Edges[o].From != ed.From && d.parallel(e, EdgeIndex(o)) && d.lane(&d.Edges[o]) {
			n++
			if EdgeIndex(o) < e {
				k++
			}
		}
	}
	return k, n
}

// lane reports whether an edge runs in a lane: no bends and no ports.
func (d *Document) lane(e *Edge) bool {
	return len(e.Bends) == 0 && e.FromPort == None && e.ToPort == None
}

// checkLabel refuses a label past the byte limit or one XML cannot
// carry.
func (d *Document) checkLabel(text string) error {
	if len(text) > d.caps.LabelBytes {
		return refusal("label of " + itoa(len(text)) + " bytes exceeds the limit of " + itoa(d.caps.LabelBytes))
	}
	return checkText("label", text)
}

// addVertex validates and adds a vertex at the next slot.
func (d *Document) addVertex(c Command) error {
	if c.Kind < 0 || int(c.Kind) >= len(d.Schema.VertexKinds) {
		return refusal("vertex kind " + itoa(int(c.Kind)) + " out of range")
	}
	kind := d.Schema.VertexKinds[c.Kind]
	if len(d.Vertices) == d.caps.Vertices {
		return limitError("vertex", d.caps.Vertices)
	}
	if kind.Max > 0 && int(d.kindCount[c.Kind]) >= kind.Max {
		return refusal("kind " + strconv.Quote(kind.Name) + " limit " + itoa(kind.Max) + " reached")
	}
	if err := d.checkLabel(c.Text); err != nil {
		return err
	}
	if err := d.checkID(c.ID); err != nil {
		return err
	}
	if c.ID != "" && d.idUsed(c.ID) {
		return refusal("id " + strconv.Quote(string(c.ID)) + " already used")
	}
	if !point(c.Pos) {
		return refusal(posRule)
	}
	if c.Size != (Size{}) && (!extent(c.Size.W) || !extent(c.Size.H)) {
		return refusal(sizeRule)
	}
	n := len(d.Vertices)
	d.Vertices = d.Vertices[:n+1]
	v := &d.Vertices[n]
	*v = Vertex{ID: c.ID, Kind: c.Kind, Label: c.Text, Pos: c.Pos, Size: c.Size, Live: true, Parent: None}
	if v.ID == "" {
		v.ID = d.mintVertexID()
	}
	if v.Size == (Size{}) {
		v.Size = kind.Size
	} else if kind.Container {
		v.Min = v.Size
	}
	v.Ports = d.portSlab[n*d.caps.PortsPerVertex : n*d.caps.PortsPerVertex+len(kind.Ports)]
	copy(v.Ports, kind.Ports)
	v.Attrs = d.vattrSlab[n*d.caps.AttrsPerElement : n*d.caps.AttrsPerElement]
	v.Attrs = fillDefaults(v.Attrs, kind.Attrs)
	d.outHead[n], d.inHead[n] = None, None
	d.kindCount[c.Kind]++
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: VertexIndex(n)}) //nolint:gosec // bounded by the capacity
	d.touchVertex(VertexIndex(n))                                 //nolint:gosec // bounded by the capacity
	return nil
}

// fillDefaults appends every attribute with a default; the slice has the
// capacity for all of a kind's specifications.
func fillDefaults(attrs []Attr, specs []AttrSpec) []Attr {
	for _, s := range specs {
		if s.Default != "" {
			attrs = attrs[:len(attrs)+1]
			attrs[len(attrs)-1] = Attr{Key: s.Key, Value: s.Default}
		}
	}
	return attrs
}

// deleteVertex tombstones a vertex and the live edges at it.
func (d *Document) deleteVertex(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	d.seq++
	d.killVertex(c.Vertex, d.seq)
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex})
	return nil
}

// killVertex tombstones v, what it holds when it is a container, and
// the live edges at any of them, under the sequence number.
func (d *Document) killVertex(v VertexIndex, seq uint64) {
	d.kill(v, seq)
	if d.isContainer(v) {
		for i := range d.Vertices {
			if u := VertexIndex(i); d.Vertices[i].Live && d.Within(u, v) { //nolint:gosec // bounded by the capacity
				d.kill(u, seq)
			}
		}
	}
	for i := range d.Edges {
		if e := &d.Edges[i]; e.Live && (!d.Vertices[e.From].Live || !d.Vertices[e.To].Live) {
			e.Live = false
			e.diedBy = seq
			d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		}
	}
}

// kill tombstones one vertex under the sequence number.
func (d *Document) kill(v VertexIndex, seq uint64) {
	d.Vertices[v].Live = false
	d.Vertices[v].diedBy = seq
	d.kindCount[d.Vertices[v].Kind]--
	d.touchVertex(v)
}

// reviveVertex brings back v and the vertices and edges that died with
// it.
func (d *Document) reviveVertex(v VertexIndex, seq uint64) {
	for i := range d.Vertices {
		u := &d.Vertices[i]
		if !u.Live && u.diedBy == seq && d.Within(VertexIndex(i), v) { //nolint:gosec // bounded by the capacity
			u.Live = true
			d.kindCount[u.Kind]++
			d.touchVertex(VertexIndex(i)) //nolint:gosec // bounded by the capacity
		}
	}
	for i := range d.Edges {
		if e := &d.Edges[i]; !e.Live && e.diedBy == seq && d.Vertices[e.From].Live && d.Vertices[e.To].Live {
			e.Live = true
			d.touchEdge(EdgeIndex(i)) //nolint:gosec // bounded by the capacity
		}
	}
}

// addEdge validates and adds an edge at the next slot.
func (d *Document) addEdge(c Command) error {
	if err := d.checkEdgeEnds(c); err != nil {
		return err
	}
	if len(d.Edges) == d.caps.Edges {
		return limitError("edge", d.caps.Edges)
	}
	if err := d.checkLabel(c.Text); err != nil {
		return err
	}
	if err := d.checkID(c.ID); err != nil {
		return err
	}
	if c.ID != "" && d.idUsed(c.ID) {
		return refusal("id " + strconv.Quote(string(c.ID)) + " already used")
	}
	if !(c.Weight >= 0) || c.Weight > MaxCoord {
		return refusal(weightRule)
	}
	if d.Schema.Acyclic {
		if err := d.cycleThrough(c.From, c.To); err != nil {
			return err
		}
	}
	n := len(d.Edges)
	d.Edges = d.Edges[:n+1]
	e := &d.Edges[n]
	*e = Edge{ID: c.ID, Kind: c.Kind, From: c.From, To: c.To, FromPort: c.FromPort, ToPort: c.ToPort, Label: c.Text, Weight: c.Weight, Live: true}
	if e.ID == "" {
		e.ID = d.mintEdgeID()
	}
	e.Attrs = d.eattrSlab[n*d.caps.AttrsPerElement : n*d.caps.AttrsPerElement]
	e.Attrs = fillDefaults(e.Attrs, d.Schema.EdgeKinds[c.Kind].Attrs)
	d.nextOut[n], d.outHead[c.From] = d.outHead[c.From], int32(n) //nolint:gosec // bounded by the capacity
	d.nextIn[n], d.inHead[c.To] = d.inHead[c.To], int32(n)        //nolint:gosec // bounded by the capacity
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, edge: EdgeIndex(n)}) //nolint:gosec // bounded by the capacity
	d.touchEdge(EdgeIndex(n))                                 //nolint:gosec // bounded by the capacity
	return nil
}

// checkEdgeEnds validates an edge's kind, its two vertices and their
// kinds, and the ports with their limits.
func (d *Document) checkEdgeEnds(c Command) error {
	if c.Kind < 0 || int(c.Kind) >= len(d.Schema.EdgeKinds) {
		return refusal("edge kind " + itoa(int(c.Kind)) + " out of range")
	}
	for _, v := range [2]VertexIndex{c.From, c.To} {
		if !d.vertexLive(v) {
			return refusal("vertex " + itoa(int(v)) + " is not live")
		}
	}
	kind := d.Schema.EdgeKinds[c.Kind]
	from, to := d.Vertices[c.From], d.Vertices[c.To]
	if !kind.allows(from.Kind, to.Kind) {
		return refusal("edge kind " + strconv.Quote(kind.Name) + " cannot join " + d.kindName(from.Kind) + " to " + d.kindName(to.Kind))
	}
	if d.Schema.VertexKinds[to.Kind].Root {
		return refusal("vertex " + d.vertexName(c.To) + " of kind " + d.kindName(to.Kind) + " takes no incoming edge")
	}
	if d.Schema.VertexKinds[from.Kind].Leaf {
		return refusal("vertex " + d.vertexName(c.From) + " of kind " + d.kindName(from.Kind) + " takes no outgoing edge")
	}
	if err := d.checkPort(c.From, c.FromPort, true); err != nil {
		return err
	}
	return d.checkPort(c.To, c.ToPort, false)
}

// checkPort validates a port index on a vertex (None when the vertex has
// no ports) and that the port has room for one more edge.
func (d *Document) checkPort(v VertexIndex, p PortIndex, out bool) error {
	ports := d.Vertices[v].Ports
	if p == None {
		if len(ports) > 0 {
			return refusal("vertex " + d.vertexName(v) + " has ports; the edge must name one")
		}
		return nil
	}
	if p < 0 || int(p) >= len(ports) {
		return refusal("port " + itoa(int(p)) + " of vertex " + d.vertexName(v) + " out of range")
	}
	if ports[p].Max > 0 && d.portEdges(v, p, out) >= ports[p].Max {
		return refusal("port " + strconv.Quote(ports[p].Name) + " of vertex " + d.vertexName(v) + " is full (" + itoa(ports[p].Max) + " of " + itoa(ports[p].Max) + ")")
	}
	return nil
}

// portEdges counts the live edges leaving (out) or entering a port.
func (d *Document) portEdges(v VertexIndex, p PortIndex, out bool) int {
	n := 0
	if out {
		// bound: the out list visits each edge of v once
		for e := d.outHead[v]; e != None; e = d.nextOut[e] {
			if d.Edges[e].Live && d.Edges[e].FromPort == p {
				n++
			}
		}
		return n
	}
	// bound: the in list visits each edge of v once
	for e := d.inHead[v]; e != None; e = d.nextIn[e] {
		if d.Edges[e].Live && d.Edges[e].ToPort == p {
			n++
		}
	}
	return n
}

// deleteEdge tombstones an edge.
func (d *Document) deleteEdge(c Command) error {
	if !d.edgeLive(c.Edge) {
		return refusal("edge " + itoa(int(c.Edge)) + " is not live")
	}
	d.seq++
	d.Edges[c.Edge].Live = false
	d.Edges[c.Edge].diedBy = d.seq
	d.log.push(entry{cmd: c, seq: d.seq, edge: c.Edge})
	d.touchEdge(c.Edge)
	return nil
}

// move places a vertex.
func (d *Document) move(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	if !point(c.Pos) {
		return refusal(posRule)
	}
	v := &d.Vertices[c.Vertex]
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, prev: Command{Pos: v.Pos}, placed: v.Placed})
	d.moveTo(c.Vertex, c.Pos, true)
	return nil
}

// moveTo puts a vertex at a position with its flag, and what it holds
// by the same offset.
func (d *Document) moveTo(v VertexIndex, p Point, placed bool) {
	dx, dy := p.X-d.Vertices[v].Pos.X, p.Y-d.Vertices[v].Pos.Y
	d.Vertices[v].Pos, d.Vertices[v].Placed = p, placed
	d.touchVertex(v)
	d.touchEdgesAt(v)
	d.carry(v, dx, dy)
	d.shift(v, dx, dy)
}

// resize sets a vertex's size, and a container's own size, which the
// fit then sizes it by.
func (d *Document) resize(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	if !extent(c.Size.W) || !extent(c.Size.H) {
		return refusal(sizeRule)
	}
	v := &d.Vertices[c.Vertex]
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, prev: Command{Size: v.Size}, min: v.Min})
	d.setSize(c.Vertex, c.Size, c.Size)
	return nil
}

// setSize gives a vertex a size and, when it is a container, its own
// size, and names it and its edges as changed.
func (d *Document) setSize(v VertexIndex, size, own Size) {
	d.Vertices[v].Size = size
	if d.isContainer(v) {
		d.Vertices[v].Min = own
	}
	d.touchVertex(v)
	d.touchEdgesAt(v)
}

// relabel sets the label of a vertex, or of an edge when Vertex is None.
func (d *Document) relabel(c Command) error {
	if err := d.checkLabel(c.Text); err != nil {
		return err
	}
	if c.Vertex != None {
		if !d.vertexLive(c.Vertex) {
			return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
		}
		v := &d.Vertices[c.Vertex]
		d.seq++
		d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, edge: None, prev: Command{Text: v.Label}})
		v.Label = c.Text
		d.touchVertex(c.Vertex)
		return nil
	}
	if !d.edgeLive(c.Edge) {
		return refusal("edge " + itoa(int(c.Edge)) + " is not live")
	}
	e := &d.Edges[c.Edge]
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: None, edge: c.Edge, prev: Command{Text: e.Label}})
	e.Label = c.Text
	d.touchEdge(c.Edge)
	return nil
}

// setPorts puts a vertex's ports back to its kind's, when the edges at
// it fit the kind's ports.
func (d *Document) setPorts(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	v := &d.Vertices[c.Vertex]
	kind := d.Schema.VertexKinds[v.Kind]
	for i := range d.Edges {
		e := &d.Edges[i]
		if !e.Live {
			continue
		}
		if (e.From == c.Vertex && int(e.FromPort) >= len(kind.Ports)) || (e.To == c.Vertex && int(e.ToPort) >= len(kind.Ports)) {
			return refusal("edge " + strconv.Quote(string(e.ID)) + " uses a port the kind " + d.kindName(v.Kind) + " does not have")
		}
	}
	d.seq++
	en := entry{cmd: c, seq: d.seq, vertex: c.Vertex}
	en.ports = d.log.portsFor(d.log.next(), len(v.Ports))
	copy(en.ports, v.Ports)
	d.log.push(en)
	v.Ports = v.Ports[:len(kind.Ports)]
	copy(v.Ports, kind.Ports)
	d.touchVertex(c.Vertex)
	return nil
}

// layout marks the layout stale, sets the flow a Value names and, for
// "all", unplaces every vertex. An unknown flow is refused.
// The positions before it are kept in the next of UndoLayouts snapshots
// for its undo; the layout that last used that snapshot, and everything
// before it, fall out of the log.
func (d *Document) layout(c Command) error {
	flow := d.Flow
	if c.Value != "" {
		f, ok := FlowNamed(c.Value)
		if !ok {
			return refusal("unknown flow " + strconv.Quote(c.Value))
		}
		flow = f
	}
	d.layouts++
	if n := uint64(d.caps.UndoLayouts); d.layouts > n { //nolint:gosec // bounded by MaxCap
		d.log.dropLayout(d.layouts - n)
	}
	d.savePositions(d.layoutSnap(d.layouts, false))
	if c.Text == "all" {
		for i := range d.Vertices {
			if d.Vertices[i].Live {
				d.Vertices[i].Placed = false
			}
		}
	}
	for i := range d.Vertices {
		d.touchVertex(VertexIndex(i)) //nolint:gosec // bounded by the capacity
	}
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, layout: d.layouts, flows: [2]Flow{d.Flow, flow}})
	d.Flow = flow
	return nil
}

// layoutSnap is the snapshot of positions before (or after) the layout
// numbered k, and the count of slots it covers.
func (d *Document) layoutSnap(k uint64, after bool) (pos []Point, placed []bool, n *int) {
	i := 2 * int(k%uint64(d.caps.UndoLayouts)) //nolint:gosec // bounded by MaxCap
	if after {
		i++
	}
	v := d.caps.Vertices
	return d.layoutPos[i*v : (i+1)*v], d.layoutPlaced[i*v : (i+1)*v], &d.layoutN[i]
}

// savePositions copies every slot's position and flag into a snapshot.
func (d *Document) savePositions(pos []Point, placed []bool, n *int) {
	*n = len(d.Vertices)
	for i := range d.Vertices {
		pos[i], placed[i] = d.Vertices[i].Pos, d.Vertices[i].Placed
	}
}

// loadPositions puts the position and flag of every slot the snapshot
// covers back and names those vertices as changed. Slots made since are
// dead, their additions undone, and keep their own positions for a redo.
func (d *Document) loadPositions(pos []Point, placed []bool, n *int) {
	for i := range d.Vertices[:*n] {
		d.Vertices[i].Pos, d.Vertices[i].Placed = pos[i], placed[i]
		d.touchVertex(VertexIndex(i)) //nolint:gosec // bounded by the capacity
	}
}
