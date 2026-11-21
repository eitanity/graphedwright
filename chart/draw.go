package chart

import "unicode/utf8"

// line writes a line of the class given.
func (r *Renderer) line(class string, x1, y1, x2, y2 float64) {
	o := r.o
	o.Str(`<line class="`)
	o.Str(class)
	o.Byte('"')
	o.NumAttr("x1", x1)
	o.NumAttr("y1", y1)
	o.NumAttr("x2", x2)
	o.NumAttr("y2", y2)
	o.Str("/>")
}

// textOpen opens a text element at x, y, anchored as given, of the
// class given; its content and textClose follow.
func (r *Renderer) textOpen(x, y float64, anchor, class string) {
	o := r.o
	o.Str(`<text class="`)
	o.Str(class)
	o.Byte('"')
	o.NumAttr("x", x)
	o.NumAttr("y", y)
	if anchor != "start" {
		o.Str(` text-anchor="`)
		o.Str(anchor)
		o.Byte('"')
	}
	o.Byte('>')
}

func (r *Renderer) textClose() { r.o.Str("</text>") }

// label writes s at x, y as textOpen does, cut to n characters, with its
// whole text as its title when cut.
func (r *Renderer) label(x, y float64, anchor, class, s string, n int) {
	r.textOpen(x, y, anchor, class)
	if r.cut(s, n) {
		r.o.Str("<title>")
		r.o.Text(s)
		r.o.Str("</title>")
	}
	r.textClose()
}

// cut writes s escaped, cut to n characters with an ellipsis when it is
// longer, and reports whether it was cut.
func (r *Renderer) cut(s string, n int) bool {
	if utf8.RuneCountInString(s) <= n || n < 2 {
		r.o.Text(s)
		return false
	}
	i, k := 0, 0
	// bound: the first n-1 runes of s
	for k < n-1 {
		_, w := utf8.DecodeRuneInString(s[i:])
		i += w
		k++
	}
	r.o.Text(s[:i])
	r.o.Str("…")
	return true
}

// chars is how many characters fit in width w.
func chars(w float64) int { return int(w / charWidth) }

// width is the width of n characters.
func width(n int) float64 { return float64(float64(n) * charWidth) }

// runes is the number of characters in s.
func runes(s string) int { return utf8.RuneCountInString(s) }

// titleOpen and titleClose wrap a mark's title, the text the browser
// shows on hover.
func (r *Renderer) titleOpen()  { r.o.Str("<title>") }
func (r *Renderer) titleClose() { r.o.Str("</title>") }

// seriesClass writes the class of series k, from s1.
func (r *Renderer) seriesClass(k int) {
	r.o.Byte('s')
	r.o.Int(k + 1)
}

// rectRest ends a rectangle whose class is written but not closed.
func (r *Renderer) rectRest(x, y, w, h float64, titled bool) {
	o := r.o
	o.Byte('"')
	o.NumAttr("x", x)
	o.NumAttr("y", y)
	o.NumAttr("width", max(w, 0))
	o.NumAttr("height", max(h, 0))
	if titled {
		o.Byte('>')
		r.titleOpen()
		return
	}
	o.Str("/>")
}

// rectClose closes a titled rectangle after its title.
func (r *Renderer) rectClose() {
	r.titleClose()
	r.o.Str("</rect>")
}

// swatch writes a legend's entry at x, y: a square of series k's class,
// or of class when k is below 0, and the name after it, and returns
// where the next goes.
func (r *Renderer) swatch(x, y float64, k int, class, name string) float64 {
	r.o.Str(`<rect class="swatch `)
	if k >= 0 {
		r.seriesClass(k)
	} else {
		r.o.Str(class)
	}
	r.rectRest(x, y-9, 10, 10, false)
	r.label(x+14, y, "start", "legend-label", name, 24)
	return x + 14 + width(min(runes(name), 24)) + 16
}
