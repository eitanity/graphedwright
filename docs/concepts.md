# Concepts

This guide explains the ideas graphedwright is built on. The
[tutorial](tutorial.md) shows them at work; the [Go API guide](go.md)
and the [browser guide](browser.md) give the details of each call.

## The document

A **document** is a directed graph: vertices joined by edges, each of a
kind that its **schema** defines, with positions for drawing and a
viewport.

- **Vertices** have an ID, a kind, a label, a position (their centre)
  and size, the kind's ports, attributes, and whether they were
  **placed** (by a user or a host) or left for the layout.
- **Edges** have an ID, a kind, the vertices they join (`From`, `To`)
  and the ports they use at each end, a label, attributes, a weight
  (which can drive the stroke width) and the bends a layout routed them
  through.
- **IDs** are strings. A host can give its own; otherwise the document
  mints `v1, v2, …` and `e1, e2, …`. An ID is never reused within a
  document, so an undo can bring an element back as it was.
- **Indices** name elements in Go: `d.Vertices[v]`, `d.Edges[e]`. A
  deleted element stays in place as a tombstone until `Compact`, which
  renumbers and returns the map from old indices to new.

### Capacities

A document is made with **capacities** (`gw.Caps`): how many vertices,
edges, ports per vertex, attributes per element, bytes per label, undo
steps, layouts the undo can reach back across, and bend points. Zero
takes the default:

| Capacity | Default |
|---|---|
| `Vertices` | 4,096 |
| `Edges` | 16,384 |
| `PortsPerVertex` | 8 |
| `AttrsPerElement` | 16 |
| `LabelBytes` | 256 |
| `UndoDepth` | 256 commands |
| `UndoLayouts` | 8 |
| `BendsPerEdge` | 16, shared among the edges |

Everything the document will ever hold is allocated when it is made, so
editing never allocates and never grows. A command that would pass a
capacity is refused, with an error that names the capacity. This is
what lets the same code run in TinyGo and keeps the browser editor free
of garbage-collection pauses.

## Schemas

A schema says what a document may contain: its vertex kinds and edge
kinds, how each is drawn, the ports and attributes each has, and the
rules a document must keep. The editor's palette is the schema's kinds.
[Schemas](schemas.md) describes every option, and the four schemas
built in: `basic`, `use-case`, `threat-model` and `state-machine`.

The rules a schema can set:

- **Which kinds an edge joins:** each edge kind can list the vertex
  kinds allowed at each end.
- **Root and leaf kinds:** a root kind takes no incoming edge, and a
  leaf kind no outgoing one.
- **Limits:** a vertex kind can set the most vertices of that kind a
  document holds (`Max`), and a port the most edges it takes.
- **Attributes:** each attribute is free text, a number, or one of a
  list of choices. It can be required, which needs a default.
- **No cycles:** `Acyclic` refuses an edge that would close a cycle,
  naming it: `edge would make a cycle: log → app → cli → log`.
- **Reachability:** `Connected` asks that every vertex be reachable from
  a root kind. This one is a warning, not a refusal:
  `Document.Unreachable` lists the vertices that are not reachable.

## Commands and undo

Every change to a document is a **command** passed to `Apply`. The
editor, a host's own code and the tests all change documents the same
way, and undo is a log of commands.

