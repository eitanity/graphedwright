package analyzers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// FuncOf checks the browser binding: js.FuncOf is named in one function
// alone, funcOf, which makes the binding's functions once and counts
// them, so that no listener is made, and none left to release, after
// start-up.
var FuncOf = &analysis.Analyzer{
	Name: "funcof",
	Doc:  "js.FuncOf only in dom's funcOf",
	Run:  runFuncOf,
}

// funcOfHome is the one function in dom that may name js.FuncOf.
const funcOfHome = "funcOf"

func runFuncOf(pass *analysis.Pass) (any, error) {
	if !isConfig(pass) {
		return nil, nil
	}
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == funcOfHome {
				continue
			}
			ast.Inspect(d, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if fn, ok := pass.TypesInfo.Uses[id].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "syscall/js" && fn.Name() == "FuncOf" {
					pass.Reportf(id.Pos(), "js.FuncOf outside funcOf: make the function once there, and route by the event")
				}
				return true
			})
		}
	}
	return nil, nil
}
