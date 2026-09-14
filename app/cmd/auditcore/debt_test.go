package auditcore

import "testing"

func TestCompareProvenFindingsClassifiesExistingNewAndFixed(t *testing.T) {
	proven := func(id string) Finding {
		return Finding{
			ID:        id,
			Rule:      "AUDIT-TEST-001",
			Class:     FindingProvenViolation,
			Subject:   "tenant",
			Summary:   "test",
			Invariant: "test invariant",
			Evidence:  []Evidence{{Kind: EvidenceSource, Source: "test"}},
		}
	}
	observation := Finding{
		ID:       "observation",
		Rule:     "AUDIT-OBS-001",
		Class:    FindingEvidenceObservation,
		Subject:  "tenant",
		Summary:  "observation",
		Evidence: []Evidence{{Kind: EvidenceSource, Source: "test"}},
	}

	result := CompareProvenFindings(
		[]Finding{proven("fixed"), proven("existing"), observation},
		[]Finding{proven("existing"), proven("new"), observation},
	)
	if len(result.Existing) != 1 || result.Existing[0].ID != "existing" {
		t.Fatalf("existing=%#v", result.Existing)
	}
	if len(result.New) != 1 || result.New[0].ID != "new" {
		t.Fatalf("new=%#v", result.New)
	}
	if len(result.Fixed) != 1 || result.Fixed[0].ID != "fixed" {
		t.Fatalf("fixed=%#v", result.Fixed)
	}
}

func TestNamingDebtDeltaBlocksOnlyNewProvenIdentityLeaks(t *testing.T) {
	genericModel := GoSourceFile{
		Path:    "internal/access/domain/model.go",
		Package: "domain",
		Declarations: []SourceDeclaration{
			{Kind: "type", Name: "DataScope"},
			{Kind: "type", Name: "Tenant"},
			{Kind: "type", Name: "User"},
			{Kind: "type", Name: "Membership"},
			{Kind: "type", Name: "Role"},
		},
	}
	base := SourceSnapshot{Files: []GoSourceFile{
		genericModel,
		{Path: "internal/access/domain/ce09_replay.go", Package: "domain"},
	}}
	current := SourceSnapshot{Files: []GoSourceFile{
		genericModel,
		{Path: "internal/access/domain/ce09_replay.go", Package: "domain"},
		{Path: "internal/access/domain/b12_tenant.go", Package: "domain"},
	}}

	delta := CompareProvenFindings(evaluateNaming(base), evaluateNaming(current))
	if len(delta.Existing) != 1 || delta.Existing[0].Rule != RuleHistoricalSourceIdentity {
		t.Fatalf("existing naming debt=%#v", delta.Existing)
	}
	if len(delta.New) != 1 || delta.New[0].Rule != RuleHistoricalSourceIdentity || delta.New[0].Path != "internal/access/domain/b12_tenant.go" {
		t.Fatalf("new naming debt=%#v", delta.New)
	}
	if len(delta.Fixed) != 0 {
		t.Fatalf("fixed naming debt=%#v", delta.Fixed)
	}
	for _, finding := range append(append([]Finding{}, delta.Existing...), delta.New...) {
		if finding.Rule == RuleGenericContainerCohesion {
			t.Fatalf("generic-container observation must not participate in debt delta: %#v", finding)
		}
	}
}
