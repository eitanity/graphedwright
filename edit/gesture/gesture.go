// Package gesture is the editor's control state as a value machine: what
// each pointer, key and host event means in each state, and what the
// editor must then do. It holds no geometry and no document. The edit
// package decodes the host's events into Events, fires them, and carries
// out the Actions in order. Every state–event pair is decided here, where
// merilo checks it: Total fires every event at every state, and merilovet
// holds this package and the edit package to merilo's FSM rules.
package gesture

import "code.eitanity.com/merilo/fsm"

// State is where the editor is between events.
type State uint8

// The states. The zero value is a sentinel that no machine built with
// New holds.
const (
	stateNone   State = iota
	Idle              // nothing in progress
	Selecting         // a marquee from the pointer-down point
	Dragging          // the selection follows the pointer
	Connecting        // an edge from a port or an outline to the pointer
	Placing           // a palette item follows the pointer
	Panning           // the viewport follows the pointer
	EditingText       // the host shows an input for a label, or the attribute its kind edits
	Resizing          // the selected vertex's corner follows the pointer

	// View mode's states and the host's hold's: each does what the edit
	// state Activity reports it as does, and returns only to its own
	// mode.
	ViewIdle        // view mode, nothing in progress
	ViewPanning     // view mode, the viewport follows the pointer
	ViewSelecting   // view mode, a marquee from the pointer-down point
	HeldIdle        // held in edit mode, nothing in progress
	HeldPanning     // held in edit mode, the viewport follows the pointer
	ViewHeldIdle    // held in view mode, nothing in progress
	ViewHeldPanning // held in view mode, the viewport follows the pointer
)

// EventKind is what happened, as the editor decodes a host's event.
type EventKind uint8

// The event kinds.
const (
	eventNone       EventKind = iota
	EvPress                   // a pointer went down on the drawing
	EvMove                    // the pointer moved
	EvRelease                 // the pointer went up
	EvEscape                  // Escape: cancel what is in progress
	EvTab                     // Tab: select the next vertex
	EvDelete                  // Delete or Backspace
	EvNudge                   // an arrow key
	EvUndo                    // Ctrl-Z
	EvRedo                    // Ctrl-Y or Ctrl-Shift-Z
	EvStartText               // Enter or F2: edit the selected element's label
	EvType                    // a printable key: edit the label, starting with it
	EvOtherKey                // any other key
	EvPaletteVertex           // a vertex kind pressed in the palette
	EvPaletteEdge             // an edge kind pressed in the palette
	EvBlur                    // the drawing lost the focus
	EvTextDone                // the host's label input committed its text
	EvTextCancel              // the host's label input was cancelled
	EvDoubleClick             // a double-click on the drawing
	EvView                    // switch to view mode
	EvEdit                    // switch to edit mode
	EvContext                 // a context request: the host's own actions on what is hit
	EvHold                    // the host holds the editor while it waits on work of its own
	EvUnhold                  // the host releases the hold
)

// Hit is what a press or a double-click is over.
type Hit uint8

// The hits. A press, a double-click or a context request with the zero
// Hit is refused.
const (
	hitNone       Hit = iota
	HitBackground     // nothing
	HitVertex         // a vertex's body
	HitOutline        // a vertex's outline, where an edge starts without a port
	HitPort           // one of a vertex's ports
	HitEdge           // an edge
)

// Event is one input. Hit is read for EvPress, EvDoubleClick and
// EvContext; Pan,
// Shift and Handle for EvPress; One for EvStartText and EvType.
type Event struct {
	Kind   EventKind
	Hit    Hit
	Pan    bool // the middle button or Alt: the press pans
	Shift  bool // Shift held: the press adds to the selection
	Handle bool // the press is on the resize handle of the one selected vertex
	One    bool // exactly one element is selected, so its label can be edited
}

// Actions is the set of things the editor must do after an event, one
// bit each. The zero value is the empty set.
type Actions uint64

