package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC9OperationCompilerAndExecutorBoundaries(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}
	plan := read("pkg/operationplan/plan.go")
	executor := read("framework/operation/executor.go")
	security := read("gateway/authz/execution_phase.go")
	codegen := read("pkg/contract/c9_application_codegen.go")
	artifact := read("pkg/contract/artifact.go")

	for _, forbidden := range []string{"github.com/hvritual/yunka.io/framework", "github.com/hvritual/yunka.io/gateway", "gorm.io/", "database/sql"} {
		if strings.Contains(plan, forbidden) {
			t.Errorf("leaf operationplan IR depends on forbidden runtime package %q", forbidden)
		}
	}
	if strings.Contains(executor, "github.com/hvritual/yunka.io/gateway") {
		t.Error("framework/operation must not own or import gateway authorization")
	}
	if !strings.Contains(security, "PolicyFromOperationPlan") || !strings.Contains(security, "NewExecutionSecurity") {
		t.Error("gateway/authz must adapt the compiled OperationPlan into the canonical authorization boundary")
	}
	if !strings.Contains(codegen, "operation.ExecuteTyped") || !strings.Contains(codegen, "OperationPlan") {
		t.Error("C9 generated transports must enter the unified OperationPlan/Executor path")
	}
	if strings.Contains(codegen, ".Prepare(request.Context()") || strings.Contains(codegen, "runtime.Prepare") {
		t.Error("C9 generated transports must not own independent authorization sequencing")
	}
	if !strings.Contains(artifact, "OperationPlansFilename") || !strings.Contains(artifact, "CompileOperationPlans") {
		t.Error("contract artifact pipeline must own deterministic operation-plans.json generation")
	}
}
