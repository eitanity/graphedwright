package graphedwright_test

import (
	"errors"
	"runtime"
	"slices"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
)

// ledger counts what a document tells a store, per token, so that a test
// can check that every token Do accepted is forgotten exactly once and
// no token Do refused is ever forgotten.
type ledger struct {
	accepted  map[uint64]bool // Do accepted it; false if Do refused it
	forgotten map[uint64]int
	next      uint64
}

func newLedger() ledger {
	return ledger{accepted: map[uint64]bool{}, forgotten: map[uint64]int{}}
}

// issue is a fresh token.
func (l *ledger) issue() uint64 { l.next++; return l.next }

func (l *ledger) forget(t *testing.T, token uint64) {
	t.Helper()
	if !l.accepted[token] {
		t.Errorf("Forget(%d) of a token Do did not accept", token)
	}
	l.forgotten[token]++
	if l.forgotten[token] > 1 {
		t.Errorf("Forget(%d) called %d times", token, l.forgotten[token])
	}
}

// settled checks that every accepted token has been forgotten once and
// no other has.
func (l *ledger) settled(t *testing.T) {
	t.Helper()
	for tok, ok := range l.accepted {
		if n := l.forgotten[tok]; ok && n != 1 {
			t.Errorf("token %d accepted, forgotten %d times", tok, n)
		}
	}
}

// pending is the number of accepted tokens not yet forgotten.
func (l *ledger) pending() int {
	n := 0
	for tok, ok := range l.accepted {
		if ok && l.forgotten[tok] == 0 {
			n++
		}
	}
	return n
}

// dist is a frequency or value distribution with its parameters.
type dist struct {
	Name   string
	Params []float64
}

// recordStore keeps structured records per vertex: a list of
// distributions, keyed by the vertex's ID. Behind each token is the list
// before and after.
type recordStore struct {
	t       *testing.T
	ledger  ledger
	byID    map[gw.ID][]dist
	changes map[uint64]recordChange
	refuse  bool // Do refuses
}

type recordChange struct {
	id            gw.ID
	before, after []dist
}

func newRecordStore(t *testing.T) *recordStore {
	return &recordStore{t: t, ledger: newLedger(), byID: map[gw.ID][]dist{}, changes: map[uint64]recordChange{}}
}

// set issues the token of a change to the vertex's distributions.
func (s *recordStore) set(id gw.ID, after []dist) uint64 {
	tok := s.ledger.issue()
	s.changes[tok] = recordChange{id: id, before: slices.Clone(s.byID[id]), after: after}
	return tok
}

func (s *recordStore) Do(tok uint64) error {
	if s.refuse {
		s.ledger.accepted[tok] = false
		return errors.New("distributions do not sum to one")
	}
	s.ledger.accepted[tok] = true
	c := s.changes[tok]
	s.byID[c.id] = c.after
	return nil
}

func (s *recordStore) Undo(tok uint64) { c := s.changes[tok]; s.byID[c.id] = c.before }
func (s *recordStore) Redo(tok uint64) { c := s.changes[tok]; s.byID[c.id] = c.after }
func (s *recordStore) Forget(tok uint64) {
	s.ledger.forget(s.t, tok)
	delete(s.changes, tok)
}

// Released: a record store keeps a vertex's records after it is gone.
func (s *recordStore) Released(gw.IDList) {}

// blobStore keeps one opaque blob per edge, and settings of the
// document's own. Behind each token is a diff: the key and the blob
// before and after.
type blobStore struct {
	t        *testing.T
	ledger   ledger
	blobs    map[gw.ID][]byte
	settings map[string]string
	changes  map[uint64]blobChange
	released []gw.ID
	releases int // calls of Released
}

type blobChange struct {
	edge          gw.ID // "" for a setting
	key           string
	before, after []byte
}

func newBlobStore(t *testing.T) *blobStore {
	return &blobStore{t: t, ledger: newLedger(), blobs: map[gw.ID][]byte{}, settings: map[string]string{}, changes: map[uint64]blobChange{}}
}

func (s *blobStore) blob(id gw.ID, b []byte) uint64 {
	tok := s.ledger.issue()
	s.changes[tok] = blobChange{edge: id, before: s.blobs[id], after: b}
	return tok
}

