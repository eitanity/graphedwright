package svg

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

var (
	cutRE      = regexp.MustCompile(`<text class="([a-z]+) cut" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="([a-z]+)">([^<]*)…</text><text class="([a-z]+) full" display="none" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="([a-z]+)">([^<]*)</text><title>([^<]*)</title>`)
	expandedRE = regexp.MustCompile(`<g class="expanded" transform="translate\(([-0-9.]+) ([-0-9.]+)\)"><rect class="backing" x="([-0-9.]+)" y="[-0-9.]+" width="([-0-9.]+)" height="[-0-9.]+"/><text class="([a-z]+)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="([a-z]+)">([^<]*)</text></g>`)
)

// TestExpanded: laid out in either flow and drawn in either mode, each
// cut text of a vertex is followed by itself in full, not displayed, at
// the same place; and with the vertex hovered the overlay draws each of
// them in full, in order, at that place in the vertex, over a box as
// wide as the width that cut it. A vertex with nothing cut, a dead one
// or none hovered draws nothing in the overlay.
func TestExpanded(t *testing.T) {
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, opts := cutTextsDoc(t)
		d.SetFlow(flow)
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		r := New()
		for _, mode := range []Mode{View, Edit} {
			opts.Mode = mode
			want := map[gw.ID]string{"part": "keyword label entry", "sys": "label", "noted": "annotation", "plain": ""}
			for i := range d.VertexSlots() {
				v := gw.VertexIndex(i) //nolint:gosec // a test's few vertices
				var drawn, over bytes.Buffer
				if err := r.RenderVertex(d, opts, v, &drawn); err != nil {
					t.Fatal(err)
				}
				hovered := opts
				hovered.Hover, hovered.HasHover = v, true
				if err := r.RenderOverlay(d, hovered, &over); err != nil {
					t.Fatal(err)
				}
				cuts := cutRE.FindAllStringSubmatch(drawn.String(), -1)
				if n := strings.Count(drawn.String(), "…"); n != len(cuts) {
					t.Errorf("flow %d mode %d: %s has %d cut texts, %d followed in full", flow, mode, d.Vertex(v).ID(), n, len(cuts))
				}
				classes := make([]string, 0, len(cuts))
				for _, c := range cuts {
					classes = append(classes, c[1])
					if c[6] != c[1] || c[7] != c[2] || c[8] != c[3] || c[9] != c[4] || c[10] != c[11] || !strings.HasPrefix(c[10], c[5]) {
						t.Errorf("flow %d mode %d: %s cut %q not followed by itself in full at its place: %q", flow, mode, d.Vertex(v).ID(), c[0], c[6:])
					}
				}
				if got := strings.Join(classes, " "); got != want[d.Vertex(v).ID()] {
					t.Errorf("flow %d mode %d: %s cut %q, want %q", flow, mode, d.Vertex(v).ID(), got, want[d.Vertex(v).ID()])
				}
				exp := expandedRE.FindAllStringSubmatch(over.String(), -1)
				if len(cuts) == 0 && over.String() != `<g class="overlay"></g>` {
					t.Errorf("flow %d mode %d: %s with nothing cut draws %s", flow, mode, d.Vertex(v).ID(), over.String())
				}
				if len(exp) != len(cuts) {
					t.Fatalf("flow %d mode %d: %s has %d cut texts, %d expanded in\n%s", flow, mode, d.Vertex(v).ID(), len(cuts), len(exp), over.String())
				}
				pos := d.Vertex(v).Pos()
				for k, e := range exp {
					c := cuts[k]
					if e[1] != num(pos.X) || e[2] != num(pos.Y) || e[5] != c[1] || e[6] != c[2] || e[7] != c[3] || e[8] != c[4] || e[9] != c[10] {
						t.Errorf("flow %d mode %d: %s expands %q as %q", flow, mode, d.Vertex(v).ID(), c[0], e[0])
					}
					cw := float64(defaultCharWidth)
					if c[1] == "entry" {
						cw = gw.CharWidth
					}
					w := float64(runeCount(c[10]))*cw + 4
					x, _ := strconv.ParseFloat(c[2], 64)
					left := map[string]float64{"start": x - 2, "middle": x - w/2, anchorEnd: x + 2 - w}[c[4]]
					if e[3] != num(left) || e[4] != num(w) {
						t.Errorf("flow %d mode %d: %s %q backed at x %s width %s, want %v %v", flow, mode, d.Vertex(v).ID(), c[10], e[3], e[4], left, w)
					}
				}
			}
			var none, dead bytes.Buffer
			if err := r.RenderOverlay(d, opts, &none); err != nil {
				t.Fatal(err)
			}
			gone := opts
			gone.Hover, gone.HasHover = gw.VertexIndex(d.VertexSlots()), true //nolint:gosec // a test's few vertices
			if err := r.RenderOverlay(d, gone, &dead); err != nil {
				t.Fatal(err)
			}
			if none.String() != `<g class="overlay"></g>` || dead.String() != none.String() {
				t.Errorf("flow %d mode %d: no hover draws %s, a dead one %s", flow, mode, none.String(), dead.String())
			}
		}
	}
}

