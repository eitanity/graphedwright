package analyzers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// NoAny checks that the engine packages use no any or interface{}, and
// that no package but config uses a type switch or type assertion.
var NoAny = &analysis.Analyzer{
	Name: "noany",
	Doc:  "no any in the core packages; no type switches or assertions outside dom",
	Run:  runNoAny,
}

func runNoAny(pass *analysis.Pass) (any, error) {
	if !inModule(pass) {
		return nil, nil
	}
	engine := inEngine(pass)
	config := isConfig(pass)
	for _, f := range pass.Files {
		if isTest(pass, f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSwitchStmt:
				if !config {
					pass.Reportf(x.Pos(), "type switch outside dom")
				}
			case *ast.TypeAssertExpr:
				if !config && x.Type != nil {
					pass.Reportf(x.Pos(), "type assertion outside dom")
				}
			case *ast.InterfaceType:
				if engine && len(x.Methods.List) == 0 {
					pass.Reportf(x.Pos(), "empty interface in the core")
				}
			case *ast.Ident:
				if engine && isAnyType(pass, x) {
					pass.Reportf(x.Pos(), "any in the core")
				}
			}
			return true
		})
	}
	return nil, nil
}

// isAnyType reports whether id denotes the predeclared any.
func isAnyType(pass *analysis.Pass, id *ast.Ident) bool {
	if id.Name != "any" {
		return false
	}
	obj := pass.TypesInfo.Uses[id]
	tn, ok := obj.(*types.TypeName)
	return ok && tn.Pkg() == nil
}
