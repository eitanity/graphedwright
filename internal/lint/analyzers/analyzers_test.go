package analyzers_test

import (
	"path/filepath"
	"testing"

	"code.eitanity.com/graphedwright/internal/lint/analyzers"
	"golang.org/x/tools/go/analysis/analysistest"
)

const mod = "code.eitanity.com/graphedwright"

func TestLoopBound(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.LoopBound, mod, mod+"/dom")
}

func TestNoFuncType(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.NoFuncType, mod+"/layout")
}

func TestNoAny(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.NoAny, mod+"/svg", mod+"/dom", mod+"/cmd/x")
}

func TestDiscarded(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Discarded, mod+"/dc")
}

func TestUnordered(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Unordered, mod+"/um")
}

func TestSealed(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Sealed, mod+"/edit")
}

func TestStmts(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Stmts, mod+"/st", mod+"/dom")
}

// TestFuncOf loads the binding's fixture, a tree of its own, as the
// browser build, where syscall/js is.
func TestFuncOf(t *testing.T) {
	t.Setenv("GOOS", "js")
	t.Setenv("GOARCH", "wasm")
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "js"), analyzers.FuncOf, mod+"/dom")
}
