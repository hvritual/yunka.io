package contract

import (
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
	"yunka.io/pkg/operationplan"
)

func assemblyManifestFixture() Manifest {
	return Manifest{
		SchemaVersion: ManifestVersion,
		Services: []Service{
			{
				Domain: "device", Name: "TransferService", FullName: "demo.device.TransferService",
				Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/query"}},
				Methods: []Method{{
					Name: "Transfer", FullName: "demo.device.TransferService.Transfer",
					Request: "demo.TransferRequest", Response: "demo.TransferResponse",
					HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices:transfer"}},
					Operation: &OperationDeclaration{
						ID: "device.transfer", UseCase: "Transfer", PermissionMode: "all",
						RequiresOperations: []string{"site.validate"},
						Execution:          &ExecutionPolicy{Transaction: "local", Idempotency: "none"},
					},
				}},
			},
			{
				Domain: "site", Name: "QueryService", FullName: "demo.site.QueryService",
				Application: &ApplicationDeclaration{
					Name: "query",
					Operations: []OperationDeclaration{{
						ID: "site.validate", UseCase: "Validate", PermissionMode: "all",
						ApplicationMethod: "Validate", RequestType: "demo.ValidateRequest", ResponseType: "demo.ValidateResponse",
					}},
				},
			},
		},
	}
}

func TestCompileAssemblyPlanReusesCanonicalApplicationAndBindingFacts(t *testing.T) {
	manifest := assemblyManifestFixture()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
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

func TestCompileAssemblyPlanFailsWithoutQualifiedModuleSnapshot(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion}
	operations := operationplan.Set{SchemaVersion: operationplan.SchemaVersion}
	_, err := CompileAssemblyPlan(manifest, operations, nil)
	if err == nil || !strings.Contains(err.Error(), "qualified module snapshot") {
		t.Fatalf("expected qualified module snapshot failure, got %v", err)
	}
}

func TestCompileAssemblyPlanFailsWhenOperationPlanDriftsFromManifest(t *testing.T) {
	manifest := assemblyManifestFixture()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for index := range operations.Operations {
		if operations.Operations[index].OperationID == "device.transfer" {
			operations.Operations[index].Bindings.HTTP[0].Path = "/v1/stale-transfer"
		}
	}
	_, err = CompileAssemblyPlan(manifest, operations, []assemblyplan.ModuleInput{})
	if err == nil || !strings.Contains(err.Error(), "does not match canonical manifest projection") {
		t.Fatalf("expected stale operation-plan failure, got %v", err)
	}
}
