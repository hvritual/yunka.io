package add

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

func TestAddApplicationCreatesOnlyTypedStructuralService(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedDomainProto("tenant", "tenant.v1"),
	})
	report, err := AddApplication(ApplicationOptions{Root: root, Key: "tenant/lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != "application" || report.Identity["capability"] != "tenant/lifecycle" {
		t.Fatalf("report=%#v", report)
	}
	contents := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
	for _, expected := range []string{
		"service LifecycleApplication",
		`name: "lifecycle"`,
		"TODO(agent): add Operations",
	} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("application scaffold missing %q:\n%s", expected, contents)
		}
	}
	for _, forbidden := range []string{"permissions:", "transaction:", "idempotency:", "rpc "} {
		if strings.Contains(contents, forbidden) {
			t.Fatalf("application scaffold invented %q:\n%s", forbidden, contents)
		}
	}
	if _, err := AddApplication(ApplicationOptions{Root: root, Key: "tenant/lifecycle"}); err == nil {
		t.Fatal("expected duplicate application conflict")
	} else if item := Diagnose(err); item.Code != diagnostic.CodeScaffoldConflict {
		t.Fatalf("diagnostic=%#v", item)
	}
}

func TestAddOperationRequiresExplicitSemanticsAndCreatesLandingFile(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication"),
	})
	if _, err := AddOperation(OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend"}); err == nil {
		t.Fatal("expected missing explicit semantics failure")
	} else if item := Diagnose(err); item.Code != diagnostic.CodeScaffoldRequest {
		t.Fatalf("diagnostic=%#v", item)
	}

	report, err := AddOperation(OperationOptions{
		Root:           root,
		ApplicationKey: "tenant/lifecycle",
		OperationID:    "tenant.suspend",
		UseCase:        "suspend_tenant",
		Access:         "public",
		Tenant:         "optional",
		Transaction:    "none",
		Idempotency:    "none",
		Composition:    "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Identity["operationId"] != "tenant.suspend" || len(report.Mutations) != 2 {
		t.Fatalf("report=%#v", report)
	}
	proto := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
	for _, expected := range []string{
		"rpc Suspend(SuspendRequest) returns (SuspendResponse)",
		`id: "tenant.suspend"`,
		`use_case: "suspend_tenant"`,
		"public: true",
		"TRANSACTION_NONE",
		"IDEMPOTENCY_NONE",
		"message SuspendRequest",
		"kind: DTO_INPUT",
		"message SuspendResponse",
		"kind: DTO_OUTPUT",
	} {
		if !strings.Contains(proto, expected) {
			t.Fatalf("operation scaffold missing %q:\n%s", expected, proto)
		}
	}
	for _, forbidden := range []string{"permissions:", "requires_operations:", "COMPOSITION_LOCAL", "COMPOSITION_REMOTE_SAGA"} {
		if strings.Contains(proto, forbidden) {
			t.Fatalf("operation scaffold invented %q:\n%s", forbidden, proto)
		}
	}
	landing := readFile(t, filepath.Join(root, "internal", "tenant", "application", "tenant_suspend.go"))
	if !strings.Contains(landing, "TODO(agent)") || strings.Contains(landing, "func ") {
		t.Fatalf("landing file contains business implementation:\n%s", landing)
	}
	if !strings.Contains(landing, "Do not edit zz_yunka_*") {
		t.Fatalf("landing file lacks generator boundary:\n%s", landing)
	}
}

func TestAddOperationRefusesExistingImplementationLandingBeforeProtoMutation(t *testing.T) {
	original := typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")
	root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": original})
	landing := filepath.Join(root, "internal", "tenant", "application", "tenant_suspend.go")
	mustWriteFile(t, landing, "package application\n\n// existing developer code\n")
	_, err := AddOperation(OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend", UseCase: "suspend_tenant",
		Access: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
	})
	if err == nil {
		t.Fatal("expected landing conflict")
	}
	if item := Diagnose(err); item.Code != diagnostic.CodeScaffoldConflict {
		t.Fatalf("diagnostic=%#v", item)
	}
	if got := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto")); got != original {
		t.Fatalf("proto mutated before landing conflict:\n%s", got)
	}
}

