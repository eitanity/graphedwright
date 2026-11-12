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
- **Indices** name elements in Go: `d.Vertex(v)`, `d.Edge(e)`. A
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
- **Compartments:** a rectangle or rounded rectangle kind can show
  attributes as compartments below its label, as a class shows its
  fields and methods. Each with a value is ruled off from what is above
  it, one line of text for each line of the value; an empty one is left
  out. The vertex grows to hold them, as wide as its widest line up to
  the kind's `MaxWidth`, beyond which a line is cut, and is never
  smaller than its own size. Its size follows from the document, so a
  layout places it at the size it is drawn; setting a compartment's
  attribute that changes the size marks the layout stale, and undo
  restores it.
- **Label formats:** a vertex or edge kind can build the label it draws
  from the label as set and the attributes. Text outside braces is
  drawn as it is; `{key}` is the label, for `label`, or the attribute's
  value; `{before|key}` and `{before|key|after}` add the text either
  side only when the value is set. So `{label}{ : |type}{ [|mult|]}`
  draws `wheels : Wheel [4]`, or `wheels` alone, and
  `{trigger}{ [|guard|]}{ / |effect}` draws `start [ready] / run()`.
  Editing a label in place edits the label as set; the layout makes
  room for the label as drawn. A schema refuses a format whose braces
  are unpaired or nested, with more than two bars in a placeholder, or
  naming a key the kind does not declare.
- **Lines:** a label's line breaks (a line feed, a carriage return, or
  the two together) start new lines, wherever the format's parts put
  them, as a compartment's do. The lines are drawn one under another,
  14 apart: a vertex's centred where its one line would be, at an
  actor's foot up from it with the figure shorter above them, and under
  a provided shape's box down from it. A container's title, and the
  label above a vertex's compartments, are given a line's more room
  for each line after the first, so the box grows when the label does.
  An edge's label stacks away from the stroke: above it the last line
  is the nearest, and beside it the lines are centred on its middle.
  The room a label takes is its widest line across and its lines
  down, wherever the layout, the label placer and the input measure
  it, and a line too wide for its room is cut on its own, the others
  drawn whole.
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
| `SetPorts` | puts a vertex's ports back to its kind's, dropping its own | `Vertex` |
| `AddPort` | gives a vertex a port of its own, after its others | `Vertex`, `Port` (`Name`, `Side`, `Max`) |
| `DeletePort` | removes a port of the vertex's own that no edge uses | `Vertex`, `Text` (the port's name) |
| `Layout` | records positions before a layout, so it can be undone | `Text: "all"` to lay out placed vertices too; `Value` a flow |
| `SetParent` | puts a vertex in a container, or takes it out with `To: gw.None` | `Vertex`, `To` |
| `Compact` | drops tombstones and renumbers; clears the undo log | |
| `HostChange` | a change to the host's own data, made by its `Store` | `Vertex`, or `Edge` with `Vertex: gw.None`, or neither for the document; `Token` |

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

### Host data

A host keeps data of its own on vertices, edges and the document: of
its own type, as large as it needs, in its own store. graphedwright
never reads it and never draws it, but it gives the data a place in
the undo log, so that "add a node with its settings" is one step.

The host gives the document a `gw.Store` with `SetStore`, before the log
holds anything. A change to the data is a `HostChange` command carrying
a `Token` the store issued; the store keeps behind the token whatever
the change needs, such as the value before and after. `Apply` calls the
store's `Do`, whose error refuses the command, and records the token.
`Undo` and `Redo` call the store's, in order with every other command
and within groups. When the log drops a token, because it fell off the
ring, a new command replaced what could be redone, or `Discard`,
`ClearUndo` or `Compact` emptied it, the store's `Forget` is called,
once for each token `Do` accepted.

The store keys by element ID, which deletion, its undo and `Compact`
keep, so it need not follow graphedwright's bookkeeping. `Compact`
gives its `Released` the IDs of the tombstones it removed.

While a store's method runs, the document refuses to change: `Apply`
returns `gw.ErrReentered` and the other methods do nothing, as under
`Lock`. A reaction that needs a command is applied after the call
returns. A store's methods must not panic, since TinyGo on wasm cannot
recover from one.

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
   the label or stereotype of an edge drawn in the gap between them
   (twice as wide for a right-angled edge, whose text is on the half
   past its turn), or where edges have far to go across the flow in the
   room between them (uprights, below), or, in either flow, for a label
   run (below), or, in a container, for the runs that come along a gap
   a label is drawn in (below). Within a layer, vertices are
   centred on their neighbours and kept a gap (24) apart; then each long
   edge's placeholders are brought into one row where they fit, so that
   it runs straight past the layers between its ends, and a placeholder
   next to an end is kept out of the spread of the ends there (below).
   Everything lands on a grid (8).