| Op | What it does | Fields |
|---|---|---|
| `AddVertex` | adds a vertex | `Kind`, `ID` (minted if empty), `Text` (the label), `Pos`, `Size` (the kind's if zero) |
| `DeleteVertex` | deletes a vertex, its edges, and what it holds if it is a container | `Vertex` |
| `AddEdge` | adds an edge | `Kind`, `ID`, `From`, `To`, `FromPort`, `ToPort` (`gw.None` without ports), `Text`, `Weight` |
| `DeleteEdge` | deletes an edge | `Edge` |
| `Move` | moves a vertex, which becomes placed; a container carries what it holds | `Vertex`, `Pos` |
| `Resize` | resizes a vertex; a container's size becomes its own size | `Vertex`, `Size` |
| `Relabel` | sets a label | `Vertex`, or `Edge` with `Vertex: gw.None`; `Text` |
| `SetAttr` | sets an attribute; an empty value removes it | `Vertex` or `Edge` as for `Relabel`; `Key`, `Value` |
| `SetPorts` | puts a vertex's ports back to its kind's | `Vertex` |
| `Layout` | records positions before a layout, so it can be undone | `Text: "all"` to lay out placed vertices too; `Value` a flow |
| `SetParent` | puts a vertex in a container, or takes it out with `To: gw.None` | `Vertex`, `To` |
| `Compact` | drops tombstones and renumbers; clears the undo log | |

`Apply` either applies the command and records how to undo it, or
refuses it with an error that names the rule, and leaves the document
exactly as it was. Some examples:

```text
edge kind "include" cannot join "actor" to "use case"
vertex "s" of kind "source" takes no incoming edge
port "out" of vertex "a" is full (2 of 2)
attribute "p" of vertex "v1" must be a number: "x"
vertex limit 4096 reached
```

`Apply` returns a **`Change`** naming the vertices and edges the command
touched, and whether the layout is now stale, so a renderer can redraw
only what changed. Its slices are the document's scratch space and are
valid until the next `Apply`, `Undo` or `Redo`: copy out what you need
at once.

`Undo` and `Redo` walk the log. It is a ring of `UndoDepth` commands,
and the oldest fall off. A new command clears what could be redone.
The commands between `Group` and `EndGroup` are one step to undo and
redo: the editor groups the moves of a drag or a nudge, and the
commands of a delete, so one Ctrl+Z takes back the whole action.
A layout can be undone too: the positions before and after each of the
last `UndoLayouts` layouts are kept. `ClearUndo` empties the log for a
host whose setup should not be undoable, and a document read from JSON
starts with an empty log.

## Positions and layout

A vertex's position comes from one of two places. Either it was
**placed**, by a user dragging it, a host's `Move`, or a document that
says `"placed": true`; or the **layout** gave it one. The layout leaves
placed vertices where they are and lays the rest out around them.
`Layout` with `"all"` lays out everything.

The layout (`layout.Layouter`) is layered:

1. **Layering:** each vertex is put one layer past the furthest vertex
   with an edge into it, roots first. A document with cycles is laid
   out as if a few edges were reversed; `Result.Reversed` lists them so
   a host can mark them.
2. **Long edges:** an edge that spans layers gets a placeholder in each
   layer it crosses, within a budget of 8 per edge capacity.
3. **Order:** each layer's order is chosen to cross few edges, sweeping
   down and up (8 times by default) and keeping the best order seen.
4. **Coordinates:** layers are a pitch apart (160 units, or 96 top to
   bottom), or further where a wide node needs it, or, left to right,
   the label of an edge drawn in the gap between them. Within a layer,
   vertices are centred on their neighbours and kept a gap (24) apart.
   Everything lands on a grid (8).
5. **Routes:** each long edge is routed through its placeholders as
   bends (below).

Every pass is a loop with a stated bound, nothing recurses, and the
working arrays are allocated once, in `layout.New`. The same input gives
the same layout.

### Flow

A document's **flow** is the way its layers run: `LeftToRight` (the
default), or `TopToBottom`, as a dependency tree or an org chart is
drawn. It changes the layout and how curved and right-angled edges
leave and enter their vertices: right and left sides for left to right,
bottom and top for top to bottom. Edge labels sit above the line left
to right and beside it top to bottom.

The flow changes only through a `Layout` command whose `Value` names
it, `"left-to-right"` or `"top-to-bottom"`, so undoing the layout puts
the old flow back. It is saved in the document.

## Edges

Each edge kind has a **route**, an arrowhead and a few styles.

- **Routes:** `Straight` is a line, `Curve` a curve that leaves and
  arrives along the flow, and `Orthogonal` right angles with the turn
  halfway along. An edge back against the flow leaves the near side of
  its vertex and arrives at the far side.
