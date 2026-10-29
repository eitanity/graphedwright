package verify_test

import (
	"fmt"
	"testing"

	"code.eitanity.com/graphedwright/edit/gesture"
	"code.eitanity.com/merilo"
	"code.eitanity.com/merilo/fsm"
)

// The gesture machine's fields are unexported, so this table cannot set
// them: its machines are the zero machine and every one Fire reaches from
// New. Its event kinds and hits are every value with a name, so a kind
// or hit added to the package is checked here without being listed:
// merilovet makes String, like every other switch, name it. A kind the
// machine refuses everywhere is still fired, and fails.

// named is the values 1 to 255 whose String is a name.
func named(name func(v uint8) string) []uint8 {
	var vs []uint8
	for v := 1; v < 256; v++ {
		if name(uint8(v)) != "invalid" {
			vs = append(vs, uint8(v))
		}
	}
	return vs
}

func gestureEvents() []gesture.Event {
	hits := named(func(h uint8) string { return gesture.Hit(h).String() })
	kinds := named(func(k uint8) string { return gesture.EventKind(k).String() })
	evs := make([]gesture.Event, 1, 1+16*len(hits)*len(kinds)) // position 0 is the zero event
	for _, k := range kinds {
		for _, h := range hits {
			for flags := range 16 {
				evs = append(evs, gesture.Event{Kind: gesture.EventKind(k), Hit: gesture.Hit(h),
					Pan: flags&1 != 0, Shift: flags&2 != 0, Handle: flags&4 != 0, One: flags&8 != 0})
			}
		}
	}
	return evs
}

// reachable is every machine Fire reaches from New with the events, in
// the order a breadth-first walk finds them. Machines are comparable
// values, so the walk keeps them in a map. It ends when no new machine
// appears, which it must: a machine is three small fields.
func reachable(evs []gesture.Event) []gesture.Machine {
	ms := []gesture.Machine{gesture.New()}
	seen := map[gesture.Machine]bool{ms[0]: true}
	for i := 0; i < len(ms); i++ {
		for _, e := range evs {
			next, _, out := ms[i].Fire(e)
			if out == fsm.Moved && !seen[next] {
				seen[next] = true
				ms = append(ms, next)
			}
		}
	}
	return ms
}

type gestureTable struct {
	machines []gesture.Machine
	events   []gesture.Event
}

func (t gestureTable) Machines() int { return len(t.machines) }
func (t gestureTable) Events() int   { return len(t.events) }

func (t gestureTable) Fire(m, e int) (fsm.Outcome, bool) {
	before := t.machines[m]
	next, acts, out := before.Fire(t.events[e])
	return out, next == before && acts == 0
}

func (t gestureTable) Name(m, e int) string {
	g, ev := t.machines[m], t.events[e]
	return fmt.Sprintf("%s view=%t machine=%+v / %s on %s pan=%t shift=%t handle=%t one=%t",
		g.State(), g.View(), g, ev.Kind, ev.Hit, ev.Pan, ev.Shift, ev.Handle, ev.One)
}

// TestGestureTotal is merilo's Total for the gesture machine. It also
// fails on a named state no event reaches, which no test would then try.
func TestGestureTotal(t *testing.T) {
	evs := gestureEvents()
	ms := reachable(evs)
	t.Logf("%d machines reachable, %d events", len(ms), len(evs))
	got := map[gesture.State]bool{}
	for _, m := range ms {
		got[m.State()] = true
	}
	for _, s := range named(func(s uint8) string { return gesture.State(s).String() }) {
		if !got[gesture.State(s)] {
			t.Errorf("state %s is never reached", gesture.State(s))
		}
	}
	merilo.Total(t, gestureTable{machines: append([]gesture.Machine{{}}, ms...), events: evs})
}
