package chart

import (
	"io"
	"strconv"
)

// Draw is how a plot's series is drawn.
type Draw uint8

// The ways a series is drawn.
const (
	Line       Draw = iota // a line through its points, hit by a strip at each
	Points                 // a dot at each point
	LinePoints             // a line, and a dot at each point
)

// Series is one of a plot's series: a point for each X and Y, and when
// Lo and Hi are given, as long, a band from Lo to Hi at each X drawn
// under it.
type Series struct {
	Name string
	X, Y []float64
	Lo   []float64
	Hi   []float64
	Draw Draw
}

// Mark is a point labelled on a plot: a level, a threshold.
type Mark struct {
	X, Y  float64
	Label string
}

// Plot is series of points on two axes, with marks. It draws an
// exceedance curve (both axes Log, the band the host's error), a
// scatter (Points) or a line of two figures, and a fan over periods (a
// Line with its band, and Points for the exact points). LoLabel and
// HiLabel head the band's columns in its table.
type Plot struct {
	Title, Desc      string
	X, Y             Axis
	Series           []Series
	Marks            []Mark
	LoLabel, HiLabel string
}

// The plot's frame: its height and the room kept around the area the
// points are drawn in.
const (
	plotHeight = 260
	plotTop    = 12
	plotBottom = 36
	plotRight  = 16
	legendRoom = 20
	tickDrop   = 17 // an x tick's baseline below the area, clear of the least y tick's text
	pointR     = 4
)

// plotFrame is where a plot is drawn: its scales and the area's edges.
type plotFrame struct {
	x, y        scale
	top, bottom float64 // the area's top and bottom
}

// Plot draws plot p to w.
func (r *Renderer) Plot(w io.Writer, p Plot) error {
	f, err := r.plotFrame(&p)
	if err != nil {
		return err
	}
	r.begin(w, "plot", plotHeight, p.Title, p.Desc)
	r.axes(&p.X, &p.Y, &f)
	for k := range p.Series {
		r.plotSeries(&p, &f, k)
	}
	r.plotMarks(&p, &f)
	if len(p.Series) > 1 {
		r.o.Str(`<g class="legend">`)
		x := f.x.a
		for k := range p.Series {
			x = r.swatch(x, 14, k, "", p.Series[k].Name)
		}
		r.o.Str("</g>")
	}
	r.plotHits(&p, &f)
	return r.end()
}

// checkPlot refuses a plot past the capacities, of lengths that do not
// match or of values that are not finite.
func (r *Renderer) checkPlot(p *Plot) error {
	if n := len(p.Series) + len(p.Marks); n > r.caps.Series {
		return capErr("series and marks", n, r.caps.Series)
	}
	var t text
	t.add(p.Title)
	t.add(p.Desc)
	t.add(p.X.Label)
	t.add(p.Y.Label)
	t.add(p.LoLabel)
	t.add(p.HiLabel)
	t.add(p.X.Format.Prefix)
	t.add(p.X.Format.Suffix)
	t.add(p.Y.Format.Prefix)
	t.add(p.Y.Format.Suffix)
	points := 0
	for k := range p.Series {
		s := &p.Series[k]
		if err := checkSeries(k, s); err != nil {
			return err
		}
		points += len(s.X)
		t.add(s.Name)
	}
	if points > r.caps.Points {
		return capErr("points", points, r.caps.Points)
	}
	for i := range p.Marks {
		m := &p.Marks[i]
		if checkValue("mark", i, m.X) != nil || checkValue("mark", i, m.Y) != nil {
			return refuse("mark", i, "not a finite value within 1e30")
		}
		t.add(m.Label)
	}
	if err := checkAxis("x axis", &p.X); err != nil {
		return err
	}
	if err := checkAxis("y axis", &p.Y); err != nil {
		return err
	}
	return r.checkText(t)
}

