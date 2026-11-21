package chart

import (
	"io"
	"math"
)

// Each kind's table is HTML text: its title as the caption, a header
// row and a row for each point, bar, bin or row, each value in the
// chart's format with data-value holding it in full. It is checked and
// ordered as the chart is, and escaped as the SVG is.

// tableBegin starts a table of the kind given, its caption the title.
func (r *Renderer) tableBegin(w io.Writer, kind, title string) {
	o := r.o
	o.Reset(w)
	o.Str(`<table class="gw-chart-table chart-`)
	o.Str(kind)
	o.Str(`"><caption>`)
	o.Text(title)
	o.Str(`</caption><thead><tr>`)
}

// tableBody ends the header row and starts the body.
func (r *Renderer) tableBody() { r.o.Str(`</tr></thead><tbody>`) }

// tableEnd closes the table and writes it out.
func (r *Renderer) tableEnd() error {
	r.o.Str("</tbody></table>\n")
	r.o.Flush()
	return r.o.Err()
}

// th writes a header cell; in a body row, the row's own.
func (r *Renderer) th(s string, row bool) {
	if row {
		r.o.Str(`<th scope="row">`)
	} else {
		r.o.Str(`<th scope="col">`)
	}
	r.o.Text(s)
	r.o.Str("</th>")
}

// td writes a value's cell.
func (r *Renderer) td(v float64, f Format) {
	r.o.Str(`<td data-value="`)
	r.exact(v)
	r.o.Str(`">`)
	r.value(v, f)
	r.o.Str("</td>")
}

func (r *Renderer) rowOpen()  { r.o.Str("<tr>") }
func (r *Renderer) rowClose() { r.o.Str("</tr>") }

// PlotTable writes plot p's points as a table: a row for each point of
// each series, with the series named when there are more than one, and
// the band's columns when a series has one.
func (r *Renderer) PlotTable(w io.Writer, p Plot) error {
	if _, err := r.plotFrame(&p); err != nil {
		return err
	}
	named, band := len(p.Series) > 1, false
	for k := range p.Series {
		band = band || p.Series[k].Lo != nil
	}
	r.tableBegin(w, "plot", p.Title)
	if named {
		r.th("", false)
	}
	r.th(p.X.Label, false)
	r.th(p.Y.Label, false)
	if band {
		r.th(p.LoLabel, false)
		r.th(p.HiLabel, false)
	}
	r.tableBody()
	for k := range p.Series {
		s := &p.Series[k]
		for i := range s.X {
			r.rowOpen()
			if named {
				r.th(s.Name, true)
			}
			r.td(s.X[i], p.X.Format)
			r.td(s.Y[i], p.Y.Format)
			if s.Lo != nil {
				r.td(s.Lo[i], p.Y.Format)
				r.td(s.Hi[i], p.Y.Format)
			} else if band {
				r.o.Str("<td></td><td></td>")
			}
			r.rowClose()
		}
	}
	return r.tableEnd()
}

// BarsTable writes bars b as a table: a row for each bar drawn, in the
// order drawn, Others among them, and a column for each series.
func (r *Renderer) BarsTable(w io.Writer, b Bars) error {
	f, err := r.barsFrame(&b)
	if err != nil {
		return err
	}
	r.tableBegin(w, "bars", b.Title)
	r.th(b.Header, false)
	for k := range b.Series {
		r.th(b.Series[k].Name, false)
	}
	r.tableBody()
	for i := range f.rows {
		r.rowOpen()
		r.th(r.rowLabel(&b, &f, i), true)
		for k := range b.Series {
			r.td(r.barValue(&b, &f, i, k), b.Format)
		}
		r.rowClose()
	}
	return r.tableEnd()
}

// HistogramTable writes histogram h as a table: a row for each bin, its
// edges and its count.
func (r *Renderer) HistogramTable(w io.Writer, h Histogram) error {
	if err := r.checkHistogram(&h); err != nil {
		return err
	}
	r.tableBegin(w, "histogram", h.Title)
	r.th(h.X.Label, false)
	r.th(h.Y.Label, false)
	r.tableBody()
	o := r.o
	for i := range h.Counts {
		r.rowOpen()
		o.Str(`<th scope="row" data-lo="`)
		r.exact(h.Edges[i])
		o.Str(`" data-hi="`)
		r.exact(h.Edges[i+1])
		o.Str(`">`)
		r.value(h.Edges[i], h.X.Format)
		o.Str(" – ")
		r.value(h.Edges[i+1], h.X.Format)
		o.Str("</th>")
		r.td(h.Counts[i], h.Y.Format)
		r.rowClose()
	}
	return r.tableEnd()
}

// TornadoTable writes tornado t as a table: a row for each input, in the
// order drawn, its two outcomes and the settings that gave them.
func (r *Renderer) TornadoTable(w io.Writer, t Tornado) error {
	if _, err := r.tornadoFrame(&t); err != nil {
		return err
	}
	r.tableBegin(w, "tornado", t.Title)
	r.th(t.Header, false)
	r.th(t.LowName, false)
	r.th(t.HighName, false)
	r.tableBody()
	for i := range t.Rows {
		row := &t.Rows[r.order[i]]
		r.rowOpen()
		r.th(row.Label, true)
		r.tdSet(row.Low, row.LowSet, t.Format)
		r.tdSet(row.High, row.HighSet, t.Format)
		r.rowClose()
	}
	return r.tableEnd()
}

// tdSet writes a value's cell with the setting that gave it.
func (r *Renderer) tdSet(v float64, set string, f Format) {
	o := r.o
	o.Str(`<td data-value="`)
	r.exact(v)
	o.Str(`">`)
	r.value(v, f)
	if set != "" {
		o.Str(" (")
		o.Text(set)
		o.Byte(')')
	}
	o.Str("</td>")
}

// HeatTable writes heat table h as a table: its rows and columns, a
// cell with no value left empty.
func (r *Renderer) HeatTable(w io.Writer, h Heat) error {
	if err := r.checkHeat(&h); err != nil {
		return err
	}
	r.tableBegin(w, "heat", h.Title)
	r.th(h.Header, false)
	for _, c := range h.Cols {
		r.th(c, false)
	}
	r.tableBody()
	for i := range h.Rows {
		r.rowOpen()
		r.th(h.Rows[i], true)
		for j := range h.Cols {
			if v := h.Cells[i*len(h.Cols)+j]; !math.IsNaN(v) {
				r.td(v, h.Format)
			} else {
				r.o.Str("<td></td>")
			}
		}
		r.rowClose()
	}
	return r.tableEnd()
}
