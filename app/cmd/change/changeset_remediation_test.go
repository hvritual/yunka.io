package change

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
)

func TestT4RemediationBindingRejectsUnknownFinding(t *testing.T) {
	fixture, value, _, _ := newRemediationPressureFixture(t)
	_, err := BuildRemediationBinding(fixture.Root, value, []string{"AUDIT-NOT-REAL"})
	if err == nil || !strings.Contains(err.Error(), "must be a proven finding present") {
		t.Fatalf("unknown remediation finding was not rejected: %v", err)
	}
}

func TestT4RemediationCheckRejectsRemainingTarget(t *testing.T) {
	fixture, value, binding, findingID := newRemediationPressureFixture(t)
	report, err := ReconcileRemediation(fixture.Root, value, binding)
	if err != nil {
		t.Fatal(err)
	}
	if report.ChangeSet.Conformant != true {
		t.Fatalf("unchanged authorized ChangeSet should itself conform: %#v", report.ChangeSet)
	}
	if report.Conformant || len(report.Audit.Fixed) != 0 || len(report.Audit.Remaining) != 1 || report.Audit.Remaining[0] != findingID || len(report.Audit.NewDebt) != 0 {
		t.Fatalf("remaining remediation target was not preserved as a failure: %#v", report)
	}
}

func TestT4RemediationCheckRejectsReplacementWithNewDebt(t *testing.T) {
	fixture, value, binding, findingID := newRemediationPressureFixture(t)
	badPath := remediationPressurePath(fixture.Root)
	writePressureFile(t, badPath, "package application\n\nimport _ \"github.com/hvritual/yunka.io/gateway/authz\"\n")

	report, err := ReconcileRemediation(fixture.Root, value, binding)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ChangeSet.Conformant {
		t.Fatalf("same authorized path should remain ChangeSet-conformant: %#v", report.ChangeSet)
	}
	if report.Conformant || len(report.Audit.Fixed) != 1 || report.Audit.Fixed[0] != findingID || len(report.Audit.Remaining) != 0 {
		t.Fatalf("replacement pressure did not fix only the declared target: %#v", report)
	}
	if len(report.Audit.NewDebt) != 1 {
		t.Fatalf("replacement pressure must introduce exactly one new proven debt finding: %#v", report.Audit)
	}
}

func TestT4RemediationCheckPassesOnlyWhenFindingIsActuallyFixed(t *testing.T) {
	fixture, value, binding, findingID := newRemediationPressureFixture(t)
	if err := os.Remove(remediationPressurePath(fixture.Root)); err != nil {
		t.Fatal(err)
	}

	report, err := ReconcileRemediation(fixture.Root, value, binding)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ChangeSet.Conformant || !report.Audit.Conformant || !report.Conformant {
		t.Fatalf("actual fixed remediation must conform: %#v", report)
	}
	if len(report.Audit.Fixed) != 1 || report.Audit.Fixed[0] != findingID || len(report.Audit.Remaining) != 0 || len(report.Audit.NewDebt) != 0 {
		t.Fatalf("unexpected fixed remediation evidence: %#v", report.Audit)
	}
}

func TestT4RemediationBindingIsDigestBoundAndStrict(t *testing.T) {
	fixture, value, binding, _ := newRemediationPressureFixture(t)
	path, err := WriteRemediationBinding(fixture.Root, "", binding)
	if err != nil {
		t.Fatal(err)
	}
	if path != DefaultRemediationBindingPath {
		t.Fatalf("binding path=%q", path)
	}
	loaded, _, err := LoadRemediationBinding(fixture.Root, "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ChangeSetDigest != binding.ChangeSetDigest || loaded.BaseSHA != binding.BaseSHA {
		t.Fatalf("binding roundtrip drift: want=%#v got=%#v", binding, loaded)
	}

	tampered := value
	tampered.Subjects[0].Existing.Intent = IntentBoth
	if _, err := ReconcileRemediation(fixture.Root, tampered, binding); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered ChangeSet escaped remediation digest binding: %v", err)
	}
}

func newRemediationPressureFixture(t *testing.T) (pressureFixture, ChangeSet, RemediationBinding, string) {
	t.Helper()
	fixture := newPressureFixture(t)
	badRelative := filepath.ToSlash(filepath.Join("internal", "tenant", "application", "t4_audit_pressure.go"))
	writePressureFile(t, filepath.Join(fixture.Root, filepath.FromSlash(badRelative)), "package application\n\nimport _ \"github.com/hvritual/yunka.io/framework/platform\"\n")
	gitPressure(t, fixture.Root, "add", badRelative)
	gitPressure(t, fixture.Root, "commit", "-m", "T4 remediation audit pressure baseline")

	contractValue, _, err := BuildChangeContract(
		fixture.Root,
		"tenant.suspend",
		IntentImplementation,
		"HEAD",
		[]string{badRelative},
		nil,
		3,
	)
	if err != nil {
		t.Fatalf("build remediation change contract: %v", err)
	}
	contractPath := filepath.ToSlash(filepath.Join(".yunka", "t4-remediation-contract.json"))
	if _, err := WriteChangeContract(fixture.Root, contractPath, contractValue); err != nil {
		t.Fatalf("write remediation change contract: %v", err)
	}
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", []string{contractPath}, nil)
	if err != nil {
		t.Fatalf("build remediation ChangeSet: %v", err)
	}

	auditReport, err := audit.Build(fixture.Root)
	if err != nil {
		t.Fatalf("build remediation audit: %v", err)
	}
	findingID := ""
	for _, finding := range auditReport.Findings {
		if finding.Rule == auditcore.RulePlatformProviderBypass {
			findingID = finding.ID
			break
		}
	}
	if findingID == "" {
		t.Fatalf("pressure baseline did not produce %s: %#v", auditcore.RulePlatformProviderBypass, auditReport.Findings)
	}
	binding, err := BuildRemediationBinding(fixture.Root, value, []string{findingID})
	if err != nil {
		t.Fatalf("bind remediation finding: %v", err)
	}
	return fixture, value, binding, findingID
}

func remediationPressurePath(root string) string {
	return filepath.Join(root, "internal", "tenant", "application", "t4_audit_pressure.go")
}
