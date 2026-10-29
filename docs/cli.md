# The command line

`cmd/graphedwright` draws a document as SVG, or checks it against a
schema, with the built-in schemas (`basic`, `use-case`, `threat-model`,
`state-machine`).

From a clone of the repository:

```sh
go install ./cmd/graphedwright
```

## render

```sh
graphedwright render -f document.json [--layout [--flow top-to-bottom]] [--css] > graph.svg
```

Writes the drawing to standard output. A document without positions is
laid out as it is read.

| Flag | Meaning |
|---|---|
| `-f` | the document file; required |
| `--layout` | lay every vertex out afresh, placed or not |
| `--flow` | with `--layout`, the way the layers run: `left-to-right` or `top-to-bottom` |
| `--css` | embed the default stylesheet, so the SVG looks right on its own |

The pictures in these docs are made this way, from the demo's examples:

```sh
graphedwright render --css -f web/examples/dependencies.json > docs/images/dependencies.svg
```

## check

```sh
graphedwright check -f document.json [--schema use-case]
```

Reads the document and reports `ok: 12 vertices, 14 edges`, or the
first thing wrong with it, with where it is:

```text
document.vertices[2].kind: unknown vertex kind "blob" at byte 74
```

With `--schema`, the document must name that schema.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | done |
| 1 | the document was refused; the reason is on standard error |
| 2 | bad arguments; the usage is on standard error |
