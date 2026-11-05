# Document format

A document is saved as JSON. The [`doc`](go.md#doc) package reads and
writes it in Go; the browser build's `load` and `export` take and
return the same shape as a string or an object.

```json
{
  "schema": "basic",
  "flow": "top-to-bottom",
  "vertices": [
    {"id": "app", "kind": "box", "label": "app", "x": 160, "y": 0, "placed": true, "attrs": {"owner": "web team"}},
    {"id": "db", "kind": "box", "label": "db", "x": 160, "y": 96, "parent": "back"},
    {"id": "back", "kind": "group", "label": "back end", "x": 160, "y": 96}
  ],
  "edges": [
    {"id": "e1", "kind": "arrow", "from": "app", "to": "db", "label": "SQL"}
  ],
  "view": {"x": 0, "y": 0, "w": 600, "h": 400}
}
```

## The document

| Key | Meaning |
|---|---|
| `schema` | **required, first.** The schema's name. The reader must supply a schema by that name, and the document is checked against it. |
| `flow` | `"left-to-right"` (the default, not written) or `"top-to-bottom"`. |
| `vertices` | the vertices, in order. |
| `edges` | the edges, in order, after the vertices. |
| `view` | the viewport: `x`, `y`, `w`, `h` in document units, the corner within 10⁹ of the origin and the width and height not negative. Zero width or height means "fit the drawing". |
| `data` | the host's data on the document: one JSON value ([Host data](#host-data)). |

## A vertex

| Key | Meaning |
|---|---|
| `kind` | **required.** The vertex kind's name. |
| `id` | the vertex's ID; minted (`v1`, `v2`, …) when missing. Must be unique among vertices and edges. |
| `label` | the label text. |
| `x`, `y` | the position of its centre, within 10⁹ of the origin. A document in which any vertex has no position is laid out when it is read. |
| `w`, `h` | its size, when not the kind's: both positive, at most 10⁹. A container's are its own size: what it is when it holds nothing, and least when it holds something; a container fitted to what it holds writes none. |
| `placed` | `true` when a user or a host placed it: a layout leaves it where it is. |
| `parent` | the ID of the container it is in. The container may come later in the list. A container that holds anything is fitted around it when read, whatever its own position says, at least its own size. |
| `attrs` | its attributes, an object of strings, keyed by lowercase letters, digits, `-` and `_` (not `kind`, `from` or `to`). |
| `ports` | its own ports, after its kind's: an array of `{"name", "side", "max"}`, with `side` one of `"left"`, `"right"`, `"top"` and `"bottom"`, and `max`, the most edges it takes, left out for any. A vertex with ports of its own writes its own size in `w` and `h`, as a container does. |
| `data` | the host's data on the vertex: one JSON value. |

## An edge

| Key | Meaning |
|---|---|
| `kind`, `from`, `to` | **required.** The edge kind's name and the IDs of the vertices it joins. |
| `id` | the edge's ID; minted (`e1`, `e2`, …) when missing. |
| `from_port`, `to_port` | the names of the ports it uses, required at an end whose vertex's kind has ports. |
| `label` | the label text. |
| `weight` | a number that drives the stroke width when the kind is weighted: not negative, at most 10⁹. |
| `bends` | the points a layout routed it through, `[[x, y], …]`. They hold for the positions of its ends as read. |
| `attrs` | its attributes, as for a vertex. |
| `data` | the host's data on the edge: one JSON value. |

## Reading

Every element goes through the same commands an editor uses, so a
document that reads is one the schema allows and that fits the
capacities. The reader is strict and bounded:

- **Refusals name the place, the rule and the byte:**
  `document.vertices[2].kind: unknown vertex kind "blob" at byte 74`.
- **Unknown keys are refused,** with their path.
- **Text XML can carry:** strings must be valid UTF-8. Labels and
  attribute values may hold tabs and line breaks but no other control
  characters, and IDs none at all, whether written raw or as `\u`
  escapes. Attribute keys hold only lowercase letters, digits, `-` and
  `_`, and `kind`, `from` and `to` are reserved for the drawing. The same rules hold for edits made through the Go and
  JavaScript APIs, and for a schema's names and text.
- **Sizes are checked before anything is allocated.** A file over 16 MB
  is refused unread, and strings, attribute counts and bend counts are
  checked against the document's capacities.
- **No input can make the reader panic, recurse, or go past a capacity.**
  It is fuzzed to keep it that way.
- **Positions:** a document without them is laid out on load. Turn that
  off with `doc.Options{NoLayout: true}`.
- **Undo:** a document starts with an empty undo log. The commands that
  built it can't be undone.

## Writing

`doc.Encode` writes the live content in index order, so the same
document always gives the same text. It writes a vertex's size only
when it differs from its kind's, and a flow only when it is not the
default. Numbers are written in the shortest form that reads back the
same.

## Host data

A host's own data ([Concepts](concepts.md#host-data)) is written and
read by its `doc.Codec`, keyed by element ID, `""` for the document's:

```go
err := doc.EncodeWith(d, name, doc.Options{Codec: c}, w)
d, err := doc.Decode(data, doc.Options{Schemas: schemas, Codec: c})
```

- **Writing:** `EncodeWith` asks the codec's `Encode(id, w)` for each
  live element and the document, and writes what it gives as the
  element's `data`. The codec writes through a writer that refuses
  more than `Options.DataBytes` (64 KiB by default, at most 16 MB), and
  what it writes must be one well-formed JSON value; otherwise the
  encode fails, naming the element. The document is held while the
  codec runs, so a call back into it is refused. `Encode` writes no
  data, and the text is only as deterministic as what the codec writes.
- **Reading:** a `data` member must be well-formed JSON, nested no more
  than 64 deep and no longer than `DataBytes`, or the document is
  refused, naming where but not quoting the data. Each member is handed
  to the codec's `Decode(id, raw)` once the whole document is read and
  built, so never for a document that is then refused. A document with
  `data` and no codec is refused rather than read without it.
