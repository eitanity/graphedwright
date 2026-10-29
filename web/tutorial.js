"use strict";

// 1. A schema described in JavaScript: packages with a version and a
//    licence, joined by dependencies and dev dependencies; no cycles.
const deps = {
  name: "deps",
  acyclic: true,
  vertexKinds: [
    { name: "package", shape: "round", w: 120, h: 36, attrs: [
      { key: "version", kind: "text" },
      { key: "licence", kind: "choice", choices: ["MIT", "Apache-2.0", "BSD-3-Clause"], default: "MIT" },
    ] },
  ],
  edgeKinds: [
    { name: "depends on", arrow: true },
    { name: "dev dependency", arrow: true, dashed: true },
  ],
};

// 2. A document: packages by id, dependencies by the ids they join, no
//    positions, so that it is laid out when loaded, top to bottom.
const tree = {
  schema: "deps",
  flow: "top-to-bottom",
  vertices: ["app", "cli", "server", "router", "db", "log", "testkit"].map((id) => ({ id, kind: "package", label: id })),
  edges: [
    ["app", "cli"], ["app", "server"], ["server", "router"], ["server", "db"],
    ["cli", "log"], ["router", "log"], ["db", "log"],
  ].map(([from, to]) => ({ kind: "depends on", from, to }))
    .concat([{ kind: "dev dependency", from: "app", to: "testkit" }]),
};

// Every call returns {ok: value} or {error: text}.
function ok(res) {
  document.getElementById("error").textContent = res.error || "";
  return res.error ? null : res.ok;
}

// 4. The properties panel: the selected element's label and attributes,
//    each field a command, so the schema checks it and undo takes it back.
function showProperties(g) {
  const box = document.getElementById("props");
  const p = ok(graphedwright.properties(g));
  if (!p) { box.innerHTML = "<p>Select a package or a dependency.</p>"; return; }
  box.replaceChildren();
  const heading = document.createElement("h3");
  heading.textContent = p.kind + " " + p.id;
  box.appendChild(heading);
  const field = (name, value, choices, commit) => {
    const label = document.createElement("label");
    label.textContent = name;
    const el = choices ? document.createElement("select") : document.createElement("input");
    for (const c of choices || []) el.appendChild(new Option(c, c));
    el.value = value;
    el.onchange = () => commit(el.value);
    el.onkeydown = (ev) => ev.stopPropagation(); // typing here is not the drawing's
    box.append(label, el);
  };
  field("label", p.label, null, (v) => ok(graphedwright.setLabel(g, v)));
  for (const a of p.attrs) {
    field(a.key, a.value, a.kind === "choice" ? a.choices : null, (v) => ok(graphedwright.setAttr(g, a.key, v)));
  }
}

const go = new Go();
WebAssembly.instantiateStreaming(fetch("graphedwright.wasm"), go.importObject).then(({ instance }) => {
  go.run(instance);

  // 3. Mount an editor with the schema, and load the document into it.
  const g = ok(graphedwright.create(document.getElementById("graph"), { schema: deps, mode: "edit" }));
  ok(graphedwright.load(g, tree));

  graphedwright.on(g, "select", () => showProperties(g));
  graphedwright.on(g, "change", () => showProperties(g));
  graphedwright.on(g, "error", (msg) => { document.getElementById("error").textContent = msg; });
  document.getElementById("layout").onclick = () => ok(graphedwright.layout(g, { all: true, flow: "top-to-bottom" }));
  document.getElementById("undo").onclick = () => graphedwright.undo(g);
  document.getElementById("redo").onclick = () => graphedwright.redo(g);
});
