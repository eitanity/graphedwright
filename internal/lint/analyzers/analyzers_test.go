package analyzers_test

import (
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

func TestUnordered(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Unordered, mod+"/um")
}

func TestStmts(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analyzers.Stmts, mod+"/st", mod+"/dom")
}