func (s *blobStore) setting(key, value string) uint64 {
	tok := s.ledger.issue()
	s.changes[tok] = blobChange{key: key, before: []byte(s.settings[key]), after: []byte(value)}
	return tok
}

func (s *blobStore) put(c blobChange, b []byte) {
	if c.edge != "" {
		s.blobs[c.edge] = b
		return
	}
	s.settings[c.key] = string(b)
}

func (s *blobStore) Do(tok uint64) error {
	s.ledger.accepted[tok] = true
	s.put(s.changes[tok], s.changes[tok].after)
	return nil
}
func (s *blobStore) Undo(tok uint64) { s.put(s.changes[tok], s.changes[tok].before) }
func (s *blobStore) Redo(tok uint64) { s.put(s.changes[tok], s.changes[tok].after) }
func (s *blobStore) Forget(tok uint64) {
	s.ledger.forget(s.t, tok)
	delete(s.changes, tok)
}
func (s *blobStore) Released(ids gw.IDList) {
	s.releases++
	if ids.At(-1) != "" || ids.At(ids.Len()) != "" {
		s.t.Errorf("IDList.At out of range: %q, %q", ids.At(-1), ids.At(ids.Len()))
	}
	for i := range ids.Len() {
		s.released = append(s.released, ids.At(i))
		delete(s.blobs, ids.At(i))
	}
}

func distsEqual(a, b []dist) bool {
	return slices.EqualFunc(a, b, func(x, y dist) bool { return x.Name == y.Name && slices.Equal(x.Params, y.Params) })
}

// TestStoreRecords: structured records per vertex round-trip through a
// host change, its undo and redo, a group with AddVertex and AddEdge
// undone as one step, a deletion and its undo, and Compact.
func TestStoreRecords(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, b, _ := chain(t, d)
	d.ClearUndo()
	s := newRecordStore(t)
	if err := d.SetStore(s); err != nil {
		t.Fatal(err)
	}
	freq := []dist{{Name: "poisson", Params: []float64{0.2}}}
	vals := []dist{{Name: "lognormal", Params: []float64{10, 2}}, {Name: "pert", Params: []float64{1, 5, 20}}}

	ch := apply(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: s.set("a", freq)})
	if !slices.Equal(ch.Vertices, []gw.VertexIndex{a}) || len(ch.Edges) != 0 || ch.Layout {
		t.Fatalf("a host change reported %+v, want vertex %d alone", ch, a)
	}
	apply(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: s.set("a", vals)})
	if !distsEqual(s.byID["a"], vals) {
		t.Fatalf("after Do: %v", s.byID["a"])
	}
	ch, _ = d.Undo()
	if !distsEqual(s.byID["a"], freq) || !slices.Equal(ch.Vertices, []gw.VertexIndex{a}) {
		t.Fatalf("after Undo: %v, change %+v", s.byID["a"], ch)
	}
	ch, _ = d.Redo()
	if !distsEqual(s.byID["a"], vals) || !slices.Equal(ch.Vertices, []gw.VertexIndex{a}) {
		t.Fatalf("after Redo: %v, change %+v", s.byID["a"], ch)
	}

	// A node with its settings, and its edge, as one step.
	d.Group()
	n := addVertex(t, d, kNode, "n")
	apply(t, d, gw.Command{Op: gw.HostChange, Vertex: n, Edge: gw.None, Token: s.set("n", freq)})
	addEdge(t, d, eOutcome, b, n, 1, 0)
	d.EndGroup()
	before := snapshot(d)
	d.Undo()
	if d.Vertex(n).Live() || s.byID["n"] != nil {
		t.Fatalf("the group's undo left the vertex live %v, its data %v", d.Vertex(n).Live(), s.byID["n"])
	}
	d.Redo()
	if after := snapshot(d); after != before || !distsEqual(s.byID["n"], freq) {
		t.Fatalf("the group's redo: data %v, document\n%s\nwant\n%s", s.byID["n"], after, before)
	}

	// The store keys by ID, which deletion and its undo keep.
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: a})
	d.Undo()
	if id := d.Vertex(a).ID(); id != "a" || !distsEqual(s.byID[id], vals) {
		t.Fatalf("after deletion and undo: id %q, data %v", id, s.byID[id])
	}

	// Compact renumbers; the IDs, and so the data, stay.
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: b})
	d.Compact()
	if v := d.VertexByID("n"); v == gw.None || !distsEqual(s.byID["n"], freq) {
		t.Fatalf("after Compact: n at %d, data %v", v, s.byID["n"])
	}
	if s.ledger.pending() != 0 {
		t.Fatalf("Compact left %d tokens unforgotten", s.ledger.pending())
	}
	s.ledger.settled(t)
}

