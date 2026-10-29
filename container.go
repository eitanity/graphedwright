package graphedwright

import "math"

// A container is a vertex of a kind marked Container. A vertex is in at
// most one container, its Parent, and containers nest. A container that
// holds a live vertex is sized to the box around what it holds, with
// ContainerPad on every side and ContainerTitle more above for its
// label, and grown right and down to its own size, Min, when that is
// larger; moving it moves what it holds, and deleting it deletes what
// it holds. A container that holds nothing is its own size, or its
// kind's when it has none, with its top left corner where it was.

// ContainerPad is the margin inside a container around what it holds.
const ContainerPad = 16

// ContainerTitle is the room above what a container holds for its
// label.
const ContainerTitle = 20

// Depth is the number of containers a vertex is in.
func (d *Document) Depth(v VertexIndex) int {
	n := 0
	// bound: the parent chain is acyclic, so at most the vertex count
	for p := d.Vertices[v].Parent; p != None && n < len(d.Vertices); p = d.Vertices[p].Parent {
		n++
	}
	return n
}

// Within reports whether v is c or is in c, directly or through the
// containers between.
func (d *Document) Within(v, c VertexIndex) bool {
	// bound: the parent chain is acyclic, so at most the vertex count
	for n := 0; v != None && n <= len(d.Vertices); n++ {
		if v == c {
			return true
		}
		v = d.Vertices[v].Parent
	}
	return false
}

// isContainer reports whether the vertex's kind is a container.
func (d *Document) isContainer(v VertexIndex) bool {
	return d.Schema.VertexKinds[d.Vertices[v].Kind].Container
}

// hasContainers reports whether the document holds a live container.
func (d *Document) hasContainers() bool {
	for i, k := range d.Schema.VertexKinds {
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
		k := d.Schema.VertexKinds[d.Vertices[c.To].Kind]
		if !k.Container {
			return refusal("vertex " + d.vertexName(c.To) + " of kind " + d.kindName(d.Vertices[c.To].Kind) + " is not a container")
		}
		if d.Within(c.To, c.Vertex) {
			return refusal("vertex " + d.vertexName(c.Vertex) + " cannot go inside itself or what it holds")
		}
		if !kindIn(k.Contains, d.Vertices[c.Vertex].Kind) {
			return refusal("container kind " + d.kindName(d.Vertices[c.To].Kind) + " cannot hold kind " + d.kindName(d.Vertices[c.Vertex].Kind))
		}
	}
	v := &d.Vertices[c.Vertex]
	if v.Parent == c.To {
		return nil
	}
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, prev: Command{To: v.Parent}})
	d.putIn(c.Vertex, c.To)
	return nil
}

// putIn sets a vertex's container and names it and its edges as changed.
func (d *Document) putIn(v, parent VertexIndex) {
	d.Vertices[v].Parent = parent
	d.touchVertex(v)
	d.touchEdgesAt(v)
}

// shift moves what a container holds by the offset, with the container,
// and names what moved.
func (d *Document) shift(c VertexIndex, dx, dy float64) {
	if !d.isContainer(c) || (dx == 0 && dy == 0) {
		return
	}
	for i := range d.Vertices {
		u := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if u != c && d.Vertices[i].Live && d.Within(u, c) {
			d.Vertices[i].Pos.X += dx
			d.Vertices[i].Pos.Y += dy
			d.touchVertex(u)
			d.touchEdgesAt(u)
		}
	}
}

// Fit sizes every container that holds a live vertex to what it holds,
// for a layout engine after it has placed the vertices; the change is
// not reported. Every command that moves a vertex fits the containers
// itself.
func (d *Document) Fit() {
	d.resetChange()
	d.fit()
	d.rebend()
}

