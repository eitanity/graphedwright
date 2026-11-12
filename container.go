package graphedwright

import "math"

// A container is a vertex of a kind marked Container. A vertex is in at
// most one container, its Parent, and containers nest. A container that
// holds a live vertex is sized to the box around what it holds, with
// ContainerPad on every side and ContainerTitle more above for its
// label, a line more for each line of the label after its first
// (TitleHeight), and grown right and down to its own size, Min, when that is
// larger; moving it moves what it holds, and deleting it deletes what
// it holds. A container that holds nothing is its own size, or its
// kind's when it has none, with its top left corner where it was.

// ContainerPad is the margin inside a container around what it holds.
const ContainerPad = 16

// ContainerTitle is the room above what a container holds for its
// label.
const ContainerTitle = 20

// BandRoom is how much further out than its padding a container's sides
// across the flow stand where an edge from outside runs in to a member
// that others stand wholly before or after along the flow: a layout
// takes such a run round them, along a band beside what the container
// holds, and the room keeps the band clear of the side. Either way the
// run may go, so both sides stand out.
const BandRoom = ContainerPad

// Depth is the number of containers a vertex is in; 0 for an index out
// of range.
func (d *Document) Depth(v VertexIndex) int {
	if v < 0 || int(v) >= len(d.vertices) {
		return 0
	}
	n := 0
	// bound: the parent chain is acyclic, so at most the vertex count
	for p := d.vertices[v].Parent; p != None && n < len(d.vertices); p = d.vertices[p].Parent {
		n++
	}
	return n
}

// Within reports whether v is c or is in c, directly or through the
// containers between; false for a v out of range.
func (d *Document) Within(v, c VertexIndex) bool {
	if v < 0 || int(v) >= len(d.vertices) {
		return false
	}
	// bound: the parent chain is acyclic, so at most the vertex count
	for n := 0; v != None && n <= len(d.vertices); n++ {
		if v == c {
			return true
		}
		v = d.vertices[v].Parent
	}
	return false
}

// An edge between a container and a vertex it holds, as a transition
// from a composite state to one of its substates, runs inside the
// container from its side: in from the side facing back along the flow
// to the vertex, or out from the vertex to the side facing on, level
// with the vertex's end. That side stands further out by its texts'
// room (EnclosedRoom), so that the run holds them.

// Enclosing reports whether vertex from holds vertex to, 1, or to holds
// from, -1, directly or through the containers between; 0 when neither
// does, for a loop, and for an index out of range.
func (d *Document) Enclosing(from, to VertexIndex) int {
	switch {
	case from == to:
		return 0
	case d.Within(to, from):
		return 1
	case d.Within(from, to):
		return -1
	}
	return 0
}

// EnclosedRoom is how much further out than its padding container c's
// side facing back along the flow, back, and the side facing on, on,
// stand for the edges that run in from the one and out to the other
// (Enclosing): each the room of the widest text among them, its label
// or its kind's stereotype, at CharWidth a character and 8 either side;
// 0 where none runs. A straight edge, drawn between centres, has none.
func (d *Document) EnclosedRoom(c VertexIndex) (back, on float64) {
	if c < 0 || int(c) >= len(d.vertices) {
		return 0, 0
	}
	// bound: the out list visits each edge of c once
	for o := d.outHead[c]; o != None; o = d.nextOut[o] {
		back = math.Max(back, d.enclosedText(EdgeIndex(o), 1))
	}
	// bound: the in list visits each edge of c once
	for o := d.inHead[c]; o != None; o = d.nextIn[o] {
		on = math.Max(on, d.enclosedText(EdgeIndex(o), -1))
	}
	return back, on
}

// enclosedText is the room EnclosedRoom gives edge e's texts, 0 for an
// edge that is not live, is straight or is not Enclosing the way given.
func (d *Document) enclosedText(e EdgeIndex, way int) float64 {
	x := &d.edges[e]
	k := &d.kinds.edge[x.Kind]
	if !x.Live || k.Route == Straight || d.Enclosing(x.From, x.To) != way {
		return 0
	}
	return float64(float64(max(d.EdgeLabelRunes(e), runes(k.Stereotype)))*CharWidth) + 16
}

// isContainer reports whether the vertex's kind is a container.
func (d *Document) isContainer(v VertexIndex) bool {
	return d.kinds.vertex[d.vertices[v].Kind].Container
}

// hasContainers reports whether the document holds a live container.
func (d *Document) hasContainers() bool {
	for i, k := range d.kinds.vertex {
		if k.Container && d.kindCount[i] > 0 {
			return true
		}
	}
	return false
}

