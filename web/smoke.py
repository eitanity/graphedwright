#!/usr/bin/env python3
"""The browser smoke test: serve the demo, open smoke.html in headless
Firefox through geckodriver for the toolchain named, and compare what
the browser build did with the native command.

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
    if "2.1/period" not in out["svgAnnotated"]["ok"] or 'class="edge curve" id="e2"' not in out["svgAnnotated"]["ok"]:
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


if __name__ == "__main__":
    main()
