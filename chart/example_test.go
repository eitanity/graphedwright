package chart_test

import (
	"fmt"
	"os"
	"strings"

	"code.eitanity.com/graphedwright/chart"
)

// An exceedance curve on log scales with its band and a marked level,
// drawn and written as a table.
func Example() {
	r, err := chart.New(chart.DefaultCaps())
	if err != nil {
		panic(err)
	}
	plot := chart.Plot{
		Title: "The chance a period's total reaches each value",
		X:     chart.Axis{Label: "total", Scale: chart.Log, Format: chart.Format{Style: chart.Compact, Prefix: "€"}},
		Y:     chart.Axis{Label: "chance", Scale: chart.Log, Format: chart.Format{Style: chart.Percent, Digits: 1}},
		Series: []chart.Series{{Name: "total",
			X:  []float64{1000, 10000, 100000},
			Y:  []float64{0.6, 0.1, 0.01},
			Lo: []float64{0.55, 0.08, 0.006},
			Hi: []float64{0.65, 0.12, 0.014}}},
		Marks: []chart.Mark{{X: 10000, Y: 0.1, Label: "1 in 10"}},
	}
	var svg, table strings.Builder
	if err := r.Plot(&svg, plot); err != nil {
		panic(err)
	}
	if err := r.PlotTable(&table, plot); err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stdout, strings.HasPrefix(svg.String(), "<svg"), strings.Count(table.String(), "<tr>"))
	// Output: true 4
}