// TestStoreBlobs: an opaque blob per edge and the document's own
// settings round-trip the same way, and the store is given the IDs of
// the tombstones Compact removes.
func TestStoreBlobs(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, _, b, c := chain(t, d)
	d.ClearUndo()
	s := newBlobStore(t)
	if err := d.SetStore(s); err != nil {
		t.Fatal(err)
	}
	e := d.EdgeByID("e2") // a → b
	id := d.Edge(e).ID()
	ch := apply(t, d, gw.Command{Op: gw.HostChange, Vertex: gw.None, Edge: e, Token: s.blob(id, []byte{0, 1, 2, 0xff})})
	if !slices.Equal(ch.Edges, []gw.EdgeIndex{e}) || len(ch.Vertices) != 0 {
		t.Fatalf("an edge's host change reported %+v, want edge %d alone", ch, e)
	}
	ch = apply(t, d, gw.Command{Op: gw.HostChange, Vertex: gw.None, Edge: gw.None, Token: s.setting("seed", "42")})
	if len(ch.Vertices)+len(ch.Edges) != 0 || s.settings["seed"] != "42" {
		t.Fatalf("the document's host change reported %+v, settings %v", ch, s.settings)
	}
	d.Undo()
	d.Undo()
	if s.blobs[id] != nil || s.settings["seed"] != "" {
		t.Fatalf("after two undos: blob %v, settings %v", s.blobs[id], s.settings)
	}
	d.Redo()
	d.Redo()
	if !slices.Equal(s.blobs[id], []byte{0, 1, 2, 0xff}) || s.settings["seed"] != "42" {
		t.Fatalf("after two redos: blob %v, settings %v", s.blobs[id], s.settings)
	}

	// A group of a vertex, an edge and the edge's blob, undone as one.
	d.Group()
	n := addVertex(t, d, kLeaf, "n")
	f := addEdge(t, d, eOutcome, b, n, 1, gw.None)
	apply(t, d, gw.Command{Op: gw.HostChange, Vertex: gw.None, Edge: f, Token: s.blob(d.Edge(f).ID(), []byte("x"))})
	d.EndGroup()
	fid := d.Edge(f).ID()
	d.Undo()
	if d.Vertex(n).Live() || d.Edge(f).Live() || s.blobs[fid] != nil {
		t.Fatalf("the group's undo left n %v, f %v, blob %v", d.Vertex(n).Live(), d.Edge(f).Live(), s.blobs[fid])
	}
	d.Redo()
	if !slices.Equal(s.blobs[fid], []byte("x")) {
		t.Fatalf("the group's redo: blob %v", s.blobs[fid])
	}

	// Deleting c takes its edge from b; Compact releases both.
	cid, bc := d.Vertex(c).ID(), gw.ID("e3") // c, and its edge from b
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: c})
	d.Undo()
	if !d.Vertex(c).Live() {
		t.Fatal("undo did not bring c back")
	}
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: c})
	d.Compact()
	slices.Sort(s.released)
	want := []gw.ID{cid, bc}
	slices.Sort(want)
	if !slices.Equal(s.released, want) {
		t.Fatalf("Released %v, want %v", s.released, want)
	}
	if !slices.Equal(s.blobs[id], []byte{0, 1, 2, 0xff}) || s.settings["seed"] != "42" {
		t.Fatalf("Compact lost live data: blob %v, settings %v", s.blobs[id], s.settings)
	}
	s.releases = 0
	d.Compact()
	if s.releases != 0 {
		t.Fatalf("a Compact with no tombstones called Released %d times", s.releases)
	}
	s.ledger.settled(t)
}

