package svg_test

import (
	"bytes"
	"strings"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/svg"
)

// render draws the document with the options, or fails.
func render(t *testing.T, d *gw.Document, opts svg.Options) string {
	t.Helper()
	var b bytes.Buffer
	if err := svg.Render(d, opts, &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestFoldDrawn: a folded container is drawn as one vertex with the
// number it holds, and what it holds is not drawn or hit; an edge into
// the fold is drawn to it, one within it is not, and two the fold brings
// to the same ends are drawn as one with their number; a lone element
// rendered for a hidden vertex or an edge not drawn is nothing.
func TestFoldDrawn(t *testing.T) {
	d, opts := notation(t)
	admin, order, pay, shop := d.VertexByID("admin"), d.VertexByID("place order"), d.VertexByID("pay"), d.VertexByID("shop")
	e6 := edge(t, d, 0, admin, order, gw.None, gw.None, "", 0)
	e7 := edge(t, d, 0, admin, pay, gw.None, gw.None, "", 0)
	must(t, d, gw.Command{Op: gw.Fold, Vertex: shop})
	s := render(t, d, opts)
	for _, want := range []string{`class="vertex kind-system container folded unplaced" id="shop"`, `data-holds="2"`, `<text class="count"`, `id="e1"`, `id="e4"`, `id="e5"`, `class="edge kind-association merged" id="e6"`, `data-merged="2"`} {
		if !strings.Contains(s, want) {
			t.Errorf("no %s in\n%s", want, s)
		}
	}
	for _, gone := range []string{`id="place order"`, `id="pay"`, `id="e2"`, `id="e7"`} {
		if strings.Contains(s, gone) {
			t.Errorf("%s drawn", gone)
		}
	}
	if first, n := svg.Merged(d, e7); first != e6 || n != 2 || !svg.EdgeDrawn(d, e6) || svg.EdgeDrawn(d, e7) || svg.EdgeDrawn(d, 1) || !svg.EdgeDrawn(d, 0) {
		t.Errorf("merged %d of %d", first, n)
	}
	if h := svg.HitTest(d, d.Vertex(shop).Pos()); h.Kind != svg.VertexHit || h.Vertex != shop {
		t.Errorf("hit at the fold: %+v", h)
	}
	r := svg.New()
	var b bytes.Buffer
	for _, write := range []func() error{
		func() error { return r.RenderVertex(d, opts, order, &b) },
		func() error { return r.RenderEdge(d, opts, 1, &b) },
		func() error { return r.RenderEdge(d, opts, e7, &b) },
	} {
		b.Reset()
		if err := write(); err != nil || b.Len() != 0 {
			t.Errorf("a lone element not drawn wrote %q, %v", b.String(), err)
		}
	}
	must(t, d, gw.Command{Op: gw.Unfold, Vertex: shop})
	if s := render(t, d, opts); !strings.Contains(s, `id="place order"`) || !strings.Contains(s, `id="e7"`) || strings.Contains(s, "merged") || strings.Contains(s, "folded") {
		t.Errorf("unfolded:\n%s", s)
	}
}

// TestFocusDrawn: out of the focus is dimmed, or with FocusHide not
// drawn; an edge is in it when both its drawn ends are.
func TestFocusDrawn(t *testing.T) {
	d, opts := notation(t)
	user, admin := d.VertexByID("user"), d.VertexByID("admin")
	opts.Focus = make([]bool, d.VertexSlots())
	opts.Focus[user], opts.Focus[admin] = true, true
	s := render(t, d, opts)
	if !strings.Contains(s, `class="vertex kind-actor" id="user"`) || !strings.Contains(s, `class="vertex kind-use_case dim" id="pay"`) || !strings.Contains(s, `class="edge kind-generalisation" id="e3"`) || !strings.Contains(s, `class="edge kind-association dim" id="e1"`) {
		t.Errorf("dimmed:\n%s", s)
	}
	opts.FocusHide = true
	s = render(t, d, opts)
	if !strings.Contains(s, `id="user"`) || !strings.Contains(s, `id="e3"`) || strings.Contains(s, `id="pay"`) || strings.Contains(s, `id="e1"`) || strings.Contains(s, " dim") {
		t.Errorf("hidden:\n%s", s)
	}
}

// TestFrameless: a frameless container unfolded is an empty group, not
// hit, though what it holds is; folded, it is drawn as any fold.
func TestFrameless(t *testing.T) {
	d, err := makeDoc(gw.Schema{VertexKinds: []gw.VertexKind{
		{Name: "box", Size: gw.Size{W: 40, H: 20}},
		{Name: "group", Size: gw.Size{W: 100, H: 60}, Container: true, Frameless: true},
	}}, gw.Caps{})
	if err != nil {
		t.Fatal(err)
	}
	a := vertex(t, d, 0, "a", 0, 0)
	g := vertex(t, d, 1, "g", 0, 0)
	must(t, d, gw.Command{Op: gw.SetParent, Vertex: a, To: g})
	opts := svg.Options{}
	s := render(t, d, opts)
	if !strings.Contains(s, `class="vertex kind-group container frameless" id="g" data-kind="group" transform="translate(0 -10)"></g>`) || !strings.Contains(s, `id="a"`) {
		t.Errorf("frameless:\n%s", s)
	}
	corner := gw.Point{X: d.Vertex(g).Pos().X - d.Vertex(g).Size().W/2 + 2, Y: d.Vertex(g).Pos().Y - d.Vertex(g).Size().H/2 + 2}
	if h := svg.HitTest(d, corner); h.Kind != svg.Background {
		t.Errorf("the frameless group was hit: %+v", h)
	}
	if h := svg.HitTest(d, gw.Point{}); h.Vertex != a {
		t.Errorf("what it holds: %+v", h)
	}
	if c := svg.ContainerAt(d, corner, a); c != g {
		t.Errorf("dropped within it: %d", c)
	}
	must(t, d, gw.Command{Op: gw.Fold, Vertex: g})
	if s := render(t, d, opts); !strings.Contains(s, `class="vertex kind-group container folded" id="g"`) || strings.Contains(s, `id="a"`) || !strings.Contains(s, `<rect class="shape"`) {
		t.Errorf("frameless, folded:\n%s", s)
	}
	if c := svg.ContainerAt(d, d.Vertex(g).Pos(), gw.None); c != gw.None {
		t.Errorf("a vertex could be dropped in a fold: %d", c)
	}
}
