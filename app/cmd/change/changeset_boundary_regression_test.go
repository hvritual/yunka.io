package change

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"yunka.io/app/cmd/add"
)

// This regression uses only pre-existing public APIs so the exact same test can
// establish RED on the prerequisite and GREEN on the completed implementation.
func TestIssue161SetBoundaryRejectsSourceOnlyIntentDrift(t *testing.T) {
	fixture := newPressureFixture(t)
	path := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(changeSetCreateOptions(fixture.Root, true)); err != nil {
		t.Fatal(err)
	}
	generatePressureProject(t, fixture)
	valid, err := ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), value)
	if err != nil || !valid.Conformant {
		t.Fatalf("valid creation must pass before adversarial edit: %v %#v", err, valid)
	}
	plans := filepath.Join(fixture.Root, "contracts", "generated", "operation-plans.json")
	originalPlans := readPressureFile(t, plans)
	mutateRPCOption(t, fixture, "Archive", fmt.Sprintf("context: %q", pressureBoundary().Context), `context: "unrelated.billing"`)
	// Do NOT regenerate. Boundary intent is absent from runtime OperationPlan.
	changed, err := ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), value)
	if err != nil {
		t.Fatalf("valid source must produce a structured nonconformance report: %v", err)
	}
	if changed.Conformant {
		t.Fatal("BOUNDARY_PROOF_GAP: source-only architectural context drift was accepted by ChangeSet check")
	}
	if readPressureFile(t, plans) != originalPlans {
		t.Fatal("readonly check modified generated plans")
	}
}