- **Heads:** with `Arrow`, the head is `FilledHead`, `OpenHead` (two
  strokes, as for «include») or `HollowHead` (as for generalisation).
- **Styles:** `Dashed`, a `Stereotype` drawn on every edge of the kind
  (such as `«include»`), and `Weighted`, which makes the stroke width
  follow the edge's weight.

**Ports.** A vertex kind can have ports, named places on its sides where
edges attach; a port can limit how many edges it takes. An edge to a
vertex with ports must name one. A vertex without ports is joined at
its outline: a straight edge leaves where the line towards the other
end crosses the shape (the box, the diamond's sides, the ellipse), so
shapes can sit at any angle to each other, as in use case and data flow
diagrams.

**Bends.** A long edge is routed around the layers it crosses. At each
layer it runs past the layer's widest node, clear of the vertices
there, and changes height only in the room between layers. Its points
are the edge's `Bends`.

Bends are derived from the positions, not edited. A route holds for
where the edge's two ends were when it was routed. Move or resize an
end and the edge is drawn straight; put the end back, by an undo or a
redone layout, and the bends return. Moving a container carries the
routes of the edges inside it. Only the latest layout's routes are kept.

**Lanes.** Edges between the same two vertices, either way, without
bends or ports, would be drawn on top of each other. Each runs instead
in a lane of its own through a point midway, a label's height from the
next, so that their labels stack (`Document.Parallel`).

## Containers

A vertex of a **container** kind holds other vertices: the system
boundary of a use case diagram, the trust boundaries of a threat model,
a group. A vertex is in at most one container, its `Parent`, and
containers nest.

- **Putting a vertex in or out:** `SetParent` does it, and is refused
  for a container that doesn't take the vertex's kind (`Contains`), or
  for a vertex going inside itself or something it holds.
- **Fitting:** a container that holds anything is fitted around it,
  with 16 units of padding and 20 more at the top for its title. It is
  fitted again after every command that moves, resizes, deletes or
  re-parents a vertex.
- **Its own size:** a container that holds nothing is its own size: the
  size it was given when added or last resized (`Vertex.Min`), or its
  kind's when it was given none. A container that holds something is
  at least its own size, grown right and down from the corner the fit
  gives it. Taking out the last thing it holds, by any command or by
  undo, brings it back to its own size with its top left corner kept.
- **Moving and deleting:** moving a container moves what it holds, and
  deleting it deletes what it holds. One undo brings either back.
- **Layout:** each container's contents are laid out first, as a graph
  of their own, and the container is then one block at its own level.
- **Editing:** in the editor, dropping a vertex in a container's box
  puts it in, dragging it out takes it out, and dragging the container
  carries its contents. Dragging the handle on a selected container's
  corner resizes it, which gives it its own size.

## Drawing and hit testing

`svg.Render` writes a whole document as SVG text. The output is the
same, byte for byte, on every platform and with either toolchain, so
drawings can be compared in tests. Containers are drawn first, then
edges, then vertices, then an overlay for the selection and the
editor's feedback. Containers are drawn in the order
`Document.Containers` lists them: each before what it holds, and each
with all it holds before the next one beside it, so two containers
that overlap are drawn one wholly over the other, with what each holds,
never interleaved by depth.

`svg.HitTest` names what was drawn at a point, from the same geometry:
- **Ports:** a port's circle comes first.
- **Shapes:** next comes a vertex's outline band, where dragging starts
  an edge, and then its inside, where dragging moves it.
- **Edges:** an edge's path counts within a small tolerance, and so
  does its label's text.
- **Containers:** the container drawn uppermost whose box holds the
  point, which of nested ones is the innermost.
- **Background:** if none of the above.

The editor uses the hit test instead of the browser's, so pointer
behaviour is the same everywhere and can be tested without a browser.

## The editor

`edit.Editor` turns input events (pointer, wheel, keys, palette, text)
into commands on the document. Everything it does is a command, so undo
covers it, and a test can drive it with events and check the result
without any browser. The [browser guide](browser.md#using-the-editor)
lists what each gesture does.
