package contract

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

func TestCompileAssemblyEmitsPlanAndGoFromOneJoinedFactSet(t *testing.T) {
	manifest, qualified := c102AssemblyFixture(t)
	modules := assemblyModuleInputs(qualified)
	compilation, err := CompileAssembly(manifest, modules, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	wantPlan, err := assemblyplan.CanonicalJSON(qualified)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compilation.PlanJSON, wantPlan) {
		t.Fatalf("joined compiler emitted different AssemblyPlan:\nwant=%s\ngot=%s", wantPlan, compilation.PlanJSON)
	}
	if len(compilation.GoFiles) != 1 || compilation.GoFiles[0].Path != AssemblyCodePath {
		t.Fatalf("unexpected joined Go artifacts: %#v", compilation.GoFiles)
	}

	contractOut := filepath.Join(t.TempDir(), "contracts", "generated")
	codeRoot := filepath.Join(t.TempDir(), "internal")
	if err := WriteAssemblyCompilation(contractOut, codeRoot, compilation); err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckAssemblyCompilation(contractOut, codeRoot, compilation); err != nil || len(drift) != 0 {
		t.Fatalf("unexpected joined compiler drift=%v err=%v", drift, err)
	}
	planBytes, err := os.ReadFile(filepath.Join(contractOut, AssemblyPlanFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(planBytes, compilation.PlanJSON) {
		t.Fatal("committed AssemblyPlan bytes differ from joined compiler output")
	}
}

func TestCompileAssemblyRejectsMissingQualifiedModuleSnapshot(t *testing.T) {
	manifest, _ := c102AssemblyFixture(t)
	_, err := CompileAssembly(manifest, nil, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err == nil {
		t.Fatal("expected missing module snapshot to fail")
	}
}
