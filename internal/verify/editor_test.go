package verify_test

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/edit"
	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/merilo"
)

// The steps are generated from the gesture machine's own enums: every
// event kind it names, pressed and double-clicked on every hit it names,
// each sent the way a browser host sends it at a fixed point of the
// fixture. A kind or hit added to the machine is then explored with no one
// updating a list here; hostSteps and hitPoint must handle it, or
// TestStepsCoverTheMachine fails. A pointer press counts as held until its
// release; a second press while one is held is a second finger.

// how varies a step beyond its event kind and hit.
type how uint8

const (
	plain   how = iota
	shiftB      // a shift-press on b, which adds to the selection
	middle      // a middle-button press on a, which pans
	handle      // a press on a's resize handle, when a alone is selected
	second      // a move to b's in port, not the background
	undoAll     // Ctrl-Z until nothing is left, then Ctrl-Y until nothing is left
)

// step is one thing the host can do.
type step struct {
	kind gesture.EventKind
	hit  gesture.Hit
	how  how
}

// steps is every step the explorer takes, in a fixed order.
func steps() []step {
	var ss []step
	for _, k := range named(func(v uint8) string { return gesture.EventKind(v).String() }) {
		ss = append(ss, hostSteps(gesture.EventKind(k))...)
	}
	return append(ss, step{how: undoAll})
}

// hits is every hit the gesture machine names.
func hits() []gesture.Hit {
	var hs []gesture.Hit
	for _, h := range named(func(v uint8) string { return gesture.Hit(v).String() }) {
		hs = append(hs, gesture.Hit(h))
	}
	return hs
}

// hostSteps is the steps that give the gesture machine event kind k.
func hostSteps(k gesture.EventKind) []step {
	switch k {
	case gesture.EvPress:
		ss := []step{{k, gesture.HitVertex, shiftB}, {k, gesture.HitVertex, middle}, {k, gesture.HitVertex, handle}}
		for _, h := range hits() {
			ss = append(ss, step{k, h, plain})
		}
		return ss
	case gesture.EvDoubleClick:
		var ss []step
		for _, h := range hits() {
			ss = append(ss, step{k, h, plain})
		}
		return ss
	case gesture.EvMove:
		return []step{{k, 0, plain}, {k, 0, second}}
	case gesture.EvRelease, gesture.EvEscape, gesture.EvTab, gesture.EvDelete, gesture.EvNudge,
		gesture.EvUndo, gesture.EvRedo, gesture.EvStartText, gesture.EvType, gesture.EvOtherKey,
		gesture.EvPaletteVertex, gesture.EvPaletteEdge, gesture.EvBlur, gesture.EvTextDone,
		gesture.EvTextCancel, gesture.EvView, gesture.EvEdit:
		return []step{{k, 0, plain}}
	default:
		return nil
	}
}

// hitPoint is where on the fixture a press or double-click lands on h,
// following the vertices as they move. TestStepsCoverTheMachine checks
// that each lands on its hit in the fixture.
func (w *world) hitPoint(h gesture.Hit) (gw.Point, bool) {
	switch h {
	case gesture.HitBackground:
		return atBG, true
	case gesture.HitVertex:
		return w.on("a", atA, 0, 0), true
	case gesture.HitOutline:
		return w.on("s", atS, 48, 0), true // the pill's right end; s has no ports
	case gesture.HitPort:
		return w.on("a", atA, 60, 0), true
	case gesture.HitEdge:
		return atEdge, true // on the edge from s to a's in port
	default:
		return gw.Point{}, false
	}
}

const (
	kSource gw.KindIndex = iota
	kNode
	kLeaf
)

const eTo, eOutcome gw.KindIndex = 0, 1

func pt(x, y float64) gw.Point { return gw.Point{X: x, Y: y} }

