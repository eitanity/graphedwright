package svg

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/layout"
)

var (
	partRE  = regexp.MustCompile(`<g class="vertex [^"]*" id="([^"]+)"[^>]*>(.*?)</g></g>`)
	ruleRE  = regexp.MustCompile(`<line x1="([-0-9.]+)" y1="([-0-9.]+)" x2="([-0-9.]+)" y2="([-0-9.]+)"/>`)
	entryRE = regexp.MustCompile(`<text class="(entry|label)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="(start|middle)">([^<]*)</text>`)
)

// TestCompartmentsInside: laid out in either flow, each compartment's
// rule runs across its vertex below the label, and each of its lines,
// as drawn, cut or not, is inside the vertex's outline and below the
// rule above it.
func TestCompartmentsInside(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{{
			Name: "part", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part def»",
			Attrs:        []gw.AttrSpec{{Key: "attributes"}, {Key: "doc"}},
			Compartments: []string{"attributes", "doc"}, MaxWidth: 180,
		}},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}
	values := []struct{ attributes, doc string }{
		{"mass : Real\nwheels : Integer = 4\ncolour : String", "A vehicle is anything with wheels that carries people or goods."},
		{"", "one line"},
		{"rpm : Real", ""},
		{"", ""},
	}
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := gw.New(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.Flow = flow
		var prev gw.VertexIndex = gw.None
		for i, val := range values {
			v := apply(t, d, gw.Command{Op: gw.AddVertex, Kind: 0, ID: gw.ID("p" + strconv.Itoa(i)), Text: "part " + strconv.Itoa(i)}).Vertices[0]
			apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: v, Key: "attributes", Value: val.attributes})
			apply(t, d, gw.Command{Op: gw.SetAttr, Vertex: v, Key: "doc", Value: val.doc})
			if prev != gw.None {
				apply(t, d, gw.Command{Op: gw.AddEdge, Kind: 0, From: prev, To: v, FromPort: gw.None, ToPort: gw.None})
			}
			prev = v
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Render(d, Options{}, &buf); err != nil {
			t.Fatal(err)
		}
		shown := 0
		for _, m := range partRE.FindAllStringSubmatch(buf.String(), -1) {
			x := d.Vertices[d.VertexByID(gw.ID(m[1]))]
			w, h := x.Size.W, x.Size.H
			rules := ruleRE.FindAllStringSubmatch(m[2], -1)
			texts := entryRE.FindAllStringSubmatch(m[2], -1)
			if len(texts) == 0 || texts[0][1] != "label" {
				t.Fatalf("flow %d: %s draws no label first", flow, m[1])
			}
			labelY, _ := strconv.ParseFloat(texts[0][3], 64)
			rule := -h / 2
			for _, tm := range texts[1:] {
				y, _ := strconv.ParseFloat(tm[3], 64)
				// The rule this line is under is the last above it.
				for _, r := range rules {
					if ry, _ := strconv.ParseFloat(r[2], 64); ry < y {
						rule = ry
					}
				}
				left, _ := strconv.ParseFloat(tm[2], 64)
				b := gw.Viewport{X: left, Y: y - 11, W: float64(runeCount(tm[5])) * gw.CharWidth, H: 14}
				if b.X < -w/2 || b.X+b.W > w/2 || b.Y < rule || b.Y+b.H > h/2 {
					t.Errorf("flow %d: %s line %q at %v is outside %vx%v or above its rule at %v", flow, m[1], tm[5], b, w, h, rule)
				}
				shown++
			}
			for _, r := range rules {
				x1, _ := strconv.ParseFloat(r[1], 64)
				y, _ := strconv.ParseFloat(r[2], 64)
				x2, _ := strconv.ParseFloat(r[3], 64)
				if x1 != -w/2 || x2 != w/2 || y <= labelY || y >= h/2 {
					t.Errorf("flow %d: %s rule %v from %v to %v is not across %vx%v below the label at %v", flow, m[1], y, x1, x2, w, h, labelY)
				}
			}
		}
		if shown != 6 {
			t.Errorf("flow %d: %d compartment lines drawn, want 6", flow, shown)
		}
	}
}

func apply(t *testing.T, d *gw.Document, c gw.Command) gw.Change {
	t.Helper()
	ch, err := d.Apply(c)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}