func TestAddApplicationFailsClosedWhenDomainSourceIsAmbiguous(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/a.proto": typedDomainProto("tenant", "tenant.a.v1"),
		"contracts/proto/b.proto": typedDomainProto("tenant", "tenant.b.v1"),
	})
	_, err := AddApplication(ApplicationOptions{Root: root, Key: "tenant/lifecycle"})
	if err == nil {
		t.Fatal("expected ambiguous source failure")
	}
	item := Diagnose(err)
	if item.Code != diagnostic.CodeScaffoldSource || !strings.Contains(item.Detail, "pass --source") {
		t.Fatalf("diagnostic=%#v", item)
	}
}

func TestAddEventDeclaresSchemaOnly(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedDomainProto("tenant", "tenant.v1"),
	})
	report, err := AddEvent(EventOptions{Root: root, Domain: "tenant", Name: "tenant_suspended"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Identity["message"] != "TenantSuspended" {
		t.Fatalf("report=%#v", report)
	}
	contents := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
	if !strings.Contains(contents, "message TenantSuspended") || !strings.Contains(contents, "kind: DTO_EVENT") {
		t.Fatalf("event scaffold missing DTO_EVENT:\n%s", contents)
	}
	for _, forbidden := range []string{"topic", "outbox", "publisher", "event_bus"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Fatalf("event scaffold invented %q semantics:\n%s", forbidden, contents)
		}
	}
}

func TestAddModuleUsesDeclarativeSpecOnly(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedDomainProto("tenant", "tenant.v1"),
	})
	report, err := AddModule(ModuleOptions{Root: root, Name: "audit", Version: "v0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mutations) != 1 || report.Mutations[0].Owner != "developer-module" {
		t.Fatalf("report=%#v", report)
	}
	contents := readFile(t, filepath.Join(root, "modules", "audit", "module.yunka.json"))
	if !strings.Contains(contents, `"schemaVersion": 1`) || strings.Contains(contents, "database") || strings.Contains(contents, "eventBus") || strings.Contains(contents, "dependsOn") {
		t.Fatalf("module scaffold contains inferred capabilities:\n%s", contents)
	}
	if _, statErr := os.Stat(filepath.Join(root, "modules", "audit", "module.go")); !os.IsNotExist(statErr) {
		t.Fatalf("module scaffold created runtime source: %v", statErr)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	report := Report{
		SchemaVersion: SchemaVersion,
		Kind:          "operation",
		Identity:      map[string]string{"z": "2", "a": "1"},
		Mutations:     []Mutation{{Path: "z", Action: "modified", Owner: "developer"}, {Path: "a", Action: "created", Owner: "developer"}},
		Effects:       []Effect{{Stage: "z", Scope: "z", Reason: "z"}, {Stage: "a", Path: "a", Reason: "a"}},
	}
	first, err := Render(report, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(report, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.Contains(first, `"path": "a"`) {
		t.Fatalf("non-deterministic output:\n%s\n---\n%s", first, second)
	}
}

func scaffoldProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for relative, contents := range files {
		mustWriteFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)
	}
	return root
}

func typedDomainProto(domain, pkg string) string {
	return `syntax = "proto3";
package ` + pkg + `;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/demo/contracts/` + domain + `;` + domain + `v1";
option (yunka.dsl.v1.domain) = { name: "` + domain + `" version: "v1" };
`
}

func typedApplicationProto(domain, pkg, application, service string) string {
	return typedDomainProto(domain, pkg) + `
service ` + service + ` {
  option (yunka.dsl.v1.application) = { name: "` + application + `" };
}
`
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFailureUnwrap(t *testing.T) {
	cause := errors.New("cause")
	failure := &Failure{Kind: FailureRequest, Err: cause}
	if !errors.Is(failure, cause) {
		t.Fatal("failure does not unwrap")
	}
}
