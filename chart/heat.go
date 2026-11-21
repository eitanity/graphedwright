package chart

import (
	"io"
	"math"
)

// Heat is a table of cells, a row for each of Rows and a column for each
// of Cols, Cells row by row: each cell shaded on a ramp from 0 to the
// largest and its value written in it. A cell of NaN has no value and
// is left empty. Header heads the rows' column in its table.
type Heat struct {
	Title, Desc string
	Header      string
	Rows, Cols  []string
	Cells       []float64
	Format      Format
}

// The heat table's sizes: the row of column labels, a row's height, and
// the ramp's steps, from which the text is set light.
const (
	heatTop    = 22.0
	heatRow    = 26.0
	rampSteps  = 7
	darkFrom   = 3
	heatMaxCol = 0.3
)

// Heat draws heat table h to w.
func (r *Renderer) Heat(w io.Writer, h Heat) error {
	if err := r.checkHeat(&h); err != nil {
		return err
	}
	longest, most := 0, 0.0
	for _, l := range h.Rows {
		longest = max(longest, runes(l))
	}
	for _, v := range h.Cells {
		if !math.IsNaN(v) {
			most = max(most, v)
		}
	}
	left := min(float64(width(longest)+12), Width*heatMaxCol)
	cw := (Width - left) / float64(len(h.Cols))
	height := heatTop + float64(float64(len(h.Rows))*heatRow) + 2
	r.begin(w, "heat", height, h.Title, h.Desc)
	for j, c := range h.Cols {
		r.label(left+float64((float64(j)+0.5)*cw), 14, "middle", "col-label", c, chars(cw-4))
	}
	for i := range h.Rows {
		r.heatRow(&h, i, left, cw, most)
	}
	return r.end()
}

// heatRow writes row i: its label and its cells.
func (r *Renderer) heatRow(h *Heat, i int, left, cw, most float64) {
	o := r.o
	y := heatTop + float64(float64(i)*heatRow)
	o.Str(`<g class="row" data-i="`)
	o.Int(i)
	o.Str(`">`)
	r.label(left-8, y+heatRow/2+4, "end", "row-label", h.Rows[i], chars(left-12))
	for j := range h.Cols {
		v := h.Cells[i*len(h.Cols)+j]
		if math.IsNaN(v) {
			continue
		}
		step := 0
		if most > 0 && v > 0 {
			step = min(int(v/most*rampSteps), rampSteps-1)
		}
		x := left + float64(float64(j)*cw)
		o.Str(`<rect class="cell ramp-`)
		o.Int(step)
		r.rectRest(x+1, y+1, cw-2, heatRow-2, true)
		o.Text(h.Rows[i])
		o.Str(" → ")
		o.Text(h.Cols[j])
		o.Str(": ")
		r.value(v, h.Format)
		r.rectClose()
		class := "value on-light"
		if step >= darkFrom {
			class = "value on-dark"
		}
		r.textOpen(x+float64(cw/2), y+heatRow/2+4, "middle", class)
		r.value(v, h.Format)
		r.textClose()
	}
	o.Str("</g>")
}

// checkHeat refuses a heat table past the capacities, of no rows or
// columns, of cells that are not one for each, or of values that are
// not finite, NaN apart.
func (r *Renderer) checkHeat(h *Heat) error {
	if len(h.Rows) > r.caps.Rows || len(h.Cols) > r.caps.Rows {
		return capErr("rows or columns", max(len(h.Rows), len(h.Cols)), r.caps.Rows)
	}
	if len(h.Cells) > r.caps.Points {
		return capErr("cells", len(h.Cells), r.caps.Points)
	}
	if len(h.Rows) == 0 || len(h.Cols) == 0 {
		return ErrEmpty
	}
	if len(h.Cells) != len(h.Rows)*len(h.Cols) {
		return refuse("heat", -1, "not a cell for each row and column")
	}
	for i, v := range h.Cells {
		if math.IsNaN(v) {
			continue
		}
		if err := checkValue("cell", i, v); err != nil {
			return err
		}
	}
	var t text
	t.add(h.Title)
	t.add(h.Desc)
	t.add(h.Header)
	t.add(h.Format.Prefix)
	t.add(h.Format.Suffix)
	for _, l := range h.Rows {
		t.add(l)
	}
	for _, l := range h.Cols {
		t.add(l)
	}
	return r.checkText(t)
}