// setParent puts a vertex in a container, or in none with To None,
// when the container's kind may hold the vertex's and the container is
// not the vertex or in it.
func (d *Document) setParent(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	if c.To != None {
		if !d.vertexLive(c.To) {
			return refusal("vertex " + itoa(int(c.To)) + " is not live")
		}
		k := d.kinds.vertex[d.vertices[c.To].Kind]
		if !k.Container {
			return refusal("vertex " + d.vertexName(c.To) + " of kind " + d.kindName(d.vertices[c.To].Kind) + " is not a container")
		}
		if d.Within(c.To, c.Vertex) {
			return refusal("vertex " + d.vertexName(c.Vertex) + " cannot go inside itself or what it holds")
		}
		if !kindIn(k.Contains, d.vertices[c.Vertex].Kind) {
			return refusal("container kind " + d.kindName(d.vertices[c.To].Kind) + " cannot hold kind " + d.kindName(d.vertices[c.Vertex].Kind))
		}
	}
	v := &d.vertices[c.Vertex]
	if v.Parent == c.To {
		return nil
	}
	d.seq++
	d.record(entry{cmd: c, seq: d.seq, vertex: c.Vertex, prev: Command{To: v.Parent}})
	d.putIn(c.Vertex, c.To)
	return nil
}

// putIn sets a vertex's container and names it and its edges as changed.
func (d *Document) putIn(v, parent VertexIndex) {
	d.vertices[v].Parent = parent
	d.touchVertex(v)
	d.touchEdgesAt(v)
}

// shift moves what a container holds by the offset, with the container,
// and names what moved.
func (d *Document) shift(c VertexIndex, dx, dy float64) {
	if !d.isContainer(c) || (dx == 0 && dy == 0) {
		return
	}
	for i := range d.vertices {
		u := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if u != c && d.vertices[i].Live && d.Within(u, c) {
			d.vertices[i].Pos.X += dx
			d.vertices[i].Pos.Y += dy
			d.touchVertex(u)
			d.touchEdgesAt(u)
		}
	}
}

// Fit sizes every vertex with compartments to them and every container
// that holds a live vertex to what it holds,
// for a layout engine after it has placed the vertices; the change is
// not reported. Every command that moves a vertex fits the containers
// itself.
func (d *Document) Fit() {
	if d.locked {
		return
	}
	d.resetChange()
	d.fit()
	d.rebend()
}

// fit sizes every container that holds a live vertex to the box around
// what it holds, deepest first so that a nested container is sized
// before the one it is in, and names each that changed.
func (d *Document) fit() {
	if !d.hasContainers() && !d.hasCompartments() {
		return
	}
	deepest := 0
	for i := range d.vertices {
		if d.vertices[i].Live {
			d.depth[i] = int32(d.Depth(VertexIndex(i))) //nolint:gosec // bounded by the capacity
			deepest = max(deepest, int(d.depth[i]))
		}
		b := d.box[fitSlots*i : fitSlots*i+fitSlots]
		b[0], b[1], b[2], b[3] = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		b[4], b[5], b[6], b[7] = math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(1)
	}
	d.markCrossing()
	// bound: the deepest nesting, at most the vertex count
	for k := deepest; k >= 0; k-- {
		for i := range d.vertices {
			if d.vertices[i].Live && int(d.depth[i]) == k {
				d.fitOne(VertexIndex(i)) //nolint:gosec // bounded by the capacity
			}
		}
	}
}

// fitSlots is how many numbers the fit gathers for each container: the
// box x0 y0 x1 y1 and the band span (bandSpan).
const fitSlots = 8

// markCrossing marks each vertex that an edge from outside its container
// runs to, or into: those on the way from each end of an edge up to the
// innermost container holding both, but for an edge between a container
// and what it holds, which runs in from the container's own side.
func (d *Document) markCrossing() {
	clear(d.crossing)
	for i := range d.edges {
		e := &d.edges[i]
		if !e.Live || e.From == e.To || d.Enclosing(e.From, e.To) != 0 {
			continue
		}
		level := d.vertices[e.From].Parent
		// bound: the parent chain is acyclic, so at most the vertex count
		for n := 0; level != None && !d.Within(e.To, level) && n <= len(d.vertices); n++ {
			level = d.vertices[level].Parent
		}
		for _, v := range [2]VertexIndex{e.From, e.To} {
			// bound: the parent chain is acyclic, so at most the vertex count
			for n := 0; v != None && d.vertices[v].Parent != level && n <= len(d.vertices); n++ {
				d.crossing[v] = true
				v = d.vertices[v].Parent
			}
		}
	}
}

// bandSpan gathers, into the four numbers b, what decides whether a
// container stands out by BandRoom: the least far end along the flow of
// what it holds and the greatest near end, and the greatest near end and
// least far end of those an edge from outside runs to (crossing).
// A member wholly before a crossed one, or wholly after, is then one a
// run to it goes round (banded).
func bandSpan(b []float64, lo, hi float64, crossed bool) {
	b[0], b[1] = math.Min(b[0], hi), math.Max(b[1], lo)
	if crossed {
		b[2], b[3] = math.Max(b[2], lo), math.Min(b[3], hi)
	}
}

// banded reports whether the band span b has a member wholly before or
// after one an edge from outside runs to.
func banded(b []float64) bool { return b[0] <= b[2] || b[1] >= b[3] }

