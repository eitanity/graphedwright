# Upgrading to v0.2.0

v0.2.0 makes the document, its schema and the editor's state reachable
only through their methods. In v0.1, any holder of a `*gw.Document`
could write its fields and slices directly, past the schema check, the
undo log, the change report and the lock. Now every read is a value or
a read-only list, and every change is a command or a method that checks
it. The build tells you each place to change; this page says what each
becomes.

## The schema

A schema is compiled once, and documents share what it compiles to.

| v0.1 | v0.2 |
|---|---|
| `d, err := gw.New(schema, caps)` | `kinds, err := gw.Compile(schema)`, then `d, err := gw.New(kinds, caps)` |
| `doc.Named{Name: n, Schema: s}` | `doc.Named{Name: n, Kinds: kinds}` |
| `dom.Register(name, schema)` | pass `doc.Named` values to `dom.Install` or `dom.Main` |
| `d.Schema` | `d.Kinds()` |
| `d.Schema.VertexKinds[k].Size` | `d.Kinds().Vertex(k).Size()` |
| `d.Schema.EdgeKinds[k].Route` | `d.Kinds().Edge(k).Route()` |
| `len(d.Schema.VertexKinds)` | `d.Kinds().VertexKinds()` |
| `d.Schema.Acyclic`, `.Connected` | `d.Kinds().Acyclic()`, `.Connected()` |
| a loop over the kinds for a name | `d.Kinds().VertexNamed(name)`, `EdgeNamed(name)` |
| `kind.Ports`, `kind.Attrs` | `kind.Ports()`, `kind.Attrs()`, read-only lists |
| `kind.Contains`, `kind.Compartments` | `kind.Contains()`, `kind.Compartments()` |
| `kind.From`, `kind.To` of an edge kind | `kind.From()`, `kind.To()`; `kind.Allows(from, to)` checks both |
| `spec.Choices` | `spec.Choices()` |
| `gw.TitleRoom(&kind)` | `gw.TitleRoom(kind)`, with the kind's view |

`gw.Compile` does the checks `gw.New` did. `gw.New` refuses nil kinds,
and kinds with more ports or attributes than the capacities allow. In
the browser, `registerSchema` now checks a schema when it is given and
refuses an inconsistent one, which is then not known.

## Vertices and edges

| v0.1 | v0.2 |
|---|---|
| `d.Vertices[v].Label` | `d.Vertex(v).Label()` |
| `d.Edges[e].From` | `d.Edge(e).From()` |
| `&d.Vertices[v]` passed to a helper | `d.Vertex(v)`, a `gw.VertexView` |
| `len(d.Vertices)`, `len(d.Edges)` | `d.VertexSlots()`, `d.EdgeSlots()` |
| `for i := range d.Vertices` | `for i := range d.VertexSlots()`, with `gw.VertexIndex(i)` |
| `v.Ports`, `v.Attrs` | `v.Ports()`, `v.Attrs()` |
| `e.Bends`, `e.Attrs` | `e.Bends()`, `e.Attrs()` |
| `gw.AttrValue(v.Attrs, key)` | `v.Attrs().Value(key)` |
| `gw.Vertex`, `gw.Edge` | gone: read through the views |

Every field of a vertex and an edge is a method of its view. Out of
range, a view's reads are the zero value, or `None` for an index.

## Lists

The lists a vertex, an edge or a kind holds are read-only values:
`Len()` and `At(i)`, and `Value(key)` for attributes. They copy nothing
and give no way to change the document.

| v0.1 | v0.2 |
|---|---|
| `len(list)` | `list.Len()` |
| `list[i]` | `list.At(i)`, the zero value out of range |
| `for _, x := range list` | `for i := range list.Len() { x := list.At(i) … }` |
| `v.Ports[n:]` | `v.Ports().Rest(n)` |
| a slice of your own, for `gw.LabelRunes`, `NextLabelPart` or `AppendLabel` | `gw.AttrsOf(slice)`; and `gw.PortsOf`, `gw.PointsOf` |

## Changing a document

A write to a field no longer compiles. Make the change with a command
through `Apply`, or with the methods that change a document outside the
command log, each of which checks its parameters and the lock:

| v0.1 | v0.2 |
|---|---|
| `d.View = v` | `d.SetView(v)`; read it with `d.View()` |
| `d.Flow = f` | `d.SetFlow(f)`; read it with `d.Flow()` |
| `b := d.ReserveBends(e, n)`, then `b[i] = p` and `d.SetBends(e, b)` | `w := d.ReserveBends(e, n)`, then `w.Set(i, p)` and `w.Keep(n)` |
| `d.Containers()` as a slice | a `gw.VertexList`: `Len()` and `At(i)` |
| `vmap, emap := d.Compact()`, then `vmap[v]` | `vmap.At(int(v))`, `None` for a vertex dropped |

`SetView` refuses a viewport whose corner is beyond `gw.MaxCoord` of the
origin, or whose width or height is negative or above it. A
`gw.BendWriter` whose reservation is no longer the edge's own, after
`ClearBends`, another reservation or `Compact`, writes nothing.

## The editor and the renderer

| v0.1 | v0.2 |
|---|---|
| `sv, se := e.Selected()` | `sv, se := e.Selected(dstV, dstE)`, into buffers of your own |
| `svg.Options{SelectedV: …, SelectedE: …}` | `svg.Options{Selection: svg.Selected{V: …, E: …}}`; `e.Overlay(&opts)` sets the editor as the selection |
| `gesture.ActRelabel` | `gesture.ActCommitText`, which relabels or sets the attribute the kind edits |

`dom.Host.Selected` is given copies, valid only for the call.

## Added

- **A host written in Go:** `dom.Mount` puts an editor in an element and
  reports to a `dom.Host` ([browser guide](browser.md#a-host-written-in-go)).
- **Follow-ups:** a command a host's `Changed` returns is applied in the
  same undo step, and an edit and its follow-ups stand or fall together.
- **Attributes edited on the drawing:** a kind's `EditAttr` names the
  attribute its label input edits ([Schemas](schemas.md)).
- **Number bounds:** an `AttrSpec`'s `Bounded`, `Min` and `Max`.
- **`Document.Lock` and `Discard`.**
