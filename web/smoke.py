#!/usr/bin/env python3
"""The browser smoke test: serve the demo, open smoke.html in headless
Firefox through geckodriver for the toolchain named, and compare what
the browser build did with the native command, then open gohost.html,
whose editor a host written in Go mounts and drives, and require every
check it reports.

    python3 -I web/smoke.py go|tinygo native.svg

native.svg is the graphedwright command's render of
web/examples/diamond.json. The page's export must read back as the
example, its drawing must equal the native one, the annotated drawing
must carry the annotation, the DOM must hold every element, and the
refusals must come back as {error}.
"""
import json
import os
import signal
import subprocess
import sys
import time
import urllib.request

PORT = 8019
DRIVER_PORT = 4446


def call(base, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.load(r)["value"]


def main():
    toolchain, native_path = sys.argv[1], sys.argv[2]
    native = open(native_path, encoding="utf-8").read()
    example = json.load(open("web/examples/diamond.json", encoding="utf-8"))
    # The server is a process group of its own, so that the binary go run
    # starts goes with it; its output is not ours.
    server = subprocess.Popen(["go", "run", "./cmd/graphedwright-demo", "-addr", f"127.0.0.1:{PORT}"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    driver = subprocess.Popen(["geckodriver", "--port", str(DRIVER_PORT)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    base = f"http://127.0.0.1:{DRIVER_PORT}"
    time.sleep(2)
    sess = None
    try:
        sess = call(base, "POST", "/session", {"capabilities": {"alwaysMatch": {"browserName": "firefox", "moz:firefoxOptions": {"args": ["-headless"]}}}})["sessionId"]
        s = f"/session/{sess}"
        for _ in range(30):
            try:
                urllib.request.urlopen(f"http://127.0.0.1:{PORT}/smoke.html", timeout=5)
                break
            except Exception:
                time.sleep(1)
        call(base, "POST", s + "/url", {"url": f"http://127.0.0.1:{PORT}/smoke.html?toolchain={toolchain}"})
        out = None
        for _ in range(60):
            text = call(base, "POST", s + "/execute/sync", {"script": "return document.getElementById('out').textContent", "args": []})
            if text:
                out = json.loads(text)
                break
            time.sleep(0.5)
        if out is None:
            raise SystemExit("the page produced nothing")
        if "fatal" in out:
            raise SystemExit("page: " + out["fatal"])
        check(out, native, example)
        print(f"smoke {toolchain}: ok")
        # The Go host: a page whose only script is the loader, and an
        # editor mounted and driven from Go, which writes its checks.
        call(base, "POST", s + "/url", {"url": f"http://127.0.0.1:{PORT}/gohost.html?toolchain={toolchain}"})
        checks = None
        for _ in range(60):
            text = call(base, "POST", s + "/execute/sync", {"script": "return document.getElementById('out').textContent", "args": []})
            if text:
                checks = json.loads(text)
                break
            time.sleep(0.5)
        if not checks:
            raise SystemExit("the Go host produced nothing")
        failed = [c for c in checks if not c["ok"]]
        if failed:
            raise SystemExit("go host:\n" + "\n".join(f"{c['name']}: {c['detail']}" for c in failed))
        print(f"smoke {toolchain} go host: ok, {len(checks)} checks")
        # The example application, an event-tree editor written in Go,
        # worked by a script that uses only what a person could.
        call(base, "POST", s + "/url", {"url": f"http://127.0.0.1:{PORT}/eventtree.html?toolchain={toolchain}"})
        script = open("web/smoke_eventtree.js", encoding="utf-8").read()
        seen = call(base, "POST", s + "/execute/async", {"script": script, "args": []})
        if "fatal" in seen:
            raise SystemExit("event tree: " + seen["fatal"])
        check_eventtree(seen)
        print(f"smoke {toolchain} event tree: ok")
    finally:
        if sess:
            call(base, "DELETE", f"/session/{sess}")
        for p in (driver, server):
            try:
                os.killpg(p.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass


def check(out, native, example):
    for key in ("load", "loadObject", "setMode", "annotate", "registered", "tinyLoad", "destroyed"):
        if out[key].get("ok") is not True:
            raise SystemExit(f"{key}: {out[key]}")
    exported = out["export"]["ok"]
    if exported != example:
        raise SystemExit("export differs from the example:\n" + json.dumps(exported, indent=1))
    if out["svg"]["ok"] != native:
        raise SystemExit("the browser's drawing differs from the native one:\n" + out["svg"]["ok"][:400])
    if "2.1/period" not in out["svgAnnotated"]["ok"] or 'class="edge kind-curve" id="e2"' not in out["svgAnnotated"]["ok"]:
        raise SystemExit("annotation missing")
    if out["domVertices"] != 5 or out["domEdges"] != 5:
        raise SystemExit(f"DOM holds {out['domVertices']} vertices and {out['domEdges']} edges")
    if "not known" not in out["badSchema"].get("error", "") or "unknown vertex kind" not in out["badLoad"].get("error", ""):
        raise SystemExit(f"refusals: {out['badSchema']} {out['badLoad']}")
    if "capacity vertices 2147483647 is not between" not in out["hugeCaps"].get("error", ""):
        raise SystemExit(f"huge capacity: {out['hugeCaps']}")
    if "layout needs 2162683904 dummy vertices" not in out["wideLoad"].get("error", ""):
        raise SystemExit(f"wide document: {out['wideLoad']}")
    if "layout needs 1048577 layers" not in out["farLoad"].get("error", ""):
        raise SystemExit(f"far document: {out['farLoad']}")
    tiny = out["tinyExport"]["ok"]
    if len(tiny["vertices"]) != 2 or len(tiny["edges"]) != 1 or tiny["vertices"][1]["x"] <= tiny["vertices"][0]["x"]:
        raise SystemExit("the registered schema's document: " + json.dumps(tiny))
    if "no such handle" not in out["afterDestroy"].get("error", ""):
        raise SystemExit(f"after destroy: {out['afterDestroy']}")
    if out["clashLoad"].get("ok") is not True or out["clashAnnotate"].get("ok") is not True:
        raise SystemExit(f"clash load and annotate: {out['clashLoad']} {out['clashAnnotate']}")
    if not out["panSlip"] < 0.01:
        raise SystemExit(f"a middle-button drag let the drawing slip {out['panSlip']} units from under the pointer")
    for mode in ("view", "edit"):
        if out["hover"][mode] != {"long": ["a label far too long to fit"], "short": [], "background": []}:
            raise SystemExit(f"hover in {mode} mode drew {out['hover'][mode]}")
    if out["fullText"] is not True or out["hoverOff"] != [] or out["fullTextOff"] is not False:
        raise SystemExit(f"switches: full text {out['fullText']}, hover off {out['hoverOff']}, full text off {out['fullTextOff']}")
    if out["clashSelected"] != {"a": True, "selection": True} or not out["clashDeleted"] or not out["clashRestored"]:
        raise SystemExit(f"kinds and classes named as the binding's groups: selected {out['clashSelected']}, deleted {out['clashDeleted']}, restored {out['clashRestored']}")

    f = out["fns"]
    for key in ("on", "zoom", "fit", "setLabel", "setAttr", "addPort", "deletePort", "redo"):
        if f[key].get("ok") is not True:
            raise SystemExit(f"{key}: {f[key]}")
    laid = f["layout"].get("ok")
    if not isinstance(laid, dict) or not isinstance(laid.get("layers"), int) or not f["zoomed"] or not f["fitted"]:
        raise SystemExit(f"layout, zoom and fit: {f['layout']} zoomed {f['zoomed']} fitted {f['fitted']}")
    if (f["properties"].get("ok") or {}).get("id") != "p":
        raise SystemExit(f"properties of the vertex clicked: {f['properties']}")
    if f["effects"] != {"ported": True, "unported": True, "undone": True, "redone": True, "label": "renamed", "note": True} or f["heard"] < 5:
        raise SystemExit(f"the functions' effects: {f['effects']}, changes heard {f['heard']}")


def check_eventtree(seen):
    """What a person using the event-tree example would see: the figures
    the app computed, the split its rule made, one undo for an edit and
    its split, the panel, a refusal, Save and Open, and a split longer
    than the default follow-up bound."""
    def fail(step):
        raise SystemExit(f"event tree, {step}: " + json.dumps(seen[step]))

    def drawn(figs, want):
        # A vertex's text is its label then its figure, the figure in full
        # last when it is cut; an edge's is its figure alone.
        return all(figs.get(k) is not None and (figs[k] == v if v.startswith("p = ") else figs[k].endswith(v)) for k, v in want.items())

    opened = {"fire": "P = 1", "detected": "P = 0.8", "contained": "P = 0.4, branches sum to 0.9", "minor": "P = 0.24", "major": "P = 0.52", "loss": "P = 0.2", "b1": "p = 0.8", "b6": "p = 0.3"}
    o = seen["opened"]
    if not drawn(o["figures"], opened) or o["unbalanced"] != ["contained"] or o["status"] != "" or not o["panel"].startswith("Select"):
        fail("opened")
    if len(seen["dropped"]["added"]) != 1 or seen["dropped"]["status"] != "":
        fail("dropped")
    c = seen["connected"]
    third = "p = 0.3333"
    if len(c["branch"]) != 1 or not drawn(c["figures"], {"b3": third, "b4": third, c["branch"][0]: third, "contained": "P = 0.2667, branches sum to 0.9", "major": "P = 0.3467"}) or c["status"] != "" or c["unbalanced"] != ["contained"]:
        fail("connected")
    u = seen["undone"]
    if u["edges"] != 6 or not drawn(u["figures"], {"b3": "p = 0.5", "b4": "p = 0.5", "contained": "P = 0.4, branches sum to 0.9", "major": "P = 0.52"}):
        fail("undone")
    if seen["undoneTwice"]["vertices"] != 6:
        fail("undoneTwice")
    lines = seen["selected"].split("\n")
    if lines[0] != "detected: P = 0.8" or sorted(lines[1:]) != ["  → contained, p = 0.5", "  → major damage, p = 0.5"]:
        fail("selected")
    f = seen["forbidden"]
    if f["edges"] != f["edgesBefore"] or f["status"] != "no edge kind joins these vertices":
        fail("forbidden")
    if dict(seen["saved"]) != {"b1": "0.8", "b2": "0.2", "b3": "0.5", "b4": "0.5", "b5": "0.6", "b6": "0.3"}:
        fail("saved")
    if not drawn(seen["reopened"], {k: v for k, v in opened.items() if not k.startswith("b")}):
        fail("reopened")
    if "unknown vertex kind" not in seen["badOpen"]:
        fail("badOpen")
    w = seen["wide"]
    ninth = "p = 0.1111"
    if len(w["ninth"]) != 1 or len(w["figures"]) != 9 or any(v != ninth for v in w["figures"].values()) or w["status"] != "" or w["unbalanced"] != []:
        fail("wide")
    wu = seen["wideUndone"]
    if wu["edges"] != 8 or any(v != "p = 0.125" for v in wu["figures"].values()):
        fail("wideUndone")


if __name__ == "__main__":
    main()