5. **Routes:** each long edge is routed through its placeholders as
   bends, and an edge that would cross a vertex beside one of its ends
   is routed around it (below).
6. **Detours:** in a document with containers, an edge whose route
   would still pass through a vertex or container that holds neither
   of its ends is given a detour round them (below).

Every pass is a loop with a stated bound, nothing recurses, and the
working arrays are allocated once, in `layout.New`. The same input gives
the same layout.

### Flow

A document's **flow** is the way its layers run: `LeftToRight` (the
default), or `TopToBottom`, as a dependency tree or an org chart is
drawn. It changes the layout and how curved and right-angled edges
leave and enter their vertices: right and left sides for left to right,
bottom and top for top to bottom. Edge labels sit above the line left
to right and beside it top to bottom, at the middle of the edge, or of
a right-angled edge's last leg, past its turn. Top to bottom, a label
on a run through the layers, straight along the flow and long enough
for it, is turned to read up along the run, so that it takes a line's
width across the flow rather than its length.

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
  strokes, as for «include»), `HollowHead` (as for generalisation),
  `TypedHead` (hollow with two dots, as for typing) or `BarHead`
  (hollow with a bar, as for redefinition).
- **Tails:** `Tail` draws a decoration where the edge leaves its
  source: `FilledDiamond` (composition), `HollowDiamond` (aggregation)
  or `CircledPlus` (containment). Edges that leave the same side of a
  vertex share their start, and so their tails.
- **Styles:** `Dashed`, a `Stereotype` drawn on every edge of the kind
  (such as `«include»`), and `Weighted`, which makes the stroke width
  follow the edge's weight.