// TestStoreForget: each accepted token is forgotten once, when it falls
// off a full log, when a new command replaces what could be redone,
// when Discard takes it back, when a layout drops it, on ClearUndo and
// on Compact; a refused one never.
func TestStoreForget(t *testing.T) {
	d := newDoc(t, gw.Caps{UndoDepth: 3, UndoLayouts: 1})
	_, a, _, _ := chain(t, d)
	d.ClearUndo()
	s := newRecordStore(t)
	if err := d.SetStore(s); err != nil {
		t.Fatal(err)
	}
	host := func() uint64 {
		t.Helper()
		tok := s.set("a", []dist{{Name: "n", Params: []float64{float64(s.ledger.next)}}})
		apply(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: tok})
		return tok
	}
	forgotten := func(tok uint64, want int) {
		t.Helper()
		if n := s.ledger.forgotten[tok]; n != want {
			t.Fatalf("token %d forgotten %d times, want %d", tok, n, want)
		}
	}

	// Off a full log: the fourth entry pushes out the first.
	t1, t2, t3 := host(), host(), host()
	forgotten(t1, 0)
	t4 := host()
	forgotten(t1, 1)
	forgotten(t2, 0)

	// A new command replaces what could be redone.
	d.Undo()
	d.Undo()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 1, Y: 1}})
	forgotten(t3, 1)
	forgotten(t4, 1)
	forgotten(t2, 0)

	// Discard takes back the open group and forgets it.
	d.Group()
	t5 := host()
	if _, ok := d.Discard(); !ok {
		t.Fatal("Discard did nothing")
	}
	d.EndGroup()
	forgotten(t5, 1)

	// The log is t2, the move and t6; the first layout pushes t2 off.
	// The second drops the entries up to the first, which the log can no
	// longer reach back across.
	t6 := host()
	apply(t, d, gw.Command{Op: gw.Layout})
	forgotten(t2, 1)
	forgotten(t6, 0)
	apply(t, d, gw.Command{Op: gw.Layout})
	forgotten(t6, 1)

	// ClearUndo.
	t7 := host()
	d.ClearUndo()
	forgotten(t7, 1)

	// A refused Do is never forgotten, and leaves the document and the
	// log as they were.
	prev := slices.Clone(s.byID["a"])
	t8 := host()
	s.refuse = true
	before, undo := snapshot(d), d.CanUndo()
	bad := s.set("a", nil)
	if _, err := d.Apply(gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: bad}); err == nil || err.Error() != "distributions do not sum to one" {
		t.Fatalf("a refused Do gave %v", err)
	}
	s.refuse = false
	if snapshot(d) != before || d.CanUndo() != undo || d.CanRedo() {
		t.Fatal("a refused Do changed the document or the log")
	}
	d.Undo()
	if !distsEqual(s.byID["a"], prev) {
		t.Fatalf("the undo after a refused Do took back %v, not t8", s.byID["a"])
	}

	// Compact forgets the rest, t8 among them.
	d.Redo()
	d.Compact()
	forgotten(t8, 1)
	forgotten(bad, 0)
	s.ledger.settled(t)
}

// TestHostChangeRefused: a host change without a store, on a dead or
// out-of-range element, or naming both a vertex and an edge, is refused
// before the store is called.
func TestHostChangeRefused(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, _, c := chain(t, d)
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: 1}, "no store")
	s := newRecordStore(t)
	d.ClearUndo()
	if err := d.SetStore(s); err != nil {
		t.Fatal(err)
	}
	e := d.EdgeByID("e2")
	dead := d.EdgeByID("e3") // b → c, which goes with c
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: c})
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: e, Token: 1}, "not both")
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: c, Edge: gw.None, Token: 1}, "is not live")
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: 999, Edge: gw.None, Token: 1}, "is not live")
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: gw.None, Edge: dead, Token: 1}, "is not live")
	refuse(t, d, gw.Command{Op: gw.HostChange, Vertex: gw.None, Edge: 9999, Token: 1}, "is not live")
	if len(s.ledger.accepted) != 0 {
		t.Fatalf("the store was called for a refused command: %v", s.ledger.accepted)
	}
	d.Lock()
	if _, err := d.Apply(gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: 1}); !errors.Is(err, gw.ErrLocked) {
		t.Fatalf("locked: %v", err)
	}
}

