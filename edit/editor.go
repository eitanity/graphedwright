// Package edit turns pointer and keyboard input into commands on a
// document. The editor is a state machine, driven by events a host
// feeds it and answering with what changed, so that pointer behaviour
// is tested without a browser and is the same in every host.
package edit

import (
	"errors"
	"math"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/graphedwright/svg"
	"code.eitanity.com/merilo/fsm"
)

// Mode is what the editor allows: looking, or editing.
type Mode uint8

// The modes.
const (
	View Mode = iota
	Edit
)

// State is where the editor is between events: the state of its
// gesture machine, which decides what every event means in every state.
type State = gesture.State

// The states.
const (
	Idle        = gesture.Idle
	Selecting   = gesture.Selecting   // a marquee from the pointer-down point
	Dragging    = gesture.Dragging    // the selection follows the pointer
	Connecting  = gesture.Connecting  // an edge from a port to the pointer
	Placing     = gesture.Placing     // a palette item follows the pointer
	Panning     = gesture.Panning     // the viewport follows the pointer
	EditingText = gesture.EditingText // the host shows an input for a label
	Resizing    = gesture.Resizing    // the selected vertex's corner follows the pointer
)

// EventKind is what happened.
type EventKind uint8

// The events a host sends. Resize carries the element's size in
// Point; Wheel carries the scroll in Delta; TextDone carries the label
// in Text.
const (
	PointerDown EventKind = iota
	PointerMove
	PointerUp
	Wheel
	Key
	PaletteStart
	Blur
	Resize
	TextDone
	TextCancel
	DoubleClick // at Point: edits the label of what is there
)

// Modifiers are the keys held during an event.
type Modifiers uint8

// The modifier bits.
const (
	Shift Modifiers = 1 << iota
	Ctrl
	Alt
)

// Event is one input. Point is in the SVG element's client
// coordinates; the editor applies the viewport. Button is 0 for the
// main button, 1 for the middle one, 2 for the secondary.
type Event struct {
	Kind      EventKind
	Point     gw.Point
	Button    int
	Modifiers Modifiers
	Key       string
	Item      gw.KindIndex // for PaletteStart: the vertex kind to place, or the edge kind to choose
	IsEdge    bool         // for PaletteStart: an edge kind, which chooses it for new edges, or unchooses it
	Delta     float64      // for Wheel: the vertical scroll in document units, positive down; with Ctrl it zooms, out when positive, 100 a notch
	DeltaX    float64      // for Wheel: the horizontal scroll in document units
	Text      string       // for TextDone
}

// ErrView is the refusal of any command in view mode.
var ErrView = errors.New("view mode: the document cannot be changed")

// ErrNoEdgeKind is the refusal of a connection no edge kind allows.
var ErrNoEdgeKind = errors.New("no edge kind joins these vertices")

// errInvalid is the gesture machine's refusal of an input it cannot hold,
// such as a hit kind the editor does not know.
var errInvalid = errors.New("edit: the editor refused an impossible input")

// Editor wraps a document with the state between events, the
// selection, and what the overlay should show. The gesture machine holds
// the control state and the mode; the editor holds the geometry and does
// what the machine's actions ask.
type Editor struct {
	d     *gw.Document
	g     gesture.Machine
	grid  float64
	elemW float64 // the element's size in client units, for the viewport
	elemH float64

	selV  []gw.VertexIndex
	selE  []gw.EdgeIndex
	hover svg.Hit

	down     gw.Point // where the pointer went down, in document units
	last     gw.Point // the last pointer position, in document units
	orig     []gw.Point
	dragV    []gw.VertexIndex // what a drag moves: the selection and what its containers hold
	dropTo   []gw.VertexIndex // per vertex, the container a drag drops it in
	connFrom gw.VertexIndex
	connPort gw.PortIndex
	connKind gw.KindIndex   // the edge kind of the connection in progress, or None for the schema's first that allows
	edgeKind gw.KindIndex   // the edge kind chosen from the palette for new edges, or None
	connAt   gw.Point       // where a connection without a port began, for the ghost
	sizeV    gw.VertexIndex // the vertex being resized
	corner   gw.Point       // its top left corner, which a resize keeps
	item     gw.KindIndex
	typed    string // the character typed to start editing a label, or none
	editV    gw.VertexIndex
	editE    gw.EdgeIndex

	changeV []gw.VertexIndex
	changeE []gw.EdgeIndex
	layout  bool
}

