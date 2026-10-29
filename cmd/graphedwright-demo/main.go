// Command graphedwright-demo serves the demo page with the browser
// build: the page from web/, the wasm from bin/ for either toolchain,
// each toolchain's wasm_exec.js from its own installation, and the
// default stylesheet.
//
//	graphedwright-demo [-addr 127.0.0.1:8001]
package main

import (
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.eitanity.com/graphedwright/svg"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8001", "the address to serve on")
	flag.Parse()
	srv := &http.Server{Addr: *addr, Handler: headers(handler(execJS(goToolchain), execJS(tinyToolchain))), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

// handler routes the pages, the builds and the stylesheet, given where
// each toolchain's wasm_exec.js is, found once at start.
func handler(goExec, tinyExec string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", noListing(http.FileServer(http.Dir("web"))))
	mux.HandleFunc("/graphedwright.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(svg.Stylesheet))
	})
	mux.HandleFunc("/graphedwright.wasm", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeFile(w, r, filepath.Join("bin", "graphedwright."+toolchain(r)+".wasm"))
	})
	mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		path := goExec
		if toolchain(r) == tinyToolchain {
			path = tinyExec
		}
		http.ServeFile(w, r, path)
	})
	return mux
}

// The toolchains a build comes from.
const (
	goToolchain   = "go"
	tinyToolchain = "tinygo"
)

// toolchain is "go" or "tinygo", from the query.
func toolchain(r *http.Request) string {
	if r.URL.Query().Get("toolchain") == tinyToolchain {
		return tinyToolchain
	}
	return goToolchain
}

// execJS is the path of a toolchain's wasm_exec.js.
func execJS(tc string) string {
	var cmd *exec.Cmd
	if tc == tinyToolchain {
		cmd = exec.Command("tinygo", "env", "TINYGOROOT")
	} else {
		cmd = exec.Command("go", "env", "GOROOT")
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(out))
	if tc == tinyToolchain {
		return filepath.Join(root, "targets", "wasm_exec.js")
	}
	return filepath.Join(root, "lib", "wasm", "wasm_exec.js")
}

// csp is the policy the demo runs under, the one a host page needs:
// scripts from the page's own origin and WebAssembly compiled from them
// ('wasm-unsafe-eval'), nothing inline. Inline styles are allowed only
// for the demo pages' own <style> blocks; the library needs none.
const csp = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// headers adds the security headers to every response, and keeps the
// browser from caching a build between runs.
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// noListing refuses a directory, which the file server would list,
// apart from the root, where it serves index.html.
func noListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
