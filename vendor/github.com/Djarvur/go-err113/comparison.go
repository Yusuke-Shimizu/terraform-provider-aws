package err113

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

func inspectComparision(pass *analysis.Pass, n ast.Node) bool { // nolint: unparam
	// check whether the call expression matches time.Now().Sub()
	be, ok := n.(*ast.BinaryExpr)
	if !ok {
		return true
	}

	// check if it is a comparison operation
	if be.Op != token.EQL && be.Op != token.NEQ {
		return true
	}

	// check that both left and right hand side are not nil
	if pass.TypesInfo.Types[be.X].IsNil() || pass.TypesInfo.Types[be.Y].IsNil() {
		return true
	}

	// check that both left and right hand side are not io.EOF
	if isEOF(be.X, pass.TypesInfo) || isEOF(be.Y, pass.TypesInfo) {
		return true
	}

	// check that both left and right hand side are errors
	if !isError(be.X, pass.TypesInfo) && !isError(be.Y, pass.TypesInfo) {
		return true
	}

	oldExpr := render(pass.Fset, be)

	negate := ""
	if be.Op == token.NEQ {
		negate = "!"
	}

	newExpr := fmt.Sprintf("%s%s.Is(%s, %s)", negate, "errors", be.X, be.Y)

	pass.Report(
		analysis.Diagnostic{
			Pos:     be.Pos(),
			Message: fmt.Sprintf("do not compare errors directly, use errors.Is() instead: %q", oldExpr),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: fmt.Sprintf("should replace %q with %q", oldExpr, newExpr),
					TextEdits: []analysis.TextEdit{
						{
							Pos:     be.Pos(),
							End:     be.End(),
							NewText: []byte(newExpr),
						},
					},
				},
			},
		},
	)

	return true
}

func isError(v ast.Expr, info *types.Info) bool {
	if intf, ok := info.TypeOf(v).Underlying().(*types.Interface); ok {
		return intf.NumMethods() == 1 && intf.Method(0).FullName() == "(error).Error"
	}

	return false
}

func isEOF(ex ast.Expr, info *types.Info) bool {
	se, ok := ex.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != "EOF" {
		return false
	}

	if ep, ok := asImportedName(se.X, info); !ok || ep != "io" {
		return false
	}

	return true
}

func asImportedName(ex ast.Expr, info *types.Info) (string, bool) {
	ei, ok := ex.(*ast.Ident)
	if !ok {
		return "", false
	}

	ep, ok := info.ObjectOf(ei).(*types.PkgName)
	if !ok {
		return "", false
	}

	return ep.Imported().Name(), true
}
