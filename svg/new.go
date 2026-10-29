package svg

import (
	_ "embed"
	"io"

	gw "code.eitanity.com/graphedwright"
)

// Stylesheet is the default look of a drawing, for a host to serve as
// a file or to embed through Options.Style.
//
//go:embed graphedwright.css
var Stylesheet string

// New makes a renderer with its buffer, the one allocation a renderer
// ever makes.
func New() *Renderer { return &Renderer{o: newOut()} }

// Render writes a document as SVG text to w with a fresh renderer.
func Render(d *gw.Document, opts Options, w io.Writer) error {
	return New().Render(d, opts, w)
}
