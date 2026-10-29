package analyzers

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Stmts enforces the statement bans: no goto, no recover, and no go
// statement anywhere in the module.
var Stmts = &analysis.Analyzer{
	Name: "stmts",
	Doc:  "no goto, no recover, no goroutines",
	Run:  runStmts,
}

func runStmts(pass *analysis.Pass) (any, error) {
	if !inModule(pass) {
		return nil, nil
	}
	for _, f := range pass.Files {
		if isTest(pass, f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BranchStmt:
				if x.Tok == token.GOTO {
					pass.Reportf(x.Pos(), "goto")
				}
			case *ast.GoStmt:
				pass.Reportf(x.Pos(), "go statement")
			case *ast.CallExpr:
				if isBuiltin(pass, x.Fun, "recover") {
					pass.Reportf(x.Pos(), "recover")
				}
			}
			return true
		})
	}
	return nil, nil
}

// isBuiltin reports whether e names the predeclared function name.
func isBuiltin(pass *analysis.Pass, e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name != name {
		return false
	}
	_, ok = pass.TypesInfo.Uses[id].(*types.Builtin)
	return ok
}
