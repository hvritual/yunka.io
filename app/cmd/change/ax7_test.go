package change

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestChangeCommandExposesAX7Protocol(t *testing.T) {
	command := Command()
	seen := map[string]bool{}
	for _, subcommand := range command.Subcommands {
		seen[subcommand.Name] = true
	}
	for _, expected := range []string{"plan", "begin", "check", "verify"} {
		if !seen[expected] {
			t.Fatalf("missing change subcommand %s: %#v", expected, seen)
		}
	}
}

func TestChangeContractWriteLoadIsDeterministicAndTransient(t *testing.T) {
	root := t.TempDir()
	value := ChangeContract{
		SchemaVersion: ChangeContractSchemaVersion,
		BaseSHA:       "abc123",
		Intent:        IntentImplementation,
		Operation:     ChangeOperation{NodeID: "operation:tenant.suspend", OperationID: "tenant.suspend", Domain: "tenant", Application: "tenant_management"},
		AllowedSemantic: []string{SemanticTenant, SemanticPermission, SemanticTenant},
		EditablePaths:   []string{"internal/tenant/application/suspend.go", "internal/tenant/application/suspend.go"},
		EditableScopes:  []string{"internal/tenant/application"},
		GeneratedPaths:  []string{"contracts/generated/manifest.json"},
	}
	normalizeChangeContract(&value)
	path, err := WriteChangeContract(root, "", value)
	if err != nil {
		t.Fatal(err)
	}
	if path != DefaultChangeContractPath {
		t.Fatalf("path=%q", path)
	}
	loaded, _, err := LoadChangeContract(root, "")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(value)
	second, _ := json.Marshal(loaded)
	if string(first) != string(second) {
		t.Fatalf("roundtrip mismatch\nwant=%s\ngot =%s", first, second)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(DefaultChangeContractPath)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("contract mode=%o", info.Mode().Perm())
	}
}

func TestParseNameStatusZHandlesRename(t *testing.T) {
	changes, err := parseNameStatusZ([]byte("M\x00internal/tenant/application/suspend.go\x00R100\x00internal/tenant/application/old.go\x00internal/tenant/application/new.go\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("changes=%#v", changes)
	}
	if changes[1].PreviousPath != "internal/tenant/application/old.go" || changes[1].Path != "internal/tenant/application/new.go" {
		t.Fatalf("rename=%#v", changes[1])
	}
}

func TestReconcileFileRejectsOutOfEnvelopeAndAcceptsExpectedGenerated(t *testing.T) {
	contractValue := ChangeContract{
		SchemaVersion:   ChangeContractSchemaVersion,
		EditableScopes:  []string{"internal/tenant/application"},
		GeneratedPaths:  []string{"contracts/generated/manifest.json"},
	}
	generated, violation, err := reconcileFile(t.TempDir(), contractValue, FileChange{Status: "M", Path: "contracts/generated/manifest.json"})
	if err != nil || violation != nil || generated.Class != "generated" {
		t.Fatalf("generated=%#v violation=%#v err=%v", generated, violation, err)
	}
	outside, violation, err := reconcileFile(t.TempDir(), contractValue, FileChange{Status: "A", Path: "internal/common/global_service.go"})
	if err != nil || violation == nil || outside.Class != "outside" || violation.Kind != "scope" {
		t.Fatalf("outside=%#v violation=%#v err=%v", outside, violation, err)
	}
}

func TestSemanticDeltaRejectsTenantDriftUnlessExplicitlyAllowed(t *testing.T) {
	before := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{testOperationPlan(false)}}
	after := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{testOperationPlan(true)}}
	contractValue := ChangeContract{Operation: ChangeOperation{OperationID: "tenant.suspend"}}
	deltas := compareOperationPlans(contractValue, before, after)
	if len(deltas) != 1 || deltas[0].Category != SemanticTenant || deltas[0].Allowed {
		t.Fatalf("deltas=%#v", deltas)
	}
	contractValue.AllowedSemantic = []string{SemanticTenant}
	deltas = compareOperationPlans(contractValue, before, after)
	if len(deltas) != 1 || !deltas[0].Allowed {
		t.Fatalf("allowed deltas=%#v", deltas)
	}
}

func TestSemanticDeltaRejectsUnrelatedOperationChange(t *testing.T) {
	before := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{testOperationPlan(false), otherOperationPlan("none")}}
	after := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{testOperationPlan(false), otherOperationPlan("local")}}
	contractValue := ChangeContract{Operation: ChangeOperation{OperationID: "tenant.suspend"}, AllowedSemantic: []string{SemanticTransaction}}
	deltas := compareOperationPlans(contractValue, before, after)
	if len(deltas) != 1 || deltas[0].Category != "scope" || deltas[0].Allowed {
		t.Fatalf("deltas=%#v", deltas)
	}
}

func TestApplicationCapabilityDeltaRequiresExplicitAllowance(t *testing.T) {
	before := testManifest(nil)
	after := testManifest([]contract.CapabilityRequirement{{Name: "cache", Package: "example.com/cache", Type: "Cache"}})
	contractValue := ChangeContract{Operation: ChangeOperation{OperationID: "tenant.suspend", Domain: "tenant", Application: "tenant_management"}}
	deltas := compareApplicationSemantics(contractValue, before, after)
	if len(deltas) != 1 || deltas[0].Category != SemanticCapabilities || deltas[0].Allowed {
		t.Fatalf("deltas=%#v", deltas)
	}
	contractValue.AllowedSemantic = []string{SemanticCapabilities}
	deltas = compareApplicationSemantics(contractValue, before, after)
	if len(deltas) != 1 || !deltas[0].Allowed {
		t.Fatalf("allowed deltas=%#v", deltas)
	}
}

func testOperationPlan(tenantRequired bool) operationplan.Plan {
	return operationplan.Plan{
		OperationID: "tenant.suspend",
		Domain:      "tenant",
		Application: "tenant_management",
		UseCase:     "suspend",
		RequestType: "tenant.v1.SuspendRequest",
		ResponseType: "tenant.v1.SuspendResponse",
		Security: operationplan.Security{TenantRequired: tenantRequired, PermissionMode: "all"},
		Execution: operationplan.Execution{Transaction: "local", Idempotency: "none"},
	}
}

func otherOperationPlan(transaction string) operationplan.Plan {
	return operationplan.Plan{
		OperationID: "tenant.resume",
		Domain:      "tenant",
		Application: "tenant_management",
		UseCase:     "resume",
		RequestType: "tenant.v1.ResumeRequest",
		ResponseType: "tenant.v1.ResumeResponse",
		Security:    operationplan.Security{PermissionMode: "all"},
		Execution:   operationplan.Execution{Transaction: transaction, Idempotency: "none"},
	}
}

func testManifest(capabilities []contract.CapabilityRequirement) contract.Manifest {
	value := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Services: []contract.Service{{
			Name:     "TenantApplication",
			FullName: "tenant.v1.TenantApplication",
			Domain:   "tenant",
			Application: &contract.ApplicationDeclaration{
				Name:         "tenant_management",
				Capabilities: capabilities,
			},
		}},
	}
	value.Normalize()
	return value
}