// New makes an editor for the document, allocating its working arrays
// from the document's capacities. The grid is the nudge and snap unit,
// 8 when zero, or when it is not a positive size (gw.MaxCoord at most):
// a grid of NaN or infinity would make every position it snaps one the
// document refuses.
func New(d *gw.Document, mode Mode, grid float64) *Editor {
	if !(grid > 0) || grid > gw.MaxCoord {
		grid = 8
	}
	caps := d.Caps()
	e := &Editor{
		d: d, g: gesture.New(), grid: grid,
		selV: make([]gw.VertexIndex, 0, caps.Vertices), selE: make([]gw.EdgeIndex, 0, caps.Edges),
		orig: make([]gw.Point, caps.Vertices), hover: background(),
		dragV: make([]gw.VertexIndex, 0, caps.Vertices), dropTo: make([]gw.VertexIndex, caps.Vertices),
		connFrom: gw.None, connPort: gw.None, connKind: gw.None, edgeKind: gw.None, editV: gw.None, editE: gw.None, sizeV: gw.None,
		changeV: make([]gw.VertexIndex, 0, caps.Vertices), changeE: make([]gw.EdgeIndex, 0, caps.Edges),
	}
	e.SetMode(mode)
	return e
}

func background() svg.Hit {
	return svg.Hit{Kind: svg.Background, Vertex: gw.None, Port: gw.None, Edge: gw.None}
}

// Document is the document the editor edits.
func (e *Editor) Document() *gw.Document { return e.d }

// Mode is the editor's mode.
func (e *Editor) Mode() Mode {
	if e.g.View() {
		return View
	}
	return Edit
}

// SetMode switches between viewing and editing, cancelling whatever
// was in progress.
func (e *Editor) SetMode(m Mode) {
	k := gesture.EvEdit
	if m != Edit {
		k = gesture.EvView
	}
	// A mode change has no refusal and changes no document.
	_ = e.fire(gesture.Event{Kind: k}, input{})
}

// EdgeKind is the edge kind chosen from the palette, which every new
// edge takes, or None when the editor takes the schema's first kind
// that allows the two vertices.
func (e *Editor) EdgeKind() gw.KindIndex { return e.edgeKind }

// State is where the editor is.
func (e *Editor) State() State { return e.g.State() }

// Selected lists the selected vertices and edges; the slices are the
// editor's and valid until the next event.
func (e *Editor) Selected() ([]gw.VertexIndex, []gw.EdgeIndex) { return e.selV, e.selE }

// Hover is what the pointer was last over.
func (e *Editor) Hover() svg.Hit { return e.hover }

// Editing is the vertex or edge whose label the host is editing, when
// the state is EditingText.
func (e *Editor) Editing() (gw.VertexIndex, gw.EdgeIndex) { return e.editV, e.editE }

// Typed is the character whose key began the label editing, which the
// host's input starts with in place of the label, or "" when the
// editing began by Enter, F2 or a double-click.
func (e *Editor) Typed() string { return e.typed }

// Ghost is the line the overlay draws while connecting, from the port
// to the pointer.
func (e *Editor) Ghost() (from, to gw.Point, ok bool) {
	if e.g.State() != Connecting {
		return gw.Point{}, gw.Point{}, false
	}
	if e.connPort == gw.None {
		return e.connAt, e.last, true
	}
	return e.portPoint(e.connFrom, e.connPort), e.last, true
}

// Target is the vertex an edge can start from or end at under the
// pointer: an outline to start from when idle, or, while connecting, a
// vertex the edge can join that is not where it began.
func (e *Editor) Target() (gw.VertexIndex, bool) {
	if e.g.View() {
		return gw.None, false
	}
	h := e.hover
	switch e.g.State() {
	case Idle:
		return h.Vertex, h.Kind == svg.OutlineHit
	case Connecting:
		ok := h.Kind == svg.PortHit || h.Kind == svg.OutlineHit ||
			h.Kind == svg.VertexHit && len(e.d.Vertices[h.Vertex].Ports) == 0
		return h.Vertex, ok && h.Vertex != e.connFrom && e.joins(e.connFrom, h.Vertex)
	case Selecting, Dragging, Placing, Panning, EditingText, Resizing:
		return gw.None, false
	default:
		return gw.None, false
	}
}

// Marquee is the rectangle the overlay draws while selecting, or while
// resizing the box the vertex will have.
func (e *Editor) Marquee() (gw.Viewport, bool) {
	switch e.g.State() {
	case Selecting:
		return rect(e.down, e.last), true
	case Resizing:
		return e.sizeBox(), true
	case Idle, Dragging, Connecting, Placing, Panning, EditingText:
		return gw.Viewport{}, false
	default:
		return gw.Viewport{}, false
	}
}