// Fixture positions: s at (60,100), a at (220,100), b at (420,100), c at
// (620,100); b's in port at (360,100). Presses on a and b follow them
// where they have moved (on), so a second drag is a drag.
var (
	atS, atA, atB = pt(60, 100), pt(220, 100), pt(420, 100)
	atBG          = pt(220, 170)
	atEdge        = pt(134, 100)
	atM1, atM2    = pt(300, 160), pt(360, 100)
)

// on is where vertex id is now, offset by dx and dy: a's out port is 60
// right of its centre and its resize handle 60 right and 18 down. A
// deleted vertex is pressed where the fixture put it.
func (w *world) on(id gw.ID, at gw.Point, dx, dy float64) gw.Point {
	if v := w.d.VertexByID(id); v != gw.None && w.d.Vertices[v].Live {
		at = w.d.Vertices[v].Pos
	}
	return pt(at.X+dx, at.Y+dy)
}

func schema() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "source", Shape: gw.Pill, Size: gw.Size{W: 96, H: 24}, Root: true},
			{Name: "node", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36},
				Ports: []gw.Port{{Name: "in", Side: gw.Left}, {Name: "out", Side: gw.Right, Max: 2}}},
			{Name: "leaf", Shape: gw.Rect, Size: gw.Size{W: 100, H: 30}, Leaf: true},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "to", From: []gw.KindIndex{kSource}, To: []gw.KindIndex{kNode}, Arrow: true},
			{Name: "outcome", From: []gw.KindIndex{kNode}, To: []gw.KindIndex{kNode, kLeaf}, Route: gw.Curve},
		},
		Acyclic: true,
	}
}

// world is one editor on the fixture, with what the host knows: how many
// pointers are down and where the pointer is.
type world struct {
	d     *gw.Document
	e     *edit.Editor
	held  int
	at    gw.Point
	input bool           // the host shows the label input, which has the focus
	label gw.VertexIndex // the vertex the input was shown for
	edge  gw.EdgeIndex   // or the edge
	start string         // the fixture without its view, which undo-all must restore
	idle  string         // the document without its view at the last Idle
	fault string         // the first invariant a step broke, with what it saw
	steps []step         // the step kinds, fixed when the world is built
}

func newWorld(t testing.TB) *world {
	t.Helper()
	if _, err := fixture(); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	w := &world{steps: steps()}
	w.Reset()
	return w
}

// fixture builds the document every session starts from: the edit
// package's own test fixture, with an undo log that holds a whole session,
// so undo-all can return to the start: a step adds at most one entry per
// element, a nudge one per selected vertex, so a fuzz session of 40 steps
// over 8 vertices and 8 edges needs 640.
func fixture() (*gw.Document, error) {
	d, err := gw.New(schema(), gw.Caps{Vertices: 8, Edges: 8, UndoDepth: 1024})
	if err != nil {
		return nil, err
	}
	for _, v := range []struct {
		kind gw.KindIndex
		id   string
		x    float64
	}{{kSource, "s", 60}, {kNode, "a", 220}, {kNode, "b", 420}, {kLeaf, "c", 620}} {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: v.kind, ID: gw.ID(v.id), Text: v.id})
		if err != nil {
			return nil, err
		}
		if _, err := d.Apply(gw.Command{Op: gw.Move, Vertex: ch.Vertices[0], Pos: pt(v.x, 100)}); err != nil {
			return nil, err
		}
	}
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: eTo, From: 0, To: 1, FromPort: gw.None, ToPort: 0}); err != nil {
		return nil, err
	}
	d.View = gw.Viewport{W: 800, H: 200}
	d.ClearUndo()
	return d, nil
}

// Reset starts a session. The fixture is checked once by the test before
// any search, so a failure here is reported as a fault, not skipped.
func (w *world) Reset() {
	d, err := fixture()
	if err != nil {
		w.fault = "fixture: " + err.Error()
		return
	}
	*w = world{d: d, e: edit.New(d, edit.Edit, 8), at: atA, label: gw.None, edge: gw.None, steps: w.steps}
	w.start = w.content()
	w.idle = w.start
}

