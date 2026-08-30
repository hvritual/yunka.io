package assemblyplan

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAssemblyPlanProductionCodeIsLeafSafe(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Clean(entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("decode import %s in %s: %v", imported.Path.Value, path, err)
			}
			first := strings.Split(value, "/")[0]
			if strings.Contains(first, ".") {
				t.Fatalf("assemblyplan production code must stay leaf-safe/std-lib-only; %s imports %s", path, value)
			}
		}
	}
}
