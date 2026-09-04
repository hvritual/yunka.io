package add

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanOperationIsReadOnlyDeterministicAndMatchesApply(t *testing.T) {
	original := typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")
	root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": original})
	options := OperationOptions{
		Root:           root,
		ApplicationKey: "tenant/lifecycle",
		OperationID:    "tenant.suspend",
		UseCase:        "suspend_tenant",
		Access:         "protected",
		Permissions:    []string{"tenant.manage"},
		PermissionMode: "all",
		Tenant:         "required",
		Authentication: []string{"jwt"},
		Transaction:    "local",
		Idempotency:    "required",
		Composition:    "local",
		HTTPMethod:     "POST",
		HTTPPath:       "/tenants/{id}:suspend",
		HTTPBody:       "*",
	}

	first, err := PlanOperation(options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanOperation(options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != "operation-plan" || second.Kind != "operation-plan" {
		t.Fatalf("plan kind first=%q second=%q", first.Kind, second.Kind)
	}
	wantSemantics := &OperationSemantics{
		UseCase:        "suspend_tenant",
		Access:         "protected",
		Permissions:    []string{"tenant.manage"},
		PermissionMode: "all",
		Tenant:         "required",
		Authentication: []string{"jwt"},
		Transaction:    "local",
		Idempotency:    "required",
		Composition:    "local",
		RequiresOperations: []string{},
		HTTP: &OperationHTTPSemantics{Method: "POST", Path: "/tenants/{id}:suspend", Body: "*"},
	}
	if !reflect.DeepEqual(first.ExplicitSemantics, wantSemantics) {
		t.Fatalf("explicit semantics=%#v want=%#v", first.ExplicitSemantics, wantSemantics)
	}
	firstJSON, err := Render(first, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := Render(second, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	if firstJSON != secondJSON {
		t.Fatalf("operation plan is not byte-stable:\n--- first ---\n%s\n--- second ---\n%s", firstJSON, secondJSON)
	}
	if got := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto")); got != original {
		t.Fatalf("plan mutated canonical protobuf source:\n%s", got)
	}
	landing := filepath.Join(root, "internal", "tenant", "application", "tenant_suspend.go")
	if _, statErr := os.Stat(landing); !os.IsNotExist(statErr) {
		t.Fatalf("plan created implementation landing file: %v", statErr)
	}
	if len(first.Mutations) != 2 || len(first.Effects) == 0 || len(first.NextActions) != 0 {
		t.Fatalf("unexpected prospective report: %#v", first)
	}

	applied, err := AddOperation(options)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Kind != "operation" {
		t.Fatalf("apply kind=%q", applied.Kind)
	}
	if applied.ExplicitSemantics != nil {
		t.Fatalf("existing apply report unexpectedly exposes plan-only semantics: %#v", applied.ExplicitSemantics)
	}
	if !reflect.DeepEqual(first.Identity, applied.Identity) {
		t.Fatalf("plan/apply identity drift:\nplan=%#v\napply=%#v", first.Identity, applied.Identity)
	}
	if !reflect.DeepEqual(first.Mutations, applied.Mutations) {
		t.Fatalf("plan/apply mutation drift:\nplan=%#v\napply=%#v", first.Mutations, applied.Mutations)
	}
	if !reflect.DeepEqual(first.Effects, applied.Effects) {
		t.Fatalf("plan/apply generated effects drift:\nplan=%#v\napply=%#v", first.Effects, applied.Effects)
	}
	if _, statErr := os.Stat(landing); statErr != nil {
		t.Fatalf("apply did not create implementation landing: %v", statErr)
	}
}

func TestPlanOperationFailsClosedOnExistingLandingWithoutProtoMutation(t *testing.T) {
	original := typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")
	root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": original})
	landing := filepath.Join(root, "internal", "tenant", "application", "tenant_suspend.go")
	mustWriteFile(t, landing, "package application\n\n// existing developer code\n")

	_, err := PlanOperation(OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend", UseCase: "suspend_tenant",
		Access: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
	})
	if err == nil {
		t.Fatal("expected plan to reject existing implementation landing")
	}
	if got := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto")); got != original {
		t.Fatalf("failed plan mutated canonical protobuf source:\n%s", got)
	}
	if got := readFile(t, landing); got != "package application\n\n// existing developer code\n" {
		t.Fatalf("failed plan mutated existing developer file:\n%s", got)
	}
}
