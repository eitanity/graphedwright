package chart_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"code.eitanity.com/graphedwright/chart"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// The examples: one of each kind and of each use the design names.

func exceedance() chart.Plot {
	return chart.Plot{
		Title: "The chance a period's total reaches each value",
		Desc:  "On log scales, with a band of two standard errors either side.",
		X:     chart.Axis{Label: "total in a period, at least", Scale: chart.Log, Format: chart.Format{Style: chart.Compact, Digits: 2, Prefix: "€"}},
		Y:     chart.Axis{Label: "chance", Scale: chart.Log, Format: chart.Format{Style: chart.Percent, Digits: 1}},
		Series: []chart.Series{{Name: "total",
			X:  []float64{0, 1000, 5000, 20000, 1e5, 5e5},
			Y:  []float64{1, 0.9, 0.3, 0.08, 0.01, 0.0015},
			Lo: []float64{1, 0.85, 0.27, 0.06, 0.007, 0.0005},
			Hi: []float64{1, 0.95, 0.33, 0.1, 0.013, 0.0025}}},
		Marks:   []chart.Mark{{X: 21000, Y: 0.1, Label: "1 in 10"}, {X: 1.1e5, Y: 0.01, Label: "1 in 100"}},
		LoLabel: "low", HiLabel: "high",
	}
}

func scatter() chart.Plot {
	return chart.Plot{
		Title: "Spend against spend plus downtime cost",
		X:     chart.Axis{Label: "spend", Format: chart.Format{Style: chart.Compact, Prefix: "$"}},
		Y:     chart.Axis{Label: "spend + downtime", Format: chart.Format{Style: chart.Compact, Prefix: "$"}},
		Series: []chart.Series{
			{Name: "policy A", X: []float64{10e3, 20e3, 30e3, 40e3}, Y: []float64{90e3, 70e3, 62e3, 65e3}, Draw: chart.Points},
			{Name: "policy B <tiered>", X: []float64{15e3, 25e3, 35e3}, Y: []float64{80e3, 61e3, 58e3}, Draw: chart.LinePoints},
		},
	}
}

func fan() chart.Plot {
	return chart.Plot{
		Title: "Loss by period",
		X:     chart.Axis{Label: "period", Format: chart.Format{}},
		Y:     chart.Axis{Label: "loss", Format: chart.Format{Style: chart.Number, Digits: 1}},
		Series: []chart.Series{
			{Name: "mean", X: []float64{1, 2, 3, 4, 5}, Y: []float64{1.5, 2.25, 3, 3.5, 3.75}, Lo: []float64{1, 1.5, 2, 2.25, 2.5}, Hi: []float64{2, 3, 4, 4.75, 5}},
			{Name: "exact", X: []float64{1, 3, 5}, Y: []float64{1.5, 3.05, 3.7}, Draw: chart.Points},
		},
		LoLabel: "lo", HiLabel: "hi",
	}
}

func bars() chart.Bars {
	return chart.Bars{
		Title: "Share of the mean and of the bad periods", Header: "category",
		Labels: []string{"ransomware", "phishing", "insider & misuse", "outage", "fraud", "a very long category name that is cut"},
		Series: []chart.BarSeries{{Name: "mean", Values: []float64{0.31, 0.22, 0.05, 0.27, 0.1, 0.05}}, {Name: "bad periods", Values: []float64{0.45, 0.12, 0.08, 0.2, 0.1, 0.05}}},
		Format: chart.Format{Style: chart.Percent, Digits: 2}, Order: chart.Largest, Top: 4, Others: "others",
	}
}

func signed() chart.Bars {
	return chart.Bars{
		Title: "Change against last year", Labels: []string{"up", "down", "flat"},
		Series: []chart.BarSeries{{Values: []float64{12.5, -7.25, 0}}},
		Format: chart.Format{Digits: 1, Suffix: " pp"},
	}
}

