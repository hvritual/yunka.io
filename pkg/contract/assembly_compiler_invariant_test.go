package contract

import (
	"strings"
	"testing"
)

func TestAssemblyCompilationRejectsPlanArtifactMismatch(t *testing.T) {
	manifest, qualified := c102AssemblyFixture(t)
	compilation, err := CompileAssembly(manifest, assemblyModuleInputs(qualified), AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	compilation.PlanJSON = append([]byte(nil), compilation.PlanJSON...)
	compilation.PlanJSON[len(compilation.PlanJSON)-2] = ' '
	if err := validateAssemblyCompilation(compilation); err == nil || !strings.Contains(err.Error(), "plan and plan artifact") {
		t.Fatalf("expected compilation artifact mismatch, got %v", err)
	}
}
