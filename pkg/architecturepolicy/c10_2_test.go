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
	capabilityCompiler := read("pkg/contract/assembly_capability_codegen.go")
	join := read("pkg/contract/assembly_compiler.go")
	moduleBinding := read("pkg/contract/module_binding.go")
	moduleSnapshot := read("pkg/contract/module_snapshot.go")
	moduleCodegen := read("pkg/contract/assembly_module_codegen.go")
	cli := read("app/cmd/assembly/command.go")
	combined := compiler + capabilityCompiler + join + moduleBinding + moduleSnapshot + moduleCodegen + cli

	for _, forbidden := range []string{
		"reflect.", "plugin.Open", "go/packages", "filepath.Walk", "ServiceLocator", "serviceLocator",
		"map[string]any", "map[string]interface{}", "exec.Command(",
	} {
		if strings.Contains(combined, forbidden) {
			t.Errorf("C10.2 compiler contains forbidden runtime/discovery token %q", forbidden)
		}
	}
	for _, forbidden := range []string{"kernel.New(", ".Start("} {
		if strings.Contains(compiler+capabilityCompiler+join+moduleCodegen, forbidden) {
			t.Errorf("C10.2 generated compiler surface crosses into C10.3 runtime closure with %q", forbidden)
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
		"DiscoverModuleSnapshot",
		"ModuleName",
		"autoload",
		"GeneratedDescriptor",
		"func NewCatalog(additional ...modulecatalog.Descriptor)",
		"for _, descriptor := range additional",
		"func BindAssemblyCapabilities",
		"BuildApplicationsWithCapabilities",
		"CompileBoundAssembly",
		"assembly check ok",
	} {
		if !strings.Contains(combined, required) {
			t.Errorf("C10.2 compiler is missing structural reuse marker %q", required)
		}
	}
	if strings.Contains(compiler+moduleCodegen, `rootImport+"/"+module.Name`) || strings.Contains(compiler+moduleCodegen, "module.Name+\"/\"") {
		t.Error("C10.2 must not infer module Go import paths from module names")
	}
	if strings.Contains(moduleBinding, "Name: entry.Name()") || strings.Contains(moduleSnapshot, "Name: entry.Name()") {
		t.Error("C10.2 must not derive module identity from directory names")
	}
	if strings.Contains(moduleCodegen+capabilityCompiler, "modulecatalog.Default()") {
		t.Error("C10.2 generated composition must remain explicit and must not discover a default catalog")
	}
}
