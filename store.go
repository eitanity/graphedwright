package graphedwright

import (
	"errors"
	"io"
)

// Store is a host's data on a document's elements and on the document.
// graphedwright never reads the data and never draws it. It records the
// host's changes in its undo log as tokens the store issues, in order
// with every other command and within groups, and tells the store when
// one is to be undone, redone or forgotten. The store keys by element
// ID, which deletion, its undo and Compact keep.
//
// A Store is called at most once per entry a command, an undo or a redo
// takes, and never per draw. While one of its methods runs, every
// method that would change the document refuses, as under Lock, and
// Apply returns ErrReentered: a reaction that needs a command is the
// host's to apply once the call has returned. Its methods must not
// panic: TinyGo on wasm has no recover, and a panic ends the program.
type Store interface {
	// Do makes the change the token names. An error refuses the
	// HostChange, which then leaves the document and the log unchanged.
	Do(token uint64) error
	// Undo takes back the change of a token Do accepted.
	Undo(token uint64)
	// Redo makes the change again, after Undo.
	Redo(token uint64)
	// Forget is called once for each token Do accepted, when the log no
	// longer holds it: it fell off a full log, a new command replaced
	// what could be redone, Discard took it back, or ClearUndo or
	// Compact emptied the log. The store releases what it kept behind it.
	Forget(token uint64)
	// Released is given the IDs of the tombstones a Compact removed, so
	// that the store can free their data; it is not called when there
	// were none. The list is valid for the call only.
	Released(ids IDList)
}

// ErrReentered is Apply's refusal when it is called from inside a
// Store's method or another call into host code on the document's
// behalf.
var ErrReentered = errors.New("the document was called back from inside host code: apply the command after the call returns")

// errNoStore refuses a HostChange on a document without a store.
var errNoStore = errors.New("host change: the document has no store")

// errNilStore refuses SetStore(nil).
var errNilStore = errors.New("store is nil")

// errNilEncoder refuses EncodeData without an encoder or a writer.
var errNilEncoder = errors.New("data encoder or writer is nil")

// errLogHeld refuses SetStore once the log holds anything.
var errLogHeld = errors.New("the undo log is not empty: call ClearUndo before setting a store")

// host is a document's store, and what calling it needs.
type host struct {
	store   Store
	ids     []ID // the IDs Compact removes, for Released
	calling bool // a call into host code is running
}

// SetStore gives the document a host's store. It is refused for nil,
// while the document is locked, and once the undo log holds anything,
// so that every token the log will hold is the store's. It allocates
// the room Released needs, once.
func (d *Document) SetStore(s Store) error {
	switch {
	case d.host.calling:
		return ErrReentered
	case d.locked:
		return ErrLocked
	case s == nil:
		return errNilStore
	case d.log.n > 0:
		return errLogHeld
	}
	d.host.store = s
	if d.host.ids == nil {
		d.host.ids = make([]ID, 0, d.caps.Vertices+d.caps.Edges)
	}
	return nil
}

// enter marks a call into host code as running, the document locked
// while it runs, and returns whether it was locked before.
func (d *Document) enter() bool {
	was := d.locked
	d.locked, d.host.calling = true, true
	return was
}

// leave ends a call into host code.
func (d *Document) leave(was bool) { d.locked, d.host.calling = was, false }

// DataEncoder writes a host's data on an element, or on the document
// for the ID "", as one JSON value; ok is false when there is none. It
// is the encoding half of doc's Codec.
type DataEncoder interface {
	Encode(id ID, w io.Writer) (ok bool, err error)
}

// EncodeData calls the encoder for the element with the ID, or the
// document for "", with the document held as while a Store's method
// runs, so that the encoder cannot change what is being encoded. It
// works on a locked document, which stays locked; it is refused with
// ErrReentered from inside host code, and for a nil encoder or writer.
func (d *Document) EncodeData(c DataEncoder, id ID, w io.Writer) (bool, error) {
	switch {
	case d.host.calling:
		return false, ErrReentered
	case c == nil || w == nil:
		return false, errNilEncoder
	}
	was := d.enter()
	ok, err := c.Encode(id, w)
	d.leave(was)
	return ok, err
}