// encode is the document as the host saves it.
func (w *world) encode() string {
	var b bytes.Buffer
	if err := doc.Encode(w.d, "verify", &b); err != nil {
		return "encode: " + err.Error()
	}
	return b.String()
}

// content is the document without its view, which panning and zooming
// change without a command.
func (w *world) content() string {
	v := w.d.View
	w.d.View = gw.Viewport{}
	s := w.encode()
	w.d.View = v
	return s
}

func (w *world) Kinds() int { return len(w.steps) }

// stepAt is step kind k, or false for a kind outside the steps.
func (w *world) stepAt(k int) (step, bool) {
	if k < 0 || k >= len(w.steps) {
		return step{}, false
	}
	return w.steps[k], true
}

func (w *world) Invariants() []string {
	return []string{
		"undo-all from Idle restores the start, and redo-all restores the document",
		"a refused event leaves the document as it was",
		"in View mode no event changes the document",
		"Escape leaves the editor Idle",
		"the selection, and while editing text the label being edited, name live elements",
		"a label committed in the host's input is applied or refused, never dropped",
	}
}

// Enabled reports whether a host can send step kind k now. While the host
// shows the label input it has the focus, so keys go to it; a press
// elsewhere blurs it.
func (w *world) Enabled(k int) bool {
	st, ok := w.stepAt(k)
	if !ok {
		return false
	}
	if st.how == undoAll {
		// View mode refuses undo and redo (edit.ErrView).
		return w.held == 0 && w.e.State() == edit.Idle && w.e.Mode() == edit.Edit
	}
	typing := w.input
	switch st.kind {
	case gesture.EvPress:
		return w.held < 2 && (st.how != middle || w.held == 0)
	case gesture.EvPaletteVertex, gesture.EvPaletteEdge:
		return w.held == 0
	case gesture.EvDoubleClick:
		// A dblclick follows two presses on the drawing, and the first
		// blurs the label input.
		return w.held == 0 && !typing
	case gesture.EvMove:
		return true
	case gesture.EvRelease:
		return w.held > 0
	case gesture.EvEscape, gesture.EvTab, gesture.EvDelete, gesture.EvNudge, gesture.EvUndo, gesture.EvRedo,
		gesture.EvStartText, gesture.EvType, gesture.EvOtherKey, gesture.EvBlur:
		return !typing
	case gesture.EvTextDone, gesture.EvTextCancel:
		return typing
	case gesture.EvView:
		return w.e.Mode() == edit.Edit
	case gesture.EvEdit:
		return w.e.Mode() == edit.View
	default:
		return false
	}
}

func (w *world) Apply(k int) error {
	st, ok := w.stepAt(k)
	if !ok {
		return fmt.Errorf("unknown step kind %d", k)
	}
	if st.how == undoAll {
		w.undoAll()
		return nil
	}
	if st.kind == gesture.EvView || st.kind == gesture.EvEdit {
		// The host's setMode hides the label input: SetMode cancels the
		// editing it was for.
		if w.e.State() == edit.EditingText {
			w.input = false
		}
		mode := edit.Edit
		if st.kind == gesture.EvView {
			mode = edit.View
		}
		w.e.SetMode(mode)
		w.after(st, false)
		return nil
	}
	ev, ok := w.event(st)
	if !ok {
		return fmt.Errorf("step kind %d (%s) has no host event", k, st)
	}
	if (ev.Kind == edit.PointerDown || ev.Kind == edit.PaletteStart) && w.input {
		// A press on the drawing blurs the input, and the host's
		// inputBlur hides it and commits the text while the editor is
		// still EditingText. A press on the palette keeps the focus, and
		// its handler does the same.
		w.input = false
		if w.e.State() == edit.EditingText {
			w.commit()
		}
	}
	view := w.e.Mode() == edit.View
	switch st.kind {
	case gesture.EvTextDone:
		w.input = false
		w.commit()
	case gesture.EvTextCancel:
		w.input = false
		w.send(ev)
	default:
		w.send(ev)
	}
	if (ev.Kind == edit.Key || ev.Kind == edit.DoubleClick) && w.e.State() == edit.EditingText {
		// The host's keydown and dblclick handlers show the input.
		w.input = true
		w.label, w.edge = w.e.Editing()
	}
	w.after(st, view && w.e.Mode() == edit.View)
	return nil
}

