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
	vertexRE = regexp.MustCompile(`<g class="vertex [^"]*" id="([^"]+)"[^>]*transform="translate\(([-0-9.]+) ([-0-9.]+)\)"[^>]*>(.*?)</g>`)
	kwRE     = regexp.MustCompile(`<text class="(keyword|label)" x="([-0-9.]+)" y="([-0-9.]+)" text-anchor="(start|middle|end)">([^<]*)</text>`)
)

// TestKeywordClear: laid out in either flow, a vertex's keyword line
// overlaps neither its label nor its shape's outline: inside a box,
// above an actor's head, and before a container's title on its line.
func TestKeywordClear(t *testing.T) {
	s := gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "block", Shape: gw.Rect, Size: gw.Size{W: 120, H: 40}, Keyword: "«part def»"},
			{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 76}, Keyword: "«actor»"},
			{Name: "system", Size: gw.Size{W: 100, H: 60}, Container: true, Keyword: "«system»"},
		},
		EdgeKinds: []gw.EdgeKind{{Name: "flow", Arrow: true}},
	}
	for _, flow := range []gw.Flow{gw.LeftToRight, gw.TopToBottom} {
		d, err := makeDoc(s, gw.Caps{})
		if err != nil {
			t.Fatal(err)
		}
		d.SetFlow(flow)
		add := func(kind gw.KindIndex, label string) gw.VertexIndex {
			ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: kind, Text: label})
			if err != nil {
				t.Fatal(err)
			}
			return ch.Vertices[0]
		}
		user, car, engine := add(1, "driver"), add(0, "vehicle"), add(0, "an engine with a long name")
		sys := add(2, "the vehicle system")
		for _, c := range []gw.Command{
			{Op: gw.SetParent, Vertex: car, To: sys}, {Op: gw.SetParent, Vertex: engine, To: sys},
			{Op: gw.AddEdge, Kind: 0, From: user, To: car, FromPort: gw.None, ToPort: gw.None},
			{Op: gw.AddEdge, Kind: 0, From: car, To: engine, FromPort: gw.None, ToPort: gw.None},
		} {
			if _, err := d.Apply(c); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Render(d, Options{}, &buf); err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, m := range vertexRE.FindAllStringSubmatch(buf.String(), -1) {
			v := d.VertexByID(gw.ID(m[1]))
			kind := d.Kinds().Vertex(d.Vertices[v].Kind)
			var kw, label gw.Viewport
			found := 0
			for _, tm := range kwRE.FindAllStringSubmatch(m[4], -1) {
				x, _ := strconv.ParseFloat(tm[2], 64)
				y, _ := strconv.ParseFloat(tm[3], 64)
				b := textBox(x, y, tm[4], tm[5])
				if tm[1] == "keyword" {
					// A keyword is drawn at 11px, against a label's 12.
					w := b.W * 0.9
					switch tm[4] {
					case "middle":
						b.X += (b.W - w) / 2
					case "end":
						b.X += b.W - w
					}
					b.W = w
					kw = b
					found++
				} else {
					label = b
				}
			}
			if found != 1 {
				t.Errorf("flow %d: %s has %d keywords", flow, m[1], found)
				continue
			}
			checked++
			size := d.Vertices[v].Size
			if overlaps(kw, label) {
				t.Errorf("flow %d: %s's keyword %v overlaps its label %v", flow, m[1], kw, label)
			}
			switch {
			case kind.Shape() == gw.Actor:
				if kw.Y+kw.H > -size.H/2 {
					t.Errorf("flow %d: %s's keyword %v reaches its figure, top %g", flow, m[1], kw, -size.H/2)
				}
			case !kind.Container():
				if kw.X < -size.W/2 || kw.X+kw.W > size.W/2 || kw.Y < -size.H/2 || kw.Y+kw.H > size.H/2 {
					t.Errorf("flow %d: %s's keyword %v outside its %v box", flow, m[1], kw, size)
				}
			default:
				if kw.Y < -size.H/2 || kw.Y+kw.H > -size.H/2+gw.ContainerTitle || kw.X < -size.W/2 {
					t.Errorf("flow %d: %s's keyword %v outside its title room", flow, m[1], kw)
				}
			}
		}
		if checked != 4 {
			t.Errorf("flow %d: checked %d vertices, want 4", flow, checked)
		}
	}
}

func overlaps(a, b gw.Viewport) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}