// checkSeries refuses series k with lengths that do not match or
// values that are not finite.
func checkSeries(k int, s *Series) error {
	if len(s.Y) != len(s.X) {
		return refuse("series", k, "X and Y differ in length")
	}
	if (s.Lo == nil) != (s.Hi == nil) || s.Lo != nil && (len(s.Lo) != len(s.X) || len(s.Hi) != len(s.X)) {
		return refuse("series", k, "Lo and Hi are not both given as long as X")
	}
	if s.Draw > LinePoints {
		return refuse("series", k, "no such Draw")
	}
	for _, vs := range [...][]float64{s.X, s.Y, s.Lo, s.Hi} {
		for i, v := range vs {
			if checkValue("series", k, v) != nil {
				return refuse("series", k, "point "+strconv.Itoa(i)+" is not a finite value within 1e30")
			}
		}
	}
	return nil
}

// checkAxis refuses an axis of no such scale, or a range that is not
// finite, runs backwards, or on a log scale is not above 0.
func checkAxis(what string, a *Axis) error {
	if a.Scale > Log {
		return refuse(what, -1, "no such Scale")
	}
	if checkValue(what, -1, a.Min) != nil || checkValue(what, -1, a.Max) != nil {
		return refuse(what, -1, "its range is not finite within 1e30")
	}
	if fixed(a) && (a.Max <= a.Min || a.Scale == Log && a.Min <= 0) {
		return refuse(what, -1, "its range does not run up from Min to Max, above 0 on a log scale")
	}
	return nil
}

// fixed reports whether the host fixed the axis's range.
func fixed(a *Axis) bool { return a.Min != 0 || a.Max != 0 }

// kept reports whether point i of series s is drawn: on a log axis, a
// value of 0 or less is not.
func kept(p *Plot, s *Series, i int) bool {
	return (p.X.Scale != Log || s.X[i] > 0) && (p.Y.Scale != Log || s.Y[i] > 0)
}

// valueRange is the least and the greatest of the values drawn on an
// axis, and whether there are any.
type valueRange struct {
	lo, hi float64
	any    bool
}

func (v *valueRange) add(x float64, log bool) {
	if log && x <= 0 {
		return
	}
	if !v.any {
		v.lo, v.hi, v.any = x, x, true
		return
	}
	v.lo, v.hi = min(v.lo, x), max(v.hi, x)
}

// plotFrame checks p and finds its scales.
func (r *Renderer) plotFrame(p *Plot) (plotFrame, error) {
	if err := r.checkPlot(p); err != nil {
		return plotFrame{}, err
	}
	var xs, ys valueRange
	xlog, ylog := p.X.Scale == Log, p.Y.Scale == Log
	for k := range p.Series {
		s := &p.Series[k]
		for i := range s.X {
			if !kept(p, s, i) {
				continue
			}
			xs.add(s.X[i], xlog)
			ys.add(s.Y[i], ylog)
			if s.Lo != nil {
				ys.add(s.Lo[i], ylog)
				ys.add(s.Hi[i], ylog)
			}
		}
	}
	for _, m := range p.Marks {
		if (!xlog || m.X > 0) && (!ylog || m.Y > 0) {
			xs.add(m.X, xlog)
			ys.add(m.Y, ylog)
		}
	}
	if !xs.any {
		return plotFrame{}, ErrEmpty
	}
	return r.frame(&p.X, &p.Y, xs, ys, len(p.Series) > 1), nil
}

// frame is the frame of a chart on axes x and y over the values given,
// with room for a legend above.
func (r *Renderer) frame(xa, ya *Axis, xs, ys valueRange, legend bool) plotFrame {
	f := plotFrame{x: axisScale(xa, xs), y: axisScale(ya, ys), top: plotTop, bottom: plotHeight - plotBottom}
	if legend {
		f.top += legendRoom
	}
	left := float64(width(r.tickChars(ya, &f.y)) + 10)
	if ya.Label != "" {
		left += 16
	}
	// The last tick's label is centred on the right edge, so as much
	// room is kept past it as half its width.
	last := float64(width(r.valueWidth(f.x.tick(f.x.ticks()-1), xa.Format))/2) + 2
	f.x.a, f.x.b = left, Width-max(plotRight, last)
	f.y.a, f.y.b = f.bottom, f.top
	return f
}

