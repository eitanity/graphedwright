package analyzers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Discarded checks every package of the module outside tests: an error
// assigned to the blank identifier carries a "// discarded:" comment
// saying why nothing can come of it, since every error is checked (the
// coding rules' rule 24). errcheck does not look at blank assignments.
var Discarded = &analysis.Analyzer{
	Name: "discarded",
	Doc:  "an error assigned to _ states why it is discarded",
	Run:  runDiscarded,
}

func runDiscarded(pass *analysis.Pass) (any, error) {
	errType, _ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	for _, f := range pass.Files {
		if isTest(pass, f.Pos()) {
			continue
		}
		cmap := ast.NewCommentMap(pass.Fset, f, f.Comments)
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || !blankError(pass, as, errType) || hasComment(pass, cmap, as, "discarded:") {
				return true
			}
			pass.Reportf(as.Pos(), "an error assigned to _ without a stated reason: check it, or add a // discarded: comment saying why nothing can come of it")
			return true
		})
	}
	return nil, nil
}

// blankError reports whether the assignment gives an error to the blank
// identifier, from a value of its own or from one of a call's results.
func blankError(pass *analysis.Pass, as *ast.AssignStmt, errType *types.Interface) bool {
	var results *types.Tuple
	if len(as.Rhs) == 1 {
		results, _ = pass.TypesInfo.TypeOf(as.Rhs[0]).(*types.Tuple)
	}
	for i, lhs := range as.Lhs {
		if id, ok := lhs.(*ast.Ident); !ok || id.Name != "_" {
			continue
		}
		var t types.Type
		switch {
		case results != nil && i < results.Len():
			t = results.At(i).Type()
		case len(as.Rhs) == len(as.Lhs):
			t = pass.TypesInfo.TypeOf(as.Rhs[i])
		}
		if t != nil && types.Implements(t, errType) {
			return true
		}
	}
	return false
}
