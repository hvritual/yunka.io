package contract

import (
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
	"yunka.io/pkg/operationplan"
)

func TestCompileAssemblyPlanReusesCanonicalApplicationAndBindingFacts(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Services: []Service{
			{Domain: "device", Name: "TransferService", FullName: "demo.device.TransferService", Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/query"}}},
			{Domain: "site", Name: "QueryService", FullName: "demo.site.QueryService", Application: &ApplicationDeclaration{Name: "query"}},
		},
	}
	operations := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{
		{
			OperationID: "device.transfer", Domain: "device", Application: "transfer", UseCase: "Transfer", RequestType: "demo.TransferRequest", ResponseType: "demo.TransferResponse",
			Security: operationplan.Security{PermissionMode: "all"}, Execution: operationplan.Execution{Transaction: "local", Idempotency: "none"},
			Composition: operationplan.Composition{RequiresOperations: []string{"site.validate"}}, ApplicationRequires: []string{"site/query"},
			Bindings: operationplan.Bindings{RPC: "/demo.device.TransferService/Transfer", HTTP: []operationplan.HTTPBinding{{Method: "POST", Path: "/v1/devices:transfer"}}},
		},
		{
			OperationID: "site.validate", Domain: "site", Application: "query", UseCase: "Validate", RequestType: "demo.ValidateRequest", ResponseType: "demo.ValidateResponse",
			Security: operationplan.Security{PermissionMode: "all"}, Execution: operationplan.Execution{Transaction: "none", Idempotency: "none"},
			Composition: operationplan.Composition{},
		},
	}}
	modules := []assemblyplan.ModuleInput{{
		Name: "device", Requirements: assemblyplan.ModuleRequirements{Databases: []string{"primary"}},
		Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "modulecatalog", Ref: "modules/device"},
	}}
	plan, err := CompileAssemblyPlan(manifest, operations, modules)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ApplicationDependencyClosure) != 1 || plan.ApplicationDependencyClosure[0].From != "device/transfer" || plan.ApplicationDependencyClosure[0].To != "site/query" {
		t.Fatalf("unexpected application closure: %#v", plan.ApplicationDependencyClosure)
	}
	var transferBindings int
	for _, operation := range plan.Operations {
		if operation.ID == "device.transfer" {
			transferBindings = len(operation.Bindings)
		}
		if operation.ID == "site.validate" && len(operation.Bindings) != 0 {
			t.Fatalf("internal operation gained external bindings: %#v", operation.Bindings)
		}
	}
	if transferBindings != 2 {
		t.Fatalf("expected RPC+HTTP binding references, got %d", transferBindings)
	}
	if len(plan.Requirements) != 1 || plan.Requirements[0].Kind != "database" || plan.Requirements[0].Name != "primary" {
		t.Fatalf("module requirements were not retained: %#v", plan.Requirements)
	}
}

func TestCompileAssemblyPlanFailsWhenOperationApplicationIsUnknown(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion}
	operations := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{{
		OperationID: "missing.operation", Domain: "missing", Application: "app", UseCase: "Missing", RequestType: "demo.Request", ResponseType: "demo.Response",
		Security: operationplan.Security{PermissionMode: "all"}, Execution: operationplan.Execution{Transaction: "none", Idempotency: "none"},
	}}}
	_, err := CompileAssemblyPlan(manifest, operations, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown application") {
		t.Fatalf("expected unknown application failure, got %v", err)
	}
}
