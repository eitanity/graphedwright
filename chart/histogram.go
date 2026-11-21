package chart

import "io"

// Histogram is a count for each bin between edges that rise: Counts[i]
// for the bin from Edges[i] to Edges[i+1], drawn up from 0 side by
// side. X and Y are its axes, both linear; their labels head its
// table's columns.
type Histogram struct {
	Title, Desc string
	X, Y        Axis
	Edges       []float64
	Counts      []float64
}

// Histogram draws histogram h to w.
func (r *Renderer) Histogram(w io.Writer, h Histogram) error {
	f, err := r.histogramFrame(&h)
	if err != nil {
		return err
	}
	r.begin(w, "histogram", plotHeight, h.Title, h.Desc)
	r.axes(&h.X, &h.Y, &f)
	o := r.o
	o.Str(`<g class="series s1">`)
	for i, c := range h.Counts {
		x0, x1 := f.x.at(h.Edges[i]), f.x.at(h.Edges[i+1])
		y := f.y.at(c)
		o.Str(`<rect class="bar" data-i="`)
		o.Int(i)
		r.rectRest(x0+0.5, y, x1-x0-1, f.y.at(0)-y, true)
		r.binTitle(&h, i)
		r.rectClose()
	}
	o.Str("</g>")
	return r.end()
}

// binTitle writes bin i's title's text: its edges and its count.
func (r *Renderer) binTitle(h *Histogram, i int) {
	r.value(h.Edges[i], h.X.Format)
	r.o.Str(" – ")
	r.value(h.Edges[i+1], h.X.Format)
	r.o.Str(": ")
	r.value(h.Counts[i], h.Y.Format)
}

// checkHistogram refuses a histogram past the capacities, with edges
// that do not rise or are not one more than the counts, counts below 0,
// values that are not finite, or a log axis.
func (r *Renderer) checkHistogram(h *Histogram) error {
	if len(h.Counts) > r.caps.Points {
		return capErr("bins", len(h.Counts), r.caps.Points)
	}
	if len(h.Counts) == 0 {
		return ErrEmpty
	}
	if len(h.Edges) != len(h.Counts)+1 {
		return refuse("histogram", -1, "not one edge more than the counts")
	}
	if err := checkValues("edge", h.Edges); err != nil {
		return err
	}
	if err := checkValues("count", h.Counts); err != nil {
		return err
	}
	for i, c := range h.Counts {
		if c < 0 {
			return refuse("count", i, "below 0")
		}
		if h.Edges[i+1] <= h.Edges[i] {
			return refuse("edge", i+1, "not above the one before")
		}
	}
	if h.X.Scale != Linear || h.Y.Scale != Linear {
		return refuse("histogram", -1, "an axis not linear")
	}
	if err := checkAxis("x axis", &h.X); err != nil {
		return err
	}
	if err := checkAxis("y axis", &h.Y); err != nil {
		return err
	}
	var t text
	t.add(h.Title)
	t.add(h.Desc)
	t.add(h.X.Label)
	t.add(h.Y.Label)
	t.add(h.X.Format.Prefix)
	t.add(h.X.Format.Suffix)
	t.add(h.Y.Format.Prefix)
	t.add(h.Y.Format.Suffix)
	return r.checkText(t)
}

// histogramFrame checks h and finds its frame: the bins' edges across,
// from 0 to the largest count up.
func (r *Renderer) histogramFrame(h *Histogram) (plotFrame, error) {
	if err := r.checkHistogram(h); err != nil {
		return plotFrame{}, err
	}
	xs := valueRange{lo: h.Edges[0], hi: h.Edges[len(h.Edges)-1], any: true}
	ys := valueRange{any: true}
	for _, c := range h.Counts {
		ys.hi = max(ys.hi, c)
	}
	f := r.frame(&h.X, &h.Y, xs, ys, false)
	if !fixed(&h.X) {
		// The bins run edge to edge, the ticks within them.
		a, b := f.x.a, f.x.b
		f.x = linearScale(xs.lo, xs.hi, true)
		f.x.a, f.x.b = a, b
	}
	return f, nil
}
