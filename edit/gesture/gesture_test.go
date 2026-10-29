package gesture

import (
	"testing"

	"code.eitanity.com/merilo/fsm"
)

// TestPressWithoutAHit: a press or a double-click must say what it is
// over; the zero Hit is refused, not taken as the background. merilo's
// Total for this machine is in internal/verify, so this module needs only
// merilo's fsm, which the machine imports.
func TestPressWithoutAHit(t *testing.T) {
	for _, k := range []EventKind{EvPress, EvDoubleClick} {
		if _, _, out := New().Fire(Event{Kind: k}); out != fsm.Invalid {
			t.Errorf("kind %d with no hit: outcome %d, want Invalid", k, out)
		}
	}
}
