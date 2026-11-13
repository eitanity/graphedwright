package svg

import (
	"math"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
)

// TestHeadsOutsideTargets: in the demo's examples, as stored and laid
// out afresh in either flow, the head of an edge with a closed arrowhead
// at the end of a straight leg
// reaches no further than a quarter of its length past where the edge meets its
// target, so it is drawn outside the target and not hidden under it,
// also where bends stand nearer the target than a head's length, in line
// with the legs before or after a turn.
func TestHeadsOutsideTargets(t *testing.T) {
	for _, name := range exampleNames {
		for k, flow := range []gw.Flow{gw.LeftToRight, gw.LeftToRight, gw.TopToBottom} {
			d := readExample(t, name)
			label := name + " as stored"
			if k > 0 {
				d.SetFlow(flow)
				if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
					t.Fatal(err)
				}
				label = name + " " + strconv.Itoa(int(flow))
			}
			headsOutside(t, label, d)
		}
	}
}

// knownHidden are the edges whose head reaches further into the target:
// laid out left to right, e11 of the dependencies runs straight across
// layers with two legs of 2 at its end, in line after a slant, so its
// head points along them and reaches 6 past the end.
var knownHidden = map[string]gw.ID{"dependencies 0": "e11"}

func headsOutside(t *testing.T, name string, d *gw.Document) {
	t.Helper()
	for i := range d.EdgeSlots() {
		e := gw.EdgeIndex(i) //nolint:gosec // bounded by the capacity
		x := d.Edge(e)
		kind := d.Kinds().Edge(x.Kind())
		if !x.Live() || !kind.Arrow() || kind.Head() == gw.OpenHead || knownHidden[name] == x.ID() {
			continue
		}
		var lane [1]gw.Point
		w := wayOf(d, e, &lane)
		end, drop := w.shortened(kind)
		bends := w.drawn(drop)
		from := lastFrom(w.a, bends)
		l := math.Hypot(end.X-from.X, end.Y-from.Y)
		if legRoute(w.route, bends.Len() > 0, from, end) != gw.Straight || l == 0 {
			continue // a curve's head is along its tangent, and a curve ends unchanged
		}
		ux, uy := (end.X-from.X)/l, (end.Y-from.Y)/l
		// How far the head's tip lies past the end, along the head.
		past := float64((end.X+float64(ux*headLength)-w.b.X)*ux) + float64((end.Y+float64(uy*headLength)-w.b.Y)*uy)
		if past > headLength/4 {
			t.Errorf("%s: edge %s's head reaches %.2f past where it meets %s", name, x.ID(), past, d.Vertex(x.To()).ID())
		}
	}
}

// TestShortenedLegs: a straight edge's stroke ends a head's length back
// from its end along its last leg, when that is longer than the head;
// past a bend nearer the end than the head, along the legs in line with
// the last, leaving that bend undrawn, and past a bend given twice; a
// step into a run at the end shorter than the head that follows a turn,
// even a slight one or one turned back; and a curved last leg as a whole
// leg.
func TestShortenedLegs(t *testing.T) {
	var kind gw.EdgeKindView
	for _, n := range doc.Builtin() {
		if n.Name == "basic" {
			kind = n.Kinds.Edge(n.Kinds.EdgeNamed("arrow"))
		}
	}
	pt := func(x, y float64) gw.Point { return gw.Point{X: x, Y: y} }
	for _, c := range []struct {
		name  string
		bends []gw.Point
		b     gw.Point
		end   gw.Point
		drop  int
	}{
		{"a last leg of 10", []gw.Point{pt(100, 0)}, pt(110, 0), pt(101, 0), 0},
		{"a bend 8 from the end, in line", []gw.Point{pt(100, 0)}, pt(108, 0), pt(99, 0), 1},
		{"two legs of 4 after a turn", []gw.Point{pt(100, 0), pt(100, 50), pt(104, 50)}, pt(108, 50), pt(101, 50), 1},
		{"a leg of 4 after a slight turn", []gw.Point{pt(100, 0)}, pt(104, 1), pt(100+4/math.Hypot(4, 1), 1/math.Hypot(4, 1)), 0},
		{"a last leg of 9", []gw.Point{pt(100, 0)}, pt(109, 0), pt(100, 0), 1},
		{"a leg of 4 turned back across", []gw.Point{pt(100, 0)}, pt(96, 0), pt(99, 0), 0},
		{"a leg of 4 turned back down", []gw.Point{pt(0, 100)}, pt(0, 96), pt(0, 99), 0},
		{"a leg of a half after a turn", []gw.Point{pt(100, 0), pt(100, 50)}, pt(100.5, 50), pt(100.5, 50), 0},
		{"a bend twice, 4 from the end", []gw.Point{pt(100, 0), pt(100, 0)}, pt(104, 0), pt(95, 0), 2},
	} {
		w := way{a: pt(0, 0), through: gw.PointsOf(c.bends), b: c.b, route: gw.Straight}
		end, drop := w.shortened(kind)
		if math.Abs(end.X-c.end.X) > 1e-9 || math.Abs(end.Y-c.end.Y) > 1e-9 || drop != c.drop {
			t.Errorf("%s: the stroke ends at %v leaving %d bends undrawn, want %v and %d", c.name, end, drop, c.end, c.drop)
		}
	}
	// A curved last leg is shortened as a whole leg.
	w := way{a: pt(0, 0), through: gw.PointsOf([]gw.Point{pt(100, 0)}), b: pt(200, 50), route: gw.Curve}
	if end, drop := w.shortened(kind); end != w.shortenedLeg() || drop != 0 {
		t.Errorf("a curved last leg ends at %v leaving %d undrawn, want %v and none", end, drop, w.shortenedLeg())
	}
}