// TestNoFullText: with NoFullText a cut text is written as it was
// before the full text was added, with no mark: the default drawing
// with the marks and the full texts taken out.
func TestNoFullText(t *testing.T) {
	d, opts := cutTextsDoc(t)
	var with, without bytes.Buffer
	if err := Render(d, opts, &with); err != nil {
		t.Fatal(err)
	}
	opts.NoFullText = true
	if err := Render(d, opts, &without); err != nil {
		t.Fatal(err)
	}
	full := regexp.MustCompile(`<text class="[a-z]+ full" display="none"[^>]*>[^<]*</text>`)
	if n := len(full.FindAllString(with.String(), -1)); n != 5 {
		t.Errorf("%d full texts, want 5", n)
	}
	stripped := strings.ReplaceAll(full.ReplaceAllString(with.String(), ""), ` cut"`, `"`)
	if stripped != without.String() {
		t.Errorf("without full texts:\n%s\nwant\n%s", without.String(), stripped)
	}
}

// num writes a number as the renderer does.
func num(f float64) string {
	var b bytes.Buffer
	o := newOut()
	o.reset(&b)
	o.num(f)
	o.flush()
	return b.String()
}

// cutTextsDoc is the cut texts example with IDs to name the vertices by.
func cutTextsDoc(t *testing.T) (*gw.Document, Options) {
	t.Helper()
	d, err := makeDoc(gw.Schema{
		VertexKinds: []gw.VertexKind{
			{
				Name: "part", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part definition of a kind»",
				Attrs: []gw.AttrSpec{{Key: "doc"}}, Compartments: []string{"doc"}, MaxWidth: 160,
			},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Keyword: "«system»"},
			{Name: "plain", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind gw.KindIndex, id, label string) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, ID: gw.ID(id), Text: label})
		if err != nil {
			t.Fatal(err)
		}
		return ch.Vertices[0]
	}
	part, inner := add(0, "part", "a part with a name too long for it"), add(2, "inside", "inside")
	sys, noted, plain := add(1, "sys", "a system whose title is too long"), add(2, "noted", "noted"), add(2, "plain", "plain")
	for _, c := range []gw.Command{
		{Op: gw.SetAttr, Vertex: part, Key: "doc", Value: "short\nA line of documentation too long to fit."},
		{Op: gw.SetParent, Vertex: inner, To: sys},
		{Op: gw.AddEdge, Kind: 0, From: part, To: inner, FromPort: gw.None, ToPort: gw.None},
		{Op: gw.AddEdge, Kind: 0, From: noted, To: plain, FromPort: gw.None, ToPort: gw.None},
	} {
		if _, err := d.Apply(c); err != nil {
			t.Fatal(err)
		}
	}
	notes := make([]Annotation, d.VertexSlots())
	notes[noted] = Annotation{Text: "an annotation far too long to fit"}
	return d, Options{VertexNotes: notes}
}