// text is what the user typed into the label input.
const text = "x"

// commit sends the input's text as the host does, and checks that the
// element the input was shown for took it, unless the editor refused.
func (w *world) commit() {
	before := w.encode()
	_, err := w.e.Handle(edit.Event{Kind: edit.TextDone, Text: text})
	switch {
	case err != nil && w.encode() != before:
		w.broke("a refused event leaves the document as it was", "refused with %q, but the document changed", err)
	case err == nil && w.label != gw.None && w.d.Vertices[w.label].Live && w.d.Vertices[w.label].Label != text:
		w.broke("a label committed in the host's input is applied or refused, never dropped",
			"the host committed %q for %s, which kept %q", text, w.d.Vertices[w.label].ID, w.d.Vertices[w.label].Label)
	case err == nil && w.edge != gw.None && w.d.Edges[w.edge].Live && w.d.Edges[w.edge].Label != text:
		w.broke("a label committed in the host's input is applied or refused, never dropped",
			"the host committed %q for %s, which kept %q", text, w.d.Edges[w.edge].ID, w.d.Edges[w.edge].Label)
	}
	w.label, w.edge = gw.None, gw.None
}

// event is the host event a step sends, and moves the host's pointer and
// its count of pointers down.
func (w *world) event(st step) (edit.Event, bool) {
	switch st.kind {
	case gesture.EvPress:
		switch st.how {
		case shiftB:
			return w.press(w.on("b", atB, 0, 0), 0, edit.Shift), true
		case middle:
			return w.press(w.on("a", atA, 0, 0), 1, 0), true
		case handle:
			return w.press(w.on("a", atA, 60, 18), 0, 0), true
		case plain, second, undoAll:
		}
		p, ok := w.hitPoint(st.hit)
		return w.press(p, 0, 0), ok
	case gesture.EvDoubleClick:
		p, ok := w.hitPoint(st.hit)
		return edit.Event{Kind: edit.DoubleClick, Point: p}, ok
	case gesture.EvMove:
		w.at = atM1
		if st.how == second {
			w.at = atM2
		}
		return edit.Event{Kind: edit.PointerMove, Point: w.at}, true
	case gesture.EvRelease:
		w.held--
		return edit.Event{Kind: edit.PointerUp, Point: w.at}, true
	case gesture.EvEscape:
		return edit.Event{Kind: edit.Key, Key: "Escape"}, true
	case gesture.EvTab:
		return edit.Event{Kind: edit.Key, Key: "Tab"}, true
	case gesture.EvDelete:
		return edit.Event{Kind: edit.Key, Key: "Delete"}, true
	case gesture.EvNudge:
		return edit.Event{Kind: edit.Key, Key: "ArrowLeft"}, true
	case gesture.EvUndo:
		return edit.Event{Kind: edit.Key, Key: "z", Modifiers: edit.Ctrl}, true
	case gesture.EvRedo:
		return edit.Event{Kind: edit.Key, Key: "y", Modifiers: edit.Ctrl}, true
	case gesture.EvStartText:
		return edit.Event{Kind: edit.Key, Key: "Enter"}, true
	case gesture.EvType:
		return edit.Event{Kind: edit.Key, Key: "q"}, true
	case gesture.EvOtherKey:
		return edit.Event{Kind: edit.Key, Key: "F5"}, true
	case gesture.EvPaletteVertex, gesture.EvPaletteEdge:
		// The press is on the palette; its release reaches the editor.
		w.held++
		if st.kind == gesture.EvPaletteEdge {
			return edit.Event{Kind: edit.PaletteStart, Item: eOutcome, IsEdge: true}, true
		}
		return edit.Event{Kind: edit.PaletteStart, Item: kNode}, true
	case gesture.EvBlur:
		return edit.Event{Kind: edit.Blur}, true
	case gesture.EvTextDone:
		return edit.Event{Kind: edit.TextDone, Text: text}, true
	case gesture.EvTextCancel:
		return edit.Event{Kind: edit.TextCancel}, true
	case gesture.EvView, gesture.EvEdit:
		return edit.Event{}, false // SetMode, not an event
	default:
		return edit.Event{}, false
	}
}