func histogram() chart.Histogram {
	return chart.Histogram{
		Title:  "Periods by total",
		X:      chart.Axis{Label: "total", Format: chart.Format{Style: chart.Compact}},
		Y:      chart.Axis{Label: "periods"},
		Edges:  []float64{0, 2500, 5000, 7500, 10000, 12500},
		Counts: []float64{410, 290, 160, 90, 50},
	}
}

func tornado() chart.Tornado {
	return chart.Tornado{
		Title: "Mean total as each input moves", Header: "input", Base: 52000, BaseLabel: "base €52k",
		LowName: "low", HighName: "high", Format: chart.Format{Style: chart.Compact, Digits: 2, Prefix: "€"},
		Rows: []chart.TornadoRow{
			{Label: "attack rate", Low: 40000, High: 71000, LowSet: "p10", HighSet: "p90"},
			{Label: "recovery days", Low: 47000, High: 60000, LowSet: "2", HighSet: "9"},
			{Label: "detection", Low: 55000, High: 50000},
			{Label: "staff", Low: 51000, High: 53500},
		},
	}
}

func heat() chart.Heat {
	return chart.Heat{
		Title: "Where each source's events end", Header: "source",
		Rows: []string{"loss_of_cooling", "shipment"}, Cols: []string{"overheating", "controlled", "resend"},
		Cells:  []float64{0.074, 0.926, math.NaN(), 0, 0.8, 0.2},
		Format: chart.Format{Style: chart.Percent, Digits: 3},
	}
}

// draw draws example name by its kind, as the chart or its table.
func draw(t *testing.T, r *chart.Renderer, name string, table bool, w io.Writer) error {
	t.Helper()
	switch name {
	case "exceedance", "scatter", "fan":
		p := map[string]func() chart.Plot{"exceedance": exceedance, "scatter": scatter, "fan": fan}[name]()
		if table {
			return r.PlotTable(w, p)
		}
		return r.Plot(w, p)
	case "bars", "signed":
		b := map[string]func() chart.Bars{"bars": bars, "signed": signed}[name]()
		if table {
			return r.BarsTable(w, b)
		}
		return r.Bars(w, b)
	case "histogram":
		if table {
			return r.HistogramTable(w, histogram())
		}
		return r.Histogram(w, histogram())
	case "tornado":
		if table {
			return r.TornadoTable(w, tornado())
		}
		return r.Tornado(w, tornado())
	case "heat":
		if table {
			return r.HeatTable(w, heat())
		}
		return r.Heat(w, heat())
	}
	t.Fatalf("no example %s", name)
	return nil
}

var examples = []string{"exceedance", "scatter", "fan", "bars", "signed", "histogram", "tornado", "heat"}

