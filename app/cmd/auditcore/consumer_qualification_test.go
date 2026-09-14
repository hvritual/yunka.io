package auditcore

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestConsumerNamingQualification(t *testing.T) {
	bizRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_BIZ_ROOT"))
	iotRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_IOT_ROOT"))
	if bizRoot == "" && iotRoot == "" {
		t.Skip("qualification-only consumer roots are not configured")
	}
	if bizRoot == "" || iotRoot == "" {
		t.Fatal("both YUNKA_QUALIFY_BIZ_ROOT and YUNKA_QUALIFY_IOT_ROOT are required")
	}

	bizSnapshot, err := CollectGoSource(bizRoot, "internal")
	if err != nil {
		t.Fatalf("collect Biz source: %v", err)
	}
	bizFindings := EvaluateSource(bizSnapshot, RuleOptions{})
	assertDeterministicNamingQualification(t, bizSnapshot, bizFindings)
	assertConsumerFinding(t, bizFindings, RuleHistoricalSourceIdentity, "internal/bizruntime/ce09_replay_test.go")
	assertConsumerFinding(t, bizFindings, RuleGenericContainerCohesion, "internal/access/domain/model.go")

	iotSnapshot, err := CollectGoSource(iotRoot, "backend/internal")
	if err != nil {
		t.Fatalf("collect IoT Delivery source: %v", err)
	}
	iotFindings := EvaluateSource(iotSnapshot, RuleOptions{})
	assertDeterministicNamingQualification(t, iotSnapshot, iotFindings)

	if len(bizSnapshot.Files) == 0 || len(iotSnapshot.Files) == 0 {
		t.Fatalf("consumer qualification requires non-empty source snapshots: biz=%d iot=%d", len(bizSnapshot.Files), len(iotSnapshot.Files))
	}
	t.Logf("BIZ_NAMING files=%d findings=%d", len(bizSnapshot.Files), len(bizFindings))
	t.Logf("IOT_NAMING files=%d findings=%d", len(iotSnapshot.Files), len(iotFindings))
}

func assertDeterministicNamingQualification(t *testing.T, snapshot SourceSnapshot, first []Finding) {
	t.Helper()
	second := EvaluateSource(snapshot, RuleOptions{})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("consumer naming findings are not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}

	report := NewReport(ProjectIdentity{})
	report.Source = snapshot
	report.Findings = first
	Normalize(&report)
	if err := Validate(report); err != nil {
		t.Fatalf("consumer naming report is invalid: %v", err)
	}
	for _, finding := range first {
		if !strings.HasPrefix(finding.Rule, "AUDIT-NAME-") {
			continue
		}
		if finding.Path == "" || finding.Reason == "" || finding.Remediation == "" {
			t.Fatalf("naming finding lacks actionable evidence: %#v", finding)
		}
		if finding.Rule == RuleHistoricalSourceIdentity && finding.Symbol == "" {
			t.Fatalf("historical naming finding lacks exact symbol: %#v", finding)
		}
	}
}

func assertConsumerFinding(t *testing.T, findings []Finding, rule, path string) {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule == rule && finding.Path == path {
			return
		}
	}
	t.Fatalf("consumer finding %s at %s not found in %#v", rule, path, findings)
}
