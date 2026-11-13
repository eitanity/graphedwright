package gesture

import (
	"testing"

	"code.eitanity.com/merilo/fsm"
)

// TestPressWithoutAHit: a press, a double-click or a context request
// must say what it is over; the zero Hit is refused, not taken as the
// background. merilo's
// Total for this machine is in internal/verify, so this module needs only
// merilo's fsm, which the machine imports.
func TestPressWithoutAHit(t *testing.T) {
	for _, k := range []EventKind{EvPress, EvDoubleClick, EvContext} {
		if _, acts, out := New().Fire(Event{Kind: k}); out != fsm.Invalid || acts != 0 {
			t.Errorf("kind %d with no hit: outcome %d acts %b, want Invalid and none", k, out, acts)
		}
	}
}

// TestHoldOutcomes: a hold moves to HeldIdle, ending a gesture; a
// second hold, and a release of what is not held, are ignored; held, a
// press that would select or start a gesture and the keys that edit are
// rejected, a pan's press is not, and the release leaves the mode as it
// was.
func TestHoldOutcomes(t *testing.T) {
	m, acts, out := New().Fire(Event{Kind: EvPress, Hit: HitVertex})
	if out != fsm.Moved || m.State() != Dragging || acts&ActStartDrag == 0 {
		t.Fatalf("press on a vertex: %v state %s acts %b, want Dragging", out, m.State(), acts)
	}
	held, acts, out := m.Fire(Event{Kind: EvHold})
	if out != fsm.Moved || !held.Held() || held.State() != HeldIdle || held.Activity() != Idle || acts&ActRestoreDrag == 0 || acts&ActReset == 0 {
		t.Errorf("hold during a drag: %v held %v state %s acts %b", out, held.Held(), held.State(), acts)
	}
	if _, acts, out := held.Fire(Event{Kind: EvHold}); out != fsm.Ignored || acts != 0 {
		t.Errorf("a second hold: %v acts %b", out, acts)
	}
	if _, acts, out := New().Fire(Event{Kind: EvUnhold}); out != fsm.Ignored || acts != 0 {
		t.Errorf("a release not held: %v acts %b", out, acts)
	}
	for _, e := range []Event{
		{Kind: EvPress, Hit: HitVertex}, {Kind: EvPress, Hit: HitBackground}, {Kind: EvPress, Hit: HitPort},
		{Kind: EvDoubleClick, Hit: HitVertex}, {Kind: EvPaletteVertex}, {Kind: EvPaletteEdge},
		{Kind: EvDelete}, {Kind: EvNudge}, {Kind: EvUndo}, {Kind: EvRedo}, {Kind: EvStartText, One: true}, {Kind: EvType, One: true},
	} {
		if next, acts, out := held.Fire(e); out != fsm.Rejected || acts != 0 || next != held {
			t.Errorf("held, %s: %v acts %b", e.Kind, out, acts)
		}
	}
	if next, acts, out := held.Fire(Event{Kind: EvPress, Hit: HitVertex, Pan: true}); out != fsm.Moved || next.State() != HeldPanning || next.Activity() != Panning || acts&editWork != 0 {
		t.Errorf("held, a pan's press: %v state %s acts %b", out, next.State(), acts)
	}
	for _, e := range []Event{{Kind: EvTab}, {Kind: EvContext, Hit: HitVertex}, {Kind: EvEscape}} {
		if _, acts, out := held.Fire(e); out != fsm.Moved || acts&editWork != 0 {
			t.Errorf("held, %s: %v acts %b", e.Kind, out, acts)
		}
	}
	view, acts, out := held.Fire(Event{Kind: EvView})
	if out != fsm.Moved || !view.Held() || !view.View() || view.State() != ViewHeldIdle || acts&editWork != 0 {
		t.Errorf("view while held: %v held %v view %v acts %b", out, view.Held(), view.View(), acts)
	}
	free, acts, out := view.Fire(Event{Kind: EvUnhold})
	if out != fsm.Moved || free.Held() || !free.View() || free.State() != ViewIdle || acts&editWork != 0 {
		t.Errorf("released in view: %v held %v view %v acts %b", out, free.Held(), free.View(), acts)
	}
}

