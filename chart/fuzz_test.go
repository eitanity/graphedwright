package chart_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"code.eitanity.com/graphedwright/chart"
)

// FuzzChart: any values, within the capacities or past them, give an
// error or a well-formed chart and table, never a panic.
func FuzzChart(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, uint8(0), "a<b")
	f.Add(bytes.Repeat([]byte{0x7f, 0xf0, 0, 0, 0, 0, 0, 0}, 4), uint8(7), "")
	f.Add(bytes.Repeat([]byte{0x3f, 0xf0, 0, 0, 0, 0, 0, 1}, 9), uint8(19), "€")
	r, err := chart.New(chart.Caps{Points: 64, Series: 4, Rows: 16, Text: 256})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, data []byte, mode uint8, name string) {
		var vs []float64
		for len(data) >= 8 {
			vs = append(vs, math.Float64frombits(binary.BigEndian.Uint64(data)))
			data = data[8:]
		}
		n := len(vs) / 2
		xs, ys := vs[:n], vs[n:2*n]
		style := chart.Format{Style: chart.Style(mode % 4), Digits: int(mode%7) - 1, Prefix: name}
		axis := chart.Axis{Label: name, Scale: chart.Scale(mode >> 2 % 3), Format: style}
		names := make([]string, n)
		for i := range names {
			names[i] = name
		}
		rows := make([]chart.TornadoRow, n)
		for i := range rows {
			rows[i] = chart.TornadoRow{Label: name, Low: xs[i], High: ys[i]}
		}
		var lo, hi []float64
		if mode&64 != 0 {
			lo, hi = xs, ys
		}
		cols := min(n, 3)
		cells := ys
		if cols > 0 {
			cells = ys[:n/cols*cols]
		}
		edges := append([]float64{0}, xs...)
		p := chart.Plot{Title: name, X: axis, Y: axis, Series: []chart.Series{{Name: name, X: xs, Y: ys, Lo: lo, Hi: hi, Draw: chart.Draw(mode % 4)}}}
		if len(vs) > 0 {
			p.Marks = []chart.Mark{{X: vs[0], Y: vs[len(vs)-1], Label: name}}
		}
		b := chart.Bars{Title: name, Labels: names, Series: []chart.BarSeries{{Name: name, Values: xs}}, Format: style, Order: chart.Order(mode % 3), Top: int(mode % 5), Others: name}
		h := chart.Histogram{Title: name, X: chart.Axis{Format: style}, Edges: edges, Counts: xs}
		tr := chart.Tornado{Title: name, Base: float64(mode), Format: style, Rows: rows, LowName: name}
		ht := chart.Heat{Title: name, Rows: names[:n/max(cols, 1)], Cols: names[:cols], Cells: cells, Format: style}
		draws := []func(io.Writer) error{
			func(w io.Writer) error { return r.Plot(w, p) }, func(w io.Writer) error { return r.PlotTable(w, p) },
			func(w io.Writer) error { return r.Bars(w, b) }, func(w io.Writer) error { return r.BarsTable(w, b) },
			func(w io.Writer) error { return r.Histogram(w, h) }, func(w io.Writer) error { return r.HistogramTable(w, h) },
			func(w io.Writer) error { return r.Tornado(w, tr) }, func(w io.Writer) error { return r.TornadoTable(w, tr) },
			func(w io.Writer) error { return r.Heat(w, ht) }, func(w io.Writer) error { return r.HeatTable(w, ht) },
		}
		for _, d := range draws {
			var out bytes.Buffer
			if err := d(&out); err != nil {
				if out.Len() != 0 {
					t.Fatalf("refused (%v) after writing %d bytes", err, out.Len())
				}
				continue
			}
			wellFormed(t, "fuzzed", out.Bytes())
		}
	})
}

// TestLog10: the chart's own log10 is within a few units in the last
// place of the standard library's, across the range a chart takes.
func TestLog10(t *testing.T) {
	for e := -30.0; e <= 30; e += 0.01 {
		x := math.Pow(10, e)
		got, want := chart.Log10(x), math.Log10(x)
		if d := math.Abs(got - want); d > 4e-15*math.Max(1, math.Abs(want)) {
			t.Fatalf("log10(%g) = %.17g, want %.17g", x, got, want)
		}
	}
}
