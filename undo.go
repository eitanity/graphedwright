package graphedwright

import "math"

// entry is one applied command with what Undo needs: the sequence number
// that marked the tombstones it made, the element it touched, and the
// previous value of what it changed.
type entry struct {
	cmd     Command
	seq     uint64
	vertex  VertexIndex
	edge    EdgeIndex
	prev    Command
	placed  bool
	hadAttr bool
	at      int     // the attribute's index, for SetAttr
	layout  uint64  // the layout's number, naming its snapshots
	flows   [2]Flow // the flow before and after a layout
	ports   []Port
	min     Size   // a container's own size before a Resize
	group   uint64 // the group the command was applied in, undone and redone as one; 0 for none
}

// undoLog is a ring of entries: the oldest fall off when it is full, a
// new command clears what could be redone.
type undoLog struct {
	entries []entry
	ports   []Port // depth × portsPerVertex, the previous ports of SetPorts entries
	per     int    // ports per entry
	start   int    // ring index of the oldest entry
	n       int    // entries kept
	done    int    // entries applied; n − done can be redone
	groups  uint64 // groups begun
	group   uint64 // the group open, or 0
	open    int    // Group calls not yet ended, so a group within a group stays one
}

func newUndoLog(depth, portsPerVertex int) undoLog {
	return undoLog{entries: make([]entry, depth), ports: make([]Port, depth*portsPerVertex), per: portsPerVertex}
}

// next is the ring index the next push takes.
func (l *undoLog) next() int {
	if l.done == len(l.entries) {
		return l.start
	}
	return (l.start + l.done) % len(l.entries)
}

// portsFor is the slice of the ring's port slab for the entry at ring
// index i, with room for n ports.
func (l *undoLog) portsFor(i, n int) []Port {
	return l.ports[i*l.per : i*l.per+n]
}

// push records an applied command, in the group open, dropping anything
// that could be redone and, when the ring is full, the oldest entry.
func (l *undoLog) push(e entry) {
	if len(l.entries) == 0 {
		return
	}
	e.group = l.group
	if l.done == len(l.entries) {
		l.entries[l.start] = e
		l.start = (l.start + 1) % len(l.entries)
		l.n = l.done
		return
	}
	l.entries[(l.start+l.done)%len(l.entries)] = e
	l.done++
	l.n = l.done
}

// pop takes the last applied entry for Undo.
func (l *undoLog) pop() (entry, bool) {
	if l.done == 0 {
		return entry{}, false
	}
	l.done--
	return l.entries[(l.start+l.done)%len(l.entries)], true
}

// unpop takes the next undone entry for Redo.
func (l *undoLog) unpop() (entry, bool) {
	if l.done == l.n {
		return entry{}, false
	}
	e := l.entries[(l.start+l.done)%len(l.entries)]
	l.done++
	return e, true
}

// dropLayout drops the oldest entries up to and including the layout
// numbered k, when it is still applied, so its snapshots can be reused.
func (l *undoLog) dropLayout(k uint64) {
	// bound: the applied entries, at most the ring's length
	for i := 0; i < l.done; i++ {
		e := &l.entries[(l.start+i)%len(l.entries)]
		if e.cmd.Op == Layout && e.layout == k {
			l.start = (l.start + i + 1) % len(l.entries)
			l.done -= i + 1
			l.n -= i + 1
			return
		}
	}
}

// grouped reports whether the next entry for Undo, or for Redo when redo
// is true, is in group g, so it goes with the one just taken.
func (l *undoLog) grouped(g uint64, redo bool) bool {
	if g == 0 || len(l.entries) == 0 {
		return false
	}
	if redo {
		return l.done < l.n && l.entries[(l.start+l.done)%len(l.entries)].group == g
	}
	return l.done > 0 && l.entries[(l.start+l.done-1)%len(l.entries)].group == g
}

func (l *undoLog) clear() { l.start, l.n, l.done = 0, 0, 0 }

// Group makes the commands applied until the matching EndGroup one step
// for Undo and Redo: one user action that changes several elements, such
// as moving or deleting a selection. Groups within a group stay part of
// it. A group whose oldest entries fall off a full log is undone as far
// as it is kept.
func (d *Document) Group() {
	l := &d.log
	if l.open == 0 {
		if l.groups == math.MaxUint64 {
			l.groups = 0 // 2⁶⁴ groups: a reused number names no entry still kept
		}
		l.groups++
		l.group = l.groups
	}
	if l.open < math.MaxInt32 {
		l.open++
	}
}

// EndGroup ends the group the last Group began.
func (d *Document) EndGroup() {
	l := &d.log
	if l.open > 0 {
		l.open--
	}
	if l.open == 0 {
		l.group = 0
	}
}

// ClearUndo empties the undo log, so that what built the document, a
// decode or a host's setup, is not undone.
func (d *Document) ClearUndo() { d.log.clear() }

// CanUndo and CanRedo report whether there is anything to undo or redo.
func (d *Document) CanUndo() bool { return d.log.done > 0 }

// CanRedo reports whether an undone command can be applied again.
func (d *Document) CanRedo() bool { return d.log.done < d.log.n }

