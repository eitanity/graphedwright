# In the browser

The browser build is the library compiled to WebAssembly, with a small
JavaScript API on a global `graphedwright` object. It draws a document
in an element of your page and, in edit mode, lets people edit it with
the pointer and keyboard. The [tutorial](tutorial.md#part-2-in-a-web-page)
builds a page step by step; [`web/tutorial.html`](../web/tutorial.html)
is a complete small one, and [`web/index.html`](../web/index.html) the
full demo.

## Building and serving

Build with either toolchain:

```sh
# Go: about 3 MB
GOOS=js GOARCH=wasm go build -trimpath -o graphedwright.wasm ./cmd/graphedwright-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .

# TinyGo: under 700 KiB
tinygo build -o graphedwright.wasm -target wasm -opt=z -no-debug ./cmd/graphedwright-wasm
cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" .
```

Each toolchain needs its own `wasm_exec.js`. Serve the `.wasm` file
(as `application/wasm`), `wasm_exec.js`, the stylesheet
[`svg/graphedwright.css`](../svg/graphedwright.css) and your page.
`make demo` builds both and serves the demo on
<http://127.0.0.1:8001/>, choosing the toolchain with `?toolchain=tinygo`.

```html
<link rel="stylesheet" href="graphedwright.css">
<div id="graph" style="height: 520px"></div>
<script src="wasm_exec.js"></script>
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("graphedwright.wasm"), go.importObject).then(({ instance }) => {
    go.run(instance);
    const g = graphedwright.create(document.getElementById("graph"), { schema: "basic" }).ok;
    graphedwright.load(g, { schema: "basic", vertices: [{ id: "a", kind: "box", label: "hello" }] });
  });
</script>
```

The stylesheet holds the drawing's look and the few rules the binding's
own elements need, such as placing the label input. Give the element a
height; the drawing fills it below the palette.

### Content Security Policy

The library needs one allowance beyond a strict policy:
`'wasm-unsafe-eval'`, to compile the WebAssembly. It runs no inline
script, uses no `eval`, and positions its label input through
`element.style`, which a policy does not restrict. The demo server sends
this policy, which a host can start from:

```
default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
```

`'unsafe-inline'` in `style-src` is only for the demo pages' own
`<style>` blocks. Keep page scripts in files, as
[`web/tutorial.js`](../web/tutorial.js) is.

The binding inserts the SVG it renders with `innerHTML`. That is safe,
since every string in it is escaped, but a page that enforces Trusted
Types (`require-trusted-types-for 'script'`) will refuse it.

## The API

Every call returns `{ok: value}` or `{error: "reason"}` and never
throws. The first argument of every call after `create` is the handle
`create` returned.

| Call | What it does |
|---|---|
| `create(element, options)` | mounts an editor in the element and returns a handle |
| `load(g, document)` | replaces the document with one in the [format](format.md), as a JSON string or an object; laid out if it has no positions |
| `export(g)` | the document as an object in the format |
| `svg(g)` | the current drawing as SVG text |
| `setMode(g, "view" \| "edit")` | switches between viewing and editing |
| `layout(g, {all, flow})` | lays the document out; `all` includes placed vertices, `flow` is `"left-to-right"` or `"top-to-bottom"` (the document's own if absent). Returns `{layers, crossings}` |
| `undo(g)`, `redo(g)` | undo or redo the last command, or the editor action it was part of, such as a drag or a nudge of several vertices; `ok` is whether there was one |
| `properties(g)` | the one selected vertex or edge, described for a properties panel, or `null` (below) |
| `setLabel(g, text)` | relabels the one selected vertex or edge |
| `setAttr(g, key, value)` | sets an attribute of the one selected vertex or edge; `""` removes it |
| `addPort(g, {name, side, max})` | gives the one selected vertex a port of its own; `side` is `"left"`, `"right"`, `"top"` or `"bottom"` |
| `deletePort(g, name)` | removes a port of the one selected vertex's own that no edge uses |
| `zoom(g, factor)` | zooms the view about its centre: in by a factor above 1, out by one below |
| `fit(g)` | sets the view to the drawing's bounds |
| `annotate(g, {id: {text, class, weight}})` | draws a figure on each element named, replacing the last set: text under a vertex's label or at an edge's middle, a class added to it, and on an edge a weight that sets its stroke width |
| `on(g, event, callback)` | listens for an event (below) |
| `destroy(g)` | removes the editor and frees what it holds |
| `registerSchema(name, schema)` | adds a schema described as an object ([Schemas](schemas.md#defining-a-schema-in-a-web-page)) |

### `create` options

| Option | Meaning |
|---|---|
| `schema` | a schema's name (`"basic"`, `"use-case"`, `"threat-model"`, `"state-machine"` or one registered), or a schema object; `"basic"` by default |
| `mode` | `"edit"` (the default) or `"view"` |
| `caps` | capacities: `vertices`, `edges`, `portsPerVertex`, `attrsPerElement`, `labelBytes`, `undoDepth`, `undoLayouts`, `bendsPerEdge`; any left out take their defaults ([Concepts](concepts.md#capacities)) |
| `grid` | the snap and nudge step, 8 by default; kept when a document is loaded |
| `patch` | `false` redraws everything on every change instead of only what changed |
| `expand` | `false` stops the overlay drawing in full the cut texts of the shape under the pointer |
| `fullText` | `false` writes a cut text without its full text after it, in the page and in `svg(g)`, so the stylesheet cannot show it on hover; its tooltip still holds it |
| `idPrefix` | goes before each vertex's and edge's `id` in the page, so that a document cannot take an ID the page uses; letters, digits, `-` and `_` |

A number option that is `NaN` or infinite counts as absent and takes its
default, in `create` and in `annotate`.

### Events

| Event | Callback argument |
|---|---|
| `"change"` | `{vertices: [id], edges: [id], layout}`: what a command changed, and whether everything was redrawn |
| `"select"` | `{vertices: [id], edges: [id]}`: the selection, when it changes |
| `"error"` | the text of a refusal, such as `edge would make a cycle: a → b → a` |
| `"edit-label"` | `{id, text}`: a label is about to be edited in the input |

Callbacks run as a microtask, just after the call or the event that
caused them, never in the middle of a redraw. One that throws is
reported by the browser like any uncaught exception and doesn't affect
the editor.

### Properties

`properties(g)` describes the one selected vertex or edge:

```js
{
  id: "f3", element: "edge", kind: "data flow", label: "REST",
  from: "web", to: "api",                 // edges only
  attrs: [
    { key: "protocol", value: "HTTP", kind: "text", choices: [], required: false, known: true },
    { key: "encrypted", value: "no", kind: "choice", choices: ["unknown", "yes", "no"], required: false, known: true },
    …
  ]
}
```

The attributes come in the schema's order, then any the element carries
that its kind does not name (`known: false`). `setLabel` and `setAttr`
are commands: the schema checks them, so a word for a number comes back
as an error, and `undo` takes them back. Build the panel with DOM calls,
not HTML strings, since labels and values are user text.

## A host written in Go

An application written in Go and compiled to WebAssembly mounts the
editor from Go, holds its document and hears its changes in Go, with
no JavaScript in the page but the loader. It never needs the
`graphedwright` object: its `main` calls `dom.Mount` rather than
`dom.Main`, and the build then carries none of the object's code.

```go
type app struct {
	dom.NoHost // the methods this host does not need
	h *dom.Handle
}

// Changed is told what each change drew. It reads the document and may
// annotate; a command it returns is applied in the same undo step.
func (a *app) Changed(ch gw.Change) (gw.Command, bool) {
	d := a.h.Document()
	notes := make([]svg.Annotation, len(d.Vertices))
	for _, v := range ch.Vertices {
		notes[v] = svg.Annotation{Text: "changed"}
	}
	a.h.Annotate(notes, nil)
	return gw.Command{}, false
}

func main() {
	d, _ := doc.Decode(data, doc.Options{Schemas: doc.Builtin()})
	a := &app{}
	el := js.Global().Get("document").Call("getElementById", "graph")
	a.h, _ = dom.Mount(el, d, dom.Options{IDPrefix: "g1-"}, a)
	select {} // keep the program alive for the editor's events
}
```

- **The handle** draws whatever the host changes through it: `Apply`,
  `Undo`, `Redo`, `Load`, `Layout`, `SetMode`, `Annotate`, `Zoom`,
  `Fit` and `Destroy`. `Document` and `Editor` are for reading; change
  the document through the handle, so that the drawing follows.
- **The host** implements `dom.Host`: `Changed`, `Selected`,
  `Refused` and `EditingLabel`. It embeds `dom.NoHost` to leave out
  the ones it doesn't need. Each method runs after the change it reports
  is drawn.
- **Inside a host method the document is locked.** Calls that change
  it, through the handle or the document, are refused (`dom.ErrReentered`,
  `gw.ErrLocked`); reading and `Annotate` are allowed. To answer a
  change with a command, return it from `Changed`. It is applied in the
  same undo step as the edit, so one undo takes both back, and reported
  to `Changed` in turn, up to `Options.MaxFollow` times per event (8
  unless the host sets it). A command returned while a gesture is under
  way, such as a vertex following the pointer, is refused, since it
  would split the action.
- **An edit and its follow-ups stand or fall together.** When the host
  asks for more than `MaxFollow`, or the document refuses a follow-up,
  the edit and every follow-up are taken back. `Refused` hears why, then
  `Changed` hears the change that took them back; a follow-up returned
  for that one is not applied. A rule that may take many commands, one
  per element it changes, needs a `MaxFollow` and an undo depth
  (`gw.Caps.UndoDepth`) to match.
- **A host method must not panic.** On wasm a panic ends the program,
  and every editor in the page with it.
- **`dom.Options`** take what `create`'s options do, in Go: `View`,
  `Grid`, `NoPatch`, `NoExpand`, `NoFullText`, `IDPrefix` (the
  renderer's own IDs; give each editor in a page its own),
  `DocIDPrefix`, and `MaxFollow`.

[`cmd/graphedwright-eventtree`](../cmd/graphedwright-eventtree) is an
application built this way: an editor of event trees, with its own
schema, a probability computed and drawn on every edit, a rule kept by
follow-ups (a new branch splits its event evenly), a check that flags
an event whose branches do not sum to 1, and a panel, a status line,
and Open and Save of its own. Run `make demo` and open
`/eventtree.html`. [`cmd/graphedwright-gohost`](../cmd/graphedwright-gohost)
is the host the smoke test uses to check the rules above.

## Using the editor

In edit mode the drawing shows a palette of the schema's kinds above it.

| To | Do this |
|---|---|
| select | click a shape, an edge or an edge's label; Shift-click adds to the selection; drag on the background to select a rectangle |
| move | drag a shape; it snaps to the grid when released. Dragging a container carries what it holds; dropping a shape in a container's box puts it in, dragging it out takes it out |
| resize | select one shape and drag the square on its bottom right corner; its top left corner stays, and its size snaps to the grid. A container keeps the size given as its least, growing past it to fit what it holds and shrinking back to it as shapes leave |
| add a shape | drag a kind from the palette onto the drawing |
| connect | drag from a port, or from the edge of a shape without ports (it shows a ring and a crosshair), to another shape |
| choose a line kind | click an edge kind in the palette: new edges take it until it is clicked again; otherwise the first kind the two shapes allow is used |
| edit a label | double-click a shape, an edge or a container, or select one and press Enter, press F2 or just start typing; Enter or a click elsewhere keeps the text, Escape drops it |
| delete | Delete or Backspace |
| nudge | the arrow keys, one grid step |
| undo, redo | Ctrl+Z; Ctrl+Shift+Z or Ctrl+Y (Cmd on a Mac) |
| cancel | Escape ends a drag, a resize, a connection or an edit |
| step through | Tab selects the next shape |
| pan | scroll, drag with the middle button, or Alt+drag |
| zoom | Ctrl+scroll or pinch; the + and − buttons, or Ctrl+= and Ctrl+-; Fit, or Ctrl+0, shows the whole drawing |

A refused edit, such as an edge the schema does not allow or one that
would close a cycle, changes nothing and is reported through the
`"error"` event. In view mode, panning, zooming, selecting and hovering
work, and every edit is refused.

## Styling

Everything in the drawing is addressable by class and ID, so a page can
restyle it with CSS or script it.

- **`svg.graphedwright`** is the root, with `data-mode="view"` or
  `"edit"`.
- **Groups:** `g.containers`, `g.edges`, `g.vertices` and `g.overlay`,
  drawn in that order. The overlay's `rect.handle` is the resize handle.
- **Around the drawing:** the host holds `.graphedwright-palette` in
  edit mode, `.graphedwright-drawing`, and `.graphedwright-zoom`, the
  zoom buttons, over the drawing's bottom right corner.
- **IDs:** each editor's own IDs, the title's and the arrowheads', start
  `gw<handle>-`, so that editors in one page keep them apart. A
  document's IDs are written as they are, or after the `idPrefix` given
  to `create`. Without one, a document can hold an ID the page uses, and
  `document.getElementById` then finds whichever comes first. `svg(g)`
  writes neither prefix.
- **Kinds:** a vertex's or an edge's kind is a class, its name after
  `kind-`, with anything but letters, digits, `-` and `_` as `_`: the
  kind `use case` is `.kind-use_case`. The prefix keeps a kind's class
  apart from the drawing's own, so a kind may be named `port`, `label`
  or `overlay` and is styled as itself alone. The name as written is
  also in `data-kind`.
- **Shapes:** each is a `g.vertex` with its kind's class, its ID as
  `id`, and its attributes as `data-*`. Inside are `.shape`, `text.label`,
  `text.keyword` for a kind with a keyword, `.annotation`, and `.port`
  circles, with a `text.port-label` naming each of a vertex's own. Each compartment is a `g.compartment` with its attribute's
  key as `data-key`, holding a `line` and a `text.entry` for each line. Extra classes are `.container`,
  `.dashed`, and `.unplaced` for a vertex no layout or user has placed.
- **Cut texts:** a label, keyword, title, compartment line or
  annotation too long for its room is cut with an ellipsis and has
  `.cut` as well. The full text follows it with `.full` and
  `display="none"`, and then a `<title>` holding it. The default
  stylesheet shows the full text in the cut one's place while the
  pointer is over the vertex, so a saved or embedded drawing shows it
  too, with no script. A vertex drawn later can cover it. `fullText:
  false` at `create` leaves the full text and `.cut` out.
- **Edges:** each is a `g.edge` with its kind's class, `id`,
  `data-from`, `data-to` and its attributes. Inside are the `path`, and
  `text.label` and `text.stereotype`. A dashed kind has `.dashed`.
- **Arrowheads** are markers with the class `.arrowhead` plus the edge
  kind's class, and `.open` or `.hollow`, with `.typed` or `.bar` for
  the marked hollow heads. **Tails** are markers with the class `.tail`
  plus the edge kind's class, and `.hollow` for a hollow diamond or
  `.plus` for a circled plus.
- **Annotations:** the class given to `annotate` is added as it is. It
  shares the drawing's own names, so one such as `label` or `port`
  takes the default stylesheet's rules for those.
- **The overlay** holds `.selection` (a selected shape's dashed outline,
  or a halo over a selected edge), `.ghost` (the edge being drawn),
  `.marquee`, and `.target .ring` (where an edge can start or end).
  In either mode, it also holds a `g.expanded` for each cut text of
  the vertex under the pointer: the text in full, with its own class,
  over a `rect.backing`, above everything else in the drawing, unless
  `create` was given `expand: false`.
- **Around the drawing:** the binding makes `.graphedwright-host`,
  `.graphedwright-palette` with `.graphedwright-item` buttons
  (`.graphedwright-vertex` and `.graphedwright-edge`, the chosen one
  `.graphedwright-chosen`), `.graphedwright-drawing` and the
  `.graphedwright-label` input.

The default stylesheet sets an edge halo's colour but not its width,
which the renderer writes from the edge's own. A rule that sets
`stroke-width` on `.selection` would hide the halo.
