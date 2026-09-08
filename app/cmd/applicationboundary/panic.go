package applicationboundary

import (
	"go/ast"
	"go/types"
)

func isBuiltinPanic(info *types.Info, expr ast.Expr) bool {
	unparen := func(expr ast.Expr) ast.Expr {
		for {
			p, ok := expr.(*ast.ParenExpr)
			if !ok {
				return expr
			}
			expr = p.X
		}
	}
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok || info == nil {
		return false
	}
	id, ok := unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := info.Uses[id].(*types.Builtin)
	return ok && builtin.Name() == "panic"
}