// press is a pointer going down at p.
func (w *world) press(p gw.Point, button int, m edit.Modifiers) edit.Event {
	w.held++
	w.at = p
	return edit.Event{Kind: edit.PointerDown, Point: p, Button: button, Modifiers: m}
}

// send hands the editor one event and records a refusal that changed
// the document.
func (w *world) send(ev edit.Event) {
	before := w.encode()
	if _, err := w.e.Handle(ev); err != nil && w.encode() != before {
		w.broke("a refused event leaves the document as it was", "refused with %q, but the document changed", err)
	}
}

// after checks what a step left: View mode unchanged and Escape
// cancelled, and records the document at Idle.
func (w *world) after(st step, inView bool) {
	now := w.content()
	if inView && now != w.idle {
		w.broke("in View mode no event changes the document", "the document changed in View mode")
	}
	if st.kind == gesture.EvEscape && w.e.State() != edit.Idle {
		w.broke("Escape leaves the editor Idle", "state %s after Escape", stateName(w.e.State()))
	}
	if w.e.State() == edit.Idle {
		w.idle = now
	}
}

// undoAll presses Ctrl-Y until there is nothing to redo, then Ctrl-Z
// until there is nothing to undo, and compares with the start, then
// Ctrl-Y to the end again and compares with the first end. Each run is
// bounded by the undo log's depth.
func (w *world) undoAll() {
	const inv = "undo-all from Idle restores the start, and redo-all restores the document"
	w.keyRepeat("y", w.d.CanRedo)
	end := w.content()
	w.keyRepeat("z", w.d.CanUndo)
	if got := w.content(); got != w.start {
		w.broke(inv, "after undo-all: %s", diff(w.start, got))
		return
	}
	w.keyRepeat("y", w.d.CanRedo)
	if got := w.content(); got != end {
		w.broke(inv, "after redo-all: %s", diff(end, got))
	}
	w.idle = w.content()
}

// keyRepeat presses Ctrl and the key while more remains, at most the
// undo log's depth times.
func (w *world) keyRepeat(key string, more func() bool) {
	for i := 0; i < w.d.Caps().UndoDepth && more(); i++ {
		w.send(edit.Event{Kind: edit.Key, Key: key, Modifiers: edit.Ctrl})
	}
}

func (w *world) broke(invariant, format string, args ...any) {
	if w.fault == "" {
		w.fault = invariant + ": " + fmt.Sprintf(format, args...)
	}
}

func (w *world) Check() error {
	if w.fault != "" {
		return errors.New(w.fault)
	}
	const live = "the selection, and while editing text the label being edited, name live elements"
	sv, se := w.e.Selected()
	for _, v := range sv {
		if !w.d.Vertices[v].Live {
			return fmt.Errorf("%s: vertex %d is selected but deleted", live, v)
		}
	}
	for _, x := range se {
		if !w.d.Edges[x].Live {
			return fmt.Errorf("%s: edge %d is selected but deleted", live, x)
		}
	}
	// Editing is defined only while the state is EditingText.
	if v, x := w.e.Editing(); w.e.State() == edit.EditingText && ((v != gw.None && !w.d.Vertices[v].Live) || (x != gw.None && !w.d.Edges[x].Live)) {
		return fmt.Errorf("%s: the label being edited is deleted", live)
	}
	return nil
}

