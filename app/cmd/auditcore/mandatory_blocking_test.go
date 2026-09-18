package auditcore

import (
	"strings"
	"testing"
)

func TestApplyBlockingPolicyCannotDisableMandatoryOperationGrowth(t *testing.T) {
	findings := []Finding{
		mandatoryBoundaryFinding(false),
		{
			ID:        "AUDIT-AUTH-001:demo",
			Rule:      RuleAuthorizationBypass,
			Class:     FindingProvenViolation,
			Subject:   "demo",
			Summary:   "ordinary configurable rule",
			Invariant: "ordinary rule remains policy-controlled",
			Evidence:  []Evidence{{Kind: EvidenceCanonical, Source: "test"}},
		},
	}
	ApplyBlockingPolicy(findings, QualityPolicy{SchemaVersion: QualityPolicySchemaVersion, BlockingRules: []string{}})
	if !findings[0].Blocking {
		t.Fatal("mandatory Operation Growth finding was disabled by empty quality policy")
	}
	if findings[1].Blocking {
		t.Fatal("ordinary configurable rule ignored empty quality policy")
	}
}

func TestNormalizeRestoresMandatoryBlockingInDebtDelta(t *testing.T) {
	report := NewReport(ProjectIdentity{})
	report.Debt = &DebtDelta{
		BaseRef:  "HEAD",
		BaseSHA:  "0123456789012345678901234567890123456789",
		Existing: []Finding{},
		New:      []Finding{mandatoryBoundaryFinding(false)},
		Fixed:    []Finding{},
	}
	Normalize(&report)
	if len(report.Debt.New) != 1 || !report.Debt.New[0].Blocking {
		t.Fatalf("mandatory debt finding was not restored: %#v", report.Debt.New)
	}
}

func TestValidateRejectsMandatoryBlockingDowngrade(t *testing.T) {
	report := NewReport(ProjectIdentity{})
	report.Findings = []Finding{mandatoryBoundaryFinding(false)}
	err := Validate(report)
	if err == nil || !strings.Contains(err.Error(), "must remain blocking") {
		t.Fatalf("mandatory downgrade escaped validation: %v", err)
	}
}

func TestMandatoryOperationGrowthRuleIsNotQualityPolicyTunable(t *testing.T) {
	err := ValidateQualityPolicy(QualityPolicy{
		SchemaVersion: QualityPolicySchemaVersion,
		BlockingRules: []string{RuleOperationGrowthBoundary},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported or advisory-only") {
		t.Fatalf("mandatory rule became quality-policy tunable: %v", err)
	}
}

func mandatoryBoundaryFinding(blocking bool) Finding {
	return Finding{
		ID:          "AUDIT-BOUNDARY-001:orders.list:operation_added",
		Rule:        RuleOperationGrowthBoundary,
		Class:       FindingProvenViolation,
		Blocking:    blocking,
		Subject:     "orders.list",
		Summary:     "Operation Growth is not proven to remain inside the existing Service Boundary",
		Invariant:   "new Operation Growth must be boundary-qualified",
		Reason:      "test fixture",
		Remediation: "review boundary evidence",
		Evidence: []Evidence{
			{Kind: EvidenceGit, Source: "git.base", Detail: "0123456789012345678901234567890123456789"},
			{Kind: EvidenceCanonical, Source: "contract.source", Detail: "operation_added"},
		},
	}
}