// TestSetStore: refused for nil, while locked, and once the log holds
// anything.
func TestSetStore(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	if err := d.SetStore(nil); err == nil {
		t.Fatal("SetStore(nil) accepted")
	}
	addVertex(t, d, kLeaf, "x")
	if err := d.SetStore(newRecordStore(t)); err == nil {
		t.Fatal("SetStore with a log holding a command accepted")
	}
	d.Undo()
	if err := d.SetStore(newRecordStore(t)); err == nil {
		t.Fatal("SetStore with a log holding an undone command accepted")
	}
	d.ClearUndo()
	d.Lock()
	if err := d.SetStore(newRecordStore(t)); !errors.Is(err, gw.ErrLocked) {
		t.Fatalf("SetStore while locked: %v", err)
	}
	d.Unlock()
	if err := d.SetStore(newRecordStore(t)); err != nil {
		t.Fatalf("SetStore on an empty log: %v", err)
	}
}

// reentrant is a store each of whose methods calls back into the
// document, recording what each call returned.
type reentrant struct {
	d     *gw.Document
	v     gw.VertexIndex
	calls map[string]int
	bad   []string // calls that were not refused
}

func (r *reentrant) callBack(from string) {
	r.calls[from]++
	d := r.d
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: r.v, Pos: gw.Point{X: 77, Y: 77}}); !errors.Is(err, gw.ErrReentered) {
		r.bad = append(r.bad, from+": Apply gave "+errString(err))
	}
	if _, err := d.Apply(gw.Command{Op: gw.Compact}); !errors.Is(err, gw.ErrReentered) {
		r.bad = append(r.bad, from+": Apply(Compact) gave "+errString(err))
	}
	if _, ok := d.Undo(); ok {
		r.bad = append(r.bad, from+": Undo")
	}
	if _, ok := d.Redo(); ok {
		r.bad = append(r.bad, from+": Redo")
	}
	if _, ok := d.Discard(); ok {
		r.bad = append(r.bad, from+": Discard")
	}
	d.Group()
	d.EndGroup()
	d.EndGroup()
	d.ClearUndo()
	if m, _ := d.Compact(); m.Len() != 0 {
		r.bad = append(r.bad, from+": Compact")
	}
	if err := d.SetStore(r); !errors.Is(err, gw.ErrReentered) {
		r.bad = append(r.bad, from+": SetStore gave "+errString(err))
	}
	d.Unlock()
	if !d.Locked() {
		r.bad = append(r.bad, from+": Unlock unlocked")
	}
}

func errString(err error) string {
	if err == nil {
		return "nil"
	}
	return err.Error()
}

func (r *reentrant) Do(uint64) error    { r.callBack("Do"); return nil }
func (r *reentrant) Undo(uint64)        { r.callBack("Undo") }
func (r *reentrant) Redo(uint64)        { r.callBack("Redo") }
func (r *reentrant) Forget(uint64)      { r.callBack("Forget") }
func (r *reentrant) Released(gw.IDList) { r.callBack("Released") }

