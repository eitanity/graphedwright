package graphedwright_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

// Example draws two boxes joined by an arrow: a schema, a document,
// two commands for the vertices and one for the edge, a layout and a
// render.
func Example() {
	schema := gw.Schema{
		VertexKinds: []gw.VertexKind{{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36}}},
		EdgeKinds:   []gw.EdgeKind{{Name: "arrow", Arrow: true}},
	}
	d, err := gw.New(schema, gw.Caps{})
	if err != nil {
		fmt.Println(err)
		return
	}
	// A Change's slices are the document's scratch, valid until the
	// next Apply: take the new vertex's index out at once.
	ch, _ := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "hello"})
	hello := ch.Vertices[0]
	ch, _ = d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "world"})
	world := ch.Vertices[0]
	if _, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: 0, From: hello, To: world, FromPort: gw.None, ToPort: gw.None}); err != nil {
		fmt.Println(err)
		return
	}
	if _, err := layout.New(d.Caps()).Run(d, layout.Options{}); err != nil {
		fmt.Println(err)
		return
	}
	var out bytes.Buffer
	if err := svg.Render(d, svg.Options{Title: "Hello"}, &out); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(d.Vertices[0].Pos, d.Vertices[1].Pos)
	fmt.Println(strings.HasPrefix(out.String(), "<svg"))
	// Output:
	// {0 16} {160 16}
	// true
}

// Example_dependencyTree is the tutorial's program (docs/tutorial.md):
// a schema of packages and dependencies, a document built by commands,
// a refusal and an undo, a top-to-bottom layout, an SVG and the JSON
// that saves it.
func Example_dependencyTree() {
	// 1. The schema: what a document may hold.
	schema := gw.Schema{
		VertexKinds: []gw.VertexKind{{
			Name:  "package",
			Shape: gw.RoundRect,
			Size:  gw.Size{W: 120, H: 36},
			Attrs: []gw.AttrSpec{
				{Key: "version", Kind: gw.Text},
				{Key: "licence", Kind: gw.Choice, Choices: []string{"MIT", "Apache-2.0", "BSD-3-Clause"}, Default: "MIT"},
			},
		}},
		EdgeKinds: []gw.EdgeKind{
			{Name: "depends on", Arrow: true},
			{Name: "dev dependency", Arrow: true, Dashed: true},
		},
		Acyclic: true, // a dependency cycle is refused
	}
	const pkg, dependsOn, devDependency = 0, 0, 1

	// 2. A document, and commands that fill it.
	d, err := gw.New(schema, gw.Caps{})
	if err != nil {
		fmt.Println(err)
		return
	}
	add := func(name string) gw.VertexIndex {
		ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: pkg, ID: gw.ID(name), Text: name})
		if err != nil {
			fmt.Println(err)
			return gw.None
		}
		return ch.Vertices[0]
	}
	link := func(kind gw.KindIndex, from, to gw.VertexIndex) error {
		_, err := d.Apply(gw.Command{Op: gw.AddEdge, Kind: kind, From: from, To: to, FromPort: gw.None, ToPort: gw.None})
		return err
	}
	app, cli, server := add("app"), add("cli"), add("server")
	router, db, logs, testkit := add("router"), add("db"), add("log"), add("testkit")
	for _, e := range [][2]gw.VertexIndex{{app, cli}, {app, server}, {server, router}, {server, db}, {cli, logs}, {router, logs}, {db, logs}} {
		if err := link(dependsOn, e[0], e[1]); err != nil {
			fmt.Println(err)
		}
	}
	if err := link(devDependency, app, testkit); err != nil {
		fmt.Println(err)
	}
	if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: db, Key: "version", Value: "2.4.1"}); err != nil {
		fmt.Println(err)
	}

	// 3. The schema refuses what breaks it, and the document is left
	// as it was.
	fmt.Println(link(dependsOn, logs, app))
	if _, err := d.Apply(gw.Command{Op: gw.SetAttr, Vertex: db, Key: "licence", Value: "GPL"}); err != nil {
		fmt.Println(err)
	}

	// 4. Every command can be undone.
	if _, err := d.Apply(gw.Command{Op: gw.Relabel, Vertex: db, Text: "database"}); err != nil {
		fmt.Println(err)
	}
	d.Undo()
	fmt.Println(d.Vertices[db].Label)

	// 5. A layout, top to bottom as a dependency tree is drawn.
	if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: "top-to-bottom"}); err != nil {
		fmt.Println(err)
	}
	res, err := layout.New(d.Caps()).Run(d, layout.Options{All: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(res.Layers, "layers; app at", d.Vertices[app].Pos, "log at", d.Vertices[logs].Pos)

	// 6. Draw it, with the default stylesheet embedded.
	var picture bytes.Buffer
	if err := svg.Render(d, svg.Options{Title: "Dependencies", Style: svg.Stylesheet}, &picture); err != nil {
		fmt.Println(err)
		return
	}
	_ = os.WriteFile(os.DevNull, picture.Bytes(), 0o600) // os.WriteFile("deps.svg", …) in a real program

	// 7. Save it as JSON, and read it back under the same schema name.
	var text bytes.Buffer
	if err := doc.Encode(d, "deps", &text); err != nil {
		fmt.Println(err)
		return
	}
	again, err := doc.Decode(text.Bytes(), doc.Options{Schemas: []doc.Named{{Name: "deps", Schema: schema}}})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(again.LiveVertices(), "packages,", again.LiveEdges(), "dependencies,", again.Flow)
	// Output:
	// edge would make a cycle: log → app → cli → log
	// attribute "licence" of vertex "db" must be one of MIT, Apache-2.0, BSD-3-Clause
	// db
	// 4 layers; app at {360 0} log at {208 288}
	// 7 packages, 8 dependencies, top-to-bottom
}
