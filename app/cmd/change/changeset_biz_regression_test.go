package change

import (
	"path/filepath"
	"testing"

	"yunka.io/app/cmd/add"
)

func TestT4CreateChangeSetBindsProtobufOutputsAndNormalizesAPIKey(t *testing.T) {
	fixture := newPressureFixture(t)
	writePressureFile(t, filepath.Join(fixture.Root, ".yunka", "protobuf-go.json"), `{
  "schemaVersion": 1,
  "files": [
    "contracts/gen/tenant.pb.go",
    "contracts/gen/tenant_grpc.pb.go"
  ]
}
`)

	options := add.OperationOptions{
		Root: fixture.Root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.archive", UseCase: "archive_tenant",
		Access: "protected", Permissions: []string{"tenant.archive"}, PermissionMode: "all", Tenant: "required",
		Authentication: []string{"api-key"}, Transaction: "local", Idempotency: "none", Composition: "local",
	}
	plan, err := add.PlanOperation(options)
	if err != nil {
		t.Fatal(err)
	}
	planJSON, err := add.Render(plan, add.FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.ToSlash(filepath.Join(".yunka", "t4-biz-regression-plan.json"))
	writePressureFile(t, filepath.Join(fixture.Root, filepath.FromSlash(planPath)), planJSON)

	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{planPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Subjects) != 1 || value.Subjects[0].Create == nil {
		t.Fatalf("create subject=%#v", value.Subjects)
	}
	paths := map[string]bool{}
	for _, path := range value.Subjects[0].Create.GeneratedPaths {
		paths[path] = true
	}
	for _, expected := range []string{"contracts/gen/tenant.pb.go", "contracts/gen/tenant_grpc.pb.go"} {
		if !paths[expected] {
			t.Fatalf("exact protobuf Go ownership path %s missing from create ChangeSet: %#v", expected, value.Subjects[0].Create.GeneratedPaths)
		}
	}

	if _, err := add.AddOperation(options); err != nil {
		t.Fatal(err)
	}
	generatePressureProject(t, fixture)
	report, err := ReconcileChangeSet(fixture.Root, value)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Conformant || len(report.Reconciliation.Violations) != 0 || len(report.Semantic.Violations) != 0 {
		t.Fatalf("matching api-key create ChangeSet did not reconcile: %#v", report)
	}
}
