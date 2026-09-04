package change

import (
	"testing"

	"yunka.io/app/cmd/add"
)

func TestReconcileChangeSetAcceptsMatchingExistingAndCreateSubjects(t *testing.T) {
	fixture := newPressureFixture(t)
	planPath := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", []string{fixture.ContractPath}, []string{planPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(changeSetCreateOptions(fixture.Root, true)); err != nil {
		t.Fatalf("apply create operation: %v", err)
	}
	generatePressureProject(t, fixture)

	report, err := ReconcileChangeSet(fixture.Root, value)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Conformant || len(report.Reconciliation.Violations) != 0 || len(report.Semantic.Violations) != 0 {
		t.Fatalf("matching ChangeSet did not reconcile:\n%#v", report)
	}
}

func TestReconcileChangeSetRejectsCreateSemanticDriftFromPlannedIntent(t *testing.T) {
	fixture := newPressureFixture(t)
	planPath := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{planPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(changeSetCreateOptions(fixture.Root, false)); err != nil {
		t.Fatalf("apply drifted create operation: %v", err)
	}
	generatePressureProject(t, fixture)

	report, err := ReconcileChangeSet(fixture.Root, value)
	if err != nil {
		t.Fatal(err)
	}
	if report.Conformant {
		t.Fatalf("create semantic drift escaped ChangeSet reconciliation: %#v", report)
	}
	if !hasSemanticViolation(report.Semantic.Violations, SemanticTenant) && !hasSemanticViolation(report.Semantic.Violations, SemanticPermission) && !hasSemanticViolation(report.Semantic.Violations, SemanticAuthentication) {
		t.Fatalf("expected security/tenant drift evidence: %#v", report.Semantic.Violations)
	}
}

func TestReconcileChangeSetRejectsUndeclaredOperationDrift(t *testing.T) {
	fixture := newPressureFixture(t)
	planPath := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", []string{fixture.ContractPath}, []string{planPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(changeSetCreateOptions(fixture.Root, true)); err != nil {
		t.Fatalf("apply create operation: %v", err)
	}
	mutateRPCOption(t, fixture, "Resume", "tenant_required: true", "tenant_required: false")
	generatePressureProject(t, fixture)

	report, err := ReconcileChangeSet(fixture.Root, value)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range report.Semantic.Violations {
		if violation.Category == "scope" && violation.Subject == "operation:tenant.resume" {
			return
		}
	}
	t.Fatalf("undeclared Operation drift escaped ChangeSet reconciliation: %#v", report.Semantic.Violations)
}

func changeSetCreateOptions(root string, matching bool) add.OperationOptions {
	if matching {
		return add.OperationOptions{
			Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.archive", UseCase: "archive_tenant",
			Access: "protected", Permissions: []string{"tenant.archive"}, PermissionMode: "all", Tenant: "required",
			Authentication: []string{"jwt"}, Transaction: "local", Idempotency: "none", Composition: "local",
		}
	}
	return add.OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.archive", UseCase: "archive_tenant",
		Access: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
	}
}

func hasSemanticViolation(values []SemanticDelta, category string) bool {
	for _, value := range values {
		if value.Category == category && !value.Allowed {
			return true
		}
	}
	return false
}