// The actions, declared in the order the editor carries them out: what
// finishes a gesture, then what drops it, then what starts the next.
const (
	ActCommitText     Actions = 1 << iota // set the label, or the attribute its kind edits, of the element being edited to the host's text
	ActCommitDrag                         // move the dragged vertices by command
	ActSelectIn                           // select what the marquee holds
	ActEndResize                          // resize the vertex by command
	ActEndConnect                         // add the edge, if the pointer is over a vertex it can join
	ActPlace                              // add a vertex of that kind where the pointer is
	ActRestoreDrag                        // put the dragged vertices back where the drag began
	ActReset                              // drop what the gesture or the label editing held
	ActPress                              // remember where the pointer went down
	ActClearSelection                     // select nothing
	ActSelectHit                          // select what was hit, adding to the selection with Shift
	ActStartDrag                          // remember where the vertices to drag are
	ActStartResize                        // remember the vertex being resized and its corner
	ActBeginConnect                       // remember the port or outline the edge starts at
	ActStartText                          // begin editing the selected element's label, or the attribute its kind edits
	ActPlacing                            // remember the palette's vertex kind
	ActChooseEdge                         // choose the palette's edge kind for new edges, or unchoose it
	ActMoveDrag                           // move the dragged vertices with the pointer
	ActPan                                // move the viewport with the pointer
	ActTab                                // select the next vertex
	ActDelete                             // delete the selection
	ActNudge                              // move the selection by one grid unit
	ActUndo                               // undo one command
	ActRedo                               // redo one command
	ActContext                            // note the context request, once the selection is settled
)

// Machine is the editor's control state. Build it with New and change it
// only with Fire.
type Machine struct {
	state State
	moved bool // the pointer moved since the press that began the gesture
}

// New returns an idle machine in edit mode.
func New() Machine { return Machine{state: Idle} }

// State returns the control state.
func (m Machine) State() State { return m.state }

// Activity is what is in progress: the edit state whose work the state
// does, Idle, Panning or Selecting for view mode's and a hold's.
func (m Machine) Activity() State {
	switch m.state {
	case ViewIdle, HeldIdle, ViewHeldIdle:
		return Idle
	case ViewPanning, HeldPanning, ViewHeldPanning:
		return Panning
	case ViewSelecting:
		return Selecting
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing:
		return m.state
	default:
		return m.state
	}
}

// View reports view mode: whether the machine is in one of its states.
func (m Machine) View() bool {
	switch m.state {
	case ViewIdle, ViewPanning, ViewSelecting, ViewHeldIdle, ViewHeldPanning:
		return true
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		HeldIdle, HeldPanning:
		return false
	default:
		return false
	}
}

// Held reports whether the host holds the machine: whether it is in one
// of a hold's states.
func (m Machine) Held() bool {
	switch m.state {
	case HeldIdle, HeldPanning, ViewHeldIdle, ViewHeldPanning:
		return true
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewIdle, ViewPanning, ViewSelecting:
		return false
	default:
		return false
	}
}

// Fire applies e and returns the next machine, the actions the editor
// must carry out and what happened. Each event's transition names what
// it does in every state; an event a state of view mode or of a hold
// does not take is refused when it would change the document or start
// an edit, and ignored otherwise.
func (m Machine) Fire(e Event) (Machine, Actions, fsm.Outcome) {
	if !m.state.valid() {
		return m, 0, fsm.Invalid
	}
	var next Machine
	var acts Actions
	var out fsm.Outcome
	switch e.Kind {
	case EvPress:
		next, acts, out = m.press(e)
	case EvMove:
		next, acts, out = m.move()
	case EvRelease:
		next, acts, out = m.release()
	case EvEscape, EvBlur, EvTextCancel:
		next, acts = m.cancel(0)
		out = fsm.Moved
	case EvTab, EvDelete, EvNudge, EvUndo, EvRedo, EvStartText, EvType, EvOtherKey:
		next, acts, out = m.key(e)
	case EvPaletteVertex, EvPaletteEdge:
		next, acts, out = m.palette(e)
	case EvTextDone:
		next, acts, out = m.textDone()
	case EvDoubleClick:
		next, acts, out = m.doubleClick(e)
	case EvContext:
		next, acts, out = m.context(e)
	case EvView, EvEdit:
		next, acts = m.cancel(0)
		next.state = next.state.mode(e.Kind == EvView)
		out = fsm.Moved
	case EvHold:
		next, acts, out = m.hold()
	case EvUnhold:
		next, out = m.unhold()
	default:
		return m, 0, fsm.Invalid
	}
	return next, acts, out
}