// fitOne sizes a container to the box gathered from what it holds, or
// to its own size when it holds nothing, and a vertex to its
// compartments, and adds the vertex's own box to its container's.
func (d *Document) fitOne(i VertexIndex) {
	if d.holdsCompartments(i) && !d.isContainer(i) {
		d.fitContent(i)
	}
	v := &d.vertices[i]
	if b := d.box[fitSlots*int(i) : fitSlots*int(i)+fitSlots]; d.isContainer(i) {
		var pos Point
		var size Size
		if b[0] <= b[2] {
			back, on := d.EnclosedRoom(i)
			band := 0.0
			if banded(b[4:]) {
				band = BandRoom
			}
			if d.flow == TopToBottom {
				b[0], b[1], b[2], b[3] = b[0]-band, b[1]-back, b[2]+band, b[3]+on
			} else {
				b[0], b[1], b[2], b[3] = b[0]-back, b[1]-band, b[2]+on, b[3]+band
			}
			title := d.TitleHeight(i)
			pos = Point{X: float64((b[0] + b[2]) / 2), Y: float64((b[1]+b[3])/2) - float64(title/2)}
			size = Size{W: b[2] - b[0] + 2*ContainerPad, H: b[3] - b[1] + 2*ContainerPad + title}
			// Grown to its own size, and to its ports' room, from the
			// top left corner, which stays where what it holds puts it.
			pos, size = grow(pos, size, v.Min)
			pos, size = grow(pos, size, d.portRoom(i))
		} else {
			size = d.fitted(i, d.ownSize(i))
			pos = Point{X: v.Pos.X + float64((size.W-v.Size.W)/2), Y: v.Pos.Y + float64((size.H-v.Size.H)/2)}
		}
		if pos != v.Pos || size != v.Size {
			v.Pos, v.Size = pos, size
			d.touchVertex(i)
			d.touchEdgesAt(i)
		}
	}
	if v.Parent != None {
		b := d.box[fitSlots*int(v.Parent) : fitSlots*int(v.Parent)+fitSlots]
		b[0] = math.Min(b[0], v.Pos.X-float64(v.Size.W/2))
		b[1] = math.Min(b[1], v.Pos.Y-float64(v.Size.H/2))
		b[2] = math.Max(b[2], v.Pos.X+float64(v.Size.W/2))
		b[3] = math.Max(b[3], v.Pos.Y+float64(v.Size.H/2))
		lo, hi := v.Pos.X-float64(v.Size.W/2), v.Pos.X+float64(v.Size.W/2)
		if d.flow == TopToBottom {
			lo, hi = v.Pos.Y-float64(v.Size.H/2), v.Pos.Y+float64(v.Size.H/2)
		}
		bandSpan(b[4:], lo, hi, d.crossing[i])
	}
}

// grow is a box at pos of the size grown to at least the least size,
// its top left corner staying where it is.
func grow(pos Point, size, least Size) (Point, Size) {
	if size.W < least.W {
		pos.X += float64((least.W - size.W) / 2)
		size.W = least.W
	}
	if size.H < least.H {
		pos.Y += float64((least.H - size.H) / 2)
		size.H = least.H
	}
	return pos, size
}

// ownSize is the size of a container that holds nothing, or of a vertex
// whose compartments are empty: its own, or its kind's when it has none.
func (d *Document) ownSize(c VertexIndex) Size {
	if m := d.vertices[c].Min; m != (Size{}) {
		return m
	}
	return d.kinds.vertex[d.vertices[c].Kind].Size
}

// Containers lists the live containers in the order they are drawn,
// each under those after it: a container before what it holds, and
// containers side by side, in the same container or in none, in index
// order, each followed by all it holds. So one container and its
// contents are drawn wholly over or under another and its contents,
// never between them. The list reads the document's own, valid until
// the next command.
func (d *Document) Containers() VertexList { return VertexList{d.order} }

// reorder lists the live containers in drawing order: a walk of the
// tree they make, each container's children taken in index order.
func (d *Document) reorder() {
	d.order = d.order[:0]
	if !d.hasContainers() {
		return
	}
	for i := range d.vertices {
		d.kid[i] = None
	}
	root := VertexIndex(None)
	// Backwards, so that each list of children is in index order.
	for i := len(d.vertices) - 1; i >= 0; i-- {
		c := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if !d.vertices[i].Live || !d.isContainer(c) {
			continue
		}
		if p := d.vertices[i].Parent; p == None {
			d.sib[i], root = root, c
		} else {
			d.sib[i], d.kid[p] = d.kid[p], c
		}
	}
	// bound: each step lists a container, once each, and climbs past
	// only containers already listed
	for c := root; c != None; {
		d.order = append(d.order, c)
		if d.kid[c] != None {
			c = d.kid[c]
			continue
		}
		// bound: the parent chain is acyclic, so at most the vertex count
		for n := 0; c != None && d.sib[c] == None && n <= len(d.vertices); n++ {
			c = d.vertices[c].Parent
		}
		if c != None {
			c = d.sib[c]
		}
	}
}

// moves reports whether an op can move or resize a vertex, or change
// what a container holds, so that the containers are fitted after it.
func moves(op Op) bool {
	switch op {
	case DeleteVertex, Move, Resize, SetParent, Layout:
		return true
	case AddVertex, AddEdge, DeleteEdge, Relabel, SetAttr, SetPorts, Compact, AddPort, DeletePort, HostChange:
	}
	return false
}