// Undo reverses the last applied command, or the whole group it was
// applied in, and reports what it touched;
// ok is false when there is nothing to undo.
func (d *Document) Undo() (Change, bool) {
	d.resetChange()
	en, ok := d.log.pop()
	if !ok {
		return Change{}, false
	}
	st, mv := d.undoOne(en)
	// bound: a group's entries, at most the log's depth
	for i := 0; i < len(d.log.entries) && d.log.grouped(en.group, false); i++ {
		next, _ := d.log.pop()
		s, m := d.undoOne(next)
		st, mv = st || s, mv || m
	}
	return d.settle(st, mv), true
}

// undoOne reverses one entry and reports whether it was structural and
// whether it moved anything.
func (d *Document) undoOne(en entry) (st, mv bool) {
	switch en.cmd.Op {
	case AddVertex:
		d.killVertex(en.vertex, en.seq)
	case DeleteVertex:
		d.reviveVertex(en.vertex, en.seq)
	case AddEdge:
		d.Edges[en.edge].Live = false
		d.Edges[en.edge].diedBy = en.seq
		d.touchEdge(en.edge)
	case DeleteEdge:
		d.Edges[en.edge].Live = true
		d.touchEdge(en.edge)
	case Move:
		d.moveTo(en.vertex, en.prev.Pos, en.placed)
	case Resize:
		d.setSize(en.vertex, en.prev.Size, en.min)
	case Relabel:
		d.setLabel(en.vertex, en.edge, en.prev.Text)
	case SetAttr:
		d.undoAttr(en)
	case SetPorts:
		v := &d.Vertices[en.vertex]
		v.Ports = v.Ports[:len(en.ports)]
		copy(v.Ports, en.ports)
		d.touchVertex(en.vertex)
	case Layout:
		// The positions the layout gave are kept for its redo.
		d.savePositions(d.layoutSnap(en.layout, true))
		d.loadPositions(d.layoutSnap(en.layout, false))
		d.Flow = en.flows[0]
	case Compact:
	case SetParent:
		d.putIn(en.vertex, en.prev.To)
	}
	return structural(en.cmd.Op), moves(en.cmd.Op)
}

// settle redoes what follows from the entries just undone or redone:
// the order after a structural change, and the containers' fit and the
// routes after a move.
func (d *Document) settle(st, mv bool) Change {
	if st {
		d.reorder()
	}
	if mv {
		d.fit()
		d.rebend()
	}
	return d.change(st)
}

// Redo applies the last undone command again, or the whole group it was
// applied in; ok is false when there is nothing to redo.
func (d *Document) Redo() (Change, bool) {
	d.resetChange()
	en, ok := d.log.unpop()
	if !ok {
		return Change{}, false
	}
	st, mv := d.redoOne(en)
	// bound: a group's entries, at most the log's depth
	for i := 0; i < len(d.log.entries) && d.log.grouped(en.group, true); i++ {
		next, _ := d.log.unpop()
		s, m := d.redoOne(next)
		st, mv = st || s, mv || m
	}
	return d.settle(st, mv), true
}

// redoOne applies one undone entry again and reports whether it was
// structural and whether it moved anything.
func (d *Document) redoOne(en entry) (st, mv bool) {
	switch en.cmd.Op {
	case AddVertex:
		d.reviveVertex(en.vertex, en.seq)
	case DeleteVertex:
		d.killVertex(en.vertex, en.seq)
	case AddEdge:
		d.Edges[en.edge].Live = true
		d.touchEdge(en.edge)
	case DeleteEdge:
		d.Edges[en.edge].Live = false
		d.Edges[en.edge].diedBy = en.seq
		d.touchEdge(en.edge)
	case Move:
		d.moveTo(en.vertex, en.cmd.Pos, true)
	case Resize:
		d.setSize(en.vertex, en.cmd.Size, en.cmd.Size)
	case Relabel:
		d.setLabel(en.vertex, en.edge, en.cmd.Text)
	case SetAttr:
		d.redoAttr(en)
	case SetPorts:
		v := &d.Vertices[en.vertex]
		kind := d.Schema.VertexKinds[v.Kind]
		v.Ports = v.Ports[:len(kind.Ports)]
		copy(v.Ports, kind.Ports)
		d.touchVertex(en.vertex)
	case Layout:
		d.loadPositions(d.layoutSnap(en.layout, true))
		d.Flow = en.flows[1]
	case Compact:
	case SetParent:
		d.putIn(en.vertex, en.cmd.To)
	}
	return structural(en.cmd.Op), moves(en.cmd.Op)
}

// setLabel writes a label on a vertex, or on an edge when v is None.
func (d *Document) setLabel(v VertexIndex, e EdgeIndex, text string) {
	if v != None {
		d.Vertices[v].Label = text
		d.touchVertex(v)
		return
	}
	d.Edges[e].Label = text
	d.touchEdge(e)
}

// structural reports whether an op changes what the layout has to place.
func structural(op Op) bool {
	switch op {
	case AddVertex, DeleteVertex, AddEdge, DeleteEdge, Layout, Compact, SetParent:
		return true
	case Move, Resize, Relabel, SetAttr, SetPorts:
		return false
	}
	return false
}
