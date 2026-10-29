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

## An edge

| Key | Meaning |
|---|---|
| `kind`, `from`, `to` | **required.** The edge kind's name and the IDs of the vertices it joins. |
| `id` | the edge's ID; minted (`e1`, `e2`, …) when missing. |
| `from_port`, `to_port` | the names of the ports it uses, required at an end whose vertex has ports. |
| `label` | the label text. |
| `weight` | a number that drives the stroke width when the kind is weighted: not negative, at most 10⁹. |
| `bends` | the points a layout routed it through, `[[x, y], …]`. They hold for the positions of its ends as read. |
| `attrs` | its attributes, as for a vertex. |

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
