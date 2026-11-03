// The smoke test's user of the event-tree example (eventtree.html): sent
// by smoke.py as a WebDriver script, it works the page only as a person
// could, with pointer, keyboard and click events on what is shown, and
// reads back only what is shown. It reports what it saw; smoke.py
// decides.
const done = arguments[arguments.length - 1];
(async () => {
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  const until = async (what, f) => {
    for (let i = 0; i < 200; i++) {
      const v = f();
      if (v) return v;
      await wait(25);
    }
    throw new Error("timed out waiting for " + what);
  };
  const root = document.getElementById("drawing");
  await until("the drawing", () => document.documentElement.dataset.error || root.querySelector("svg g.vertex"));
  if (document.documentElement.dataset.error) throw new Error("load: " + document.documentElement.dataset.error);
  const svg = () => root.querySelector("svg");
  const byId = (id) => svg().querySelector("#" + CSS.escape(id));
  const text = (id) => (byId(id) ? byId(id).textContent : null);
  const ids = (sel) => Array.from(svg().querySelectorAll(sel), (g) => g.id);
  const status = () => document.getElementById("status").textContent;
  const panel = () => document.getElementById("panel").textContent;
  const shape = (id) => byId(id).querySelector("rect, path, ellipse, polygon").getBoundingClientRect();
  let pid = 10;
  const ptr = (target, type, x, y, buttons) => target.dispatchEvent(new PointerEvent(type, { clientX: x, clientY: y, pointerId: pid, button: 0, buttons, isPrimary: true, bubbles: true }));
  const press = (x, y) => { pid++; ptr(document.elementFromPoint(x, y), "pointerdown", x, y, 1); };
  const moveTo = (x, y) => ptr(window, "pointermove", x, y, 1);
  const release = (x, y) => ptr(window, "pointerup", x, y, 0);
  const centre = (r) => [r.x + r.width / 2, r.y + r.height / 2];
  const click = async (x, y) => { press(x, y); release(x, y); await wait(0); };
  const key = async (k, mods) => { root.dispatchEvent(new KeyboardEvent("keydown", Object.assign({ key: k, bubbles: true }, mods))); await wait(0); };
  // A point of the drawing with nothing on it.
  const empty = () => {
    const r = svg().getBoundingClientRect();
    for (let y = r.bottom - 30; y > r.top + 30; y -= 20) {
      for (let x = r.right - 80; x > r.left + 80; x -= 20) {
        const at = document.elementFromPoint(x, y);
        if (at && svg().contains(at) && !at.closest("g.vertex, g.edge")) {
          let clear = true;
          for (const [dx, dy] of [[-75, -30], [75, -30], [-75, 30], [75, 30]]) {
            const c = document.elementFromPoint(x + dx, y + dy);
            if (!c || !svg().contains(c) || c.closest("g.vertex, g.edge")) clear = false;
          }
          if (clear) return [x, y];
        }
      }
    }
    throw new Error("no empty place in the drawing");
  };
  // A drag from the palette item named onto the point.
  const drop = async (name, [x, y]) => {
    const item = Array.from(root.querySelectorAll(".graphedwright-palette .graphedwright-item")).find((b) => b.textContent === name);
    const [ix, iy] = centre(item.getBoundingClientRect());
    pid++;
    ptr(item, "pointerdown", ix, iy, 1);
    moveTo((ix + x) / 2, (iy + y) / 2);
    moveTo(x, y);
    release(x, y);
    await wait(0);
  };
  // A connection from the right of one shape's outline to another's middle.
  const connect = async (from, to) => {
    const f = shape(from);
    const [tx, ty] = centre(shape(to));
    press(f.right - 1, f.y + f.height / 2);
    moveTo((f.right + tx) / 2, (f.y + ty) / 2);
    moveTo(tx, ty);
    release(tx, ty);
    await wait(0);
  };
  const figures = (list) => Object.fromEntries(list.map((id) => [id, text(id)]));
  const out = {};

  // What the app drew from its example.
  out.opened = { figures: figures(["fire", "detected", "contained", "minor", "major", "loss", "b1", "b6"]), unbalanced: ids("g.vertex.unbalanced"), panel: panel(), status: status() };

  // A new outcome dropped from the palette, then a branch to it from
  // "detected": the app splits detected's branches evenly.
  const before = ids("g.vertex");
  await drop("outcome", empty());
  const added = ids("g.vertex").filter((id) => !before.includes(id));
  out.dropped = { added, status: status() };
  const edgesBefore = ids("g.edge");
  if (added.length === 1) await connect("detected", added[0]);
  const branch = ids("g.edge").filter((id) => !edgesBefore.includes(id));
  out.connected = { branch, figures: figures(["b3", "b4", "contained", "major"].concat(branch)), status: status(), unbalanced: ids("g.vertex.unbalanced") };

  // One undo takes back the branch and the split; a second, the outcome.
  await key("z", { ctrlKey: true });
  out.undone = { edges: ids("g.edge").length, figures: figures(["b3", "b4", "contained", "major"]) };
  await key("z", { ctrlKey: true });
  out.undoneTwice = { vertices: ids("g.vertex").length };

  // A click on a vertex shows it in the panel.
  await click(...centre(shape("detected")));
  out.selected = panel();
  await key("Escape", {});

  // A branch out of an outcome, which the schema forbids.
  const edges = ids("g.edge").length;
  await connect("loss", "contained");
  out.forbidden = { edges: ids("g.edge").length, edgesBefore: edges, status: status() };

  // Save, and open what was saved: the same figures.
  document.getElementById("save").click();
  const saved = document.getElementById("file").value;
  out.saved = JSON.parse(saved).edges.map((e) => [e.id, e.attrs && e.attrs.p]);
  document.getElementById("open").click();
  await wait(0);
  out.reopened = figures(["fire", "detected", "contained", "minor", "major", "loss"]);

  // Open a document that does not decode: the status says why.
  document.getElementById("file").value = '{"schema":"event-tree","vertices":[{"id":"x","kind":"nope"}]}';
  document.getElementById("open").click();
  await wait(0);
  out.badOpen = status();

  // An event with eight branches, and a ninth: the split needs nine
  // commands after the edit.
  const wide = { schema: "event-tree", vertices: [{ id: "e", kind: "event", label: "e" }], edges: [] };
  for (let i = 0; i <= 8; i++) wide.vertices.push({ id: "o" + i, kind: "outcome", label: "o" + i });
  for (let i = 0; i < 8; i++) wide.edges.push({ id: "w" + i, kind: "branch", from: "e", to: "o" + i, attrs: { p: "0.125" } });
  document.getElementById("file").value = JSON.stringify(wide);
  document.getElementById("open").click();
  await wait(0);
  const wideBefore = ids("g.edge");
  await connect("e", "o8");
  const ninth = ids("g.edge").filter((id) => !wideBefore.includes(id));
  out.wide = { ninth, figures: figures(wideBefore.concat(ninth)), status: status(), unbalanced: ids("g.vertex.unbalanced") };
  await key("z", { ctrlKey: true });
  out.wideUndone = { edges: ids("g.edge").length, figures: figures(wideBefore) };
  return out;
})().then(done, (err) => done({ fatal: String(err && err.stack || err) }));
