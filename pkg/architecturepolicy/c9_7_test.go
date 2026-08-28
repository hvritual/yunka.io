package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/pkg/operationplan"
)

func TestC97ExecutionSemanticsBoundaries(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}

	executor := read("framework/operation/executor.go")
	scope := read("framework/execution/scope.go")
	idempotency := read("framework/execution/idempotency.go")
	gormIdempotency := read("framework/execution/idempotencygorm/store.go")
	requestJoin := read("framework/requestscope/join.go")
	sagaStage := read("framework/workflow/saga/stage.go")
	codegen := read("pkg/contract/c9_application_codegen.go")
	plan := read("pkg/operationplan/plan.go")

	for name, source := range map[string]string{
		"framework/operation":              executor,
		"framework/execution/scope":        scope,
		"framework/execution/idempotency":  idempotency,
	} {
		for _, forbidden := range []string{"yunka.io/gateway", "gorm.io/", "database/sql"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s imports forbidden adapter/runtime dependency %q", name, forbidden)
			}
		}
	}
	if !strings.Contains(gormIdempotency, "gorm.io/gorm") {
		t.Error("idempotencygorm must remain the explicit GORM adapter boundary")
	}

	if !strings.Contains(executor, "execution.BeginRoot") || !strings.Contains(executor, "ExecuteChildTyped") {
		t.Error("Executor must own root ExecutionScope and typed child invocation")
	}
	if !strings.Contains(executor, "PhaseIdempotencyBegin") || !strings.Contains(executor, "PhaseTransactionFinalize") || !strings.Contains(executor, "PhaseIdempotencyFinalize") {
		t.Error("Executor phase machine must retain transaction/idempotency closure")
	}
	if !strings.Contains(scope, "JoinChild") || !strings.Contains(scope, "ErrChildUndeclared") || !strings.Contains(scope, "ErrTransactionConflict") {
		t.Error("ExecutionScope must fail closed for undeclared/incompatible child operations")
	}

	if !strings.Contains(requestJoin, "execution.UnitOfWorkFrom") {
		t.Error("joinable requestscope must consume the Executor-owned UnitOfWork")
	}
	for _, forbidden := range []string{"Commit(", "Rollback(", "Close("} {
		if strings.Contains(requestJoin, forbidden) {
			t.Errorf("requestscope View must not expose transaction lifecycle method %q", forbidden)
		}
	}

	if !strings.Contains(sagaStage, "execution.TransactionHandleFrom") || !strings.Contains(sagaStage, "EnqueueTx") {
		t.Error("Saga Stager must join the exact current ExecutionScope transaction")
	}

	for _, required := range []string{
		"operation.ExecuteChildTyped",
		"execution.WithIdempotencyKey",
		"Idempotency-Key",
		"idempotency-key",
	} {
		if !strings.Contains(codegen, required) {
			t.Errorf("C9.7 generated boundary missing %q", required)
		}
	}
	for _, forbidden := range []string{"reflect.Value", "map[string]any", ".Resolve("} {
		if strings.Contains(codegen, forbidden) {
			t.Errorf("C9.7 codegen contains forbidden service-locator/reflection pattern %q", forbidden)
		}
	}

	if !strings.Contains(plan, `case "none", "read_only", "local"`) || !strings.Contains(plan, `case "none", "required"`) {
		t.Error("OperationPlan must validate explicit execution policy values")
	}
}

func TestC97RequiredIdempotencyRequiresLocalTransaction(t *testing.T) {
	base := operationplan.Plan{
		OperationID:  "architecture.c9_7.idempotency",
		Domain:       "architecture",
		Application:  "c9_7",
		UseCase:      "Idempotency",
		RequestType:  "architecture.Request",
		ResponseType: "architecture.Response",
		Security: operationplan.Security{
			Public:         true,
			PermissionMode: "all",
		},
		Execution: operationplan.Execution{
			Transaction: "none",
			Idempotency: "required",
		},
		Bindings: operationplan.Bindings{RPC: "/architecture.C97/Idempotency"},
	}

	if err := operationplan.Validate(operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{base}}); err == nil {
		t.Fatal("required idempotency without local transaction must fail closed")
	}
	base.Execution.Transaction = "read_only"
	if err := operationplan.Validate(operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{base}}); err == nil {
		t.Fatal("required idempotency with read-only transaction must fail closed")
	}
	base.Execution.Transaction = "local"
	if err := operationplan.Validate(operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{base}}); err != nil {
		t.Fatalf("required idempotency with local transaction should validate: %v", err)
	}
}
