package auditcore

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestMeasureSourceCountsDeclaredStructuralMetrics(t *testing.T) {
	contents := []byte(`package sample

var A = 1

type B struct{}

func Work(v int) {
	if v > 0 {
		for i := 0; i < v; i++ {
			switch i {
			case 0:
			case 1:
			}
		}
	}
}
`)
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", contents, 0)
	if err != nil {
		t.Fatal(err)
	}
	metrics := measureSource(file, contents)
	if metrics.Lines != 16 {
		t.Fatalf("lines=%d want=16", metrics.Lines)
	}
	if metrics.TopLevelDeclarations != 3 {
		t.Fatalf("topLevelDeclarations=%d want=3", metrics.TopLevelDeclarations)
	}
	if metrics.BranchPoints != 5 {
		t.Fatalf("branchPoints=%d want=5", metrics.BranchPoints)
	}
}
