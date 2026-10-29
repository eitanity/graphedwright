// Command graphedvet runs the custom analyzers as a go vet tool:
//
//	go vet -vettool=bin/graphedvet ./...
package main

import (
	"code.eitanity.com/graphedwright/internal/lint/analyzers"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		analyzers.LoopBound,
		analyzers.NoFuncType,
		analyzers.NoAny,
		analyzers.Stmts,
		analyzers.Unordered,
	)
}
