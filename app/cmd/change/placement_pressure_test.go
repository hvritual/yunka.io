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
		if violation.Path == path && (violation.Kind == "placement" || violation.Kind == "scope") {
			return
		}
	}
	t.Fatalf("pressure-proven placement escape: newly introduced handwritten path %s was accepted by Git reconciliation: %#v", path, report)
}
