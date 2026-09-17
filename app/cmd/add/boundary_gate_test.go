package add

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent(t *testing.T) {
	root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")})
	first := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.read", UseCase: "read_tenant", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant"}
	if _, err := AddOperation(first); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
	second := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.list", UseCase: "list_tenants", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none"}
	plan, err := PlanOperation(second)
	if err != nil {
		t.Fatal(err)
	}
	if plan.BoundaryDecision == nil || plan.BoundaryDecision.Outcome != "architecture_review_required" {
		t.Fatalf("decision=%#v", plan.BoundaryDecision)
	}
	if _, err := AddOperation(second); err == nil || !strings.Contains(err.Error(), "Operation Growth boundary") {
		t.Fatalf("expected boundary block, got %v", err)
	}
	after := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
	if before != after {
		t.Fatal("blocked Operation Growth mutated protobuf source")
	}
}
