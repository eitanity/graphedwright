package analyzers

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Sealed checks that a package keeps its state its own, in the core
// packages and dom: a state type has no exported field, and no exported
// method returns a slice, a map, or a pointer to a struct with an
// exported field, taken from its receiver, through which a caller could
// change the state past the methods that check it. A slice the caller
// passed in, a value that holds one in an unexported field, and a
// pointer to a struct with no exported field are allowed.
var Sealed = &analysis.Analyzer{
	Name: "sealed",
	Doc:  "state types export no fields, and exported methods return no slice, map or open pointer taken from the receiver",
	Run:  runSealed,
}

// stateTypes are the types that hold a package's state.
var stateTypes = [...]string{"Document", "Kinds", "Editor", "Handle", "Layouter", "Renderer"}

// aliasPasses bounds the passes that find the locals holding part of the
// receiver: each pass follows one more step of a chain of assignments.
const aliasPasses = 8

func runSealed(pass *analysis.Pass) (any, error) {
	if !inEngine(pass) && !isConfig(pass) {
		return nil, nil
	}
	for _, f := range pass.Files {
		if isTest(pass, f.Pos()) {
			continue
		}
		for _, decl := range f.Decls {
			switch x := decl.(type) {
			case *ast.GenDecl:
				checkStateFields(pass, x)
			case *ast.FuncDecl:
				checkReturns(pass, x)
			}
		}
	}
	return nil, nil
}

// checkStateFields reports an exported field of a state type.
func checkStateFields(pass *analysis.Pass, g *ast.GenDecl) {
	if g.Tok != token.TYPE {
		return
	}
	for _, spec := range g.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || !isState(ts.Name.Name) {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if name.IsExported() {
					pass.Reportf(name.Pos(), "state type %s exports field %s", ts.Name.Name, name.Name)
				}
			}
			if len(field.Names) == 0 && isExportedEmbed(field.Type) {
				pass.Reportf(field.Pos(), "state type %s embeds an exported type", ts.Name.Name)
			}
		}
	}
}

// isExportedEmbed reports whether an embedded field's type is exported,
// which makes the field exported.
func isExportedEmbed(t ast.Expr) bool {
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.Ident:
		return x.IsExported()
	case *ast.SelectorExpr:
		return x.Sel.IsExported()
	}
	return false
}

func isState(name string) bool {
	for _, s := range stateTypes {
		if s == name {
			return true
		}
	}
	return false
}

// checkReturns reports an exported method on an exported type that
// returns, as a slice, a map or an open pointer, something taken from
// its receiver.
func checkReturns(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Recv == nil || fn.Body == nil || !fn.Name.IsExported() || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
		return
	}
	recvObj, ok := pass.TypesInfo.Defs[fn.Recv.List[0].Names[0]].(*types.Var)
	if !ok || !exportedNamed(recvObj.Type()) {
		return
	}
	derived := map[types.Object]bool{recvObj: true}
	// bound: aliasPasses passes, each over the function's assignments
	for range aliasPasses {
		grew := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != len(as.Rhs) {
				return true
			}
			for i, l := range as.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok {
					continue
				}
				obj := pass.TypesInfo.ObjectOf(id)
				if obj != nil && !derived[obj] && leaks(pass, as.Rhs[i]) && from(pass, as.Rhs[i], derived) {
					derived[obj], grew = true, true
				}
			}
			return true
		})
		if !grew {
			break
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, r := range ret.Results {
			if leaks(pass, r) && from(pass, r, derived) {
				pass.Reportf(r.Pos(), "%s returns %s taken from its receiver, through which a caller could change it", fn.Name.Name, kindOf(pass.TypesInfo.TypeOf(r)))
			}
		}
		return true
	})
}

// exportedNamed reports whether t, or what it points to, is an exported
// named type.
func exportedNamed(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj().Exported()
}

// leaks reports whether a value of the expression's type could write
// into what it came from: a slice, a map, or a pointer to a struct with
// an exported field.
func leaks(pass *analysis.Pass, e ast.Expr) bool {
	switch t := pass.TypesInfo.TypeOf(e).Underlying().(type) {
	case *types.Slice, *types.Map:
		return true
	case *types.Pointer:
		st, ok := t.Elem().Underlying().(*types.Struct)
		if !ok {
			return true
		}
		for i := range st.NumFields() {
			if st.Field(i).Exported() {
				return true
			}
		}
	}
	return false
}

// from reports whether the expression reaches, through selectors,
// indices, slicing, dereferences and addresses, an object marked
// derived: the receiver or a local holding part of it.
func from(pass *analysis.Pass, e ast.Expr, derived map[types.Object]bool) bool {
	// bound: one step down the expression each time, at most its depth
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return false
			}
			e = x.X
		case *ast.Ident:
			return derived[pass.TypesInfo.ObjectOf(x)]
		default:
			return false
		}
	}
}

// kindOf names what is returned.
func kindOf(t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Slice:
		return "a slice"
	case *types.Map:
		return "a map"
	}
	return "a pointer"
}
