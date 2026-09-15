package auditcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQualityPolicyDeclaresLimitsAndBlockingRules(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(QualityPolicyRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `{
  "schemaVersion": 1,
  "limits": {
    "maxFileLines": 120,
    "maxTopLevelDeclarations": 20,
    "maxBranchPoints": 15
  },
  "blockingRules": ["AUDIT-SIZE-001", "AUDIT-NAME-001"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, evidence, err := LoadQualityPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Present || evidence.SHA256 == "" || evidence.Path != QualityPolicyRelativePath {
		t.Fatalf("policy evidence=%#v", evidence)
	}
	if policy.Limits.MaxFileLines != 120 || policy.Limits.MaxTopLevelDeclarations != 20 || policy.Limits.MaxBranchPoints != 15 {
		t.Fatalf("policy limits=%#v", policy.Limits)
	}
	if len(policy.BlockingRules) != 2 || policy.BlockingRules[0] != RuleHistoricalSourceIdentity || policy.BlockingRules[1] != RuleFileLineLimit {
		t.Fatalf("blocking rules=%#v", policy.BlockingRules)
	}
}

func TestQualityPolicyRejectsAdvisoryBlockingAuthority(t *testing.T) {
	err := ValidateQualityPolicy(QualityPolicy{
		SchemaVersion: QualityPolicySchemaVersion,
		BlockingRules: []string{RuleGenericContainerCohesion},
	})
	if err == nil {
		t.Fatal("advisory generic-container observation was accepted as blocking authority")
	}
}

func TestDeclaredLimitsProduceStableStructuredFindings(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path: "internal/tenant/domain/service.go", Package: "domain",
		Lines: 101, TopLevelDeclarations: 11, BranchPoints: 9,
	}, {
		Path: "internal/tenant/domain/generated.go", Package: "domain", Generated: true,
		Lines: 1000, TopLevelDeclarations: 100, BranchPoints: 100,
	}, {
		Path: "internal/tenant/domain/service_test.go", Package: "domain", Test: true,
		Lines: 1000, TopLevelDeclarations: 100, BranchPoints: 100,
	}}}
	limits := QualityLimits{MaxFileLines: 100, MaxTopLevelDeclarations: 10, MaxBranchPoints: 8}
	first := evaluateDeclaredLimits(snapshot, limits)
	second := evaluateDeclaredLimits(snapshot, limits)
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("findings first=%#v second=%#v", first, second)
	}
	for index := range first {
		if first[index].ID != second[index].ID || first[index].Path == "" || first[index].Symbol == "" || first[index].Reason == "" || first[index].Remediation == "" {
			t.Fatalf("unstable/incomplete limit finding: %#v %#v", first[index], second[index])
		}
		if first[index].Class != FindingProvenViolation {
			t.Fatalf("limit finding class=%s", first[index].Class)
		}
	}
}

func TestBlockingDesignationAppliesOnlyToNewProvenDebt(t *testing.T) {
	base := []Finding{{
		ID: "AUDIT-SIZE-001:existing.go", Rule: RuleFileLineLimit, Class: FindingProvenViolation,
		Subject: "existing.go", Summary: "existing", Invariant: "fixture", Evidence: []Evidence{{Kind: EvidenceSource, Source: "fixture"}},
	}}
	current := append(cloneFindings(base), Finding{
		ID: "AUDIT-SIZE-001:new.go", Rule: RuleFileLineLimit, Class: FindingProvenViolation,
		Subject: "new.go", Summary: "new", Invariant: "fixture", Evidence: []Evidence{{Kind: EvidenceSource, Source: "fixture"}},
	})
	ApplyBlockingPolicy(current, QualityPolicy{SchemaVersion: QualityPolicySchemaVersion, BlockingRules: []string{RuleFileLineLimit}})
	delta := CompareProvenFindings(base, current)
	report := NewReport(ProjectIdentity{})
	report.Debt = &delta
	blocking := BlockingNewFindings(report)
	if len(blocking) != 1 || blocking[0].ID != "AUDIT-SIZE-001:new.go" {
		t.Fatalf("blocking new=%#v delta=%#v", blocking, delta)
	}
	if !delta.Existing[0].Blocking {
		t.Fatal("current policy designation was not retained on existing current finding")
	}
}
