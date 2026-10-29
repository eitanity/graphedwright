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

# TinyGo: under 600 KB
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
- **Shapes:** each is a `g.vertex` with its kind's name as a class
  (spaces as `_`, so `use case` is `use_case`), its ID as `id`, and its
  attributes as `data-*`. Inside are `.shape`, `text.label`,
  `.annotation`, and `.port` circles. Extra classes are `.container`,
  `.dashed`, and `.unplaced` for a vertex no layout or user has placed.
- **Edges:** each is a `g.edge` with its kind's class, `id`,
  `data-from`, `data-to` and its attributes. Inside are the `path`, and
  `text.label` and `text.stereotype`. A dashed kind has `.dashed`.
- **Arrowheads** are markers with the class `.arrowhead` plus the edge
  kind's name, and `.open` or `.hollow`.
- **The overlay** holds `.selection` (a selected shape's dashed outline,
  or a halo over a selected edge), `.ghost` (the edge being drawn),
  `.marquee`, and `.target .ring` (where an edge can start or end).
- **Around the drawing:** the binding makes `.graphedwright-host`,
  `.graphedwright-palette` with `.graphedwright-item` buttons
  (`.graphedwright-vertex` and `.graphedwright-edge`, the chosen one
  `.graphedwright-chosen`), `.graphedwright-drawing` and the
  `.graphedwright-label` input.

The default stylesheet sets an edge halo's colour but not its width,
which the renderer writes from the edge's own. A rule that sets
`stroke-width` on `.selection` would hide the halo.
