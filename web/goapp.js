// The loader of a page whose application is written in Go, and all the
// page's script: it runs the WebAssembly build its script tag names in
// data-wasm, for the toolchain the query names, and puts a failure to
// load on the page's root as data-error.
"use strict";
const app = document.currentScript.dataset.wasm;
const toolchain = new URLSearchParams(location.search).get("toolchain") === "tinygo" ? "tinygo" : "go";
const exec = document.createElement("script");
exec.src = "wasm_exec.js?toolchain=" + toolchain;
exec.onload = () => {
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch(app + ".wasm?toolchain=" + toolchain), go.importObject)
    .then(({ instance }) => go.run(instance))
    .catch((err) => { document.documentElement.dataset.error = String(err); });
};
document.head.appendChild(exec);
