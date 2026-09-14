package auditcore

import (
	"reflect"
	"testing"
)

func TestEvaluateNamingRejectsHistoricalDurableIdentities(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{
		{
			Path:    "internal/access/domain/ce09_replay.go",
			Package: "domain",
			Declarations: []SourceDeclaration{
				{Kind: "type", Name: "Tenant"},
			},
		},
		{
			Path:    "internal/access/domain/tenant.go",
			Package: "domain",
			Declarations: []SourceDeclaration{
				{Kind: "type", Name: "B12Tenant"},
				{Kind: "func", Name: "TaskMigrationRunner"},
				{Kind: "type", Name: "Taskflow"},
				{Kind: "func", Name: "NewTenant"},
			},
		},
		{
			Path:    "internal/access/domain/tenant_test.go",
			Package: "domain",
			Test:    true,
			Declarations: []SourceDeclaration{
				{Kind: "test", Name: "TestCE09Replay"},
				{Kind: "test", Name: "TestWaveReplay"},
			},
		},
	}}

	findings := evaluateNaming(snapshot)
	var historical []Finding
	for _, finding := range findings {
		if finding.Rule == RuleHistoricalSourceIdentity {
			historical = append(historical, finding)
		}
	}
	if len(historical) != 5 {
		t.Fatalf("historical findings = %d, want 5: %#v", len(historical), historical)
	}
	for _, finding := range historical {
		if finding.Class != FindingProvenViolation {
			t.Fatalf("finding %s class = %q, want proven violation", finding.ID, finding.Class)
		}
		if finding.Path == "" || finding.Symbol == "" || finding.Reason == "" || finding.Remediation == "" {
			t.Fatalf("finding lacks structured remediation evidence: %#v", finding)
		}
	}
	for _, finding := range historical {
		if finding.Symbol == "NewTenant" || finding.Symbol == "Taskflow" {
			t.Fatalf("durable semantic identity must not be treated as delivery history: %#v", finding)
		}
	}
}

func TestEvaluateNamingGenericContainerRequiresCohesionEvidence(t *testing.T) {
	candidate := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/access/domain/model.go",
		Package: "domain",
		Declarations: []SourceDeclaration{
			{Kind: "type", Name: "DataScope"},
			{Kind: "type", Name: "Tenant"},
			{Kind: "type", Name: "User"},
			{Kind: "type", Name: "Membership"},
			{Kind: "type", Name: "Role"},
			{Kind: "type", Name: "PermissionGrant"},
			{Kind: "type", Name: "Credential"},
		},
	}}}
	findings := evaluateNaming(candidate)
	if len(findings) != 1 {
		t.Fatalf("generic candidate findings = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].Rule != RuleGenericContainerCohesion || findings[0].Class != FindingEvidenceObservation {
		t.Fatalf("generic candidate finding = %#v", findings[0])
	}

	cohesive := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/commercial/domain/plan/model.go",
		Package: "plan",
		Declarations: []SourceDeclaration{
			{Kind: "type", Name: "Plan"},
			{Kind: "type", Name: "PlanVersion"},
			{Kind: "type", Name: "PlanItem"},
			{Kind: "type", Name: "PlanState"},
		},
	}}}
	if got := evaluateNaming(cohesive); len(got) != 0 {
		t.Fatalf("cohesive generic container must not be reported: %#v", got)
	}
}

func TestEvaluateNamingHonorsScopedReviewedException(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/workflow/domain/state.go",
		Package: "domain",
		Declarations: []SourceDeclaration{
			{
				Kind:      "type",
				Name:      "Stage2State",
				Exception: "business-concept: Stage2 is a persisted external workflow vocabulary term",
			},
		},
	}}}
	if got := evaluateNaming(snapshot); len(got) != 0 {
		t.Fatalf("reviewed exception must suppress only its declaration: %#v", got)
	}
}

func TestEvaluateNamingFindingIdentityIsStable(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:         "internal/access/domain/cg07_tenant.go",
		Package:      "domain",
		Declarations: []SourceDeclaration{{Kind: "type", Name: "Tenant"}},
	}}}
	first := evaluateNaming(snapshot)
	second := evaluateNaming(snapshot)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("naming evaluation is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
}
