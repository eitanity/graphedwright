# Charts

The `chart` package draws charts beside your graphs: a plot of series
on two axes, bars, a histogram, a tornado and a heat table. You give it
plain values; it writes SVG text, the same byte for byte on every
platform and toolchain, and the same values as an HTML table you can
show beside the chart or instead of it. It knows nothing about what
the values mean: an error band, a level to mark or a sort order is
something you give it.

```go
import "code.eitanity.com/graphedwright/chart"

r, err := chart.New(chart.DefaultCaps()) // make once; drawing allocates nothing
err = r.Plot(w, chart.Plot{
	Title: "The chance a period's total reaches each value",
	X: chart.Axis{Label: "total", Scale: chart.Log, Format: chart.Format{Style: chart.Compact, Prefix: "€"}},
	Y: chart.Axis{Label: "chance", Scale: chart.Log, Format: chart.Format{Style: chart.Percent, Digits: 1}},
	Series: []chart.Series{{Name: "total", X: xs, Y: ps, Lo: lo, Hi: hi}},
	Marks:  []chart.Mark{{X: x10, Y: 0.1, Label: "1 in 10"}},
})
err = r.PlotTable(w, plot) // the same values as a <table>
```

[`chart/example_test.go`](../chart/example_test.go) holds a runnable
version.

## The kinds

| Draw | Table | What it shows |
|---|---|---|
| `Plot` | `PlotTable` | series of points on two axes, linear or log, each a line, dots or both, with an optional band and labelled marks |
| `Bars` | `BarsTable` | a value per label in one or two series, as given or largest first, with an optional "others" row |
| `Histogram` | `HistogramTable` | counts in bins between rising edges |
| `Tornado` | `TornadoTable` | each input's outcome at its low and its high setting, against a base, widest at the top |
| `Heat` | `HeatTable` | a cell per row and column, shaded from 0 to the largest |

A plot covers several familiar charts:

- **An exceedance curve:** both axes `Log`, the chance against the
  value, the band your ±2 standard errors (`Lo`, `Hi`), the marks your
  levels.
- **A scatter or line of two figures:** linear axes, `Draw: chart.Points`
  or `chart.Line`.
- **A fan over periods:** the periods across, a `Line` with its band
  for the mean, and a `Points` series for exact points.

On a log axis a value of 0 or less is left out, with its point.

## Formats

A `Format` says how values are written, in the ticks, the labels, the
hover titles and the table:

| Style | `Digits` | 0.1234 | 45 600 |
|---|---|---|---|
| `Number` | decimals | `0` (0), `0.12` (2) | `45600` (0) |
| `Percent` | significant digits (2 if 0) | `12%` | — |
| `Compact` | significant digits (2 if 0) | `0.12` | `46k`, `45.6k` (3) |

`Prefix` and `Suffix` go either side: `€`, ` days`, ` pp`. There is no
format function: a chart is plain values, so it can be saved, compared
and replayed.

## Styling

A chart writes classes, never colours. `chart.Stylesheet` is a default
for the page (or a file to copy): two series colours that pass the
colour-vision checks, a one-hue ramp for heat cells, and a table style.

| Class | On |
|---|---|
| `gw-chart chart-plot` (`-bars`, `-histogram`, `-tornado`, `-heat`) | the `<svg>` |
| `series s1`, `s2`, … | a plot's series, in the order given; bars of a series carry `s1` or `s2` too |
| `line`, `band`, `point` | a series's line, band and dots |
| `mark`, `mark-label` | a mark and its label |
| `grid`, `axis`, `tick`, `axis-label` | the frame |
| `low`, `high`, `base` | a tornado's two ends and its base line |
| `ramp-0` … `ramp-6`, `on-light`, `on-dark` | a heat cell's shade and its text |
| `hit` | a strip over each point of a line drawn alone, for hover |
| `gw-chart-table chart-…` | the `<table>` |

Every mark has a `<title>` the browser shows on hover. A text cut to fit
has its whole text as its title. A chart has no `id`s, so a page can
hold any number.

## Limits and refusals

`chart.New(caps)` makes everything up front: `Points` (a plot's points
together, a histogram's bins, a heat table's cells), `Series` (a plot's
series and marks together), `Rows` (bars, tornado rows, heat rows and
columns each) and `Text` (the bytes of every string). A chart past one
is refused, with an error naming it, before anything is written.

A chart is also refused for:

- a value that is not finite (`NaN`, `±Inf`), except a heat cell's
  `NaN`, which is an empty cell;
- a value past 1e30 in size;
- lengths that do not match, and histogram edges that do not rise;
- text that is not UTF-8;
- nothing to draw (`chart.ErrEmpty`).

The browser's JavaScript object has no charts; draw them from Go.
