package dom

import (
	"testing"

	"code.eitanity.com/graphedwright/edit"
)

// TestSecondFingerIsDropped: a second pointer's press, moves and release
// never reach the editor, so its release cannot end the first pointer's
// drag; the first pointer's release does.
func TestSecondFingerIsDropped(t *testing.T) {
	var p pointers
	for i, c := range []struct {
		kind edit.EventKind
		id   int
		want bool
	}{
		{edit.PointerMove, 1, true}, // a hover
		{edit.PointerDown, 1, true},
		{edit.PointerDown, 2, false}, // a second finger
		{edit.PointerMove, 2, false},
		{edit.PointerUp, 2, false},
		{edit.PointerMove, 1, true},
		{edit.PointerUp, 1, true},
		{edit.PointerUp, 1, false}, // no pointer down
		{edit.PointerDown, 2, true},
		{edit.PointerDown, 2, true}, // its release was lost: start afresh
		{edit.Key, 3, true},
	} {
		if got := p.track(c.kind, c.id); got != c.want {
			t.Errorf("event %d (kind %d, pointer %d): forwarded %t, want %t", i, c.kind, c.id, got, c.want)
		}
	}
}

// TestCancelLetsTheFirstPointerGo: the browser cancelling the pointer that
// is down ends its gesture; cancelling another does nothing.
func TestCancelLetsTheFirstPointerGo(t *testing.T) {
	var p pointers
	p.track(edit.PointerDown, 1)
	if p.cancel(2) {
		t.Error("cancelling a pointer that is not down was taken")
	}
	if !p.cancel(1) {
		t.Error("cancelling the pointer that is down was not taken")
	}
	if p.track(edit.PointerUp, 1) {
		t.Error("a release after the cancel was forwarded")
	}
}
