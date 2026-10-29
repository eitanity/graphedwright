// The demo page: loads the browser build, mounts an editor in #host,
// and wires the toolbar to the graphedwright object.
"use strict";

const $ = (id) => document.getElementById(id);
const toolchain = new URLSearchParams(location.search).get("toolchain") === "tinygo" ? "tinygo" : "go";
$("toolchain").textContent = toolchain + " toolchain";

const EXAMPLES = [
  { name: "diamond (basic)", file: "examples/diamond.json" },
  { name: "shapes and routes (basic)", file: "examples/shapes.json" },
  { name: "online shop (use case)", file: "examples/use-case.json" },
  { name: "orders service (threat model)", file: "examples/threat-model.json" },
  { name: "editor gestures (state machine)", file: "examples/state-machine.json" },
  { name: "web app packages (dependency tree)", file: "examples/dependencies.json" },
  { name: "empty (basic)", file: null, schema: "basic" },
  { name: "empty use case diagram", file: null, schema: "use-case" },
  { name: "empty threat model", file: null, schema: "threat-model" },
  { name: "empty state machine", file: null, schema: "state-machine" },
];

let g = null;
let annotated = false;

// setAnnotated names what the annotate button does next.
function setAnnotated(on) {
  annotated = on;
  $("annotate").textContent = on ? "Hide annotations" : "Annotate";
}

function ok(res, what) {
  if (res && res.error) {
    $("error").textContent = what + ": " + res.error;
    return null;
  }
  $("error").textContent = "";
  return res.ok;
}

// exportText shows the document as text, and its flow in the flow
// selector.
function exportText() {
  const d = ok(graphedwright.export(g), "export");
  if (d === null) return;
  $("json").value = JSON.stringify(d, null, 1);
  $("flow").value = d.flow || "left-to-right";
}

// shownProps names the form the properties panel holds: the element
// and its keys, so that a change to the same element refreshes values
// instead of rebuilding the form under the field being typed in.
let shownProps = "";

// renderProps shows the one selected shape or edge in the properties
// panel, each field committing through setLabel or setAttr, which are
// commands the document can refuse and undo.
function renderProps() {
  const p = ok(graphedwright.properties(g), "properties");
  const box = $("props");
  if (!p) {
    shownProps = "";
    box.replaceChildren(Object.assign(document.createElement("p"), { className: "hint", textContent: "Select one shape or edge to see its properties." }));
    return;
  }
  const sig = p.id + "\n" + p.attrs.map((a) => a.key).join("\n");
  if (sig === shownProps) {
    for (const el of box.querySelectorAll("[data-key]")) {
      if (el === document.activeElement) continue;
      el.value = el.dataset.key === "\nlabel" ? p.label : (p.attrs.find((a) => a.key === el.dataset.key) || { value: "" }).value;
    }
    return;
  }
  shownProps = sig;
  const table = document.createElement("table");
  const row = (name, cell, cls) => {
    const tr = table.insertRow();
    if (cls) tr.className = cls;
    const th = document.createElement("th");
    th.textContent = name;
    tr.appendChild(th);
    tr.insertCell().appendChild(cell);
  };
  const fixed = (text) => Object.assign(document.createElement("span"), { className: "fixed", textContent: text });
  const field = (key, a, value) => {
    let el;
    if (a && a.kind === "choice") {
      el = document.createElement("select");
      if (!a.required) el.appendChild(new Option("", ""));
      for (const c of a.choices) el.appendChild(new Option(c, c));
    } else {
      el = document.createElement("input");
      if (a && a.kind === "number") el.inputMode = "decimal";
    }
    el.dataset.key = key;
    el.value = value;
    el.onchange = () => {
      const res = key === "\nlabel" ? graphedwright.setLabel(g, el.value) : graphedwright.setAttr(g, key, el.value);
      if (res.error) { $("error").textContent = res.error; renderProps(); }
    };
    el.onkeydown = (ev) => { if (ev.key === "Enter") el.blur(); ev.stopPropagation(); };
    return el;
  };
  row(p.element === "edge" ? "edge kind" : "kind", fixed(p.kind));
  row("id", fixed(p.id));
  if (p.element === "edge") row("joins", fixed(p.from + " → " + p.to));
  row("label", field("\nlabel", null, p.label));
  for (const a of p.attrs) {
    row(a.key + (a.required ? " *" : ""), field(a.key, a, a.value), a.known ? "" : "unknown");
  }
  box.replaceChildren(table);
}

function layOut() {
  ok(graphedwright.layout(g, { all: true, flow: $("flow").value }), "layout");
  exportText();
}

async function loadExample() {
  const ex = EXAMPLES[$("example").selectedIndex];
  let text = JSON.stringify({ schema: ex.schema });
  if (ex.file) text = await (await fetch(ex.file)).text();
  if (ok(graphedwright.load(g, text), "load") !== null) { setAnnotated(false); exportText(); renderProps(); }
}

function loadScript(src) {
  return new Promise((resolve, reject) => { const s = document.createElement("script"); s.src = src; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); });
}

async function main() {
  await loadScript("wasm_exec.js?toolchain=" + toolchain);
  const go = new Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch("graphedwright.wasm?toolchain=" + toolchain), go.importObject);
  go.run(instance);
  for (const ex of EXAMPLES) {
    const opt = document.createElement("option");
    opt.textContent = ex.name;
    $("example").appendChild(opt);
  }
  g = ok(graphedwright.create($("host"), { schema: "basic", mode: "edit" }), "create");
  graphedwright.on(g, "error", (msg) => { $("error").textContent = msg; });
  graphedwright.on(g, "select", (sel) => { $("status").textContent = sel.vertices.length + " vertices and " + sel.edges.length + " edges selected"; renderProps(); });
  graphedwright.on(g, "change", () => { exportText(); renderProps(); });
  $("load").onclick = loadExample;
  $("layout").onclick = layOut;
  $("flow").onchange = layOut;
  $("undo").onclick = () => graphedwright.undo(g);
  $("redo").onclick = () => graphedwright.redo(g);
  $("mode").onclick = () => {
    const toView = $("mode").textContent === "View mode";
    ok(graphedwright.setMode(g, toView ? "view" : "edit"), "setMode");
    $("mode").textContent = toView ? "Edit mode" : "View mode";
  };
  $("annotate").onclick = () => {
    if (annotated) {
      if (ok(graphedwright.annotate(g, {}), "annotate") !== null) setAnnotated(false);
      return;
    }
    const d = ok(graphedwright.export(g), "export");
    if (d === null) return;
    const notes = {};
    d.vertices.forEach((v, i) => { notes[v.id] = { text: (i + 1).toFixed(1) + "/period" }; });
    d.edges.forEach((e, i) => { notes[e.id] = { text: (100 / (i + 2)).toFixed(0) + "%", weight: 1 / (i + 1) }; });
    if (ok(graphedwright.annotate(g, notes), "annotate") !== null) setAnnotated(true);
  };
  $("download").onclick = () => {
    const text = ok(graphedwright.svg(g), "svg");
    if (text === null) return;
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([text], { type: "image/svg+xml" }));
    a.download = "graph.svg";
    a.click();
  };
  $("apply").onclick = () => { if (ok(graphedwright.load(g, $("json").value), "load") !== null) { setAnnotated(false); exportText(); } };
  $("export").onclick = exportText;
  await loadExample();
}

main().catch((err) => { $("error").textContent = String(err); });
