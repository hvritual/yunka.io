package change

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/add"
)

func TestBuildChangeSetComposesExistingV1AndCreatePlanOnOneBase(t *testing.T) {
	fixture := newPressureFixture(t)
	plan := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")

	value, root, err := BuildChangeSet(fixture.Root, "HEAD", []string{fixture.ContractPath}, []string{plan})
	if err != nil {
		t.Fatal(err)
	}
	if root != fixture.Root || value.SchemaVersion != ChangeSetSchemaVersion || len(value.Subjects) != 2 {
		t.Fatalf("change set=%#v root=%q", value, root)
	}
	if value.Subjects[0].Create == nil || value.Subjects[0].Create.Operation.OperationID != "tenant.archive" {
		t.Fatalf("canonical first subject=%#v", value.Subjects[0])
	}
	if value.Subjects[1].Existing == nil || value.Subjects[1].Existing.Operation.OperationID != "tenant.suspend" {
		t.Fatalf("canonical second subject=%#v", value.Subjects[1])
	}
	create := value.Subjects[0].Create
	if create.PlanDigest == "" || create.Expected.Semantics.UseCase != "archive_tenant" || len(create.EditablePaths) != 2 || len(create.GeneratedPaths) == 0 {
		t.Fatalf("create subject evidence=%#v", create)
	}
	if create.Expected.Semantics.Permissions == nil || create.Expected.Semantics.Authentication == nil || create.Expected.Semantics.RequiresOperations == nil {
		t.Fatalf("create semantic collections must be canonical arrays: %#v", create.Expected.Semantics)
	}

	path, err := WriteChangeSet(fixture.Root, "", value)
	if err != nil {
		t.Fatal(err)
	}
	if path != DefaultChangeSetPath {
		t.Fatalf("path=%q", path)
	}
	loaded, _, err := LoadChangeSet(fixture.Root, "")
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(value)
	right, _ := json.Marshal(loaded)
	if string(left) != string(right) {
		t.Fatalf("roundtrip mismatch\nwant=%s\ngot =%s", left, right)
	}
	info, err := os.Stat(filepath.Join(fixture.Root, filepath.FromSlash(DefaultChangeSetPath)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("change set mode=%o", info.Mode().Perm())
	}
}

func TestBuildChangeSetRejectsDuplicateSubjectOperation(t *testing.T) {
	fixture := newPressureFixture(t)
	_, _, err := BuildChangeSet(fixture.Root, "HEAD", []string{fixture.ContractPath, fixture.ContractPath}, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate subject operation tenant.suspend") {
		t.Fatalf("duplicate subject err=%v", err)
	}
}

func TestBuildChangeSetRejectsTamperedCreatePlanShape(t *testing.T) {
	fixture := newPressureFixture(t)
	planPath := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	absolute := filepath.Join(fixture.Root, filepath.FromSlash(planPath))
	contents := readPressureFile(t, absolute)
	contents = strings.Replace(contents, `"kind": "operation-plan"`, `"kind": "operation-plan", "authority": "safe_to_merge"`, 1)
	writePressureFile(t, absolute, contents)

	_, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{planPath})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("tampered create plan err=%v", err)
	}
}

func writeCreatePlan(t *testing.T, fixture pressureFixture, operationID, useCase string) string {
	t.Helper()
	plan, err := add.PlanOperation(add.OperationOptions{
		Root: fixture.Root, ApplicationKey: "tenant/lifecycle", OperationID: operationID, UseCase: useCase,
		Access: "protected", Permissions: []string{operationID}, PermissionMode: "all", Tenant: "required",
		Authentication: []string{"jwt"}, Transaction: "local", Idempotency: "none", Composition: "local",
	})
	if err != nil {
		t.Fatalf("plan operation %s: %v", operationID, err)
	}
	data, err := add.Render(plan, add.FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.ToSlash(filepath.Join(".yunka", strings.ReplaceAll(operationID, ".", "_")+"-plan.json"))
	writePressureFile(t, filepath.Join(fixture.Root, filepath.FromSlash(path)), data)
	return path
}