// axisScale is the scale of axis a over the values drawn on it.
func axisScale(a *Axis, v valueRange) scale {
	lo, hi := v.lo, v.hi
	if fixed(a) {
		lo, hi = a.Min, a.Max
	}
	if a.Scale == Log {
		return logScale(lo, hi)
	}
	return linearScale(lo, hi, fixed(a))
}

// tickChars is the widest of an axis's tick labels, in characters.
func (r *Renderer) tickChars(a *Axis, s *scale) int {
	n := 0
	for k := range s.ticks() {
		n = max(n, r.valueWidth(s.tick(k), a.Format))
	}
	return n
}

// axes writes the grid, the axes, their ticks and their labels.
func (r *Renderer) axes(xa, ya *Axis, f *plotFrame) {
	o := r.o
	o.Str(`<g class="grid">`)
	for k := range f.x.ticks() {
		x := f.x.at(f.x.tick(k))
		r.line("grid x", x, f.top, x, f.bottom)
	}
	for k := range f.y.ticks() {
		y := f.y.at(f.y.tick(k))
		r.line("grid y", f.x.a, y, f.x.b, y)
	}
	o.Str(`</g><g class="axes">`)
	r.line("axis x", f.x.a, f.bottom, f.x.b, f.bottom)
	r.line("axis y", f.x.a, f.top, f.x.a, f.bottom)
	for k := range f.x.ticks() {
		r.textOpen(f.x.at(f.x.tick(k)), f.bottom+tickDrop, "middle", "tick x")
		r.value(f.x.tick(k), xa.Format)
		r.textClose()
	}
	for k := range f.y.ticks() {
		r.textOpen(f.x.a-6, f.y.at(f.y.tick(k))+4, "end", "tick y")
		r.value(f.y.tick(k), ya.Format)
		r.textClose()
	}
	if xa.Label != "" {
		r.label((f.x.a+f.x.b)/2, plotHeight-4, "middle", "axis-label x", xa.Label, chars(f.x.span()))
	}
	if ya.Label != "" {
		cy := (f.top + f.bottom) / 2
		o.Str(`<g transform="rotate(-90 12 `)
		o.Num(cy)
		o.Str(`)">`)
		r.label(12, cy, "middle", "axis-label y", ya.Label, chars(f.bottom-f.top))
		o.Str("</g>")
	}
	o.Str("</g>")
}

// plotSeries writes series k: its band, its line and its points.
func (r *Renderer) plotSeries(p *Plot, f *plotFrame, k int) {
	o := r.o
	s := &p.Series[k]
	o.Str(`<g class="series `)
	r.seriesClass(k)
	o.Str(`" data-name="`)
	o.Attr(s.Name)
	o.Str(`">`)
	if s.Lo != nil {
		r.band(p, f, s)
	}
	if s.Draw != Points {
		r.path(p, f, s)
	}
	if s.Draw != Line {
		for i := range s.X {
			if kept(p, s, i) {
				r.point(p, f, s, i, "point")
			}
		}
	}
	o.Str("</g>")
}

// band writes series s's band: along Hi and back along Lo.
func (r *Renderer) band(p *Plot, f *plotFrame, s *Series) {
	o := r.o
	o.Str(`<path class="band" d="`)
	cmd := byte('M')
	for i := range s.X {
		if kept(p, s, i) {
			o.Byte(cmd)
			o.Point(f.x.at(s.X[i]), f.y.at(s.Hi[i]))
			cmd = 'L'
		}
	}
	// bound: the series's points, back
	for i := len(s.X) - 1; i >= 0; i-- {
		if kept(p, s, i) {
			o.Byte('L')
			o.Point(f.x.at(s.X[i]), f.y.at(s.Lo[i]))
		}
	}
	o.Str(`Z"/>`)
}

