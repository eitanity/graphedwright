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
  // A schema is checked when it is registered: one refused is not known.
  bounded.badEdit = graphedwright.registerSchema("bounded-bad-edit", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, editAttr: "nope" }], edgeKinds: [] });
  bounded.badBound = graphedwright.registerSchema("bounded-bad-bound", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, attrs: [{ key: "note", kind: "text", max: 3 }] }], edgeKinds: [] });
  bounded.badUnknown = graphedwright.create(clash, { schema: "bounded-bad-edit" });
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
  const input = clash.querySelector("textarea.graphedwright-label");
  bounded.editing = input.style.display === "block" ? input.value : null;
  input.value = "0.25";
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  const xv = graphedwright.export(hb).ok.vertices.find((v) => v.id === "x");
  bounded.done = { share: xv.attrs && xv.attrs.share, label: xv.label || "" };
  graphedwright.destroy(hb);
  out.bounded = bounded;
  // Provided shapes from JavaScript: a kind names a library shape by
  // name and another one the schema object defines under shapes; both
  // are drawn as their paths, and a path that carries markup, or an
  // unknown name, is refused.
  const provided = {};
  provided.registered = graphedwright.registerSchema("provided", {
    shapes: [{ name: "flag", w: 60, h: 40, margins: [0, 0, 12, 0], outline: "polygon", polygon: [[0, 0], [60, 0], [48, 40], [0, 40]], paths: [{ d: "M 0 0 H 60 L 48 40 H 0 Z" }, { d: "M 50 4 H 56 V 10 H 50 Z", fill: true }] }],
    vertexKinds: [{ name: "end", shape: "bullseye", w: 28, h: 28 }, { name: "send", shape: "flag", w: 60, h: 40 }],
    edgeKinds: [{ name: "flow", arrow: true }],
  });
  provided.badPath = graphedwright.registerSchema("provided-bad", { shapes: [{ name: "evil", w: 10, h: 10, paths: [{ d: '"/><script>' }] }], vertexKinds: [{ name: "k", shape: "evil", w: 20, h: 20 }], edgeKinds: [] });
  provided.badName = graphedwright.registerSchema("provided-unknown", { vertexKinds: [{ name: "k", shape: "blob" }], edgeKinds: [] });
  const hq = graphedwright.create(clash, { schema: "provided" }).ok;
  provided.load = graphedwright.load(hq, { schema: "provided", vertices: [{ id: "s", kind: "send", label: "send" }, { id: "f", kind: "end" }], edges: [{ kind: "flow", from: "s", to: "f" }] });
  const drawnP = graphedwright.svg(hq).ok || "";
  provided.bullseye = drawnP.includes('<g class="shape shape-bullseye"><path class="line" d="M ') && drawnP.includes('<path class="mark"');
  provided.flag = drawnP.includes('<g class="shape shape-flag"><path class="line" d="M ');
  provided.dom = clash.querySelectorAll("svg g.shape-bullseye path, svg g.shape-flag path").length;
  graphedwright.destroy(hq);
  out.provided = provided;
  // Context requests: a right-click on a vertex, with a "context"
  // listener, selects it and reports it with the browser's menu
  // prevented; one on a port or an edge names it; one from the keyboard
  // is on the selection, whether told by a point off the drawing or a
  // pointer event of no pointer type; label editing under way is kept;
  // the menu's actions are commands, each undone by Ctrl+Z. Without a
  // listener the browser's menu stays.
  const ctx = {};
  graphedwright.registerSchema("ported", { vertexKinds: [{ name: "box", shape: "rect", w: 100, h: 36, ports: [{ name: "in", side: "left" }, { name: "out", side: "right" }] }], edgeKinds: [{ name: "arrow", arrow: true }] });
  const twoBoxes = { schema: "ported", vertices: [{ id: "m", kind: "box", label: "m", x: 100, y: 80, placed: true }, { id: "n", kind: "box", label: "n", x: 400, y: 80, placed: true }], edges: [{ id: "mn", kind: "arrow", from: "m", to: "n", from_port: "out", to_port: "in" }] };
  const tick = () => new Promise((r) => setTimeout(r, 0)); // the callbacks are microtasks
  const centreOf = (sel) => { const r = clash.querySelector("svg").querySelector(sel).getBoundingClientRect(); return { clientX: r.x + r.width / 2, clientY: r.y + r.height / 2 }; };
  // A secondary click, as a pointer event: its pointer type is "".
  const menuOn = (sel) => {
    const ev = new PointerEvent("contextmenu", Object.assign({ button: 2, bubbles: true, cancelable: true }, centreOf(sel)));
    clash.querySelector("svg").querySelector(sel).dispatchEvent(ev);
    return ev.defaultPrevented;
  };
  const hx = graphedwright.create(clash, { schema: "ported" }).ok;
  ctx.load = graphedwright.load(hx, twoBoxes);
  const asked = [];
  ctx.on = graphedwright.on(hx, "context", (r) => asked.push(r));
  ctx.port = menuOn('#m circle.port[data-port="out"]');
  menuOn("#mn path");
  ctx.prevented = menuOn("#n");
  await tick();
  ctx.portRequest = asked[0] || null;
  ctx.edgeRequest = asked[1] || null;
  ctx.request = asked[2] || null;
  clash.querySelector("svg").dispatchEvent(new MouseEvent("contextmenu", { clientX: 0, clientY: 0, button: 0, bubbles: true, cancelable: true }));
  clash.querySelector("svg #m").dispatchEvent(new PointerEvent("contextmenu", Object.assign({ button: 0, bubbles: true, cancelable: true }, centreOf("#m"))));
  await tick();
  ctx.keyboard = asked[3] || null;
  ctx.keyboardTyped = asked[4] || null;
  const nr = clash.querySelector("svg #n").getBoundingClientRect();
  ctx.keyboardOnN = ctx.keyboard !== null && ctx.keyboard.clientX > nr.left && ctx.keyboard.clientX < nr.right && ctx.keyboard.clientY > nr.top && ctx.keyboard.clientY < nr.bottom;
  const labelOfN = () => (graphedwright.export(hx).ok.vertices.find((v) => v.id === "n") || {}).label;
  const vertexCount = () => graphedwright.export(hx).ok.vertices.length;
  const ctrlZ = () => clash.dispatchEvent(new KeyboardEvent("keydown", { key: "z", ctrlKey: true, bubbles: true }));
  // A label being edited is kept when a request ends the editing.
  clash.querySelector("svg #n").dispatchEvent(new MouseEvent("dblclick", Object.assign({ button: 0, bubbles: true }, centreOf("#n"))));
  clash.querySelector("textarea.graphedwright-label").value = "typed";
  menuOn("#n");
  ctx.typed = labelOfN();
  ctrlZ();
  ctx.setLabel = graphedwright.setLabel(hx, "from the menu");
  ctx.relabelled = labelOfN();
  clash.dispatchEvent(new KeyboardEvent("keydown", { key: "Delete", bubbles: true }));
  ctx.deleted = vertexCount();
  ctrlZ();
  ctx.undeleted = vertexCount();
  ctrlZ();
  ctx.unlabelled = labelOfN();
  graphedwright.destroy(hx);
  const hn = graphedwright.create(clash, { schema: "ported" }).ok;
  graphedwright.load(hn, twoBoxes);
  ctx.unlistened = menuOn("#n");
  graphedwright.destroy(hn);
  out.ctx = ctx;
  // A hold: the user's drag moves nothing and the host element carries
  // data-held; the host's own calls still apply; load keeps the hold;
  // released, a drag moves again.
  const hold = {};
  const hk = graphedwright.create(clash, { schema: "basic" }).ok;
  const oneBox = { schema: "basic", vertices: [{ id: "k", kind: "box", label: "k", x: 200, y: 80, placed: true }], edges: [] };
  graphedwright.load(hk, oneBox);
  const xOfK = () => graphedwright.export(hk).ok.vertices[0].x;
  const dragK = (id) => {
    const r = clash.querySelector("svg #k").getBoundingClientRect(), x = r.x + r.width / 2, y = r.y + r.height / 2;
    clash.querySelector("svg #k").dispatchEvent(new PointerEvent("pointerdown", { clientX: x, clientY: y, pointerId: id, button: 0, buttons: 1, bubbles: true }));
    window.dispatchEvent(new PointerEvent("pointermove", { clientX: x + 40, clientY: y, pointerId: id, buttons: 1, bubbles: true }));
    window.dispatchEvent(new PointerEvent("pointermove", { clientX: x + 80, clientY: y, pointerId: id, buttons: 1, bubbles: true }));
    window.dispatchEvent(new PointerEvent("pointerup", { clientX: x + 80, clientY: y, pointerId: id, button: 0, bubbles: true }));
  };
  const kr = clash.querySelector("svg #k").getBoundingClientRect();
  const atK = { clientX: kr.x + kr.width / 2, clientY: kr.y + kr.height / 2, pointerId: 20, button: 0, bubbles: true };
  clash.querySelector("svg #k").dispatchEvent(new PointerEvent("pointerdown", atK));
  window.dispatchEvent(new PointerEvent("pointerup", atK));
  const x0k = xOfK();
  // A label being edited is dropped by the hold, and its input hidden.
  clash.querySelector("svg #k").dispatchEvent(new MouseEvent("dblclick", { clientX: atK.clientX, clientY: atK.clientY, button: 0, bubbles: true }));
  clash.querySelector("textarea.graphedwright-label").value = "dropped";
  hold.hold = graphedwright.hold(hk);
  hold.inputHidden = clash.querySelector("textarea.graphedwright-label").style.display === "none";
  hold.labelKept = graphedwright.export(hk).ok.vertices[0].label;
  hold.marked = clash.hasAttribute("data-held");
  dragK(21);
  hold.heldMoved = xOfK() !== x0k;
  hold.setLabel = graphedwright.setLabel(hk, "by the host");
  hold.label = graphedwright.export(hk).ok.vertices[0].label;
  graphedwright.load(hk, oneBox);
  dragK(22);
  hold.loadedMoved = xOfK() !== x0k;
  hold.loadedMarked = clash.hasAttribute("data-held");
  hold.release = graphedwright.release(hk);
  hold.unmarked = !clash.hasAttribute("data-held");
  dragK(23);
  hold.releasedMoved = xOfK() !== x0k;
  graphedwright.destroy(hk);
  out.hold = hold;
  // A label of lines: the input holds them, Enter keeps them as they
  // were, Shift+Enter leaves the editing open for a new line, the input
  // grows taller for lines typed, and Enter then keeps the lines typed;
  // each is drawn as a line of its own.
  const lines = {};
  const hl = graphedwright.create(clash, { schema: "basic" }).ok;
  graphedwright.load(hl, { schema: "basic", vertices: [{ id: "l", kind: "box", label: "one\ntwo", x: 200, y: 80, placed: true }], edges: [] });
  const labelOfL = () => graphedwright.export(hl).ok.vertices[0].label;
  const area = () => clash.querySelector("textarea.graphedwright-label");
  const editL = () => { const r = clash.querySelector("svg #l").getBoundingClientRect(); clash.querySelector("svg #l").dispatchEvent(new MouseEvent("dblclick", { clientX: r.x + r.width / 2, clientY: r.y + r.height / 2, button: 0, bubbles: true })); };
  const key = (shiftKey) => area().dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", shiftKey, bubbles: true, cancelable: true }));
  editL();
  lines.editing = area().style.display === "block" ? area().value : null;
  key(false);
  lines.unchanged = labelOfL();
  editL();
  lines.shiftKept = key(true);
  lines.shiftOpen = area().style.display === "block";
  lines.shiftLabel = labelOfL();
  const h0 = parseFloat(area().style.height);
  area().value = "one\ntwo\nthree\nfour\nfive";
  area().dispatchEvent(new Event("input", { bubbles: true }));
  lines.grew = parseFloat(area().style.height) > h0;
  area().value = "one\ntwo\nthree";
  key(false);
  lines.typed = labelOfL();
  lines.drawn = clash.querySelectorAll("svg #l text.label tspan").length;
  graphedwright.destroy(hl);
  out.lines = lines;

  // A large drawing's functions: find, fold, the outline, reveal, focus
  // and unfold, and the fold's undo.
  const large = {};
  const hlg = graphedwright.create(clash, { schema: "basic" }).ok;
  large.load = graphedwright.load(hlg, { schema: "basic", vertices: [
    { id: "x", kind: "box", label: "outside", x: 60, y: 60, placed: true },
    { id: "a", kind: "box", label: "Pump one", x: 260, y: 60, placed: true, parent: "grp" },
    { id: "b", kind: "box", label: "pump two", x: 260, y: 140, placed: true, parent: "grp" },
    { id: "grp", kind: "group", label: "pumps" }],
    edges: [{ id: "xa", kind: "arrow", from: "x", to: "a" }, { id: "xb", kind: "arrow", from: "x", to: "b" }] });
  const shown = (id) => clash.querySelector(`svg [id="${id}"]`);
  large.find = [graphedwright.find(hlg, "PUMP").ok, graphedwright.find(hlg, "pump", "a").ok, graphedwright.find(hlg, "zz").ok];
  large.fold = graphedwright.fold(hlg, "grp");
  large.folded = clash.querySelector("svg g.vertex.folded") !== null && shown("a") === null && shown("xb") === null;
  large.merged = shown("xa") ? shown("xa").getAttribute("data-merged") : null;
  large.outline = graphedwright.outline(hlg).ok;
  large.saved = graphedwright.export(hlg).ok.vertices.find((v) => v.id === "grp").folded === true;
  large.reveal = graphedwright.reveal(hlg, "b");
  large.revealed = shown("b") !== null && clash.querySelector("svg g.vertex.folded") === null;
  large.focus = graphedwright.focus(hlg, 0, "both");
  large.dimmed = shown("x").classList.contains("dim") && !shown("b").classList.contains("dim");
  large.unfocus = graphedwright.unfocus(hlg);
  large.undimmed = clash.querySelector("svg .dim") === null;
  graphedwright.undo(hlg);
  large.refolded = clash.querySelector("svg g.vertex.folded") !== null;
  large.unfold = graphedwright.unfold(hlg, "grp");
  large.badFold = graphedwright.fold(hlg, "a");
  large.badWay = graphedwright.focus(hlg, 1, "sideways");
  graphedwright.destroy(hlg);
  out.large = large;
  out.afterDestroy = graphedwright.svg(h2);
  document.getElementById("out").textContent = JSON.stringify(out);
}
main().catch((err) => { document.getElementById("out").textContent = JSON.stringify({ fatal: String(err) }); });