// fit sizes every container that holds a live vertex to the box around
// what it holds, deepest first so that a nested container is sized
// before the one it is in, and names each that changed.
func (d *Document) fit() {
	if !d.hasContainers() {
		return
	}
	deepest := 0
	for i := range d.Vertices {
		if d.Vertices[i].Live {
			d.depth[i] = int32(d.Depth(VertexIndex(i))) //nolint:gosec // bounded by the capacity
			deepest = max(deepest, int(d.depth[i]))
		}
		b := d.box[4*i : 4*i+4]
		b[0], b[1], b[2], b[3] = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	}
	// bound: the deepest nesting, at most the vertex count
	for k := deepest; k >= 0; k-- {
		for i := range d.Vertices {
			if d.Vertices[i].Live && int(d.depth[i]) == k {
				d.fitOne(VertexIndex(i)) //nolint:gosec // bounded by the capacity
			}
		}
	}
}

// fitOne sizes a container to the box gathered from what it holds, or
// to its own size when it holds nothing, and adds the vertex's own box
// to its container's.
func (d *Document) fitOne(i VertexIndex) {
	v := &d.Vertices[i]
	if b := d.box[4*i : 4*i+4]; d.isContainer(i) {
		var pos Point
		var size Size
		if b[0] <= b[2] {
			pos = Point{X: float64((b[0] + b[2]) / 2), Y: float64((b[1]+b[3])/2) - ContainerTitle/2}
			size = Size{W: b[2] - b[0] + 2*ContainerPad, H: b[3] - b[1] + 2*ContainerPad + ContainerTitle}
			// Grown to its own size from the top left corner, which
			// stays where what it holds puts it.
			if size.W < v.Min.W {
				pos.X += float64((v.Min.W - size.W) / 2)
				size.W = v.Min.W
			}
			if size.H < v.Min.H {
				pos.Y += float64((v.Min.H - size.H) / 2)
				size.H = v.Min.H
			}
		} else {
			size = d.ownSize(i)
			pos = Point{X: v.Pos.X + float64((size.W-v.Size.W)/2), Y: v.Pos.Y + float64((size.H-v.Size.H)/2)}
		}
		if pos != v.Pos || size != v.Size {
			v.Pos, v.Size = pos, size
			d.touchVertex(i)
			d.touchEdgesAt(i)
		}
	}
	if v.Parent != None {
		b := d.box[4*int(v.Parent) : 4*int(v.Parent)+4]
		b[0] = math.Min(b[0], v.Pos.X-float64(v.Size.W/2))
		b[1] = math.Min(b[1], v.Pos.Y-float64(v.Size.H/2))
		b[2] = math.Max(b[2], v.Pos.X+float64(v.Size.W/2))
		b[3] = math.Max(b[3], v.Pos.Y+float64(v.Size.H/2))
	}
}

// ownSize is the size of a container that holds nothing: its own, or
// its kind's when it has none.
func (d *Document) ownSize(c VertexIndex) Size {
	if m := d.Vertices[c].Min; m != (Size{}) {
		return m
	}
	return d.Schema.VertexKinds[d.Vertices[c].Kind].Size
}

// Containers lists the live containers in the order they are drawn,
// each under those after it: a container before what it holds, and
// containers side by side, in the same container or in none, in index
// order, each followed by all it holds. So one container and its
// contents are drawn wholly over or under another and its contents,
// never between them. The slice is the document's and valid until the
// next command.
func (d *Document) Containers() []VertexIndex { return d.order }

// reorder lists the live containers in drawing order: a walk of the
// tree they make, each container's children taken in index order.
func (d *Document) reorder() {
	d.order = d.order[:0]
	if !d.hasContainers() {
		return
	}
	for i := range d.Vertices {
		d.kid[i] = None
	}
	root := VertexIndex(None)
	// Backwards, so that each list of children is in index order.
	for i := len(d.Vertices) - 1; i >= 0; i-- {
		c := VertexIndex(i) //nolint:gosec // bounded by the capacity
		if !d.Vertices[i].Live || !d.isContainer(c) {
			continue
		}
		if p := d.Vertices[i].Parent; p == None {
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
		for n := 0; c != None && d.sib[c] == None && n <= len(d.Vertices); n++ {
			c = d.Vertices[c].Parent
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
	case AddVertex, AddEdge, DeleteEdge, Relabel, SetAttr, SetPorts, Compact:
	}
	return false
}
