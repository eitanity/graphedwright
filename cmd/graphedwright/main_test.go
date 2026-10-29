package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const good = `{"schema":"basic","vertices":[{"id":"a","kind":"box","label":"a"},{"id":"b","kind":"pill","label":"b"}],"edges":[{"kind":"arrow","from":"a","to":"b"}]}`

func write(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExitCodes: 0 with the SVG or the check on stdout, 1 with the
// reason on stderr for a document refused, 2 for bad arguments.
func TestExitCodes(t *testing.T) {
	goodPath := write(t, "good.json", good)
	badPath := write(t, "bad.json", `{"schema":"basic","vertices":[{"kind":"blob"}]}`)
	for _, tc := range []struct {
		name string
		args []string
		code int
		out  string
		err  string
	}{
		{"no args", nil, 2, "", "usage:"},
		{"unknown command", []string{"draw"}, 2, "", "unknown command draw"},
		{"render without -f", []string{"render"}, 2, "", "-f is required"},
		{"bad flag", []string{"render", "--nope"}, 2, "", "flag provided but not defined"},
		{"missing file", []string{"render", "-f", "/nonexistent/x.json"}, 1, "", "no such file"},
		{"a directory", []string{"check", "-f", t.TempDir()}, 1, "", "is a directory"},
		{"refused", []string{"render", "-f", badPath}, 1, "", `unknown vertex kind "blob"`},
		{"render", []string{"render", "-f", goodPath}, 0, `<svg xmlns="http://www.w3.org/2000/svg"`, ""},
		{"render with layout and css", []string{"render", "-f", goodPath, "--layout", "--css"}, 0, `<style>`, ""},
		{"render top to bottom", []string{"render", "-f", goodPath, "--layout", "--flow", "top-to-bottom"}, 0, `<svg`, ""},
		{"render in an unknown flow", []string{"render", "-f", goodPath, "--layout", "--flow", "sideways"}, 1, "", `unknown flow "sideways"`},
		{"check", []string{"check", "-f", goodPath}, 0, "ok: 2 vertices, 1 edges", ""},
		{"check against the right schema", []string{"check", "-f", goodPath, "--schema", "basic"}, 0, "ok:", ""},
		{"check against another schema", []string{"check", "-f", goodPath, "--schema", "other"}, 1, "", `schema "basic" not supplied`},
	} {
		var stdout, stderr bytes.Buffer
		code := run(tc.args, &stdout, &stderr)
		if code != tc.code || !strings.Contains(stdout.String(), tc.out) || !strings.Contains(stderr.String(), tc.err) {
			t.Errorf("%s: code %d stdout %q stderr %q", tc.name, code, stdout.String(), stderr.String())
		}
	}
}

// TestBoundedRead: a file far past the reader's limit is refused after
// reading one byte more than the limit, not the whole file.
func TestBoundedRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.json")
	f, err := os.Create(path) //nolint:gosec // the test's own file
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(4 << 30); err != nil { // sparse: 4 GiB on paper, nothing on disk
		t.Fatal(err)
	}
	_ = f.Close()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "-f", path}, &stdout, &stderr)
	runtime.ReadMemStats(&after)
	if code != 1 || !strings.Contains(stderr.String(), "document of 16777217 bytes exceeds the limit of 16777216") {
		t.Errorf("code %d stderr %q", code, stderr.String())
	}
	if read := after.TotalAlloc - before.TotalAlloc; read > 128<<20 {
		t.Errorf("allocated %d MiB to refuse it", read>>20)
	}
}

func TestItoa(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 1234: "1234"} { //nolint:gocritic // a small table
		if itoa(n) != want {
			t.Errorf("itoa(%d) = %q", n, itoa(n))
		}
	}
}