// table is every event with a named kind and hit, under every flag, and
// every machine Fire reaches from New with them; a Fire that does not
// move does nothing.
func table(t *testing.T) ([]Event, []Machine) {
	t.Helper()
	var evs []Event
	for k := 1; k < 256; k++ {
		for h := 1; h < 256; h++ {
			if EventKind(k).String() == invalidName || Hit(h).String() == invalidName {
				continue
			}
			for f := range 16 {
				evs = append(evs, Event{Kind: EventKind(k), Hit: Hit(h), Pan: f&1 != 0, Shift: f&2 != 0, Handle: f&4 != 0, One: f&8 != 0})
			}
		}
	}
	ms := []Machine{New()}
	seen := map[Machine]bool{ms[0]: true}
	// bound: each machine is listed once, and there are few: three fields
	for i := 0; i < len(ms); i++ {
		for _, e := range evs {
			next, acts, out := ms[i].Fire(e)
			if out != fsm.Moved && acts != 0 {
				t.Errorf("%s on %s: %v with acts %b", e.Kind, ms[i].state, out, acts)
			}
			if out == fsm.Moved && !seen[next] {
				seen[next] = true
				ms = append(ms, next)
			}
		}
	}
	return evs, ms
}

// editWork is the actions that edit the document, or begin or carry on
// an edit gesture or label editing, which view mode and a hold never do.
const editWork = ActCommitText | ActCommitDrag | ActEndResize | ActEndConnect | ActPlace | ActRestoreDrag |
	ActStartDrag | ActStartResize | ActBeginConnect | ActStartText | ActPlacing | ActChooseEdge | ActMoveDrag |
	ActDelete | ActNudge | ActUndo | ActRedo

// mode is a machine's mode: view or edit, held or not.
func mode(m Machine) [2]bool { return [2]bool{m.View(), m.Held()} }

// TestModesAreStates: the reachable states are the edit states and view
// mode's and the hold's own, 15; a mode changes only by a mode event; and
// each view and held state does the work of the edit state it reports as
// its activity: never an edit, the same where its edit counterpart does
// only what view mode keeps, and a refusal only where the counterpart
// would edit or, held, the press is not a pan.
func TestModesAreStates(t *testing.T) {
	evs, ms := table(t)
	states := map[State]bool{}
	for _, m := range ms {
		states[m.state] = true
	}
	if len(states) != 15 {
		t.Errorf("%d states reachable, want 15: %v", len(states), states)
	}
	for _, m := range ms {
		twin := Machine{state: m.Activity(), moved: m.moved}
		for _, e := range evs {
			next, acts, out := m.Fire(e)
			modal := e.Kind == EvView || e.Kind == EvEdit || e.Kind == EvHold || e.Kind == EvUnhold
			if out == fsm.Moved && mode(next) != mode(m) && !modal {
				t.Errorf("%s on %s: mode %v to %v", e.Kind, m.state, mode(m), mode(next))
			}
			if m == twin || modal {
				continue
			}
			if acts&editWork != 0 {
				t.Errorf("%s on %s: edit work %b", e.Kind, m.state, acts&editWork)
			}
			tNext, tActs, tOut := twin.Fire(e)
			heldPress := m.Held() && e.Kind == EvPress && !e.Pan
			edits := tActs&editWork != 0 ||
				tOut == fsm.Moved && (tNext.state == Dragging || tNext.state == Connecting || tNext.state == Placing || tNext.state == Resizing || tNext.state == EditingText)
			if out == fsm.Rejected && !edits && !heldPress {
				t.Errorf("%s on %s: refused, though %s does only %b with it", e.Kind, m.state, twin.state, tActs)
			}
			if !edits && !heldPress && (out != tOut || acts != tActs || next.Activity() != tNext.Activity()) {
				t.Errorf("%s on %s: %v %b to %s, but %s %v %b to %s", e.Kind, m.state, out, acts, next.Activity(), twin.state, tOut, tActs, tNext.Activity())
			}
		}
	}
}
