package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandler: every response carries the security headers, a directory
// other than the root is not listed, the pages are served from web/,
// and wasm_exec.js comes from the toolchain the query names.
func TestHandler(t *testing.T) {
	dir := t.TempDir()
	goExec, tinyExec := filepath.Join(dir, "go.js"), filepath.Join(dir, "tiny.js")
	for path, text := range map[string]string{goExec: "// go", tinyExec: "// tinygo"} { //nolint:gocritic // two files
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join("..", ".."))
	h := headers(handler(goExec, tinyExec))
	for _, tc := range []struct {
		path string
		code int
		body string
	}{
		{"/", http.StatusOK, "<title>"},
		{"/tutorial.js", http.StatusOK, "graphedwright.create"},
		{"/examples/diamond.json", http.StatusOK, `"schema"`},
		{"/examples/", http.StatusNotFound, ""},
		{"/graphedwright.css", http.StatusOK, "The default look of a graphedwright drawing"},
		{"/wasm_exec.js", http.StatusOK, "// go"},
		{"/wasm_exec.js?toolchain=tinygo", http.StatusOK, "// tinygo"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s: %d %.80q", tc.path, rec.Code, rec.Body.String())
		}
		for _, k := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "Cache-Control"} {
			if rec.Header().Get(k) == "" {
				t.Errorf("%s: no %s", tc.path, k)
			}
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval';") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("%s: policy %q", tc.path, csp)
		}
	}
}
