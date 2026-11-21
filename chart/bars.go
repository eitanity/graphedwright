package chart

import "io"

// Order is the order bars are drawn in.
type Order uint8

// The orders of bars.
const (
	AsGiven Order = iota // as the labels are given
	Largest              // by the first series's value, the largest first, ties as given
)

// BarSeries is one series of bars: a value for each label.
type BarSeries struct {
	Name   string
	Values []float64
}

// Bars is a bar for each label in each of one or two series, drawn
// across from a baseline at 0 with each value written at its bar's tip.
// With Top above 0 and fewer than the labels, the first Top are drawn
// and one row named Others holds the sum of the rest, of each series.
// Header heads the labels' column in its table.
type Bars struct {
	Title, Desc string
	Header      string
	Labels      []string
	Series      []BarSeries
	Format      Format
	Order       Order
	Top         int
	Others      string
}

// The bars' sizes: a bar's height, the gap between a label's bars and
// the gap between labels.
const (
	barHeight = 12.0
	barGap    = 2.0
	groupGap  = 10.0
)

// barsFrame is where bars are drawn.
type barsFrame struct {
	top, left float64 // the first group's top; the labels' right edge
	zero      float64 // the baseline
	scale     float64 // units per unit of value
	group     float64 // a group's height
	rows      int     // the rows drawn, Others among them
	labelN    int     // the characters a label keeps
	others    [2]float64
}

// Bars draws bars b to w.
func (r *Renderer) Bars(w io.Writer, b Bars) error {
	f, err := r.barsFrame(&b)
	if err != nil {
		return err
	}
	height := f.top + float64(float64(f.rows)*(f.group+groupGap)) + 4
	r.begin(w, "bars", height, b.Title, b.Desc)
	if len(b.Series) > 1 {
		r.o.Str(`<g class="legend">`)
		x := f.left
		for k := range b.Series {
			x = r.swatch(x, 15, k, "", b.Series[k].Name)
		}
		r.o.Str("</g>")
	}
	r.o.Str(`<g class="axes">`)
	r.line("axis y", f.zero, f.top-2, f.zero, height-4)
	r.o.Str("</g>")
	for i := range f.rows {
		r.barRow(&b, &f, i)
	}
	return r.end()
}

// checkBars refuses bars past the capacities, of no series or more than
// two, or of values that are not finite or as many as the labels.
func (r *Renderer) checkBars(b *Bars) error {
	if len(b.Series) < 1 || len(b.Series) > 2 {
		return refuse("bars", -1, "not one or two series")
	}
	if len(b.Labels) > r.caps.Rows {
		return capErr("rows", len(b.Labels), r.caps.Rows)
	}
	if len(b.Labels) == 0 {
		return ErrEmpty
	}
	if b.Order > Largest || b.Top < 0 {
		return refuse("bars", -1, "no such Order, or Top below 0")
	}
	var t text
	t.add(b.Title)
	t.add(b.Desc)
	t.add(b.Header)
	t.add(b.Others)
	t.add(b.Format.Prefix)
	t.add(b.Format.Suffix)
	for _, l := range b.Labels {
		t.add(l)
	}
	for k := range b.Series {
		s := &b.Series[k]
		t.add(s.Name)
		if len(s.Values) != len(b.Labels) {
			return refuse("series", k, "not a value for each label")
		}
		if err := checkValues("bar value", s.Values); err != nil {
			return err
		}
	}
	return r.checkText(t)
}

// barsFrame checks b, orders its rows and finds its frame.
func (r *Renderer) barsFrame(b *Bars) (barsFrame, error) {
	if err := r.checkBars(b); err != nil {
		return barsFrame{}, err
	}
	n := len(b.Labels)
	first := b.Series[0].Values
	for i := range n {
		r.order[i], r.keys[i] = int32(i), first[i] //nolint:gosec // bounded by the capacity
	}
	if b.Order == Largest {
		r.sortRows(n)
	}
	f := barsFrame{top: 8, rows: n, group: float64(float64(len(b.Series))*(barHeight+barGap)) - barGap}
	if len(b.Series) > 1 {
		f.top = 28
	}
	if b.Top > 0 && b.Top < n {
		f.rows = b.Top + 1
		for _, i := range r.order[b.Top:n] {
			for k := range b.Series {
				f.others[k] += b.Series[k].Values[i]
			}
		}
	}
	r.barsScale(b, &f)
	return f, nil
}

