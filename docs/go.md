# The Go API

graphedwright is a set of packages under `code.eitanity.com/graphedwright`
that import only the standard library and merilo's `fsm`
(`code.eitanity.com/merilo/fsm`), which imports nothing. They build
with Go and with TinyGo, for the host and for WebAssembly. The full
reference is in the package documentation (`go doc`); this guide shows
how the pieces fit.

| Package | What it holds |
|---|---|
| [`graphedwright`](#graphedwright) (imported as `gw`) | the document, its schema, commands and undo |
| [`layout`](#layout) | the layered layout |
| [`svg`](#svg) | the renderer, the hit test and the default stylesheet |
| [`doc`](#doc) | the JSON format and the built-in schemas |
| [`edit`](#edit) | the editor: input events in, commands out |
| `dom` | the browser binding, for `GOOS=js` only ([browser guide](browser.md)) |

The [tutorial](tutorial.md) walks through a whole program, and
[`example_test.go`](../example_test.go) holds it as a runnable example.

## graphedwright

```go
d, err := gw.New(schema, gw.Caps{})   // check the schema, allocate everything
ch, err := d.Apply(gw.Command{Op: gw.AddVertex, Kind: 0, Text: "a"})
v := ch.Vertices[0]                    // copy out at once: valid until the next Apply
d.Undo()
d.Redo()
```

- **`New(schema, caps)`** makes an empty document and refuses an
  inconsistent schema. [Schemas](schemas.md) describes the schema, and
  [Concepts](concepts.md#capacities) the capacities.
- **`Apply(cmd)`** applies a command or refuses it, leaving the document
  as it was. It returns the `Change`: the vertices and edges touched,
  and whether the layout is stale. The commands are listed in
  [Concepts](concepts.md#commands-and-undo).
- **`Undo()`, `Redo()`, `CanUndo()`, `CanRedo()`, `ClearUndo()`** work
  the undo log.
- **`Vertices` and `Edges`** are slices you can read directly, by index.
  Deleted elements stay as tombstones (`Live == false`) until
  `Compact()`, which renumbers and returns the maps from old indices to
  new.
- **`VertexByID`, `EdgeByID`, `EdgesAt`, `LiveVertices`, `LiveEdges`**
  answer common questions, and **`Parallel(e)`** gives an edge's lane
  among those between the same two vertices.
- **`Unreachable(dst)`** lists the vertices no root kind reaches, for a
  schema with `Connected`.
- **`Depth(v)` and `Within(v, c)`** answer questions about containers,
  and **`Containers()`** lists them in the order they are drawn.
- **`Flow`** is the way the layers run, set by a `Layout` command.
  **`View`** is the viewport; a host sets it directly.

Three calls change positions outside the command log. They are for a
layout engine or a loader, not for edits a user should be able to undo:
- **`Place(v, p, placed)`** sets a vertex's position.
- **`Fit()`** fits every container around what it holds.
- **`SetBends(e, pts)` and `ReserveBends(e, n)`** give an edge its route.

## layout

```go
l := layout.New(d.Caps())     // allocate the working arrays once
d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: "top-to-bottom"})   // so it can be undone
res, err := l.Run(d, layout.Options{All: true})
```

`Run` places every vertex that isn't placed and routes the long edges.
With `All`, it places every vertex. Precede it with a `Layout` command
when the layout should be undoable, and to change the flow.

| Option | Default | Meaning |
|---|---|---|
| `Pitch` | 160, or 96 top to bottom | the least distance between layers; more where wide vertices or, left to right, edge labels need it |
| `Gap` | 24 | the least gap between vertices in a layer |
| `Grid` | 8 | the grid positions snap to |
| `Sweeps` | 8 | the crossing-reduction passes |
| `All` | false | lay out placed vertices too |

`Result` gives the number of layers, the edge crossings left, and the
edges taken as reversed to break cycles, so a host can mark them. A
document that needs more placeholder points than the budget (8 per edge
capacity) is refused with an error. The same input always gives the
same layout.

## svg

```go
err := svg.Render(d, svg.Options{Title: "My graph", Style: svg.Stylesheet}, w)
```

`Render` writes the whole document as SVG to any `io.Writer`. It
allocates nothing per element. A `svg.Renderer` (from `svg.New()`) keeps
its buffer between renders and can also write one vertex, one edge or
the overlay alone (`RenderVertex`, `RenderEdge`, `RenderOverlay`), for
patching a page in place.

| Option | Meaning |
|---|---|
| `Mode` | `svg.View` or `svg.Edit`; the overlay is drawn in edit mode |
| `Title`, `Desc` | the SVG's `<title>` and `<desc>` |
| `Style` | a stylesheet to embed, such as `svg.Stylesheet`; empty for none |
| `VertexNotes`, `EdgeNotes` | annotations by index: `{Text, Class, Weight}`, a figure under a vertex's label or at an edge's middle |
| `CharWidth`, `FontSize` | the label metrics used to cut long labels, 7 and 12 by default |
| `DefaultWeight` | the stroke width of an unweighted edge, 1.5 by default |
| `SelectedV`, `SelectedE`, `Ghost`, `Marquee`, `Target` | the overlay's contents; an `edit.Editor` fills these with `Overlay(&opts)` |
| `IDPrefix` | goes before the renderer's own IDs, the title's and the arrowheads', for more than one drawing in a page |
| `DocIDPrefix` | goes before each vertex's and edge's `id`, so that a document cannot take an ID the page uses |

The output is the same, byte for byte, on every platform, so it can be
compared in tests. The viewport is the document's `View`, or when that
is empty, the box around every vertex and bend with a margin
(`svg.Bounds`). [The browser guide](browser.md#styling) lists the
classes it writes.

To serve drawings from a web server:

```go
http.HandleFunc("/graph.svg", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	if err := svg.Render(d, svg.Options{Style: svg.Stylesheet}, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
})
```

The geometry the renderer draws from is also exported:

- **`HitTest(d, p)`** names what is drawn at a point: a port, a
  vertex's outline band, a vertex, an edge (its path or its label), a
  container, or the background.
- **`ContainerAt(d, p, v)`** is the container vertex `v` would go in if
  dropped at `p`.
- **`HandleBox(d, v)`** is the square of a vertex's resize handle, on
  its bottom right corner.
- **`VertexLabelBox` and `EdgeLabelBox`** are where a label is drawn,
  for placing an input to edit it.
- **`PortAt`** is where a port is, and **`Bounds`** is the box around
  the drawing.

## doc

```go
d, err := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
d, name, err := doc.DecodeNamed(data, opts)      // also the schema's name
err = doc.Encode(d, name, w)
```

`Decode` reads the [document format](format.md). It refuses anything
the schema or the capacities don't allow, naming where. A document
without positions is laid out unless `NoLayout` is set.

- **Schemas:** `Options.Schemas` lists the schemas the reader knows by
  name. `doc.Builtin()` gives `basic`, `use-case`, `threat-model` and
  `state-machine`;
  append your own as `doc.Named{Name, Schema}`.
- **Capacities:** `Options.Caps` gives the capacities of the document
  made.

`Encode` writes the live content deterministically.

## edit

An `edit.Editor` turns input into commands. It knows nothing of the
browser, so a test or another front end can drive it with events:

```go
e := edit.New(d, edit.Edit, 8)        // the grid is the snap and nudge step
ch, err := e.Handle(edit.Event{Kind: edit.PointerDown, Point: gw.Point{X: 220, Y: 100}})
ch, err = e.Handle(edit.Event{Kind: edit.PointerUp, Point: gw.Point{X: 300, Y: 140}})   // a drag: a Move
ch, err = e.Handle(edit.Event{Kind: edit.Key, Key: "Delete"})
```

- **Events:** `PointerDown`, `PointerMove`, `PointerUp`, `Wheel`, `Key`,
  `DoubleClick`, `PaletteStart` (a vertex kind to place, or an edge kind
  to choose), `TextDone`, `TextCancel`, `Blur` and `Resize`. Points are
  in document units, or, once a `Resize` event has given the element's
  size, in its pixels, mapped through the document's view.
- **Return value:** each `Handle` returns the `Change` and the refusal,
  if any.
- **State:** `State`, `Selected`, `Hover`, `Editing`, `Typed` and
  `ResizeHandle` report where the editor is, which is what a front end
  needs to draw it.
- **View:** `Zoom(factor)` zooms about the view's centre and `Fit()`
  shows the whole drawing, for a front end's buttons; Ctrl with `=`,
  `-` and `0` do the same from the keyboard.
  `Overlay(&opts)` fills the renderer's options to draw the selection
  and the feedback.
- **Properties:** `Properties()` describes the one selected element for
  a panel, and `SetLabel` and `SetAttr` change it as commands.
- **Mode:** `SetMode(edit.View)` turns editing off; edits are then
  refused with `edit.ErrView`.

[The browser guide](browser.md#using-the-editor) lists what each gesture
does.

## Coding rules

The packages keep to rules that let them run in TinyGo and in a browser
without pauses:
- **No allocation once made:** capacities are fixed when a document is
  made, and nothing allocates afterwards.
- **Bounded work:** no recursion, and every loop has a stated bound.
- **No `fmt`, `reflect` or `unsafe`.**
- **Allocation-free hot paths:** the render path and the editor's event
  path don't allocate.

`make ci` checks all of these, with the linters and the project's own
analyzers in `internal/lint`, and it checks the TinyGo build stays under
600 KB.