// view is the world as merilo compares it: the document with its view,
// everything the editor shows, and the host's pointer. The editor's
// unexported fields and the undo log's entries are not in it, so worlds
// differing only there are merged: the search can miss a bug there, but
// never reports a false one.
type view struct {
	doc              string
	state            edit.State
	mode             edit.Mode
	canUndo, canRedo bool
	held             int
	input            bool
	label            gw.VertexIndex
	edge             gw.EdgeIndex
	at               gw.Point
	selected         string // the selected vertices and edges, which the editor holds in slices
	editV            gw.VertexIndex
	editE            gw.EdgeIndex
	edgeKind         gw.KindIndex
	typed            string
	ghostFrom        gw.Point
	ghostTo          gw.Point
	ghost            bool
	marquee          gw.Viewport
	inMarquee        bool
	target           gw.VertexIndex
	onTarget         bool
	idle, fault      string
}

func (w *world) World() any {
	sv, se := w.e.Selected()
	v := view{doc: w.encode(), state: w.e.State(), mode: w.e.Mode(), canUndo: w.d.CanUndo(), canRedo: w.d.CanRedo(),
		held: w.held, input: w.input, label: w.label, edge: w.edge, at: w.at,
		selected: fmt.Sprint(sv, se), edgeKind: w.e.EdgeKind(), typed: w.e.Typed(), idle: w.idle, fault: w.fault}
	v.editV, v.editE = w.e.Editing()
	v.ghostFrom, v.ghostTo, v.ghost = w.e.Ghost()
	v.marquee, v.inMarquee = w.e.Marquee()
	v.target, v.onTarget = w.e.Target()
	return v
}

func b(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func stateName(s edit.State) string {
	switch s {
	case edit.Idle:
		return "Idle"
	case edit.Selecting:
		return "Selecting"
	case edit.Dragging:
		return "Dragging"
	case edit.Connecting:
		return "Connecting"
	case edit.Placing:
		return "Placing"
	case edit.Panning:
		return "Panning"
	case edit.EditingText:
		return "EditingText"
	case edit.Resizing:
		return "Resizing"
	default:
		return "state " + strconv.Itoa(int(s))
	}
}

func (w *world) Tuple() string {
	mode := "Edit"
	if w.e.Mode() == edit.View {
		mode = "View"
	}
	return stateName(w.e.State()) + "/" + mode
}

// Settled: every world can take a step, so the search ends at its bounds
// and no world is a deadlock.
func (w *world) Settled() bool { return true }

// String names a step for a trace line, such as "Press on Vertex".
func (st step) String() string {
	var b strings.Builder
	switch st.how {
	case undoAll:
		return "Ctrl-Z to the start, then Ctrl-Y to the end"
	case shiftB:
		b.WriteString("shift-")
	case middle:
		b.WriteString("middle-")
	case plain, handle, second:
	}
	b.WriteString(st.kind.String())
	switch {
	case st.how == shiftB:
		b.WriteString(" on b")
	case st.how == handle:
		b.WriteString(" on a's resize handle")
	case st.how == second:
		b.WriteString(" to b's in port")
	case st.kind == gesture.EvMove:
		b.WriteString(" to the background")
	case st.hit != 0:
		b.WriteString(" on " + st.hit.String())
	}
	return b.String()
}

func (w *world) Describe(k int) string {
	if st, ok := w.stepAt(k); ok {
		return st.String()
	}
	return "kind " + strconv.Itoa(k)
}

// diff shows two encodings that should be equal.
func diff(want, got string) string {
	if want == got {
		return "unchanged"
	}
	return "\n    want " + want + "\n    got  " + got
}

// opts holds every world within six steps of the start.
var opts = merilo.Options{MaxStates: 130000}

func TestEditor(t *testing.T) {
	merilo.Explore(t, newWorld(t), opts)
}

func FuzzEditor(f *testing.F) {
	merilo.Fuzz(f, newWorld(f), merilo.FuzzOptions{MaxDepth: 40, Explore: opts})
}