// ResizeHandle is the vertex whose resize handle the overlay draws: the one
// selected vertex, alone and idle in edit mode.
func (e *Editor) ResizeHandle() (gw.VertexIndex, bool) {
	if e.g.View() || e.g.State() != Idle {
		return gw.None, false
	}
	return e.onlyVertex()
}

// onlyVertex is the one selected vertex, when it is all that is selected
// and live.
func (e *Editor) onlyVertex() (gw.VertexIndex, bool) {
	if len(e.selV) != 1 || len(e.selE) != 0 || !e.d.Vertices[e.selV[0]].Live {
		return gw.None, false
	}
	return e.selV[0], true
}

// Overlay fills the renderer's options with the selection, the ghost
// and the marquee.
func (e *Editor) Overlay(opts *svg.Options) {
	opts.SelectedV, opts.SelectedE = e.selV, e.selE
	opts.Ghost[0], opts.Ghost[1], opts.HasGhost = e.Ghost()
	opts.Marquee, opts.HasMarquee = e.Marquee()
	opts.Target, opts.HasTarget = e.Target()
	opts.Handle, opts.HasHandle = e.ResizeHandle()
	if e.g.View() {
		opts.Mode = svg.View
	} else {
		opts.Mode = svg.Edit
	}
}

// Handle takes one event and reports what changed in the document.
// A refusal comes back as the error, with the document as it was.
func (e *Editor) Handle(ev Event) (gw.Change, error) {
	e.changeV, e.changeE, e.layout = e.changeV[:0], e.changeE[:0], false
	var err error
	switch ev.Kind {
	case Resize:
		e.elemW, e.elemH = ev.Point.X, ev.Point.Y
	case PointerDown:
		err = e.pointerDown(ev)
	case PointerMove:
		err = e.pointerMove(ev)
	case PointerUp:
		err = e.pointerUp(ev)
	case Wheel:
		e.wheel(ev)
	case Key:
		err = e.key(ev)
	case PaletteStart:
		err = e.paletteStart(ev)
	case Blur:
		err = e.fire(gesture.Event{Kind: gesture.EvBlur}, input{})
	case TextDone:
		err = e.fire(gesture.Event{Kind: gesture.EvTextDone}, input{ev: ev})
	case TextCancel:
		err = e.fire(gesture.Event{Kind: gesture.EvTextCancel}, input{})
	case DoubleClick:
		err = e.doubleClick(ev)
	}
	return gw.Change{Vertices: e.changeV, Edges: e.changeE, Layout: e.layout}, err
}

// Zoom scales the view about its centre by the factor, in when it is
// above 1, and reports the change, which names nothing but the view.
// A zoom past the view's bounds is ignored.
func (e *Editor) Zoom(f float64) gw.Change {
	e.changeV, e.changeE, e.layout = e.changeV[:0], e.changeE[:0], false
	e.zoomBy(f)
	return gw.Change{Vertices: e.changeV, Edges: e.changeE, Layout: e.layout}
}

// Fit sets the view to the drawing's bounds, when it holds anything,
// and reports the change, which names nothing but the view.
func (e *Editor) Fit() gw.Change {
	e.changeV, e.changeE, e.layout = e.changeV[:0], e.changeE[:0], false
	e.fit()
	return gw.Change{Vertices: e.changeV, Edges: e.changeE, Layout: e.layout}
}

// toDoc maps client coordinates to document units through the viewport.
func (e *Editor) toDoc(p gw.Point) gw.Point {
	v := e.d.View
	if v.W <= 0 || v.H <= 0 || e.elemW <= 0 || e.elemH <= 0 {
		return p
	}
	return gw.Point{X: v.X + p.X*v.W/e.elemW, Y: v.Y + p.Y*v.H/e.elemH}
}

// snap rounds a point to the grid.
func (e *Editor) snap(p gw.Point) gw.Point {
	return gw.Point{X: math.Round(p.X/e.grid) * e.grid, Y: math.Round(p.Y/e.grid) * e.grid}
}

// rect is the box between two corners.
func rect(a, b gw.Point) gw.Viewport {
	return gw.Viewport{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y), W: math.Abs(b.X - a.X), H: math.Abs(b.Y - a.Y)}
}

// portPoint is where a port of a vertex is, or the vertex's centre.
func (e *Editor) portPoint(v gw.VertexIndex, p gw.PortIndex) gw.Point {
	if p == gw.None {
		return e.d.Vertices[v].Pos
	}
	return svg.PortAt(&e.d.Vertices[v], p)
}