**Ports.** A vertex kind can have ports, named places on its sides where
edges attach; a port can limit how many edges it takes. An edge to a
vertex whose kind has ports must name one. A vertex can also have ports
of its own, as a part has its own ports or an action its parameters:
`AddPort` adds one after the kind's, and `DeletePort` removes one no
edge uses, naming the edges that do. An edge may name one of them, or
join the vertex at its outline. A vertex's own ports are named beside
them, outside the shape, on the side of the port away from where its
edges go, and the vertex grows so that the labels on a side clear each
other: ports on a left or right side `PortGap` (32) apart, and on the
top or bottom far enough apart for each label either side of its port.
A container's ports sit on its border as it is fitted. A vertex without
ports is joined at
its outline: a straight edge leaves where the line towards the other
end crosses the shape (the box, the diamond's sides, the ellipse), so
shapes can sit at any angle to each other, as in use case and data flow
diagrams.

**Bends.** A long edge is routed around the layers it crosses. At each
layer it runs past the layer's widest node, clear of the vertices
there, and changes height only in the room between layers. Its points
are the edge's `Bends`.

A layer is as wide as its widest node, a container say, so an edge's
end narrower than that leaves or enters the layer some way in. Where
the edge would pass another node of that layer on its way, such as a
container above or below the end, it runs level from the end to the
layer's edge instead, at the end's height, which no other node of the
layer shares, and turns only in the room between layers.

An edge into or out of a container can still meet what the container
holds, or what stands beside it. Once everything is placed, such an
edge is given a **detour**: a route of level and upright legs, a gap
(16) clear of every vertex and container that holds neither end, which
crosses the sides of the containers holding its ends but does not run
along them. It turns as little as it can and keeps to the way the edge
runs, turning back only where its ends overlap along the flow. Edges
between the same two vertices are given detours together, each a gap
further out, so that they stay apart. Documents without containers are
left as their layers route them.

Bends are derived from the positions, not edited. A route holds for
where the edge's two ends were when it was routed. Move or resize an
end and the edge is drawn straight; put the end back, by an undo or a
redone layout, and the bends return. Moving a container carries the
routes of the edges inside it. Only the latest layout's routes are kept.

**Lanes.** Edges between the same two vertices, either way, without
bends or ports, would be drawn on top of each other. Each runs instead
in a lane of its own through a point midway, a label's height from the
next, so that their labels stack (`Document.Parallel`). A lane's texts
go on its outer side, away from the middle, so that of two edges
neither's label crosses the other.

**Spread ends.** A curved or right-angled edge arrives level, so two
drawn to one side of a vertex at its middle would lie on each other
there. Their ends are spread along the side instead, ordered across the
flow by where their routes head from it, up to 20 apart (`EndGap`) and
closer where the side is short, each on the shape's outline
(`Document.EndSlot`, `Document.EndOffset`). Where a route heads is its
first bend beyond the spread, or where it goes into a container holding
its other end, or its other end for a route without one, so the edges
leaving a side do not cross each other on the way out. The layout
brings the bends next to each end level with it once every route is
found, and keeps the placeholder next to an end out of the spread there
unless it stands at the middle, where the route runs straight on. A straight edge, drawn
toward the centre, and an edge with a port keep their ends, and past 16
on a side (`MaxEndSlots`) every end stays at the middle. A lone edge
with a spread end puts its texts on the side away from the others.

**Uprights.** A curved or right-angled leg between an end and the
placeholder next to it, or an edge between adjacent layers, may have
far to go across the flow in the room between two layers. Drawn as it
arrives, level, such a leg is all but flat, and edges in lanes beside
it lie on it. A leg that goes more than four times as far across as
the room is deep is given an **upright** instead. A leg into a container
is measured to where it goes on to inside it: the end it is for, or,
where other members stand in the way, the side of the container its run
inside goes round them by. The upright is a level run, a run
straight across the flow, and a level run on, ending level with its end
where it is drawn, or with the first point of its run into a
container. The uprights in one gap stand 8 apart, nested so that they
do not cross, and the gap is deepened to hold them and an arrowhead.
Where one leg leaves a layer level with where another reaches the next,
the one leaving has the upright nearer the layer it leaves, so that the
two do not run along each other there.
A straight edge keeps its legs, and a level with a placed vertex keeps
its layers where they are. A label goes on the run past an upright,
toward the middle of the route, rather than on the upright.

The placeholders of a labelled edge are as long along the flow as its
label, so that the run through one holds it, off the gaps either side
where the next layers' labels are: left to right, the label is drawn
above the run; top to bottom, it is turned to read along it, a line
beside it that the gap between placeholders holds. A label of more
lines makes its placeholders as wide across the flow as its lines
after the first, either side, so the neighbouring runs stand clear of
them.

A labelled edge between adjacent layers whose ends are further apart
across the flow than a line of text would climb across its label. It
is given a **label run** instead: it runs level out of the end at the
side with fewer ends (the source where they have as many), at the
height that end is drawn at, for at least as long as its label, and
climbs to the other end in the last 64 of the gap, which is made as wide
as that. The label lies on the run. Every such run out of one side climbs
in the same stretch, beside the far layer, so the runs and the edges
from that side keep their order. An edge with an end placed, or at a
port, or a straight edge, has no run.

**Label places.** A label at the middle of its route can still cross
another edge, another edge's label or a vertex: edges that fan out of
one vertex run close together near it, and edges that share a short
gap share its middle. After a layout, such a label is moved along its
own route to the nearest place where its text crosses nothing, toward
the target first, on its own side of the line and then the other, or
where it crosses fewest, and keeps that place
(`Document.SetLabelAt`, `svg.PlaceLabels`) while its ends stay where
they are, as its bends do. A lane's label and a right-angled edge's on
its own last leg are not moved, and nor is any label of a document with
more than 128 edges, where the search would take too long.

A layout makes room for a name drawn under a figure, an actor's, which
is wider than the figure. A straight edge into a member of a container
comes in from the side, along the free band and across the flow at
the member's middle, where the members of its layer leave that clear;
otherwise it comes along the gap before the member's layer, level with
its end. A gap in a container that a labelled edge crosses, its label
at the middle, is 16 wider on each side, and a run along it keeps to
that room beside the layer of the member it goes to, off the label.
Runs from either side of the container then pass each other along the
band above or below what it holds, so one from the far side keeps 8
further out; the container's sides across the flow stand 16 further
out than its padding for these runs, so that both keep the padding's
width off the side. Where a band has no room for the second, the runs
from both sides keep five eighths of the way across the gap from their
members' layers instead, off each other.

## Containers

A vertex of a **container** kind holds other vertices: the system
boundary of a use case diagram, the trust boundaries of a threat model,
a group. A vertex is in at most one container, its `Parent`, and
containers nest.

- **Putting a vertex in or out:** `SetParent` does it, and is refused
  for a container that doesn't take the vertex's kind (`Contains`), or
  for a vertex going inside itself or something it holds.
- **Fitting:** a container that holds anything is fitted around it,
  with 16 units of padding and 20 more at the top for its title, and 14
  more for each line of the title after its first. Its
  sides across the flow stand 16 further out (`BandRoom`) when an edge
  from outside runs to a member that another stands wholly before or
  after along the flow, so that the run round that one, along the band
  beside what the container holds, keeps clear of the side. It is
  fitted again after every command that moves, resizes, deletes or
  re-parents a vertex, or relabels one.
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
