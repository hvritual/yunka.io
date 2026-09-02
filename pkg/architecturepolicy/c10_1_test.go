package architecturepolicy

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestC101AssemblyPlanRemainsLeafSafeDataIR(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	root := filepath.Join(repoRoot, "pkg", "assemblyplan")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(root, entry.Name())
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
				t.Errorf("leaf AssemblyPlan IR must remain stdlib-only; %s imports %s", entry.Name(), value)
			}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, forbidden := range []string{
			"reflect.", "sync.Pool", "ServiceLocator", "serviceLocator", "registry", "Register(",
			"gorm.", "grpc.", "core.App", "modulecatalog.", "gateway/", "framework/",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("leaf AssemblyPlan IR contains forbidden runtime/registry token %q in %s", forbidden, entry.Name())
			}
		}
	}
}

func TestC101AssemblyAdaptersRemainOneWay(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}
	contractAdapter := read("pkg/contract/assembly_plan.go")
	moduleAdapter := read("framework/core/modulecatalog/assembly.go")

	if strings.Contains(contractAdapter, "github.com/hvritual/yunka.io/framework") || strings.Contains(contractAdapter, "modulecatalog") {
		t.Error("pkg/contract AssemblyPlan projection must not depend on framework/modulecatalog")
	}
	if !strings.Contains(contractAdapter, "OperationPlansFilename") || !strings.Contains(contractAdapter, "BindingInput") {
		t.Error("contract adapter must project binding references from canonical OperationPlan facts")
	}
	if !strings.Contains(contractAdapter, "CompileOperationPlans") || !strings.Contains(contractAdapter, "operationplan.Digest") {
		t.Error("contract adapter must reject Manifest/OperationPlan drift before producing AssemblyPlan")
	}
	if !strings.Contains(moduleAdapter, "assemblyplan.ModuleInput") {
		t.Error("module catalog must snapshot descriptor facts into leaf-safe assembly input")
	}
	if strings.Contains(moduleAdapter, ".Build") || strings.Contains(moduleAdapter, "BuildFunc") || strings.Contains(moduleAdapter, "ContextFactory") {
		t.Error("module AssemblyPlan snapshot must not expose runtime builders/providers")
	}
}
