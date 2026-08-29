package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC102AssemblyCompilerRemainsStructuralAndOneWay(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}
	compiler := read("pkg/contract/assembly_codegen.go")
	join := read("pkg/contract/assembly_compiler.go")

	for _, forbidden := range []string{
		"reflect.", "plugin.Open", "go/packages", "filepath.Walk", "os.ReadDir",
		"ServiceLocator", "serviceLocator", "map[string]any", "map[string]interface{}",
		"kernel.New(", ".Start(", "exec.Command(",
	} {
		if strings.Contains(compiler, forbidden) || strings.Contains(join, forbidden) {
			t.Errorf("C10.2 compiler contains forbidden runtime/discovery token %q", forbidden)
		}
	}
	for _, required := range []string{
		"type ApplicationFactories interface",
		"New%sChildCapability",
		"c9RegisterName",
		"ExpectedModuleRequirements",
		"KernelOptions",
		"CompileAssemblyPlan",
		"AssemblyPlanFilename",
	} {
		if !strings.Contains(compiler+join, required) {
			t.Errorf("C10.2 compiler is missing structural reuse marker %q", required)
		}
	}
	if strings.Contains(compiler, `rootImport+"/"+module.Name`) || strings.Contains(compiler, "module.Name+\"/\"") {
		t.Error("C10.2 must not infer module Go import paths from module names")
	}
}