// hostChange applies a HostChange: the store's Do, then the entry.
func (d *Document) hostChange(c Command) error {
	if d.host.store == nil {
		return errNoStore
	}
	switch {
	case c.Vertex != None && c.Edge != None:
		return refusal("host change: name a vertex, an edge, or neither, not both")
	case c.Vertex != None && !d.vertexLive(c.Vertex):
		return refusal("vertex " + itoa(int(c.Vertex)) + " is not live")
	case c.Vertex == None && c.Edge != None && !d.edgeLive(c.Edge):
		return refusal("edge " + itoa(int(c.Edge)) + " is not live")
	}
	was := d.enter()
	err := d.host.store.Do(c.Token)
	d.leave(was)
	if err != nil {
		return err
	}
	d.seq++
	d.record(entry{cmd: c, seq: d.seq, vertex: c.Vertex, edge: c.Edge})
	d.touchHost(c.Vertex, c.Edge)
	return nil
}

// undoHost and redoHost hand an entry of a HostChange to the store.
func (d *Document) undoHost(en entry) {
	was := d.enter()
	d.host.store.Undo(en.cmd.Token)
	d.leave(was)
	d.touchHost(en.vertex, en.edge)
}

func (d *Document) redoHost(en entry) {
	was := d.enter()
	d.host.store.Redo(en.cmd.Token)
	d.leave(was)
	d.touchHost(en.vertex, en.edge)
}

// touchHost names the element a host change is on, if any, as changed.
func (d *Document) touchHost(v VertexIndex, e EdgeIndex) {
	switch {
	case v != None:
		d.touchVertex(v)
	case e != None:
		d.touchEdge(e)
	}
}

// record pushes an entry onto the log, first telling the store of the
// host changes the push drops.
func (d *Document) record(e entry) {
	if d.host.store != nil {
		d.dropping()
	}
	d.log.push(e)
}

// dropping tells the store of the host changes the next push drops:
// what could be redone, and the oldest entry when the log is full.
func (d *Document) dropping() {
	if l := &d.log; l.done == len(l.entries) {
		d.forget(&l.entries[l.start])
	} else {
		d.forgetFrom(l.done, l.n)
	}
}

// forgetFrom tells the store of the host changes among the entries from
// position from to position to of the log, oldest first, which it is
// about to drop.
func (d *Document) forgetFrom(from, to int) {
	l := &d.log
	// bound: the positions given, at most the log's depth
	for i := from; i < to && i < len(l.entries); i++ {
		d.forget(&l.entries[(l.start+i)%len(l.entries)])
	}
}

// forget tells the store that the log no longer holds an entry, when it
// is a host change.
func (d *Document) forget(e *entry) {
	if e.cmd.Op != HostChange {
		return
	}
	was := d.enter()
	d.host.store.Forget(e.cmd.Token)
	d.leave(was)
}

// clearLog empties the log, forgetting every host change it holds.
func (d *Document) clearLog() {
	d.forgetFrom(0, d.log.n)
	d.log.clear()
}

// gather notes the IDs of the tombstones Compact is about to remove,
// for Released, which empties the list again.
func (d *Document) gather() {
	h := &d.host
	if h.store == nil {
		return
	}
	for i := range d.vertices {
		if !d.vertices[i].Live {
			h.ids = append(h.ids, d.vertices[i].ID)
		}
	}
	for i := range d.edges {
		if !d.edges[i].Live {
			h.ids = append(h.ids, d.edges[i].ID)
		}
	}
}

// release hands the store the IDs gather noted, if there are any.
func (d *Document) release() {
	h := &d.host
	if h.store == nil || len(h.ids) == 0 {
		return
	}
	was := d.enter()
	h.store.Released(IDList{h.ids})
	d.leave(was)
	h.ids = h.ids[:0]
}
