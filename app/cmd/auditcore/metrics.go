package auditcore

import (
	"bytes"
	"go/ast"
	"go/token"
)

type sourceMetrics struct {
	Lines                int
	TopLevelDeclarations int
	BranchPoints         int
}

func measureSource(file *ast.File, contents []byte) sourceMetrics {
	metrics := sourceMetrics{Lines: sourceLineCount(contents)}
	if file == nil {
		return metrics
	}
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			metrics.TopLevelDeclarations++
		case *ast.GenDecl:
			if value.Tok != token.IMPORT {
				metrics.TopLevelDeclarations += len(value.Specs)
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			metrics.BranchPoints++
		}
		return true
	})
	return metrics
}

func sourceLineCount(contents []byte) int {
	if len(contents) == 0 {
		return 0
	}
	lines := bytes.Count(contents, []byte{'\n'})
	if contents[len(contents)-1] != '\n' {
		lines++
	}
	return lines
}