func newRenderer(t testing.TB) *chart.Renderer {
	t.Helper()
	r, err := chart.New(chart.DefaultCaps())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestGolden: each example and its table are drawn as in
// testdata/golden, byte for byte; -update rewrites the files. Each is
// well-formed XML.
func TestGolden(t *testing.T) {
	r := newRenderer(t)
	for _, name := range examples {
		for _, table := range []bool{false, true} {
			file := name + ".svg"
			if table {
				file = name + ".html"
			}
			var got bytes.Buffer
			if err := draw(t, r, name, table, &got); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			wellFormed(t, file, got.Bytes())
			path := filepath.Join("testdata", "golden", file)
			if *update {
				if err := os.WriteFile(path, got.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(path) //nolint:gosec // repository file
			if err != nil {
				t.Fatalf("%s: no golden file; run with -update: %v", file, err)
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Errorf("%s differs from the golden file; if intended, regenerate with -update", file)
			}
		}
	}
}

// wellFormed fails unless b parses as XML.
func wellFormed(t *testing.T, name string, b []byte) {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		_, err := d.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("%s is not well-formed: %v\n%s", name, err, b)
		}
	}
}

// TestDrawAllocates: drawing any kind, and its table, makes nothing once
// the renderer is made.
func TestDrawAllocates(t *testing.T) {
	r := newRenderer(t)
	for _, name := range examples {
		for _, table := range []bool{false, true} {
			p, b, h, tr, ht := exceedance(), bars(), histogram(), tornado(), heat()
			sc, fa, sg := scatter(), fan(), signed()
			n := testing.AllocsPerRun(20, func() {
				var err error
				switch {
				case name == "exceedance" && !table:
					err = r.Plot(io.Discard, p)
				case name == "exceedance":
					err = r.PlotTable(io.Discard, p)
				case name == "scatter":
					err = r.Plot(io.Discard, sc)
				case name == "fan" && !table:
					err = r.Plot(io.Discard, fa)
				case name == "fan":
					err = r.PlotTable(io.Discard, fa)
				case name == "bars" && !table:
					err = r.Bars(io.Discard, b)
				case name == "bars":
					err = r.BarsTable(io.Discard, b)
				case name == "signed":
					err = r.Bars(io.Discard, sg)
				case name == "histogram" && !table:
					err = r.Histogram(io.Discard, h)
				case name == "histogram":
					err = r.HistogramTable(io.Discard, h)
				case name == "tornado" && !table:
					err = r.Tornado(io.Discard, tr)
				case name == "tornado":
					err = r.TornadoTable(io.Discard, tr)
				case !table:
					err = r.Heat(io.Discard, ht)
				default:
					err = r.HeatTable(io.Discard, ht)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
			if n != 0 {
				t.Errorf("%s (table %v): %v allocations", name, table, n)
			}
		}
	}
}

// TestRefusals: each refusal is reached, and writes nothing.
func TestRefusals(t *testing.T) {
	small, err := chart.New(chart.Caps{Points: 4, Series: 2, Rows: 2, Text: 64})
	if err != nil {
		t.Fatal(err)
	}
	r := newRenderer(t)
	nan := math.NaN()
	pt := func(x, y []float64) chart.Plot { return chart.Plot{Series: []chart.Series{{X: x, Y: y}}} }
	cases := []struct {
		name string
		do   func(w io.Writer) error
		want string
	}{
		{"plot past points", func(w io.Writer) error { return small.Plot(w, pt(make([]float64, 5), make([]float64, 5))) }, "points past the capacity"},
		{"plot past series", func(w io.Writer) error {
			return small.Plot(w, chart.Plot{Series: make([]chart.Series, 2), Marks: make([]chart.Mark, 1)})
		}, "series and marks past"},
		{"plot past text", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.Title = strings.Repeat("x", 65)
			return small.Plot(w, p)
		}, "bytes of text past"},
		{"not utf-8", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.Series[0].Name = "\xac"
			return r.Plot(w, p)
		}, "not UTF-8"},
		{"lengths", func(w io.Writer) error { return r.Plot(w, pt([]float64{1, 2}, []float64{1})) }, "X and Y differ"},
		{"band", func(w io.Writer) error {
			return r.Plot(w, chart.Plot{Series: []chart.Series{{X: []float64{1}, Y: []float64{1}, Lo: []float64{1}}}})
		}, "Lo and Hi"},
		{"nan", func(w io.Writer) error { return r.Plot(w, pt([]float64{1, nan}, []float64{1, 2})) }, "not a finite value"},
		{"inf", func(w io.Writer) error { return r.Plot(w, pt([]float64{1, math.Inf(1)}, []float64{1, 2})) }, "not a finite value"},
		{"too large", func(w io.Writer) error { return r.Plot(w, pt([]float64{1, 2e30}, []float64{1, 2})) }, "not a finite value within 1e30"},
		{"mark", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.Marks = []chart.Mark{{X: nan}}
			return r.Plot(w, p)
		}, "mark 0"},
		{"draw", func(w io.Writer) error {
			return r.Plot(w, chart.Plot{Series: []chart.Series{{X: []float64{1}, Y: []float64{1}, Draw: 9}}})
		}, "no such Draw"},
		{"scale", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.X.Scale = 7
			return r.Plot(w, p)
		}, "no such Scale"},
		{"range", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.Y = chart.Axis{Min: 5, Max: 1}
			return r.Plot(w, p)
		}, "does not run up"},
		{"log range", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.Y = chart.Axis{Scale: chart.Log, Min: -1, Max: 1}
			return r.Plot(w, p)
		}, "does not run up"},
		{"range not finite", func(w io.Writer) error {
			p := pt([]float64{1}, []float64{1})
			p.X = chart.Axis{Min: nan, Max: 1}
			return r.Plot(w, p)
		}, "not finite"},
		{"plot empty", func(w io.Writer) error {
			p := pt([]float64{0, -1}, []float64{1, 2})
			p.X.Scale = chart.Log
			return r.Plot(w, p)
		}, "nothing to draw"},
		{"bars series", func(w io.Writer) error { return r.Bars(w, chart.Bars{Labels: []string{"a"}}) }, "one or two"},
		{"bars rows", func(w io.Writer) error {
			return small.Bars(w, chart.Bars{Labels: make([]string, 3), Series: []chart.BarSeries{{Values: make([]float64, 3)}}})
		}, "rows past"},
		{"bars empty", func(w io.Writer) error { return r.Bars(w, chart.Bars{Series: []chart.BarSeries{{}}}) }, "nothing to draw"},
		{"bars order", func(w io.Writer) error {
			return r.Bars(w, chart.Bars{Labels: []string{"a"}, Series: []chart.BarSeries{{Values: []float64{1}}}, Order: 3})
		}, "no such Order"},
		{"bars values", func(w io.Writer) error {
			return r.Bars(w, chart.Bars{Labels: []string{"a"}, Series: []chart.BarSeries{{Values: []float64{1, 2}}}})
		}, "a value for each"},
		{"bars nan", func(w io.Writer) error {
			return r.Bars(w, chart.Bars{Labels: []string{"a"}, Series: []chart.BarSeries{{Values: []float64{nan}}}})
		}, "not a finite value"},
		{"bars text", func(w io.Writer) error {
			return small.Bars(w, chart.Bars{Labels: []string{strings.Repeat("x", 65)}, Series: []chart.BarSeries{{Values: []float64{1}}}})
		}, "bytes of text"},
		{"histogram bins", func(w io.Writer) error {
			return small.Histogram(w, chart.Histogram{Edges: make([]float64, 6), Counts: make([]float64, 5)})
		}, "bins past"},
		{"histogram empty", func(w io.Writer) error { return r.Histogram(w, chart.Histogram{}) }, "nothing to draw"},
		{"histogram edges", func(w io.Writer) error {
			return r.Histogram(w, chart.Histogram{Edges: []float64{0, 1}, Counts: []float64{1, 2}})
		}, "one edge more"},
		{"histogram rise", func(w io.Writer) error {
			return r.Histogram(w, chart.Histogram{Edges: []float64{0, 1, 1}, Counts: []float64{1, 2}})
		}, "edge 2: not above"},
		{"histogram count", func(w io.Writer) error {
			return r.Histogram(w, chart.Histogram{Edges: []float64{0, 1}, Counts: []float64{-1}})
		}, "count 0: below 0"},
		{"histogram nan", func(w io.Writer) error {
			return r.Histogram(w, chart.Histogram{Edges: []float64{0, nan}, Counts: []float64{1}})
		}, "edge 1"},
		{"histogram log", func(w io.Writer) error {
			return r.Histogram(w, chart.Histogram{Edges: []float64{1, 2}, Counts: []float64{1}, X: chart.Axis{Scale: chart.Log}})
		}, "not linear"},
		{"tornado rows", func(w io.Writer) error { return small.Tornado(w, chart.Tornado{Rows: make([]chart.TornadoRow, 3)}) }, "rows past"},
		{"tornado empty", func(w io.Writer) error { return r.Tornado(w, chart.Tornado{}) }, "nothing to draw"},
		{"tornado base", func(w io.Writer) error {
			return r.Tornado(w, chart.Tornado{Base: nan, Rows: make([]chart.TornadoRow, 1)})
		}, "base"},
		{"tornado value", func(w io.Writer) error {
			return r.Tornado(w, chart.Tornado{Rows: []chart.TornadoRow{{}, {High: math.Inf(-1)}}})
		}, "row 1"},
		{"heat rows", func(w io.Writer) error {
			return small.Heat(w, chart.Heat{Rows: make([]string, 3), Cols: []string{"a"}, Cells: make([]float64, 3)})
		}, "rows or columns past"},
		{"heat cells", func(w io.Writer) error {
			return small.Heat(w, chart.Heat{Rows: make([]string, 2), Cols: make([]string, 2), Cells: make([]float64, 5)})
		}, "cells past"},
		{"heat empty", func(w io.Writer) error { return r.Heat(w, chart.Heat{Rows: []string{"a"}}) }, "nothing to draw"},
		{"heat shape", func(w io.Writer) error {
			return r.Heat(w, chart.Heat{Rows: []string{"a"}, Cols: []string{"b"}, Cells: []float64{1, 2}})
		}, "a cell for each"},
		{"heat inf", func(w io.Writer) error {
			return r.Heat(w, chart.Heat{Rows: []string{"a"}, Cols: []string{"b"}, Cells: []float64{math.Inf(1)}})
		}, "cell 0"},
	}
	for _, c := range cases {
		var b bytes.Buffer
		err := c.do(&b)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one holding %q", c.name, err, c.want)
		}
		if b.Len() != 0 {
			t.Errorf("%s: wrote %d bytes before refusing", c.name, b.Len())
		}
	}
	for _, caps := range []chart.Caps{{}, {Points: 1, Series: 1, Rows: 1, Text: chart.MaxCap + 1}} {
		if _, err := chart.New(caps); err == nil {
			t.Errorf("New(%+v) made a renderer", caps)
		}
	}
}

