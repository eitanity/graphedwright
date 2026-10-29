package analyzers

import (
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// modulePath is the main module every rule applies to.
const modulePath = "code.eitanity.com/graphedwright"

// corePkgs are the packages the core-only checks apply to: the document
// model, the layout, the renderer, the document format and the editor.
var corePkgs = [...]string{modulePath, modulePath + "/layout", modulePath + "/svg", modulePath + "/doc", modulePath + "/edit"}

// inModule reports whether the package being analysed is own code.
func inModule(pass *analysis.Pass) bool {
	p := pass.Pkg.Path()
	return p == modulePath || strings.HasPrefix(p, modulePath+"/")
}

// inEngine reports whether the package is one of the core packages or a
// subpackage of one.
func inEngine(pass *analysis.Pass) bool {
	p := pass.Pkg.Path()
	if p == modulePath {
		return true
	}
	for _, e := range corePkgs[1:] {
		if p == e || strings.HasPrefix(p, e+"/") {
			return true
		}
	}
	return false
}

// isConfig reports whether the package is dom, the browser binding and
// the one place that may use type switches and assertions.
func isConfig(pass *analysis.Pass) bool {
	return pass.Pkg.Path() == modulePath+"/dom"
}

// isTest reports whether pos lies in a _test.go file.
func isTest(pass *analysis.Pass, pos token.Pos) bool {
	return strings.HasSuffix(pass.Fset.Position(pos).Filename, "_test.go")
}
