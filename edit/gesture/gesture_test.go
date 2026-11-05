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
		if _, _, out := New().Fire(Event{Kind: k}); out != fsm.Invalid {
			t.Errorf("kind %d with no hit: outcome %d, want Invalid", k, out)
		}
	}
}

// TestHoldOutcomes: a hold moves to Idle held, ending a gesture; a
// second hold, and a release of what is not held, are ignored; held, a
// press that would select or start a gesture and the keys that edit are
// rejected, a pan's press is not, and the release leaves the mode as it
// was.
func TestHoldOutcomes(t *testing.T) {
	m, _, _ := New().Fire(Event{Kind: EvPress, Hit: HitVertex})
	if m.State() != Dragging {
		t.Fatalf("state %s, want Dragging", m.State())
	}
	held, acts, out := m.Fire(Event{Kind: EvHold})
	if out != fsm.Moved || !held.Held() || held.State() != Idle || acts&ActRestoreDrag == 0 || acts&ActReset == 0 {
		t.Errorf("hold during a drag: %v held %v state %s acts %b", out, held.Held(), held.State(), acts)
	}
	if _, acts, out := held.Fire(Event{Kind: EvHold}); out != fsm.Ignored || acts != 0 {
		t.Errorf("a second hold: %v acts %b", out, acts)
	}
	if _, _, out := New().Fire(Event{Kind: EvUnhold}); out != fsm.Ignored {
		t.Errorf("a release not held: %v", out)
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
	if next, _, out := held.Fire(Event{Kind: EvPress, Hit: HitVertex, Pan: true}); out != fsm.Moved || next.State() != Panning {
		t.Errorf("held, a pan's press: %v state %s", out, next.State())
	}
	for _, e := range []Event{{Kind: EvTab}, {Kind: EvContext, Hit: HitVertex}, {Kind: EvEscape}} {
		if _, _, out := held.Fire(e); out != fsm.Moved {
			t.Errorf("held, %s: %v", e.Kind, out)
		}
	}
	view, _, _ := held.Fire(Event{Kind: EvView})
	if !view.Held() || !view.View() {
		t.Errorf("view while held: held %v view %v", view.Held(), view.View())
	}
	free, _, out := view.Fire(Event{Kind: EvUnhold})
	if out != fsm.Moved || free.Held() || !free.View() {
		t.Errorf("released in view: %v held %v view %v", out, free.Held(), free.View())
	}
}
