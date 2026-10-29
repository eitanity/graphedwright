package verify_test

import (
	"testing"

	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/graphedwright/svg"
	"code.eitanity.com/merilo"
)

// kindOf is the step kind of st, failing t when the steps have none.
func kindOf(t *testing.T, w *world, st step) int {
	t.Helper()
	for k, s := range w.steps {
		if s == st {
			return k
		}
	}
	t.Fatalf("no step %s", st)
	return -1
}

// The steps the findings below are written in.
var (
	pressA      = step{gesture.EvPress, gesture.HitVertex, plain}
	shiftPressB = step{gesture.EvPress, gesture.HitVertex, shiftB}
	move1       = step{gesture.EvMove, 0, plain}
	move2       = step{gesture.EvMove, 0, second}
	release     = step{gesture.EvRelease, 0, plain}
	escape      = step{gesture.EvEscape, 0, plain}
	del         = step{gesture.EvDelete, 0, plain}
	ctrlZ       = step{gesture.EvUndo, 0, plain}
	enter       = step{gesture.EvStartText, 0, plain}
	left        = step{gesture.EvNudge, 0, plain}
	doubleA     = step{gesture.EvDoubleClick, gesture.HitVertex, plain}
	paletteV    = step{gesture.EvPaletteVertex, 0, plain}
	undoToAll   = step{how: undoAll}
)

// TestFindings replays one session for each bug the explorer found, each
// found with the kinds behind the earlier ones masked; the Ctrl-Z session
// is a hand probe of the same class. Each fails until its bug is fixed,
// and then keeps it fixed.
func TestFindings(t *testing.T) {
	nudges := []step{pressA, release, shiftPressB, release}
	for range 32 {
		nudges = append(nudges, left)
	}
	for _, c := range []struct {
		name  string
		steps []step
	}{
		{"a palette press drops the label being typed", []step{doubleA, paletteV}},
		{"Enter during a drag leaves the drag unlogged", []step{pressA, move1, enter, release, undoToAll}},
		{"Escape after ArrowLeft during a press reverts the nudge without the log", []step{pressA, left, escape, release, undoToAll}},
		{"Delete during a drag logs nothing for the drag", []step{pressA, move1, del, release, undoToAll}},
		{"Ctrl-Z during a drag undoes under the drag", []step{pressA, move1, release, pressA, move2, ctrlZ, release, undoToAll}},
		{"a second finger during a drag abandons it unlogged", []step{pressA, move1, pressA, release, release, undoToAll}},
		{"32 nudges of two vertices are all undone", append(nudges, undoToAll)},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			var tr merilo.Trace
			for _, st := range c.steps {
				tr = append(tr, kindOf(t, w, st))
			}
			if err := merilo.Replay(w, tr); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestStepsCoverTheMachine: every event kind the gesture machine names is
// given by some step, every hit it names is pressed and double-clicked,
// and each hit's point lands on that hit in the fixture. A kind or hit
// added to the machine fails here until the host's way of sending it is
// written in hostSteps and hitPoint.
func TestStepsCoverTheMachine(t *testing.T) {
	w := newWorld(t)
	given := map[gesture.EventKind]bool{}
	for _, st := range w.steps {
		given[st.kind] = true
	}
	for _, k := range named(func(v uint8) string { return gesture.EventKind(v).String() }) {
		if !given[gesture.EventKind(k)] {
			t.Errorf("no step gives %s", gesture.EventKind(k))
		}
	}
	want := map[gesture.Hit]svg.HitKind{
		gesture.HitBackground: svg.Background, gesture.HitVertex: svg.VertexHit,
		gesture.HitOutline: svg.OutlineHit, gesture.HitPort: svg.PortHit, gesture.HitEdge: svg.EdgeHit,
	}
	for _, h := range hits() {
		p, ok := w.hitPoint(h)
		if !ok {
			t.Errorf("no point for hit %s", h)
			continue
		}
		if got := svg.HitTest(w.d, p).Kind; got != want[h] {
			t.Errorf("hit %s: the point %v lands on kind %d, want %d", h, p, got, want[h])
		}
		for _, k := range []gesture.EventKind{gesture.EvPress, gesture.EvDoubleClick} {
			kindOf(t, w, step{k, h, plain})
		}
	}
}
