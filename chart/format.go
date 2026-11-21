package chart

import (
	"strconv"
	"unicode/utf8"
)

// Style is how a Format writes a value.
type Style uint8

// The styles of a Format.
const (
	Number  Style = iota // to Digits decimals
	Percent              // a fraction times 100, to Digits significant digits, with %
	Compact              // to Digits significant digits, with k, M, G or T from a thousand
)

// Format is how a value is written: in a style, to a number of digits,
// between the host's prefix and suffix (a currency, a unit), which are
// escaped. Digits below 1 are taken as 2 for Percent and Compact, and
// any above 15 as 15. The zero Format writes a whole number.
type Format struct {
	Style  Style
	Digits int
	Prefix string
	Suffix string
}

// decade is 10^(i-30), i from 0 to 60, as the literal rounds: the same
// on every target, where computing it might not be.
func decade(i int) float64 {
	return [...]float64{
		1e-30, 1e-29, 1e-28, 1e-27, 1e-26, 1e-25, 1e-24, 1e-23,
		1e-22, 1e-21, 1e-20, 1e-19, 1e-18, 1e-17, 1e-16, 1e-15,
		1e-14, 1e-13, 1e-12, 1e-11, 1e-10, 1e-9, 1e-8, 1e-7,
		1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1e0, 1e1,
		1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
		1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17,
		1e18, 1e19, 1e20, 1e21, 1e22, 1e23, 1e24, 1e25,
		1e26, 1e27, 1e28, 1e29, 1e30,
	}[i]
}

// decades is how many powers decade holds.
const decades = 61

// minDecade is the power of decade(0).
const minDecade = -30

// pow10 is 10^e, e held within the table.
func pow10(e int) float64 {
	return decade(min(max(e-minDecade, 0), decades-1))
}

// exponent is the power of ten of a's leading digit, a > 0: the largest
// e, within the table, with 10^e <= a.
func exponent(a float64) int {
	e := minDecade
	// bound: the table's powers, 61
	for e < minDecade+decades-1 && pow10(e+1) <= a {
		e++
	}
	return e
}

// sigDecimals is the decimals that write a, a magnitude, to n
// significant digits: one fewer when it rounds up to the next power.
func sigDecimals(a float64, n int) int {
	if a == 0 {
		return n - 1
	}
	e := exponent(a)
	if roundsUp(a, e, max(0, n-1-e)) {
		e++
	}
	return max(0, n-1-e)
}

// roundsUp reports whether a, with its leading digit at 10^e, written
// to d decimals, rounds up to 10^(e+1).
func roundsUp(a float64, e, d int) bool {
	return a >= pow10(e+1)-float64(0.5*pow10(-d))
}

// tier is one of Compact's letters and the power it stands for.
type tier struct {
	letter byte
	at     float64
}

// compactTier is Compact's tier k, from the least, which has no letter.
func compactTier(k int) tier {
	return [...]tier{{0, 1}, {'k', 1e3}, {'M', 1e6}, {'G', 1e9}, {'T', 1e12}}[k]
}

// compactTiers is how many tiers there are.
const compactTiers = 5

// format writes v in f's style into the renderer's scratch, without
// the prefix and suffix, and returns it.
func (r *Renderer) format(v float64, f Format) []byte {
	b := r.num[:0]
	digits := min(f.Digits, 15)
	switch f.Style {
	case Percent:
		v = float64(v * 100)
		if digits < 1 {
			digits = 2
		}
		b = strconv.AppendFloat(b, v, 'f', sigDecimals(abs(v), digits), 64)
		b = put(b, '%')
	case Compact:
		if digits < 1 {
			digits = 2
		}
		b = compact(b, v, digits)
	case Number:
		b = strconv.AppendFloat(b, v, 'f', max(digits, 0), 64)
	default: // a style past those named is written as a Number
		b = strconv.AppendFloat(b, v, 'f', max(digits, 0), 64)
	}
	return noNegativeZero(b)
}

// compact writes v to n significant digits in the largest tier it
// reaches once rounded, with the tier's letter.
func compact(b []byte, v float64, n int) []byte {
	a, k := abs(v), 0
	// bound: the tiers, 5
	for k+1 < compactTiers && reaches(a, compactTier(k+1).at, n) {
		k++
	}
	t := compactTier(k)
	w := v / t.at
	b = strconv.AppendFloat(b, w, 'f', sigDecimals(abs(w), n), 64)
	if t.letter != 0 {
		b = put(b, t.letter)
	}
	return b
}

// reaches reports whether magnitude a, written to n significant digits
// in the tier below at, rounds to at least at: a value of 999.97 in
// thousands is written as 1.00M.
func reaches(a, at float64, n int) bool {
	if a >= at {
		return true
	}
	u := a / (at / 1000)
	return exponent(u) == 2 && roundsUp(u, 2, max(0, n-3))
}

// put writes c after b, within the room the scratch keeps (numRoom),
// which a value formatted never fills.
func put(b []byte, c byte) []byte {
	n := len(b)
	b = b[:n+1]
	b[n] = c
	return b
}

// noNegativeZero drops the sign of a value that rounds to zero.
func noNegativeZero(b []byte) []byte {
	if len(b) == 0 || b[0] != '-' {
		return b
	}
	for _, c := range b[1:] {
		if c >= '1' && c <= '9' {
			return b
		}
	}
	copy(b, b[1:])
	return b[:len(b)-1]
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// value writes v in format f, its prefix and suffix escaped.
func (r *Renderer) value(v float64, f Format) {
	r.o.Text(f.Prefix)
	r.o.Bytes(r.format(v, f))
	r.o.Text(f.Suffix)
}

// valueWidth is the width value(v, f) takes, in characters.
func (r *Renderer) valueWidth(v float64, f Format) int {
	return utf8.RuneCountInString(f.Prefix) + len(r.format(v, f)) + utf8.RuneCountInString(f.Suffix)
}

// exact writes v in full, the shortest text that reads back as v.
func (r *Renderer) exact(v float64) {
	r.o.Bytes(strconv.AppendFloat(r.num[:0], v, 'g', -1, 64))
}
