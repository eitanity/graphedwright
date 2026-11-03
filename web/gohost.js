// The loader of the Go host smoke test, and all the page's script: the
// host itself is Go (cmd/graphedwright-gohost), which mounts the editor,
// drives it and writes what it saw into #out.
"use strict";
const toolchain = new URLSearchParams(location.search).get("toolchain") === "tinygo" ? "tinygo" : "go";
const exec = document.createElement("script");
exec.src = "wasm_exec.js?toolchain=" + toolchain;
exec.onload = () => {
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("graphedwright-gohost.wasm?toolchain=" + toolchain), go.importObject)
    .then(({ instance }) => go.run(instance))
    .catch((err) => { document.getElementById("out").textContent = JSON.stringify([{ name: "load", ok: false, detail: String(err) }]); });
};
document.head.appendChild(exec);
