package svg_test

import (
	"bytes"
	"encoding/xml"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestGolden: every example document renders to the SVG frozen under
// testdata/golden, byte for byte; -update rewrites the files.
func TestGolden(t *testing.T) {
	for _, ex := range []struct {
		name  string
		build func(testing.TB) (*gw.Document, svg.Options)
	}{{"diamond", diamond}, {"shapes", shapes}, {"notation", notation}, {"bends", bends}, {"tree", tree}} {
		d, opts := ex.build(t)
		var got bytes.Buffer
		if err := svg.Render(d, opts, &got); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden", ex.name+".svg")
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path) //nolint:gosec // repository file
		if err != nil {
			t.Fatalf("%s: no golden file; run with -update: %v", ex.name, err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("%s: render differs from the golden file; if the change is intended, regenerate with -update\n%s", ex.name, firstDiff(got.Bytes(), want))
		}
	}
}

func firstDiff(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo, hi := max(0, i-40), i+60
	return "at byte " + strconv.Itoa(i) + ":\n  got:  " + string(a[lo:min(hi, len(a))]) + "\n  want: " + string(b[lo:min(hi, len(b))])
}

// TestWellFormed: hostile labels, attributes and kind names render to
// well-formed XML with every element where the structure says.
func TestWellFormed(t *testing.T) {
	for _, build := range []func(testing.TB) (*gw.Document, svg.Options){diamond, shapes} {
		d, opts := build(t)
		opts.Title = `<title & "quotes">`
		var buf bytes.Buffer
		if err := svg.Render(d, opts, &buf); err != nil {
			t.Fatal(err)
		}
		checkXML(t, buf.Bytes(), d)
	}
	// Control characters written into the document past its checks are
	// dropped, in a label cut short and in one drawn whole.
	d, opts := shapes(t)
	d.Vertices[0].Label = "a\x01b\x1f\x00c" + strings.Repeat("\x02x", 40)
	d.Vertices[1].Label = "\x0bsh\uffffort\ufffe"
	d.Edges[0].Label = "e\x1bdge"
	d.Vertices[0].Attrs = append(d.Vertices[0].Attrs[:0:0], gw.Attr{Key: "k", Value: "v\x07\t\uffffw"})
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	checkXML(t, buf.Bytes(), d)
	if !strings.Contains(buf.String(), `data-k="v w"`) || !strings.Contains(buf.String(), ">short</text>") {
		t.Errorf("control characters not dropped:\n%s", buf.String())
	}
}

// checkXML parses the SVG back and counts the elements against the
// document.
func checkXML(t *testing.T, text []byte, d *gw.Document) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(text))
	groups := map[string]int{}
	var stack []string
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("not well-formed: %v\n%s", err, text)
		}
		switch x := tok.(type) {
		case xml.StartElement:
			stack = append(stack, x.Name.Local)
			for _, a := range x.Attr {
				if a.Name.Local == "class" && x.Name.Local == "g" {
					groups[strings.Fields(a.Value)[0]]++
				}
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 || groups["vertex"] != d.LiveVertices() || groups["edge"] != d.LiveEdges() || groups["overlay"] != 1 {
		t.Errorf("groups %v for %d vertices and %d edges", groups, d.LiveVertices(), d.LiveEdges())
	}
}

// TestStructure: the pieces the structure promises are in the text:
// edges before vertices, ids and data attributes, the arrow marker per
// kind, the cut label with its title, the unplaced class, the selection
// in edit mode and nothing in the overlay in view mode.
func TestStructure(t *testing.T) {
	d, opts := shapes(t)
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{
		`viewBox="`, `data-mode="edit"`, `<marker id="arrow-0" class="arrowhead to"`, `<marker id="arrow-1" class="arrowhead outcome"`,
		`<g class="edge wire" id="e2" data-kind="wire" data-from="a label far too long to fit in the box it is drawn in" data-to="decide?">`,
		`<path d="M `, ` H `, ` C `, `marker-end="url(#arrow-1)"`, `wire &lt;&amp;&gt;`,
		`<g class="vertex node" id="a label far too long to fit in the box it is drawn in" data-kind="node" transform="translate(240 60)" data-p="0.5" data-mode="x=&quot;1&quot; &lt;b&gt;&amp;" data-weird_key_="v">`,
		`<rect class="shape" x="-60" y="-18" width="120" height="36" rx="6"/>`, `…</text><title>a label far too long`,
		`<circle class="port" data-port="in" cx="-60" cy="0" r="4"/>`, `<circle class="port" data-port="out" cx="60" cy="-6" r="4"/>`,
		`<polygon class="shape" points="0 -24 40 0 0 24 -40 0"/>`, `<ellipse class="shape" cx="0" cy="0" rx="20" ry="20"/>`,
		`class="vertex round_thing"`, `class="vertex leaf unplaced"`, `<g class="overlay"><rect class="selection"`, `<path d="M 108 60 L 171 60" stroke-width="1.5" marker-end="url(#arrow-0)"/>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	if strings.Index(s, `<g class="edges">`) > strings.Index(s, `<g class="vertices">`) {
		t.Error("vertices before edges")
	}
	opts.Mode = svg.View
	buf.Reset()
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<g class="overlay"></g>`) {
		t.Error("overlay not empty in view mode")
	}
}

// TestNotation: the actor's figure with its label at the foot of its box, the store's
// two lines, the dashed classes, a marker per head with the open head's
// reference at its tip, the stereotype above the label, and a straight
// edge between ellipses clipped at both outlines.
func TestNotation(t *testing.T) {
	d, opts := notation(t)
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{
		`<rect class="shape box" x="-20" y="-38" width="40" height="76"/><circle class="figure head" cx="0" cy="-29.2" r="8.8"/>`,
		`<text class="label" x="0" y="34" text-anchor="middle">user</text>`,
		`<rect class="shape store" x="-55" y="-18" width="110" height="36"/><path class="figure" d="M -55 -18 H 55 M -55 18 H 55"/>`,
		`<g class="vertex zone dashed"`, `<g class="edge include dashed"`,
		`class="arrowhead include open" viewBox="0 0 10 10" refX="10"`, `class="arrowhead generalisation hollow"`, `class="arrowhead flow" viewBox`,
		`<text class="stereotype"`,
		`<g class="containers"><g class="vertex system container unplaced" id="shop" data-kind="system" transform="translate(290 130)"><rect class="shape" x="-96" y="-108" width="192" height="216"/><text class="label" x="-88" y="-94" text-anchor="start">shop</text></g></g><g class="edges">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	// place order (280,80) 140×44 to pay (300,200): the line leaves the
	// first ellipse and enters the second on their outlines.
	if !strings.Contains(s, `<g class="edge include dashed" id="e2" data-kind="include" data-from="place order" data-to="pay"><path d="M 283.66 101.97 L 296.34 178.03"`) {
		t.Errorf("include not clipped at both ellipses:\n%s", s)
	}
}

// TestTarget: the overlay rings a target vertex with its own shape,
// stroked as wide as its outline band, and draws nothing for a dead one
// or in view mode.
func TestTarget(t *testing.T) {
	d, opts := shapes(t)
	opts.Target, opts.HasTarget = 4, true
	var buf bytes.Buffer
	if err := svg.New().RenderOverlay(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	want := `<g class="target" transform="translate(440 60)" stroke-width="12"><ellipse class="ring" cx="0" cy="0" rx="20" ry="20"/></g>`
	if !strings.Contains(buf.String(), want) {
		t.Errorf("missing %q in\n%s", want, buf.String())
	}
	for _, o := range []svg.Options{{Mode: svg.View, Target: 4, HasTarget: true}, {Mode: svg.Edit, Target: 99, HasTarget: true}} {
		buf.Reset()
		if err := svg.New().RenderOverlay(d, o, &buf); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(buf.String(), "target") {
			t.Errorf("ring drawn for %+v: %s", o, buf.String())
		}
	}
}

// TestRenderOne: a single vertex or edge renders to the same text as its
// part of the whole, so a host can replace it in place.
func TestRenderOne(t *testing.T) {
	d, opts := diamond(t)
	r := svg.New()
	var whole, one bytes.Buffer
	if err := r.Render(d, opts, &whole); err != nil {
		t.Fatal(err)
	}
	if err := r.RenderVertex(d, opts, 1, &one); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(whole.String(), one.String()) || !strings.HasPrefix(one.String(), `<g class="vertex node" id="a"`) {
		t.Errorf("vertex alone:\n%s", one.String())
	}
	one.Reset()
	if err := r.RenderEdge(d, opts, 1, &one); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(whole.String(), one.String()) || !strings.HasPrefix(one.String(), `<g class="edge outcome" id="e2"`) {
		t.Errorf("edge alone:\n%s", one.String())
	}
}

// TestViewport: a zero viewport takes the bounds of the vertices with a
// margin; an empty document a unit box; a set viewport is used as is.
func TestViewport(t *testing.T) {
	d := newDoc(t, gw.Caps{})
	var buf bytes.Buffer
	if err := svg.Render(d, svg.Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"`) {
		t.Errorf("empty: %s", buf.String()[:80])
	}
	vertex(t, d, kSource, "s", 100, 50)
	buf.Reset()
	if err := svg.Render(d, svg.Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `viewBox="28 14 144 72"`) {
		t.Errorf("bounds: %s", buf.String()[:80])
	}
	d.View = gw.Viewport{X: -5, Y: -5, W: 300, H: 200}
	buf.Reset()
	if err := svg.Render(d, svg.Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `viewBox="-5 -5 300 200"`) {
		t.Errorf("set: %s", buf.String()[:80])
	}
}

// failing is a writer that fails, to see the error come back.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errWrite }

var errWrite = &writeError{}

type writeError struct{}

func (*writeError) Error() string { return "write failed" }

func TestWriteError(t *testing.T) {
	d, opts := diamond(t)
	if err := svg.Render(d, opts, failing{}); err == nil || err.Error() != "write failed" {
		t.Errorf("got %v", err)
	}
}

// TestSelectionHalo: a selected edge's halo is drawn as wide as the
// stroke-width the renderer writes on it, which a CSS width would
// override and hide, so no rule of the default stylesheet that can
// match the halo sets a width.
func TestSelectionHalo(t *testing.T) {
	css := svg.Stylesheet
	// bound: the comments in the stylesheet
	for i := strings.Index(css, "/*"); i >= 0; i = strings.Index(css, "/*") {
		j := strings.Index(css[i:], "*/")
		if j < 0 {
			t.Fatal("unterminated comment")
		}
		css = css[:i] + css[i+j+2:]
	}
	for _, rule := range strings.Split(css, "}") {
		sel, body, ok := strings.Cut(rule, "{")
		if !ok || !strings.Contains(body, "stroke-width") {
			continue
		}
		for _, s := range strings.Split(sel, ",") {
			if strings.Contains(s, ".selection") && !strings.Contains(s, "rect.selection") {
				t.Errorf("%q sets a width the edge halo would take", strings.TrimSpace(s))
			}
		}
	}
	d, opts := shapes(t)
	var b bytes.Buffer
	if err := svg.Render(d, opts, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `class="selection" stroke-width="`) {
		t.Error("the selected edge's halo has no width of its own")
	}
}

// TestIDPrefix: the prefixes go before the renderer's own IDs and
// every reference to them, and before each element's ID, written as
// tokens; data-from and data-to keep the document's IDs.
func TestIDPrefix(t *testing.T) {
	d, opts := diamond(t)
	opts.IDPrefix, opts.DocIDPrefix = "gw1-", "doc x-"
	var buf bytes.Buffer
	if err := svg.Render(d, opts, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	checkXML(t, buf.Bytes(), d)
	for _, want := range []string{
		`aria-labelledby="gw1-graphedwright-title"`, `<title id="gw1-graphedwright-title">`,
		`<marker id="gw1-arrow-0"`, `marker-end="url(#gw1-arrow-0)"`, `id="doc_x-a"`, `data-from="a"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, unwanted := range []string{`"graphedwright-title"`, `id="arrow-`, `(#arrow-`, `id="a"`} {
		if strings.Contains(s, unwanted) {
			t.Errorf("unprefixed %s", unwanted)
		}
	}
}

// TestOutOfRange: a viewport that cannot be drawn gives way to the
// drawing's bounds, an annotation weight that is not finite leaves the
// edge its own width, a character width or font size that is not
// positive takes the default, and a vanishing character width fits the
// whole label.
func TestOutOfRange(t *testing.T) {
	d, opts := diamond(t)
	render := func() string {
		var buf bytes.Buffer
		if err := svg.Render(d, opts, &buf); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	want := render()
	for _, v := range []gw.Viewport{{W: math.NaN(), H: 10}, {W: 10, H: math.Inf(1)}, {X: math.NaN(), W: 10, H: 10}, {W: -5, H: 10}, {W: 2e9, H: 10}} {
		d.View = v
		if got := render(); got != want {
			t.Errorf("view %v: %s", v, firstDiff([]byte(got), []byte(want)))
		}
	}
	d.View = gw.Viewport{}
	weight := opts.EdgeNotes[1].Weight
	for _, w := range []float64{math.Inf(1), math.NaN(), 2e9} {
		opts.EdgeNotes[1].Weight = w
		if got := render(); got != want {
			t.Errorf("weight %v: %s", w, firstDiff([]byte(got), []byte(want)))
		}
	}
	opts.EdgeNotes[1].Weight = weight
	for _, w := range []float64{math.NaN(), -1} {
		opts.CharWidth, opts.FontSize = w, w
		if got := render(); got != want {
			t.Errorf("character width and font size %v: %s", w, firstDiff([]byte(got), []byte(want)))
		}
	}
	opts.FontSize, opts.CharWidth = 0, 1e-300
	if got := render(); got != want {
		t.Errorf("character width 1e-300: %s", firstDiff([]byte(got), []byte(want)))
	}
}

// TestContainerStacking: a container and all it holds is drawn wholly
// under or over one beside it that overlaps, not interleaved by depth,
// and the hit test takes the one drawn over: a zone a holding a zone c,
// and a zone b, of no depth, over c.
func TestContainerStacking(t *testing.T) {
	d, err := gw.New(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Size: gw.Size{W: 40, H: 20}},
			{Name: "zone", Size: gw.Size{W: 100, H: 60}, Container: true},
		},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	a := vertex(t, d, 1, "a", 0, 0)
	b := vertex(t, d, 1, "b", 30, 0)
	c := vertex(t, d, 1, "c", 0, 0)
	x := vertex(t, d, 0, "x", 0, 0)
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: x, To: c})
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: c, To: a})
	var buf bytes.Buffer
	if err := svg.Render(d, svg.Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	ia, ib, ic := strings.Index(s, `id="a"`), strings.Index(s, `id="b"`), strings.Index(s, `id="c"`)
	if ia < 0 || ia >= ic || ic >= ib {
		t.Errorf("drawn a at %d, c at %d, b at %d; want a, c, b", ia, ic, ib)
	}
	// (30,20) is in c, [-36,36]×[-46,26], and in b, [-20,80]×[-30,30].
	if got := svg.HitTest(d, gw.Point{X: 30, Y: 20}); got.Kind != svg.VertexHit || got.Vertex != b {
		t.Errorf("where b is over c: %+v", got)
	}
	if got := svg.ContainerAt(d, gw.Point{X: 30, Y: 20}, gw.None); got != b {
		t.Errorf("container where b is over c: %d", got)
	}
	if got := svg.ContainerAt(d, gw.Point{X: -30, Y: 20}, gw.None); got != c {
		t.Errorf("container in c clear of b: %d", got)
	}
}