// path writes series s's line through its points kept.
func (r *Renderer) path(p *Plot, f *plotFrame, s *Series) {
	o := r.o
	o.Str(`<path class="line" d="`)
	cmd := byte('M')
	for i := range s.X {
		if kept(p, s, i) {
			o.Byte(cmd)
			o.Point(f.x.at(s.X[i]), f.y.at(s.Y[i]))
			cmd = 'L'
		}
	}
	o.Str(`"/>`)
}

// point writes point i of series s as a dot of the class given, with
// its title.
func (r *Renderer) point(p *Plot, f *plotFrame, s *Series, i int, class string) {
	o := r.o
	o.Str(`<circle class="`)
	o.Str(class)
	o.Str(`" data-i="`)
	o.Int(i)
	o.Byte('"')
	o.NumAttr("cx", f.x.at(s.X[i]))
	o.NumAttr("cy", f.y.at(s.Y[i]))
	o.Str(` r="`)
	o.Int(pointR)
	o.Str(`">`)
	r.titleOpen()
	r.pointTitle(p, s, i)
	r.titleClose()
	o.Str("</circle>")
}

// pointTitle writes point i's title's text: its series, its values and
// its band.
func (r *Renderer) pointTitle(p *Plot, s *Series, i int) {
	o := r.o
	if s.Name != "" {
		o.Text(s.Name)
		o.Str(": ")
	}
	r.value(s.X[i], p.X.Format)
	o.Str(", ")
	r.value(s.Y[i], p.Y.Format)
	if s.Lo != nil {
		o.Str(" (")
		r.value(s.Lo[i], p.Y.Format)
		o.Str(" – ")
		r.value(s.Hi[i], p.Y.Format)
		o.Byte(')')
	}
}

// plotMarks writes the marks: a dot, its label beside it and its title.
func (r *Renderer) plotMarks(p *Plot, f *plotFrame) {
	if len(p.Marks) == 0 {
		return
	}
	o := r.o
	o.Str(`<g class="marks">`)
	for i := range p.Marks {
		m := &p.Marks[i]
		if p.X.Scale == Log && m.X <= 0 || p.Y.Scale == Log && m.Y <= 0 {
			continue
		}
		cx, cy := f.x.at(m.X), f.y.at(m.Y)
		o.Str(`<circle class="mark"`)
		o.NumAttr("cx", cx)
		o.NumAttr("cy", cy)
		o.Str(` r="4">`)
		r.titleOpen()
		o.Text(m.Label)
		o.Str(": ")
		r.value(m.X, p.X.Format)
		o.Str(", ")
		r.value(m.Y, p.Y.Format)
		r.titleClose()
		o.Str("</circle>")
		r.label(cx-8, cy+14, "end", "mark-label", m.Label, 24)
	}
	o.Str("</g>")
}

// plotHits writes a strip for each point of a series drawn as a line
// alone, which has no dots to hover, across the plot's height.
func (r *Renderer) plotHits(p *Plot, f *plotFrame) {
	o := r.o
	o.Str(`<g class="hits">`)
	for k := range p.Series {
		s := &p.Series[k]
		if s.Draw != Line {
			continue
		}
		for i := range s.X {
			if !kept(p, s, i) {
				continue
			}
			o.Str(`<rect class="hit `)
			r.seriesClass(k)
			o.Str(`" data-i="`)
			o.Int(i)
			r.rectRest(f.x.at(s.X[i])-4, f.top, 8, f.bottom-f.top, true)
			r.pointTitle(p, s, i)
			r.rectClose()
		}
	}
	o.Str("</g>")
}