// hold holds the machine, ending what is in progress as a mode change
// does, in a hold of the mode it is in. Holding twice is ignored.
func (m Machine) hold() (Machine, Actions, fsm.Outcome) {
	if m.Held() {
		return m, 0, fsm.Ignored
	}
	next, acts := m.cancel(0)
	switch next.state {
	case ViewIdle:
		next.state = ViewHeldIdle
		return next, acts, fsm.Moved
	case Idle:
		next.state = HeldIdle
		return next, acts, fsm.Moved
	case stateNone, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewPanning, ViewSelecting, HeldIdle, HeldPanning, ViewHeldIdle, ViewHeldPanning:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
}

// unhold releases the hold to the mode it was held in, a pan going on
// as one of that mode's. Releasing what is not held is ignored.
func (m Machine) unhold() (Machine, fsm.Outcome) {
	switch m.state {
	case HeldIdle:
		m.state = Idle
	case HeldPanning:
		m.state = Panning
	case ViewHeldIdle:
		m.state = ViewIdle
	case ViewHeldPanning:
		m.state = ViewPanning
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewIdle, ViewPanning, ViewSelecting:
		return m, fsm.Ignored
	default:
		return m, fsm.Ignored
	}
	return m, fsm.Moved
}

// valid reports whether s is a state New or Fire gives.
func (s State) valid() bool {
	switch s {
	case Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewIdle, ViewPanning, ViewSelecting, HeldIdle, HeldPanning, ViewHeldIdle, ViewHeldPanning:
		return true
	case stateNone:
		return false
	default:
		return false
	}
}

// idle is the state nothing is in progress in, in s's mode.
func (s State) idle() State {
	switch s {
	case ViewIdle, ViewPanning, ViewSelecting:
		return ViewIdle
	case HeldIdle, HeldPanning:
		return HeldIdle
	case ViewHeldIdle, ViewHeldPanning:
		return ViewHeldIdle
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing:
		return Idle
	default:
		return Idle
	}
}

// mode is the idle state of view mode, or of edit mode, held if s is.
func (s State) mode(view bool) State {
	switch s.idle() {
	case HeldIdle, ViewHeldIdle:
		if view {
			return ViewHeldIdle
		}
		return HeldIdle
	case stateNone, Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewIdle, ViewPanning, ViewSelecting, HeldPanning, ViewHeldPanning:
		if view {
			return ViewIdle
		}
		return Idle
	default:
		return Idle
	}
}

// valid reports whether h names something a pointer can be over.
func (h Hit) valid() bool {
	switch h {
	case HitBackground, HitVertex, HitOutline, HitPort, HitEdge:
		return true
	default:
		return false
	}
}

// gesture reports whether a pointer gesture is under way: one its
// release finishes and Escape cancels.
func (m Machine) gesture() bool {
	switch m.state {
	case Selecting, Dragging, Connecting, Placing, Panning, Resizing,
		ViewPanning, ViewSelecting, HeldPanning, ViewHeldPanning:
		return true
	case stateNone, Idle, EditingText, ViewIdle, HeldIdle, ViewHeldIdle:
		return false
	default:
		return false
	}
}

// cancel ends whatever is in progress, after acts: a drag goes back to
// where it began, and the gesture or label editing is dropped, to the
// idle state of the mode the machine is in.
func (m Machine) cancel(acts Actions) (Machine, Actions) {
	if m.state == Dragging {
		acts |= ActRestoreDrag
	}
	m.state, m.moved = m.state.idle(), false
	return m, acts | ActReset
}

// press starts a pan, a connection, a drag, a resize or a marquee, by
// what is under the pointer. A second pointer while a gesture is under
// way is ignored, so the gesture keeps its own start and its release
// still finishes it. A press during a palette drop or a connection only
// moves the pointer: the release finishes them. In view mode a press
// pans, selects or starts a marquee; held, it only pans.
func (m Machine) press(e Event) (Machine, Actions, fsm.Outcome) {
	if !e.Hit.valid() {
		return m, 0, fsm.Invalid
	}
	acts := ActPress
	switch m.state {
	case Selecting, Dragging, Panning, Resizing, ViewPanning, ViewSelecting, HeldPanning, ViewHeldPanning:
		return m, 0, fsm.Ignored
	case ViewIdle:
		return m.pressView(e, acts)
	case HeldIdle:
		return m.pressHeld(e, acts, HeldPanning)
	case ViewHeldIdle:
		return m.pressHeld(e, acts, ViewHeldPanning)
	case EditingText:
		m, acts = m.cancel(acts)
	case Idle, Connecting, Placing:
	case stateNone:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
	m.moved = false
	if e.Pan {
		m.state = Panning
		return m, acts, fsm.Moved
	}
	if m.state == Placing || m.state == Connecting {
		return m, acts, fsm.Moved
	}
	if e.Handle {
		m.state = Resizing
		return m, acts | ActStartResize, fsm.Moved
	}
	return m.pressOn(e, acts)
}

// pressView is a press in view mode with nothing in progress: a pan, a
// marquee on the background, and otherwise what is hit selected.
func (m Machine) pressView(e Event, acts Actions) (Machine, Actions, fsm.Outcome) {
	m.moved = false
	switch {
	case e.Pan:
		m.state = ViewPanning
		return m, acts, fsm.Moved
	case e.Hit == HitBackground:
		if !e.Shift {
			acts |= ActClearSelection
		}
		m.state = ViewSelecting
		return m, acts, fsm.Moved
	}
	return m, acts | ActSelectHit, fsm.Moved
}

// pressHeld is a press while held with nothing in progress: a pan, in
// the held mode's panning, and nothing else.
func (m Machine) pressHeld(e Event, acts Actions, panning State) (Machine, Actions, fsm.Outcome) {
	if !e.Pan {
		return m, 0, fsm.Rejected
	}
	m.state, m.moved = panning, false
	return m, acts, fsm.Moved
}

// pressOn is a press in edit mode from Idle on what e.Hit names.
func (m Machine) pressOn(e Event, acts Actions) (Machine, Actions, fsm.Outcome) {
	switch e.Hit {
	case HitOutline, HitPort:
		m.state = Connecting
		return m, acts | ActBeginConnect, fsm.Moved
	case HitVertex:
		m.state = Dragging
		return m, acts | ActSelectHit | ActStartDrag, fsm.Moved
	case HitEdge:
		return m, acts | ActSelectHit, fsm.Moved
	case HitBackground:
		if !e.Shift {
			acts |= ActClearSelection
		}
		m.state = Selecting
		return m, acts, fsm.Moved
	default:
		return m, 0, fsm.Invalid
	}
}

// move follows the pointer: a pan, a drag, or the first move of a
// gesture whose release depends on whether the pointer moved.
func (m Machine) move() (Machine, Actions, fsm.Outcome) {
	switch m.state {
	case Panning, ViewPanning, HeldPanning, ViewHeldPanning:
		return m, ActPan, fsm.Moved
	case Dragging:
		m.moved = true
		return m, ActMoveDrag, fsm.Moved
	case Selecting, ViewSelecting, Connecting, Placing, Resizing:
		if m.moved {
			return m, 0, fsm.Ignored
		}
		m.moved = true
		return m, 0, fsm.Moved
	case Idle, EditingText, ViewIdle, HeldIdle, ViewHeldIdle:
		return m, 0, fsm.Ignored
	case stateNone:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
}

// release finishes the gesture under way, to the idle state of its mode.
// A release ends a gesture, not label editing, which goes on until the
// host's input commits or cancels.
func (m Machine) release() (Machine, Actions, fsm.Outcome) {
	var acts Actions
	switch m.state {
	case Idle, EditingText, ViewIdle, HeldIdle, ViewHeldIdle:
		return m, 0, fsm.Ignored
	case Dragging:
		if m.moved {
			acts = ActCommitDrag
		}
	case Selecting, ViewSelecting:
		if m.moved {
			acts = ActSelectIn
		}
	case Resizing:
		if m.moved {
			acts = ActEndResize
		}
	case Connecting:
		acts = ActEndConnect
	case Placing:
		acts = ActPlace
	case Panning, ViewPanning, HeldPanning, ViewHeldPanning:
	case stateNone:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
	m.state, m.moved = m.state.idle(), false
	return m, acts | ActReset, fsm.Moved
}

// key is a key other than Escape and the zoom keys. During a gesture or
// label editing keys do nothing: the first would change the document or
// the selection under the gesture, and the host's input has the keyboard
// for the second. A key that is no command, and one that would edit a
// label with no one element to edit, do nothing; in view mode and held,
// only Tab acts, and what would change the document is refused.
func (m Machine) key(e Event) (Machine, Actions, fsm.Outcome) {
	if m.gesture() || m.state == EditingText {
		return m, 0, fsm.Ignored
	}
	if e.Kind == EvTab {
		return m, ActTab, fsm.Moved
	}
	if e.Kind == EvOtherKey || (e.Kind == EvStartText || e.Kind == EvType) && !e.One {
		return m, 0, fsm.Ignored
	}
	switch m.state {
	case ViewIdle, HeldIdle, ViewHeldIdle:
		return m, 0, fsm.Rejected
	case Idle:
	case stateNone, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing,
		ViewPanning, ViewSelecting, HeldPanning, ViewHeldPanning:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
	switch e.Kind {
	case EvDelete:
		return m, ActDelete, fsm.Moved
	case EvNudge:
		return m, ActNudge, fsm.Moved
	case EvUndo:
		return m, ActUndo, fsm.Moved
	case EvRedo:
		return m, ActRedo, fsm.Moved
	case EvStartText, EvType:
		m.state = EditingText
		return m, ActStartText, fsm.Moved
	case EvOtherKey, EvTab, EvPress, EvMove, EvRelease, EvEscape, EvPaletteVertex, EvPaletteEdge,
		EvBlur, EvTextDone, EvTextCancel, EvDoubleClick, EvView, EvEdit, EvContext, EvHold, EvUnhold:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
}

// palette cancels what is in progress and begins placing a vertex kind,
// or chooses an edge kind for new edges; in view mode and held it is
// refused.
func (m Machine) palette(e Event) (Machine, Actions, fsm.Outcome) {
	switch m.state {
	case ViewIdle, ViewPanning, ViewSelecting, HeldIdle, HeldPanning, ViewHeldIdle, ViewHeldPanning:
		return m, 0, fsm.Rejected
	case Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing:
	case stateNone:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
	next, acts := m.cancel(0)
	if e.Kind == EvPaletteEdge {
		return next, acts | ActChooseEdge, fsm.Moved
	}
	next.state = Placing
	return next, acts | ActPlacing, fsm.Moved
}

// textDone relabels the element being edited and ends the editing.
func (m Machine) textDone() (Machine, Actions, fsm.Outcome) {
	if m.state != EditingText {
		return m, 0, fsm.Ignored
	}
	next, acts := m.cancel(ActCommitText)
	return next, acts, fsm.Moved
}

// doubleClick cancels what is in progress and edits the label of what is
// under the pointer, selecting it alone; in view mode and held only the
// cancel is done, on the background, and the label editing refused.
func (m Machine) doubleClick(e Event) (Machine, Actions, fsm.Outcome) {
	if !e.Hit.valid() {
		return m, 0, fsm.Invalid
	}
	switch m.state {
	case ViewIdle, ViewPanning, ViewSelecting, HeldIdle, HeldPanning, ViewHeldIdle, ViewHeldPanning:
		if e.Hit == HitBackground {
			next, acts := m.cancel(0)
			return next, acts, fsm.Moved
		}
		return m, 0, fsm.Rejected
	case Idle, Selecting, Dragging, Connecting, Placing, Panning, EditingText, Resizing:
	case stateNone:
		return m, 0, fsm.Invalid
	default:
		return m, 0, fsm.Invalid
	}
	next, acts := m.cancel(0)
	switch e.Hit {
	case HitBackground:
		return next, acts, fsm.Moved
	case HitVertex, HitOutline, HitPort, HitEdge:
		next.state = EditingText
		// Alone: one of several selected is selected afresh.
		return next, acts | ActClearSelection | ActSelectHit | ActStartText, fsm.Moved
	default:
		return m, 0, fsm.Invalid
	}
}

// context ends what is in progress, as Escape does, and settles the
// selection the host's actions apply to: what is hit, unless it is
// selected already, when the selection stays; nothing, on the
// background. It is a click's rule, so it holds in view mode too, and
// changes no document.
func (m Machine) context(e Event) (Machine, Actions, fsm.Outcome) {
	if !e.Hit.valid() {
		return m, 0, fsm.Invalid
	}
	next, acts := m.cancel(0)
	if e.Hit == HitBackground {
		return next, acts | ActClearSelection | ActContext, fsm.Moved
	}
	return next, acts | ActSelectHit | ActContext, fsm.Moved
}

// invalidName is what String returns for a value outside the declared
// constants.
const invalidName = "invalid"

func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Selecting:
		return "Selecting"
	case Dragging:
		return "Dragging"
	case Connecting:
		return "Connecting"
	case Placing:
		return "Placing"
	case Panning:
		return "Panning"
	case EditingText:
		return "EditingText"
	case Resizing:
		return "Resizing"
	case ViewIdle:
		return "ViewIdle"
	case ViewPanning:
		return "ViewPanning"
	case ViewSelecting:
		return "ViewSelecting"
	case HeldIdle:
		return "HeldIdle"
	case HeldPanning:
		return "HeldPanning"
	case ViewHeldIdle:
		return "ViewHeldIdle"
	case ViewHeldPanning:
		return "ViewHeldPanning"
	case stateNone:
		return invalidName
	default:
		return invalidName
	}
}

func (k EventKind) String() string {
	switch k {
	case EvPress:
		return "Press"
	case EvMove:
		return "Move"
	case EvRelease:
		return "Release"
	case EvEscape:
		return "Escape"
	case EvTab:
		return "Tab"
	case EvDelete:
		return "Delete"
	case EvNudge:
		return "Nudge"
	case EvUndo:
		return "Undo"
	case EvRedo:
		return "Redo"
	case EvStartText:
		return "StartText"
	case EvType:
		return "Type"
	case EvOtherKey:
		return "OtherKey"
	case EvPaletteVertex:
		return "PaletteVertex"
	case EvPaletteEdge:
		return "PaletteEdge"
	case EvBlur:
		return "Blur"
	case EvTextDone:
		return "TextDone"
	case EvTextCancel:
		return "TextCancel"
	case EvDoubleClick:
		return "DoubleClick"
	case EvView:
		return "View"
	case EvEdit:
		return "Edit"
	case EvContext:
		return "Context"
	case EvHold:
		return "Hold"
	case EvUnhold:
		return "Unhold"
	default:
		return invalidName
	}
}

func (h Hit) String() string {
	switch h {
	case HitBackground:
		return "Background"
	case HitVertex:
		return "Vertex"
	case HitOutline:
		return "Outline"
	case HitPort:
		return "Port"
	case HitEdge:
		return "Edge"
	default:
		return invalidName
	}
}