// TestStoreReentry: a call back into the document from inside each of a
// store's methods is refused, and leaves the document and the log as
// the call around it makes them.
func TestStoreReentry(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, b, c := chain(t, d)
	d.ClearUndo()
	r := &reentrant{d: d, v: a, calls: map[string]int{}}
	if err := d.SetStore(r); err != nil {
		t.Fatal(err)
	}
	// A plain document in step with d, for what each call should leave.
	w := newDoc(t, gw.Caps{})
	_, _, wb, wc := chain(t, w)
	w.ClearUndo()
	same := func(what string) {
		t.Helper()
		if got, want := snapshot(d), snapshot(w); got != want {
			t.Fatalf("%s: the document is\n%s\nwant\n%s", what, got, want)
		}
		if d.CanUndo() != w.CanUndo() || d.CanRedo() != w.CanRedo() {
			t.Fatalf("%s: undo %v redo %v, want %v %v", what, d.CanUndo(), d.CanRedo(), w.CanUndo(), w.CanRedo())
		}
	}
	d.Group()
	apply(t, d, gw.Command{Op: gw.Move, Vertex: b, Pos: gw.Point{X: 5, Y: 5}})
	apply(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: 1})
	d.EndGroup()
	w.Group()
	apply(t, w, gw.Command{Op: gw.Move, Vertex: wb, Pos: gw.Point{X: 5, Y: 5}})
	w.EndGroup()
	same("Do")
	d.Undo()
	w.Undo()
	same("Undo")
	d.Redo()
	w.Redo()
	same("Redo")
	d.ClearUndo()
	w.ClearUndo()
	same("Forget")
	apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: c})
	apply(t, w, gw.Command{Op: gw.DeleteVertex, Vertex: wc})
	d.Compact()
	w.Compact()
	same("Released")
	for _, m := range []string{"Do", "Undo", "Redo", "Forget", "Released"} {
		if r.calls[m] != 1 {
			t.Errorf("%s called %d times, want 1", m, r.calls[m])
		}
	}
	for _, bad := range r.bad {
		t.Error(bad)
	}
	if d.Locked() {
		t.Fatal("the document stayed locked after the store returned")
	}
	// Outside a store call, Lock and Unlock work as before.
	d.Lock()
	if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: a, Pos: gw.Point{X: 9, Y: 9}}); !errors.Is(err, gw.ErrLocked) {
		t.Fatalf("locked outside a store call: %v", err)
	}
	d.Unlock()
	if d.Locked() {
		t.Fatal("Unlock outside a store call did nothing")
	}
}

// TestAllocHostChange: applying, undoing and redoing a host change, and
// a Compact that releases a tombstone, allocate nothing on the
// document's side.
func TestAllocHostChange(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	_, a, _, _ := chain(t, d)
	d.ClearUndo()
	if err := d.SetStore(nopStore{}); err != nil {
		t.Fatal(err)
	}
	if n := testing.AllocsPerRun(50, func() {
		apply(t, d, gw.Command{Op: gw.HostChange, Vertex: a, Edge: gw.None, Token: 7})
		d.Undo()
		d.Redo()
	}); n > 0 {
		t.Errorf("%g allocations per run", n)
	}
	if n := testing.AllocsPerRun(50, func() {
		z := apply(t, d, gw.Command{Op: gw.AddVertex, Kind: kLeaf, ID: "z"}).Vertices[0]
		apply(t, d, gw.Command{Op: gw.DeleteVertex, Vertex: z})
		d.Compact()
	}); n > 0 {
		t.Errorf("Compact releasing a tombstone: %g allocations per run", n)
	}

	// Nor does a Compact without a store, counted over every run, since
	// AllocsPerRun rounds down an amortised growth.
	p := newDoc(t, gw.Caps{})
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	for range 50 {
		z := apply(t, p, gw.Command{Op: gw.AddVertex, Kind: kLeaf, ID: "z"}).Vertices[0]
		apply(t, p, gw.Command{Op: gw.DeleteVertex, Vertex: z})
		p.Compact()
	}
	runtime.ReadMemStats(&m1)
	if n := m1.Mallocs - m0.Mallocs; n > 0 {
		t.Errorf("50 Compacts without a store: %d allocations", n)
	}

	// The first Compact after SetStore, releasing many, as well: the
	// room for the IDs was made by SetStore.
	f := newDoc(t, gw.Caps{Vertices: 64, Edges: 64})
	if err := f.SetStore(nopStore{}); err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		v := addVertex(t, f, kLeaf, "f"+strconv.Itoa(i))
		apply(t, f, gw.Command{Op: gw.DeleteVertex, Vertex: v})
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f.Compact()
	runtime.ReadMemStats(&after)
	if n := after.Mallocs - before.Mallocs; n > 0 {
		t.Errorf("the first Compact releasing 40 tombstones: %d allocations", n)
	}
}

type nopStore struct{}

func (nopStore) Do(uint64) error    { return nil }
func (nopStore) Undo(uint64)        {}
func (nopStore) Redo(uint64)        {}
func (nopStore) Forget(uint64)      {}
func (nopStore) Released(gw.IDList) {}
