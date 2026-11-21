package svg

import (
	_ "embed"
	"io"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/internal/xmltext"
)

// Stylesheet is the default look of a drawing, for a host to serve as
// a file or to embed through Options.Style.
//
//go:embed graphedwright.css
var Stylesheet string

// New makes a renderer with its buffer, the one allocation a renderer
// ever makes.
func New() *Renderer { return &Renderer{o: newOut()} }

// newOut makes an output buffer.
func newOut() *out { return xmltext.New() }

// out is the buffered writer every element is written through.
type out = xmltext.Writer

// Render writes a document as SVG text to w with a fresh renderer.
func Render(d *gw.Document, opts Options, w io.Writer) error {
	return New().Render(d, opts, w)
}
