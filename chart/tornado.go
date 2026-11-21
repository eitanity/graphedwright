package chart

import "io"

// TornadoRow is one input's row of a tornado: the outcome at its low
// setting and at its high, and what the settings were.
type TornadoRow struct {
	Label     string
	Low, High float64
	LowSet    string
	HighSet   string
}

// Tornado is a row for each input: a bar from the base to its outcome
// at the low setting and one to its outcome at the high, the rows
// sorted by how far apart the two are, the widest at the top, with a
// line at the base. LowName and HighName name the two in the legend and
// head their columns in the table, Header the rows'.
type Tornado struct {
	Title, Desc string
	Header      string
	Base        float64
	BaseLabel   string
	LowName     string
	HighName    string
	Format      Format
	Rows        []TornadoRow
}

// A tornado's rows: a bar's height and the gap between rows.
const (
	tornadoBar = 14.0
	tornadoGap = 8.0
)

// tornadoFrame is where a tornado is drawn.
type tornadoFrame struct {
	top, left float64 // the first row's top; the labels' right edge
	labelN    int
	x         scale
}

// Tornado draws tornado t to w.
func (r *Renderer) Tornado(w io.Writer, t Tornado) error {
	f, err := r.tornadoFrame(&t)
	if err != nil {
		return err
	}
	n := len(t.Rows)
	height := f.top + float64(float64(n)*(tornadoBar+tornadoGap)) + 4
	r.begin(w, "tornado", height, t.Title, t.Desc)
	o := r.o
	if t.LowName != "" || t.HighName != "" {
		o.Str(`<g class="legend">`)
		x := r.swatch(f.left, 15, -1, "low", t.LowName)
		r.swatch(x, 15, -1, "high", t.HighName)
		o.Str("</g>")
	}
	base := f.x.at(t.Base)
	o.Str(`<g class="axes">`)
	r.line("base", base, f.top-4, base, height-2)
	if t.BaseLabel != "" {
		r.label(base, f.top-8, "middle", "base-label", t.BaseLabel, 40)
	}
	o.Str("</g>")
	for i := range n {
		r.tornadoRow(&t, &f, i, base)
	}
	return r.end()
}

// checkTornado refuses a tornado past the capacities, of no rows, or of
// values that are not finite.
func (r *Renderer) checkTornado(t *Tornado) error {
	if len(t.Rows) > r.caps.Rows {
		return capErr("rows", len(t.Rows), r.caps.Rows)
	}
	if len(t.Rows) == 0 {
		return ErrEmpty
	}
	if err := checkValue("base", -1, t.Base); err != nil {
		return err
	}
	var x text
	x.add(t.Title)
	x.add(t.Desc)
	x.add(t.Header)
	x.add(t.BaseLabel)
	x.add(t.LowName)
	x.add(t.HighName)
	x.add(t.Format.Prefix)
	x.add(t.Format.Suffix)
	for i := range t.Rows {
		row := &t.Rows[i]
		if checkValue("row", i, row.Low) != nil || checkValue("row", i, row.High) != nil {
			return refuse("row", i, "not a finite value within 1e30")
		}
		x.add(row.Label)
		x.add(row.LowSet)
		x.add(row.HighSet)
	}
	return r.checkText(x)
}

// tornadoFrame checks t, sorts its rows and finds its frame.
func (r *Renderer) tornadoFrame(t *Tornado) (tornadoFrame, error) {
	if err := r.checkTornado(t); err != nil {
		return tornadoFrame{}, err
	}
	lo, hi, longest, wide := t.Base, t.Base, 0, 0
	for i := range t.Rows {
		row := &t.Rows[i]
		r.order[i], r.keys[i] = int32(i), abs(row.High-row.Low) //nolint:gosec // bounded by the capacity
		lo, hi = min(lo, row.Low, row.High), max(hi, row.Low, row.High)
		longest = max(longest, runes(row.Label))
		wide = max(wide, r.valueWidth(row.Low, t.Format), r.valueWidth(row.High, t.Format))
	}
	r.sortRows(len(t.Rows))
	f := tornadoFrame{top: 24}
	if t.LowName != "" || t.HighName != "" {
		f.top += legendRoom
	}
	f.left = min(float64(width(longest)+12), Width*0.35)
	f.labelN = chars(f.left - 12)
	room := float64(width(wide) + 6)
	if hi <= lo {
		hi = lo + 1
	}
	f.x = scale{lo: lo, hi: hi, a: f.left + room, b: Width - room}
	return f, nil
}

// tornadoRow writes drawn row i: its label, its two bars and their
// values at their ends.
func (r *Renderer) tornadoRow(t *Tornado, f *tornadoFrame, i int, base float64) {
	o := r.o
	row := &t.Rows[r.order[i]]
	y := f.top + float64(float64(i)*(tornadoBar+tornadoGap))
	o.Str(`<g class="row" data-i="`)
	o.Int(int(r.order[i]))
	o.Str(`">`)
	r.label(f.left-8, y+tornadoBar/2+4, "end", "row-label", row.Label, f.labelN)
	r.tornadoBar(t, f, row.Label, "low", "value low", row.Low, row.LowSet, base, y)
	r.tornadoBar(t, f, row.Label, "high", "value high", row.High, row.HighSet, base, y)
	o.Str("</g>")
}

// tornadoBar writes one end's bar from the base to v, of the class
// given, and its value of valueClass, titled with the row, the setting and the value, and the value
// at its end.
func (r *Renderer) tornadoBar(t *Tornado, f *tornadoFrame, label, class, valueClass string, v float64, set string, base, y float64) {
	o := r.o
	x := f.x.at(v)
	o.Str(`<rect class="bar `)
	o.Str(class)
	r.rectRest(min(base, x), y, abs(x-base), tornadoBar, true)
	o.Text(label)
	if set != "" {
		o.Str(", ")
		o.Text(set)
	}
	o.Str(": ")
	r.value(v, t.Format)
	r.rectClose()
	if v < t.Base {
		r.textOpen(x-3, y+tornadoBar-3, "end", valueClass)
	} else {
		r.textOpen(x+3, y+tornadoBar-3, "start", valueClass)
	}
	r.value(v, t.Format)
	r.textClose()
}