// record merges a command's change into the event's.
func (e *Editor) record(ch gw.Change) {
	for _, v := range ch.Vertices {
		if len(e.changeV) < cap(e.changeV) {
			e.changeV = append(e.changeV, v)
		}
	}
	for _, x := range ch.Edges {
		if len(e.changeE) < cap(e.changeE) {
			e.changeE = append(e.changeE, x)
		}
	}
	e.layout = e.layout || ch.Layout
}

// apply runs a command in edit mode and records its change.
func (e *Editor) apply(c gw.Command) error {
	if e.g.View() {
		return ErrView
	}
	ch, err := e.d.Apply(c)
	if err != nil {
		return err
	}
	e.record(ch)
	return nil
}

// reset drops what a gesture or label editing held, when the machine
// ends them.
func (e *Editor) reset() {
	e.sizeV = gw.None
	e.connFrom, e.connPort, e.connKind = gw.None, gw.None, gw.None
	e.editV, e.editE, e.typed = gw.None, gw.None, ""
}

// input is what the editor knows of an event besides what the gesture
// machine reads: where the pointer is, what is under it, and the host's
// event itself, for its modifiers, key, item and text.
type input struct {
	p   gw.Point
	hit svg.Hit
	ev  Event
}

// fire hands the gesture machine one event and carries out its actions,
// in the order they are declared. A refusal in view mode is ErrView. Every action runs, so a
// gesture ends even when its command is refused, and the first error is
// the event's.
func (e *Editor) fire(ge gesture.Event, in input) error {
	var acts gesture.Actions
	var out fsm.Outcome
	e.g, acts, out = e.g.Fire(ge)
	switch out {
	case fsm.Rejected:
		return ErrView
	case fsm.Invalid:
		return errInvalid
	case fsm.Moved, fsm.Ignored:
	default:
		return errInvalid
	}
	var first error
	for a := gesture.Actions(1); a != 0; a <<= 1 {
		if acts&a == 0 {
			continue
		}
		if err := e.do(a, ge, in); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// do carries out one action.
func (e *Editor) do(a gesture.Actions, ge gesture.Event, in input) error {
	shift := in.ev.Modifiers&Shift != 0
	switch a {
	case gesture.ActPress:
		e.down, e.last = in.p, in.p
	case gesture.ActRestoreDrag:
		e.restoreDrag()
	case gesture.ActReset:
		e.reset()
	case gesture.ActSelectHit:
		e.selectHit(in.hit, ge.Shift)
	case gesture.ActClearSelection:
		e.clearSelection()
	case gesture.ActStartDrag:
		e.startDrag()
	case gesture.ActMoveDrag:
		e.moveDrag(in.p)
	case gesture.ActCommitDrag:
		return e.endDrag()
	case gesture.ActSelectIn:
		e.selectIn(rect(e.down, in.p), shift)
	case gesture.ActBeginConnect:
		e.beginConnect(in.p, in.hit)
	case gesture.ActEndConnect:
		return e.endConnect(in.p, shift)
	case gesture.ActPan:
		e.pan(in.p)
	case gesture.ActStartResize:
		e.startResize()
	case gesture.ActEndResize:
		return e.endResize()
	case gesture.ActPlacing:
		e.item = in.ev.Item
	case gesture.ActPlace:
		return e.place(in.p)
	case gesture.ActChooseEdge:
		e.chooseEdge(in.ev.Item)
	case gesture.ActTab:
		e.tab()
	case gesture.ActDelete:
		return e.deleteSelection()
	case gesture.ActNudge:
		return e.nudgeKey(in.ev.Key)
	case gesture.ActUndo:
		e.undo()
	case gesture.ActRedo:
		e.redo()
	case gesture.ActStartText:
		e.startText(ge.Kind, in.ev.Key)
	case gesture.ActRelabel:
		return e.relabel(in.ev.Text)
	default:
		return errInvalid
	}
	return nil
}

// hitOf is the gesture machine's name for what the pointer is over.
func hitOf(h svg.Hit) gesture.Hit {
	switch h.Kind {
	case svg.Background:
		return gesture.HitBackground
	case svg.VertexHit:
		return gesture.HitVertex
	case svg.OutlineHit:
		return gesture.HitOutline
	case svg.PortHit:
		return gesture.HitPort
	case svg.EdgeHit:
		return gesture.HitEdge
	}
	var none gesture.Hit // the machine refuses it
	return none
}

// selectHit selects what a press or a double-click hit.
func (e *Editor) selectHit(h svg.Hit, add bool) {
	if h.Kind == svg.EdgeHit {
		e.selectEdge(h.Edge, add)
		return
	}
	e.selectVertex(h.Vertex, add)
}
