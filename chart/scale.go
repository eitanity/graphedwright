package chart

import "math"

// Scale is how an axis spaces its values.
type Scale uint8

// The scales of an axis.
const (
	Linear Scale = iota // equal steps for equal differences
	Log                 // equal steps for equal ratios; a value of 0 or less is left out
)

// Axis is one of a plot's axes: its label, its scale, the format of its
// ticks and values, and its range, found from the values when Min and
// Max are both 0. On a log scale the range runs out to whole decades.
type Axis struct {
	Label  string
	Scale  Scale
	Format Format
	Min    float64
	Max    float64
}

// The natural logs log10 is built from, as their literals round.
const (
	ln2   = 0.6931471805599453
	ln10  = 2.302585092994046
	sqrt2 = 1.4142135623730951
)

// log10 is the base-10 log of x > 0 in plain arithmetic: the same last
// bit on every target, which math.Log, in assembly on some, is not. x is
// split into m·2^k with m within a factor √2 of 1, and ln m summed from
// the series of 2·atanh((m−1)/(m+1)), whose terms fall by 34 each.
func log10(x float64) float64 {
	m, k := math.Frexp(x)
	if m < 1/sqrt2 {
		m, k = float64(m*2), k-1
	}
	s := (m - 1) / (m + 1)
	s2 := float64(s * s)
	sum, term := 0.0, s
	// bound: 12 terms, past which a term is below 1e-18 of the first
	for i := range 12 {
		sum += term / float64(2*i+1)
		term = float64(term * s2)
	}
	return (float64(float64(k)*ln2) + float64(2*sum)) / ln10
}

// scale maps an axis's values onto the chart, from a at its least to b
// at its greatest, and lists its ticks.
type scale struct {
	log    bool
	lo, hi float64 // the range: values, or on a log scale the decades
	a, b   float64 // where lo and hi are drawn
	step   float64 // the linear ticks' step
	first  float64 // the least linear tick
}

// linearScale is a linear scale over lo to hi, out to the ticks either
// side unless fixed.
func linearScale(lo, hi float64, fixed bool) scale {
	if hi <= lo {
		d := abs(lo) / 2
		if d == 0 {
			d = 0.5
		}
		lo, hi = lo-d, hi+d
	}
	step := niceStep((hi - lo) / 5)
	if !fixed {
		lo, hi = float64(math.Floor(lo/step)*step), float64(math.Ceil(hi/step)*step)
	}
	return scale{lo: lo, hi: hi, step: step, first: float64(math.Ceil(lo/step) * step)}
}

// logScale is a log scale over the decades that hold lo to hi, both
// above 0.
func logScale(lo, hi float64) scale {
	e0, e1 := exponent(lo), exponent(hi)
	if pow10(e1) < hi {
		e1++
	}
	if e1 <= e0 {
		e1 = e0 + 1
	}
	return scale{log: true, lo: float64(e0), hi: float64(e1)}
}

// niceStep is the least of 1, 2 and 5 times a power of ten that is at
// least raw, raw > 0.
func niceStep(raw float64) float64 {
	p := pow10(exponent(raw))
	for _, m := range [...]float64{1, 2, 5} {
		if s := float64(m * p); s >= raw {
			return s
		}
	}
	return float64(10 * p)
}

// at is where value v is drawn; on a log scale a value at or below 0
// is drawn at the least.
func (s *scale) at(v float64) float64 {
	t := v
	if s.log {
		t = s.lo
		if v > 0 {
			t = log10(v)
		}
	}
	return s.a + float64((s.b-s.a)*float64((t-s.lo)/(s.hi-s.lo)))
}

// ticks is how many ticks the scale has: its decades, or its steps.
func (s *scale) ticks() int {
	if s.log {
		return int(s.hi-s.lo) + 1
	}
	return min(int(math.Floor((s.hi-s.first)/s.step+1e-9))+1, maxTicks)
}

// maxTicks bounds a linear scale's ticks, for a range a host fixed.
const maxTicks = 64

// tick is the value of tick k.
func (s *scale) tick(k int) float64 {
	if s.log {
		return pow10(int(s.lo) + k)
	}
	return s.first + float64(float64(k)*s.step)
}

// span is how far the drawn range runs, from a to b.
func (s *scale) span() float64 { return s.b - s.a }
