// Package chart draws charts as SVG from plain values the host gives:
// a plot of series on two axes, bars, a histogram, a tornado and a heat
// table. A chart is text built by the same writer as a graph's, the
// same byte for byte on every target and toolchain, and carries classes
// for the host's stylesheet and a title on every mark for hover; no
// colour is written. Each kind's values are also written as an HTML
// table. A Renderer is made once, at fixed capacities, and allocates
// nothing to draw. No kind knows what its values mean: an error, a band
// or a level is the host's, given as values.
package chart

import (
	_ "embed"
	"io"
	"math"
	"unicode/utf8"

	"code.eitanity.com/graphedwright/internal/xmltext"
)

// Width is the width of every chart in its own units; the page scales
// it to its column.
const Width = 400

// The size of a chart's text, and the width a character of it takes:
// the estimate a graph's labels are laid out by, 7 at a size of 12,
// scaled.
const (
	fontSize  = 11
	charWidth = 7.0 * fontSize / 12
)

// MaxCap is the largest capacity of each kind.
const MaxCap = 1 << 20

// Caps are a renderer's capacities: what one chart may hold.
type Caps struct {
	Points int // the points of a plot's series together, a histogram's bins, a heat table's cells
	Series int // a plot's series and its marks together
	Rows   int // bars, a tornado's rows, a heat table's rows and its columns each
	Text   int // the bytes of every string a chart holds, together
}

// DefaultCaps are capacities for the charts of a page: a curve of a few
// thousand points, a table of a hundred rows.
func DefaultCaps() Caps { return Caps{Points: 4096, Series: 16, Rows: 128, Text: 16 << 10} }

// Renderer draws charts. It holds the output buffer and the scratch a
// chart needs, made by New; drawing allocates nothing.
type Renderer struct {
	caps  Caps
	o     *xmltext.Writer
	order []int32   // the rows in the order drawn
	keys  []float64 // what the rows are ordered by
	num   []byte    // a value formatted, to measure and to write
}

// Caps are the renderer's capacities.
func (r *Renderer) Caps() Caps { return r.caps }

// numRoom is the room kept for a value formatted: enough for any
// Format of a value within MaxValue.
const numRoom = 96

// MaxValue is the largest magnitude a chart takes, and on a log scale
// MinLog the smallest positive value: the range its exact powers of ten
// cover.
const (
	MaxValue = 1e30
	MinLog   = 1e-30
)

// checkValue refuses a value that is not finite or is past MaxValue.
func checkValue(what string, index int, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return refuse(what, index, "not a finite value")
	}
	if v > MaxValue || v < -MaxValue {
		return refuse(what, index, "past the largest value, 1e30")
	}
	return nil
}

// checkValues refuses any of vs that checkValue refuses.
func checkValues(what string, vs []float64) error {
	for i, v := range vs {
		if err := checkValue(what, i, v); err != nil {
			return err
		}
	}
	return nil
}

// text is a running count of a chart's text, checked against the
// capacity, and whether any of it is not UTF-8, which XML cannot carry.
type text struct {
	n   int
	bad bool
}

func (t *text) add(s string) {
	t.n += len(s)
	t.bad = t.bad || !utf8.ValidString(s)
}

// checkText refuses a chart whose text is past the capacity or not
// UTF-8.
func (r *Renderer) checkText(t text) error {
	if t.n > r.caps.Text {
		return capErr("bytes of text", t.n, r.caps.Text)
	}
	if t.bad {
		return refuse("text", -1, "not UTF-8")
	}
	return nil
}

// begin starts a chart of the kind and height given, written to w.
func (r *Renderer) begin(w io.Writer, kind string, height float64, title, desc string) {
	o := r.o
	o.Reset(w)
	o.Str(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	o.Int(Width)
	o.Byte(' ')
	o.Num(height)
	o.Str(`" class="gw-chart chart-`)
	o.Str(kind)
	o.Str(`" role="img" font-size="`)
	o.Int(fontSize)
	o.Str(`"><title>`)
	o.Text(title)
	o.Str(`</title><desc>`)
	o.Text(desc)
	o.Str(`</desc>`)
}

// end closes the chart and writes it out.
func (r *Renderer) end() error {
	r.o.Str("</svg>\n")
	r.o.Flush()
	return r.o.Err()
}

// Stylesheet is the default look of a chart and its table, for a host
// to serve as a file or to include in its own.
//
//go:embed chart.css
var Stylesheet string
