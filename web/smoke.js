// The smoke test: load the build, create, load the diamond example,
// export it, render it, annotate, undo and redo, and write every result
// into #out as JSON for the driver to compare with the native command.
"use strict";
const toolchain = new URLSearchParams(location.search).get("toolchain") === "tinygo" ? "tinygo" : "go";
function loadScript(src) {
  return new Promise((resolve, reject) => { const s = document.createElement("script"); s.src = src; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); });
}
async function main() {
  const out = {};
  await loadScript("wasm_exec.js?toolchain=" + toolchain);
  const go = new Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch("graphedwright.wasm?toolchain=" + toolchain), go.importObject);
  go.run(instance);
  const text = await (await fetch("examples/diamond.json")).text();
  const g = graphedwright.create(document.getElementById("host"), { schema: "basic", mode: "view", patch: true }).ok;
  out.load = graphedwright.load(g, text);
  out.loadObject = graphedwright.load(g, JSON.parse(text));
  out.export = graphedwright.export(g);
  out.svg = graphedwright.svg(g);
  out.setMode = graphedwright.setMode(g, "edit");
  out.annotate = graphedwright.annotate(g, { a: { text: "2.1/period" }, e2: { text: "30%", weight: 0.3 } });
  out.svgAnnotated = graphedwright.svg(g);
  out.domVertices = document.querySelectorAll("#host svg g.vertex").length;
  out.domEdges = document.querySelectorAll("#host svg g.edge").length;
  out.badSchema = graphedwright.create(document.createElement("div"), { schema: "nope" });
  out.badLoad = graphedwright.load(g, '{"schema":"basic","vertices":[{"kind":"blob"}]}');
  out.hugeCaps = graphedwright.create(document.createElement("div"), { schema: "basic", caps: { vertices: 1e20 } });
  // 64 vertices placed at x = 0 and 64 at 528 000 layers' pitch, each of
  // the first joined to each of the second: past 2^31 dummy vertices,
  // refused on every toolchain.
  const h3 = graphedwright.create(document.createElement("div"), { schema: "basic", caps: { edges: 65536 } }).ok;
  const wide = { schema: "basic", vertices: [{ id: "u", kind: "box" }], edges: [] };
  for (let i = 0; i < 128; i++) wide.vertices.push({ id: "v" + i, kind: "box", x: i < 64 ? 0 : 528000 * 160, y: 50 * i, placed: true });
  for (let i = 0; i < 64; i++) for (let j = 64; j < 128; j++) wide.edges.push({ id: "e" + i + "_" + j, kind: "arrow", from: "v" + i, to: "v" + j });
  out.wideLoad = graphedwright.load(h3, wide);
  graphedwright.destroy(h3);
  // Two vertices placed 1e9 apart, 2^20 layers, beyond the arrays by layer.
  out.farLoad = graphedwright.load(g, { schema: "basic", vertices: [{ id: "u", kind: "box" }, { id: "a", kind: "box", x: 0, placed: true }, { id: "b", kind: "box", x: 1e9, placed: true }] });
  out.registered = graphedwright.registerSchema("tiny", { vertexKinds: [{ name: "dot", shape: "circle", w: 20, h: 20 }], edgeKinds: [{ name: "link", from: ["dot"], to: ["dot"], arrow: true }], acyclic: true });
  const h2 = graphedwright.create(document.createElement("div"), { schema: "tiny" }).ok;
  out.tinyLoad = graphedwright.load(h2, { schema: "tiny", vertices: [{ id: "x", kind: "dot" }, { id: "y", kind: "dot" }], edges: [{ kind: "link", from: "x", to: "y" }] });
  out.tinyExport = graphedwright.export(h2);
  graphedwright.undo(g);
  out.destroyed = graphedwright.destroy(h2);
  out.afterDestroy = graphedwright.svg(h2);
  document.getElementById("out").textContent = JSON.stringify(out);
}
main().catch((err) => { document.getElementById("out").textContent = JSON.stringify({ fatal: String(err) }); });
