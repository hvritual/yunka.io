package change

import (
	"path/filepath"
	"testing"
)

// This pressure test is intentionally stricter than final attestation. It
// proves that AX7 itself rejects a newly introduced handwritten file inside a
// broad Application candidate scope, rather than relying on an unrelated
// downstream gate to make the overall attestation non-conformant.
func TestAX7PlacementPressureRequiresExplicitNewHandwrittenPath(t *testing.T) {
	fixture := newPressureFixture(t)
	fixture.Reset(t)
	path := "internal/tenant/application/global_helper.go"
	writePressureFile(t, filepath.Join(fixture.Root, filepath.FromSlash(path)), "package application\n\nfunc globalHelper() {}\n")

	report, err := ReconcileGitDelta(fixture.Root, fixture.Contract)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range report.Violations {
		if violation.Path == path && violation.Kind == "placement" {
			return
		}
	}
	t.Fatalf("pressure-proven placement escape: newly introduced handwritten path %s was accepted by Git reconciliation: %#v", path, report)
}

func TestAX7PlacementPressureAllowsExplicitExactNewHandwrittenPath(t *testing.T) {
	fixture := newPressureFixture(t)
	fixture.Reset(t)
	path := "internal/tenant/application/suspend_policy.go"
	contractValue := fixture.Contract
	contractValue.EditablePaths = append(contractValue.EditablePaths, path)
	normalizeChangeContract(&contractValue)
	writePressureFile(t, filepath.Join(fixture.Root, filepath.FromSlash(path)), "package application\n\nfunc suspendPolicy() {}\n")

	report, err := ReconcileGitDelta(fixture.Root, contractValue)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("explicit exact handwritten path should be eligible for AX2 ownership validation: %#v", report)
	}
	if len(report.Changes) != 1 || report.Changes[0].Path != path || report.Changes[0].Class != "editable" || report.Changes[0].Owner != "developer-code" {
		t.Fatalf("unexpected explicit placement classification: %#v", report)
	}
}
