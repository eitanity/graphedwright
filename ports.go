package graphedwright

import "strconv"

// The room a vertex with ports of its own grows to, so that their
// labels, drawn outside it beside them, are clear of each other.
const (
	// PortGap is the least distance between the ports on a left or
	// right side, and from its ends to the first and last: room for a
	// label above one port and below the next.
	PortGap = 32
	// PortLabelChar is the estimated width of a character of a port's
	// label.
	PortLabelChar = 6
)

// portSlot is the room along a top or bottom side a port takes: its
// label's width either side of it, and a gap.
func portSlot(p Port) float64 {
	return max(PortGap, float64(float64(2*runes(p.Name))*PortLabelChar)+16)
}

// OwnPorts reports how many of a vertex's ports are its own, added by
// AddPort after its kind's.
func (d *Document) OwnPorts(v VertexIndex) int {
	x := &d.Vertices[v]
	return len(x.Ports) - len(d.kinds.vertex[x.Kind].Ports)
}

// selfSized reports whether a vertex's size follows from what it holds
// besides other vertices: compartments, or ports of its own.
func (d *Document) selfSized(v VertexIndex) bool {
	return d.holdsCompartments(v) || d.OwnPorts(v) > 0
}

// portRoom is the least size of a vertex with ports of its own, whose
// ports are spaced evenly along their sides: a left or right side long
// enough for its ports PortGap apart, and a top or bottom one for its
// ports each the widest one's slot apart.
func (d *Document) portRoom(v VertexIndex) Size {
	if d.OwnPorts(v) == 0 {
		return Size{}
	}
	var n [4]int
	slot := 0.0
	for _, p := range d.Vertices[v].Ports {
		n[p.Side]++
		if p.Side == Top || p.Side == Bottom {
			slot = max(slot, portSlot(p))
		}
	}
	return Size{
		W: float64(max(n[Top], n[Bottom])+1) * slot,
		H: float64(max(n[Left], n[Right])+1) * PortGap,
	}
}

// addPort gives a vertex a port of its own after its others, named
// uniquely on it, within the capacity.
func (d *Document) addPort(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	v := &d.Vertices[c.Vertex]
	p := c.Port
	if p.Name == "" || p.Max < 0 || p.Side > Bottom {
		return refusal("a port needs a name, a side and a non-negative max")
	}
	if err := checkName("port", p.Name); err != nil {
		return err
	}
	if len(v.Ports) >= d.caps.PortsPerVertex {
		return refusal(itoa(len(v.Ports)+1) + " ports exceed the limit of " + itoa(d.caps.PortsPerVertex))
	}
	if d.portNamed(c.Vertex, p.Name) != None {
		return refusal("vertex " + d.vertexName(c.Vertex) + " has a port " + strconv.Quote(p.Name))
	}
	if !d.selfSized(c.Vertex) && !d.isContainer(c.Vertex) {
		// Its size till now is its own, which its ports grow it from.
		v.Min = v.Size
	}
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, at: len(v.Ports)})
	d.insertPort(c.Vertex, len(v.Ports), p)
	return nil
}

// deletePort removes a vertex's own port, named in Text, when no edge
// uses it; the edges at its later ports follow them down.
func (d *Document) deletePort(c Command) error {
	if !d.vertexLive(c.Vertex) {
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	}
	i := d.portNamed(c.Vertex, c.Text)
	switch {
	case i == None:
		return refusal("vertex " + d.vertexName(c.Vertex) + " has no port " + strconv.Quote(c.Text))
	case int(i) < len(d.kinds.vertex[d.Vertices[c.Vertex].Kind].Ports):
		return refusal("port " + strconv.Quote(c.Text) + " is the kind's, not the vertex's own")
	}
	if used := d.portUsers(c.Vertex, i); used != "" {
		return refusal("port " + strconv.Quote(c.Text) + " is used by " + used)
	}
	c.Port = d.Vertices[c.Vertex].Ports[i]
	d.seq++
	d.log.push(entry{cmd: c, seq: d.seq, vertex: c.Vertex, at: int(i)})
	d.removePort(c.Vertex, int(i))
	return nil
}

// portNamed is the index of a vertex's port with the name, or None.
func (d *Document) portNamed(v VertexIndex, name string) PortIndex {
	for i, p := range d.Vertices[v].Ports {
		if p.Name == name {
			return PortIndex(i) //nolint:gosec // bounded by the ports per vertex
		}
	}
	return None
}

// portUsers names the live edges at a vertex's port, or is empty.
func (d *Document) portUsers(v VertexIndex, p PortIndex) string {
	var names string
	for i := range d.Edges {
		e := &d.Edges[i]
		if e.Live && ((e.From == v && e.FromPort == p) || (e.To == v && e.ToPort == p)) {
			if names != "" {
				names += ", "
			}
			names += strconv.Quote(string(e.ID))
		}
	}
	return names
}

// insertPort puts a port at index i of a vertex's ports, the edges at
// its ports from i on following them up, and sizes the vertex to them.
func (d *Document) insertPort(v VertexIndex, i int, p Port) {
	x := &d.Vertices[v]
	x.Ports = x.Ports[:len(x.Ports)+1]
	copy(x.Ports[i+1:], x.Ports[i:])
	x.Ports[i] = p
	for k := range d.Edges {
		e := &d.Edges[k]
		if e.From == v {
			e.FromPort = shiftUp(e.FromPort, i)
		}
		if e.To == v {
			e.ToPort = shiftUp(e.ToPort, i)
		}
	}
	d.portsChanged(v)
}

// removePort takes out port i of a vertex, the edges at its later ports
// following them down, and sizes the vertex to those left.
func (d *Document) removePort(v VertexIndex, i int) {
	x := &d.Vertices[v]
	copy(x.Ports[i:], x.Ports[i+1:])
	x.Ports = x.Ports[:len(x.Ports)-1]
	for k := range d.Edges {
		e := &d.Edges[k]
		if e.From == v {
			e.FromPort = shiftDown(e.FromPort, i)
		}
		if e.To == v {
			e.ToPort = shiftDown(e.ToPort, i)
		}
	}
	d.portsChanged(v)
}

// shiftDown is an edge end's port once port i is removed: a later one
// moves down, and one at i, which only a deleted edge an undo can bring
// back has, is parked at gone(i) for the port's return.
func shiftDown(p PortIndex, i int) PortIndex {
	switch {
	case int(p) == i:
		return gone(i)
	case int(p) > i:
		return p - 1
	}
	return p
}

// shiftUp is an edge end's port once a port is put back at i: one parked
// there returns, and one at i or later moves up.
func shiftUp(p PortIndex, i int) PortIndex {
	switch {
	case p == gone(i):
		return PortIndex(i) //nolint:gosec // bounded by the ports per vertex
	case int(p) >= i:
		return p + 1
	}
	return p
}

// gone is where an edge end at a removed port i waits: below None, so
// that it names no port.
func gone(i int) PortIndex { return PortIndex(-2 - i) } //nolint:gosec // bounded by the ports per vertex

// portsChanged sizes a vertex to its ports when it is not a container,
// whose fit does, and names it and its edges, which its ports carry.
func (d *Document) portsChanged(v VertexIndex) {
	if !d.isContainer(v) {
		d.fitContent(v)
	}
	d.touchVertex(v)
	d.touchEdgesAt(v)
}