// barsScale finds the labels' room and the scale: the values' range,
// 0 among them, across what is left with room for the values written.
func (r *Renderer) barsScale(b *Bars, f *barsFrame) {
	longest, lo, hi, wide := 0, 0.0, 0.0, 0
	for i := range f.rows {
		longest = max(longest, runes(r.rowLabel(b, f, i)))
		for k := range b.Series {
			v := r.barValue(b, f, i, k)
			lo, hi = min(lo, v), max(hi, v)
			wide = max(wide, r.valueWidth(v, b.Format))
		}
	}
	f.left = min(float64(width(longest)+12), Width*0.4)
	f.labelN = chars(f.left - 12)
	room := float64(width(wide) + 8)
	from, to := f.left, Width-room
	if lo < 0 {
		from += room
	}
	if hi <= lo {
		hi = lo + 1
	}
	f.scale = (to - from) / (hi - lo)
	f.zero = from - float64(lo*f.scale)
}

// rowLabel is the label of drawn row i.
func (r *Renderer) rowLabel(b *Bars, f *barsFrame, i int) string {
	if f.rows < len(b.Labels) && i == f.rows-1 {
		return b.Others
	}
	return b.Labels[r.order[i]]
}

// barValue is series k's value in drawn row i.
func (r *Renderer) barValue(b *Bars, f *barsFrame, i, k int) float64 {
	if f.rows < len(b.Labels) && i == f.rows-1 {
		return f.others[k]
	}
	return b.Series[k].Values[r.order[i]]
}

// barRow writes drawn row i: its label and a bar for each series with
// its value at the tip.
func (r *Renderer) barRow(b *Bars, f *barsFrame, i int) {
	o := r.o
	y := f.top + float64(float64(i)*(f.group+groupGap))
	label := r.rowLabel(b, f, i)
	o.Str(`<g class="row" data-i="`)
	o.Int(i)
	o.Str(`">`)
	r.label(f.left-8, y+float64(f.group/2)+4, "end", "row-label", label, f.labelN)
	for k := range b.Series {
		v := r.barValue(b, f, i, k)
		by := y + float64(float64(k)*(barHeight+barGap))
		tip := f.zero + float64(v*f.scale)
		o.Str(`<path class="bar `)
		r.seriesClass(k)
		o.Str(`" d="`)
		r.barPath(f.zero, by, tip-f.zero, barHeight)
		o.Str(`">`)
		r.titleOpen()
		o.Text(label)
		if b.Series[k].Name != "" {
			o.Str(", ")
			o.Text(b.Series[k].Name)
		}
		o.Str(": ")
		r.value(v, b.Format)
		r.titleClose()
		o.Str("</path>")
		if v < 0 {
			r.textOpen(tip-4, by+barHeight-2, "end", "value")
		} else {
			r.textOpen(tip+4, by+barHeight-2, "start", "value")
		}
		r.value(v, b.Format)
		r.textClose()
	}
	o.Str("</g>")
}

// barPath writes a bar from x across w, which is below 0 for a bar to
// the left, and down h: its data end rounded by up to 4, square at the
// baseline.
func (r *Renderer) barPath(x, y, w, h float64) {
	o := r.o
	d := 1.0
	if w < 0 {
		d, w = -1, -w
	}
	c := min(4, w, h/2)
	end := x + float64(d*w)
	o.Byte('M')
	o.Point(x, y)
	o.Byte('H')
	o.Num(end - float64(d*c))
	o.Byte('Q')
	o.Point(end, y)
	o.Byte(' ')
	o.Point(end, y+c)
	o.Byte('V')
	o.Num(y + h - c)
	o.Byte('Q')
	o.Point(end, y+h)
	o.Byte(' ')
	o.Point(end-float64(d*c), y+h)
	o.Byte('H')
	o.Num(x)
	o.Byte('Z')
}

// sortRows orders r.order[:n] by r.keys, the largest first and ties by
// index, a total order, so a heapsort, which makes nothing, serves.
func (r *Renderer) sortRows(n int) {
	o := r.order[:n]
	// bound: the first half of the rows, each sifted once
	for i := n/2 - 1; i >= 0; i-- {
		r.sift(o, i, n)
	}
	// bound: the rows, each taken from the heap once
	for end := n - 1; end > 0; end-- {
		o[0], o[end] = o[end], o[0]
		r.sift(o, 0, end)
	}
}

// sift moves o[i] down the heap of o[:n] whose last in order is at the
// root.
func (r *Renderer) sift(o []int32, i, n int) {
	// bound: the heap's depth, log2 n
	for {
		c := 2*i + 1
		if c >= n {
			return
		}
		if c+1 < n && r.before(o[c], o[c+1]) {
			c++
		}
		if !r.before(o[i], o[c]) {
			return
		}
		o[i], o[c] = o[c], o[i]
		i = c
	}
}

// before reports whether row a goes before row b: the larger key
// first, ties by index.
func (r *Renderer) before(a, b int32) bool {
	if r.keys[a] != r.keys[b] {
		return r.keys[a] > r.keys[b]
	}
	return a < b
}