// TestEscaped: every string a chart is given reaches the SVG and the
// table escaped.
func TestEscaped(t *testing.T) {
	r := newRenderer(t)
	bad := `<b x="1">&'`
	p := chart.Plot{Title: bad, Desc: bad, X: chart.Axis{Label: bad, Format: chart.Format{Prefix: bad, Suffix: bad}}, Y: chart.Axis{Label: bad},
		Series: []chart.Series{{Name: bad, X: []float64{1, 2}, Y: []float64{1, 2}}, {Name: bad, X: []float64{1}, Y: []float64{3}}},
		Marks:  []chart.Mark{{X: 1, Y: 1, Label: bad}}, LoLabel: bad}
	var b bytes.Buffer
	if err := r.Plot(&b, p); err != nil {
		t.Fatal(err)
	}
	if err := r.PlotTable(&b, p); err != nil {
		t.Fatal(err)
	}
	tr := chart.Tornado{Title: bad, Header: bad, BaseLabel: bad, LowName: bad, HighName: bad, Rows: []chart.TornadoRow{{Label: bad, Low: -1, High: 1, LowSet: bad, HighSet: bad}}}
	b2 := chart.Bars{Title: bad, Header: bad, Labels: []string{bad, bad}, Series: []chart.BarSeries{{Name: bad, Values: []float64{1, 2}}, {Name: bad, Values: []float64{2, 1}}}, Top: 1, Others: bad}
	h := chart.Heat{Title: bad, Header: bad, Rows: []string{bad}, Cols: []string{bad}, Cells: []float64{1}}
	for _, err := range []error{r.Tornado(&b, tr), r.TornadoTable(&b, tr), r.Bars(&b, b2), r.BarsTable(&b, b2), r.Heat(&b, h), r.HeatTable(&b, h)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(b.String(), bad) || strings.Contains(b.String(), "<b ") {
		t.Errorf("a string reached the output unescaped:\n%s", b.String())
	}
}

// TestOrderAndOthers: bars largest first by the first series, ties as
// given, and the rows past Top summed into Others, of each series; a
// tornado's widest row first.
func TestOrderAndOthers(t *testing.T) {
	r := newRenderer(t)
	b := chart.Bars{Labels: []string{"a", "b", "c", "d", "e"}, Header: "h",
		Series: []chart.BarSeries{{Name: "one", Values: []float64{1, 5, 3, 5, 2}}, {Name: "two", Values: []float64{10, 20, 30, 40, 50}}},
		Order:  chart.Largest, Top: 2, Others: "rest"}
	var w bytes.Buffer
	if err := r.BarsTable(&w, b); err != nil {
		t.Fatal(err)
	}
	got := w.String()
	order := []string{`>b</th>`, `>d</th>`, `>rest</th>`}
	at := 0
	for _, s := range order {
		i := strings.Index(got[at:], s)
		if i < 0 {
			t.Fatalf("%q not after the row before:\n%s", s, got)
		}
		at += i
	}
	if !strings.Contains(got, `<th scope="row">rest</th><td data-value="6">6</td><td data-value="90">90</td>`) {
		t.Errorf("others not the sum of the rest:\n%s", got)
	}
	w.Reset()
	if err := r.TornadoTable(&w, tornado()); err != nil {
		t.Fatal(err)
	}
	got = w.String()
	if a, d, s := strings.Index(got, "attack rate"), strings.Index(got, "detection"), strings.Index(got, ">staff<"); a < 0 || a > d || d > s {
		t.Errorf("tornado rows not widest first:\n%s", got)
	}
}

// TestFormats: each style writes as the design says.
func TestFormats(t *testing.T) {
	r := newRenderer(t)
	cases := []struct {
		v    float64
		f    chart.Format
		want string
	}{
		{12.345, chart.Format{Digits: 2}, "12.35"},
		{12, chart.Format{}, "12"},
		{-0.2, chart.Format{}, "0"},
		{0.123, chart.Format{Style: chart.Percent}, "12%"},
		{0.0016, chart.Format{Style: chart.Percent, Digits: 1}, "0.2%"},
		{0.999, chart.Format{Style: chart.Percent, Digits: 2}, "100%"},
		{0.5, chart.Format{Style: chart.Percent, Digits: 2}, "50%"},
		{1234, chart.Format{Style: chart.Compact}, "1.2k"},
		{999.7, chart.Format{Style: chart.Compact}, "1.0k"},
		{999999, chart.Format{Style: chart.Compact, Digits: 3}, "1.00M"},
		{-45600, chart.Format{Style: chart.Compact, Digits: 3, Prefix: "€"}, "€-45.6k"},
		{7, chart.Format{Style: chart.Compact}, "7.0"},
		{2.5e12, chart.Format{Style: chart.Compact}, "2.5T"},
		{0, chart.Format{Style: chart.Compact}, "0.0"},
		{3, chart.Format{Suffix: " days"}, "3 days"},
	}
	for _, c := range cases {
		var b bytes.Buffer
		h := chart.Heat{Rows: []string{"r"}, Cols: []string{"c"}, Cells: []float64{c.v}, Format: c.f}
		if err := r.HeatTable(&b, h); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), ">"+c.want+"</td>") {
			t.Errorf("%v in %+v: %s, want %s", c.v, c.f, b.String(), c.want)
		}
	}
}

// TestSameAgain: a chart drawn twice by one renderer, and by another,
// is the same.
func TestSameAgain(t *testing.T) {
	a, b := newRenderer(t), newRenderer(t)
	for _, name := range examples {
		var x, y, z bytes.Buffer
		for _, w := range []*bytes.Buffer{&x, &y} {
			if err := draw(t, a, name, false, w); err != nil {
				t.Fatal(err)
			}
		}
		if err := draw(t, b, name, false, &z); err != nil {
			t.Fatal(err)
		}
		if x.String() != y.String() || x.String() != z.String() {
			t.Errorf("%s differs when drawn again", name)
		}
	}
}

// TestStylesheet: the stylesheet styles the classes the charts carry.
func TestStylesheet(t *testing.T) {
	for _, c := range []string{".gw-chart", ".s1", ".s2", ".band", ".line", ".ramp-6", ".on-dark", ".low", ".high", ".hit", ".gw-chart-table"} {
		if !strings.Contains(chart.Stylesheet, c) {
			t.Errorf("the stylesheet has no %s", c)
		}
	}
}
