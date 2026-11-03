// The smoke test: load the build, create, load the diamond example,
// export it, render it, annotate, undo and redo, edit a drawing whose
// kinds are named as the binding's groups, pan one with the middle
// button, and write every result into #out as JSON for the driver to
// compare with the native command.
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
  // Kinds, and annotation classes, named as the binding's own groups:
  // selecting a vertex redraws the overlay group, not an element that
  // carries its name, and a delete and its undo leave each vertex in the
  // vertices group.
  graphedwright.registerSchema("clash", { vertexKinds: [{ name: "overlay", shape: "round", w: 100, h: 36 }, { name: "box", shape: "round", w: 100, h: 36 }], edgeKinds: [{ name: "vertices", arrow: true }] });
  const clash = document.getElementById("clash");
  const hc = graphedwright.create(clash, { schema: "clash" }).ok;
  out.clashLoad = graphedwright.load(hc, { schema: "clash", vertices: [{ id: "a", kind: "overlay", label: "a" }, { id: "b", kind: "box", label: "b" }, { id: "c", kind: "box", label: "c" }], edges: [{ id: "ab", kind: "vertices", from: "a", to: "b" }, { id: "ac", kind: "vertices", from: "a", to: "c" }] });
  out.clashAnnotate = graphedwright.annotate(hc, { a: { text: "1", class: "overlay" }, ac: { text: "2", class: "vertices" } });
  const drawn = () => clash.querySelector("svg"); // a redraw replaces it
  const box = drawn().querySelector("#b").getBoundingClientRect();
  const at = { clientX: box.x + box.width / 2, clientY: box.y + box.height / 2, pointerId: 1, button: 0, bubbles: true };
  drawn().dispatchEvent(new PointerEvent("pointerdown", at));
  drawn().dispatchEvent(new PointerEvent("pointerup", at));
  out.clashSelected = { a: drawn().querySelector(":scope > g.vertices > #a") !== null, selection: drawn().querySelector(":scope > g.overlay .selection") !== null };
  clash.dispatchEvent(new KeyboardEvent("keydown", { key: "Delete", bubbles: true }));
  out.clashDeleted = drawn().querySelector("#b") === null;
  graphedwright.undo(hc);
  out.clashRestored = drawn().querySelector(":scope > g.vertices > #b") !== null;
  graphedwright.destroy(hc);
  // A middle-button drag keeps the point of the drawing it grabbed under
  // the pointer at every move, through the SVG's screen matrix.
  const hp = graphedwright.create(clash, { schema: "basic" }).ok;
  graphedwright.load(hp, text);
  const under = (x, y) => {
    const s = clash.querySelector("svg");
    const q = s.createSVGPoint();
    q.x = x;
    q.y = y;
    const p = q.matrixTransform(s.getScreenCTM().inverse());
    return [p.x, p.y];
  };
  const r0 = clash.querySelector("svg").getBoundingClientRect();
  const x0 = r0.x + r0.width / 2, y0 = r0.y + r0.height / 2;
  const grabbed = under(x0, y0);
  clash.querySelector("svg").dispatchEvent(new PointerEvent("pointerdown", { clientX: x0, clientY: y0, pointerId: 3, button: 1, bubbles: true }));
  let slip = 0;
  for (let k = 1; k <= 8; k++) {
    const x = x0 + 12 * k, y = y0 + 5 * k;
    window.dispatchEvent(new PointerEvent("pointermove", { clientX: x, clientY: y, pointerId: 3, buttons: 4, bubbles: true }));
    const p = under(x, y);
    slip = Math.max(slip, Math.abs(p[0] - grabbed[0]), Math.abs(p[1] - grabbed[1]));
  }
  window.dispatchEvent(new PointerEvent("pointerup", { clientX: x0 + 96, clientY: y0 + 40, pointerId: 3, button: 1, bubbles: true }));
  out.panSlip = slip;
  graphedwright.destroy(hp);
  // A pointer over a vertex whose label is cut draws it in full in the
  // overlay, in view and in edit mode; over one with nothing cut, or
  // over the background, nothing.
  graphedwright.registerSchema("cuts", { vertexKinds: [{ name: "box", shape: "rect", w: 80, h: 30 }], edgeKinds: [{ name: "link", arrow: true }] });
  const hh = graphedwright.create(clash, { schema: "cuts", mode: "view" }).ok;
  graphedwright.load(hh, { schema: "cuts", vertices: [{ id: "long", kind: "box", label: "a label far too long to fit", x: 60, y: 40, placed: true }, { id: "short", kind: "box", label: "ok", x: 260, y: 40, placed: true }], edges: [] });
  const hoverAt = (sel) => {
    const s = clash.querySelector("svg"), r = sel ? s.querySelector(sel).getBoundingClientRect() : s.getBoundingClientRect();
    const x = sel ? r.x + r.width / 2 : r.x + 2, y = sel ? r.y + r.height / 2 : r.y + r.height - 2;
    window.dispatchEvent(new PointerEvent("pointermove", { clientX: x, clientY: y, pointerId: 4, bubbles: true }));
    return Array.from(clash.querySelectorAll("svg > g.overlay .expanded text"), (t) => t.textContent);
  };
  const cuts = { schema: "cuts", vertices: [{ id: "long", kind: "box", label: "a label far too long to fit", x: 60, y: 40, placed: true }, { id: "short", kind: "box", label: "ok", x: 260, y: 40, placed: true }], edges: [] };
  out.hover = {};
  for (const mode of ["view", "edit"]) {
    graphedwright.setMode(hh, mode);
    out.hover[mode] = { long: hoverAt("#long"), short: hoverAt("#short"), background: hoverAt(null) };
  }
  out.fullText = graphedwright.svg(hh).ok.includes(' full"');
  graphedwright.destroy(hh);
  // Either turned off at create: no overlay expansion, no full texts.
  const ho = graphedwright.create(clash, { schema: "cuts", mode: "view", expand: false, fullText: false }).ok;
  graphedwright.load(ho, cuts);
  out.hoverOff = hoverAt("#long");
  out.fullTextOff = graphedwright.svg(ho).ok.includes(' full"') || clash.querySelector("svg .full") !== null;
  graphedwright.destroy(ho);
  // The object's other functions, each on an editor of its own and
  // each checked by its own effect, so that one wired to another's work
  // shows.
  const hf = graphedwright.create(clash, { schema: "basic" }).ok;
  graphedwright.load(hf, { schema: "basic", vertices: [{ id: "p", kind: "box", label: "p" }, { id: "q", kind: "box", label: "q" }], edges: [{ id: "pq", kind: "arrow", from: "p", to: "q" }] });
  const fns = {};
  const heard = [];
  fns.on = graphedwright.on(hf, "change", (c) => heard.push(c));
  fns.layout = graphedwright.layout(hf, { all: true });
  const viewBox = () => clash.querySelector("svg").getAttribute("viewBox");
  const fitted = viewBox();
  fns.zoom = graphedwright.zoom(hf, 2);
  fns.zoomed = viewBox() !== fitted;
  fns.fit = graphedwright.fit(hf);
  fns.fitted = viewBox() === fitted;
  const pb = clash.querySelector("svg #p").getBoundingClientRect();
  const onP = { clientX: pb.x + pb.width / 2, clientY: pb.y + pb.height / 2, pointerId: 5, button: 0, bubbles: true };
  clash.querySelector("svg").dispatchEvent(new PointerEvent("pointerdown", onP));
  window.dispatchEvent(new PointerEvent("pointerup", onP));
  fns.properties = graphedwright.properties(hf);
  fns.setLabel = graphedwright.setLabel(hf, "renamed");
  fns.setAttr = graphedwright.setAttr(hf, "note", "kept");
  fns.addPort = graphedwright.addPort(hf, { name: "portx", side: "right", max: 1 });
  const ported = JSON.stringify(graphedwright.export(hf).ok).includes("portx");
  fns.deletePort = graphedwright.deletePort(hf, "portx");
  const unported = !JSON.stringify(graphedwright.export(hf).ok).includes("portx");
  graphedwright.undo(hf); // the delete: the port is back
  const undone = JSON.stringify(graphedwright.export(hf).ok).includes("portx");
  graphedwright.undo(hf); // the add: gone again
  fns.redo = graphedwright.redo(hf); // the add again, where a third undo would take the attribute
  const vp = graphedwright.export(hf).ok.vertices.find((v) => v.id === "p");
  fns.effects = { ported, unported, undone, redone: JSON.stringify(vp).includes("portx"), label: vp.label, note: JSON.stringify(vp).includes("kept") };
  await new Promise((r) => setTimeout(r, 0)); // the callbacks are microtasks
  fns.heard = heard.length;
  graphedwright.destroy(hf);
  out.fns = fns;

  // A schema's bounded numbers and the attribute its label input edits,
  // from JavaScript: the bounds reach properties and refuse a value
  // outside them, a double-click edits the attribute, and a schema that
  // names an undeclared attribute or bounds a text is refused.
  const bounded = {};
  bounded.registered = graphedwright.registerSchema("bounded", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, labelFormat: "{share}", editAttr: "share", attrs: [{ key: "share", kind: "number", min: 0, max: 1 }, { key: "rate", kind: "number", min: 0 }] }], edgeKinds: [] });
  graphedwright.registerSchema("bounded-bad-edit", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, editAttr: "nope" }], edgeKinds: [] });
  graphedwright.registerSchema("bounded-bad-bound", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, attrs: [{ key: "note", kind: "text", max: 3 }] }], edgeKinds: [] });
  bounded.badEdit = graphedwright.create(clash, { schema: "bounded-bad-edit" });
  bounded.badBound = graphedwright.create(clash, { schema: "bounded-bad-bound" });
  const hb = graphedwright.create(clash, { schema: "bounded" }).ok;
  graphedwright.load(hb, { schema: "bounded", vertices: [{ id: "x", kind: "box", attrs: { share: "0.5" } }], edges: [] });
  const xb = clash.querySelector("svg #x").getBoundingClientRect();
  const onX = { clientX: xb.x + xb.width / 2, clientY: xb.y + xb.height / 2, pointerId: 6, button: 0, bubbles: true };
  clash.querySelector("svg").dispatchEvent(new PointerEvent("pointerdown", onX));
  window.dispatchEvent(new PointerEvent("pointerup", onX));
  const props = (graphedwright.properties(hb).ok || {}).attrs || [];
  const share = props.find((a) => a.key === "share") || {};
  const rate = props.find((a) => a.key === "rate") || {};
  bounded.bounds = { shareMin: share.min, shareMax: share.max, rateMin: rate.min, rateMaxInfinite: rate.max === Infinity };
  bounded.outside = graphedwright.setAttr(hb, "share", "1.5");
  bounded.below = graphedwright.setAttr(hb, "rate", "-1");
  clash.querySelector("svg").dispatchEvent(new MouseEvent("dblclick", { clientX: onX.clientX, clientY: onX.clientY, button: 0, bubbles: true }));
  const input = clash.querySelector("input.graphedwright-label");
  bounded.editing = input.style.display === "block" ? input.value : null;
  input.value = "0.25";
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  const xv = graphedwright.export(hb).ok.vertices.find((v) => v.id === "x");
  bounded.done = { share: xv.attrs && xv.attrs.share, label: xv.label || "" };
  graphedwright.destroy(hb);
  out.bounded = bounded;
  out.afterDestroy = graphedwright.svg(h2);
  document.getElementById("out").textContent = JSON.stringify(out);
}
main().catch((err) => { document.getElementById("out").textContent = JSON.stringify({ fatal: String(err) }); });
