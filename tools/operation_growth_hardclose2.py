#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path):
    return (ROOT / path).read_text()

def write(path, text):
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text)

def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:180]!r}")
    write(path, text.replace(old, new, 1))

# Operation Growth is architecture correctness, not a configurable quality
# preference. Keep it outside QualityPolicy.blockingRules and make it a mandatory
# auditcore invariant that survives any policy application order.
replace_once(
    "app/cmd/auditcore/quality.go",
    '''\tRuleGeneratedArtifactDrift = "AUDIT-GEN-003"
\tRuleFileLineLimit          = "AUDIT-SIZE-001"
\tRuleFileDeclarationLimit   = "AUDIT-SIZE-002"
\tRuleFileBranchLimit        = "AUDIT-COMPLEXITY-001"
)''',
    '''\tRuleGeneratedArtifactDrift = "AUDIT-GEN-003"
\tRuleFileLineLimit          = "AUDIT-SIZE-001"
\tRuleFileDeclarationLimit   = "AUDIT-SIZE-002"
\tRuleFileBranchLimit        = "AUDIT-COMPLEXITY-001"

\t// RuleOperationGrowthBoundary is mandatory architecture correctness. It is
\t// intentionally excluded from configurable QualityPolicy.blockingRules.
\tRuleOperationGrowthBoundary = "AUDIT-BOUNDARY-001"
)''')

replace_once(
    "app/cmd/auditcore/quality.go",
    '''func ApplyBlockingPolicy(findings []Finding, policy QualityPolicy) {
\tblocking := stringSet(policy.BlockingRules)
\tfor index := range findings {
\t\tfinding := &findings[index]
\t\t_, enabled := blocking[finding.Rule]
\t\tfinding.Blocking = enabled && finding.Class == FindingProvenViolation
\t}
}

func BlockingNewFindings''',
    '''func IsMandatoryBlockingRule(rule string) bool {
\tswitch strings.TrimSpace(rule) {
\tcase RuleOperationGrowthBoundary:
\t\treturn true
\tdefault:
\t\treturn false
\t}
}

func EnforceMandatoryBlocking(findings []Finding) {
\tfor index := range findings {
\t\tfinding := &findings[index]
\t\tif finding.Class == FindingProvenViolation && IsMandatoryBlockingRule(finding.Rule) {
\t\t\tfinding.Blocking = true
\t\t}
\t}
}

func ApplyBlockingPolicy(findings []Finding, policy QualityPolicy) {
\tblocking := stringSet(policy.BlockingRules)
\tfor index := range findings {
\t\tfinding := &findings[index]
\t\t_, enabled := blocking[finding.Rule]
\t\tfinding.Blocking = finding.Class == FindingProvenViolation && (enabled || IsMandatoryBlockingRule(finding.Rule))
\t}
}

func BlockingNewFindings''')

# Normalize is the last defensive boundary for reports/debt deltas assembled by
# multiple callers. Mandatory findings are restored even if an intermediate layer
# accidentally cleared Blocking.
replace_once(
    "app/cmd/auditcore/model.go",
    '''\tNormalizeSource(&report.Source)
\tnormalizeFindings(report.Findings)
\tif report.Findings == nil {''',
    '''\tNormalizeSource(&report.Source)
\tnormalizeFindings(report.Findings)
\tEnforceMandatoryBlocking(report.Findings)
\tif report.Findings == nil {''')

replace_once(
    "app/cmd/auditcore/model.go",
    '''\t\tnormalizeFindings(report.Debt.Existing)
\t\tnormalizeFindings(report.Debt.New)
\t\tnormalizeFindings(report.Debt.Fixed)
\t\tif report.Debt.Existing == nil {''',
    '''\t\tnormalizeFindings(report.Debt.Existing)
\t\tnormalizeFindings(report.Debt.New)
\t\tnormalizeFindings(report.Debt.Fixed)
\t\tEnforceMandatoryBlocking(report.Debt.Existing)
\t\tEnforceMandatoryBlocking(report.Debt.New)
\t\tEnforceMandatoryBlocking(report.Debt.Fixed)
\t\tif report.Debt.Existing == nil {''')

replace_once(
    "app/cmd/auditcore/model.go",
    '''\t\tswitch finding.Class {
\t\tcase FindingProvenViolation:
\t\t\tif finding.Invariant == "" {
\t\t\t\treturn fmt.Errorf("proven finding %s invariant is required", finding.ID)
\t\t\t}
\t\tcase FindingEvidenceObservation:''',
    '''\t\tswitch finding.Class {
\t\tcase FindingProvenViolation:
\t\t\tif finding.Invariant == "" {
\t\t\t\treturn fmt.Errorf("proven finding %s invariant is required", finding.ID)
\t\t\t}
\t\t\tif IsMandatoryBlockingRule(finding.Rule) && !finding.Blocking {
\t\t\t\treturn fmt.Errorf("mandatory finding %s rule %s must remain blocking", finding.ID, finding.Rule)
\t\t\t}
\t\tcase FindingEvidenceObservation:''')

# Preserve the audit package compatibility constant while making auditcore the
# single authority for mandatory classification.
replace_once(
    "app/cmd/audit/boundary_growth.go",
    '''const RuleOperationGrowthBoundary = "AUDIT-BOUNDARY-001"''',
    '''const RuleOperationGrowthBoundary = auditcore.RuleOperationGrowthBoundary''')

# Explicitly prove that an existing quality policy with an empty blockingRules
# list cannot disable direct Operation Growth.
replace_once(
    "app/cmd/audit/boundary_growth_test.go",
    '''\twriteAuditProjectFile(t, filepath.Join(root, "internal", "sales", "application", "service.go"), "// Package application owns the fixture.\\npackage application\\n")
\tgitAudit(t, root, "init")''',
    '''\twriteAuditProjectFile(t, filepath.Join(root, "internal", "sales", "application", "service.go"), "// Package application owns the fixture.\\npackage application\\n")
\twriteAuditProjectFile(t, filepath.Join(root, auditcore.QualityPolicyRelativePath), "{\\\"schemaVersion\\\":1,\\\"blockingRules\\\":[]}\\n")
\tgitAudit(t, root, "init")''')

replace_once(
    "app/cmd/audit/boundary_growth_test.go",
    '''\tif !found {
\t\tt.Fatalf("new debt=%#v", report.Debt.New)
\t}
\tif len(auditcore.BlockingNewFindings(report)) == 0 {''',
    '''\tif !found {
\t\tt.Fatalf("new debt=%#v", report.Debt.New)
\t}
\tif !report.QualityPolicy.Present || len(report.QualityPolicy.BlockingRules) != 0 {
\t\tt.Fatalf("expected explicit empty quality policy, got %#v", report.QualityPolicy)
\t}
\tif len(auditcore.BlockingNewFindings(report)) == 0 {''')

write("app/cmd/auditcore/mandatory_blocking_test.go", r'''package auditcore

import (
    "strings"
    "testing"
)

func TestApplyBlockingPolicyCannotDisableMandatoryOperationGrowth(t *testing.T) {
    findings := []Finding{
        mandatoryBoundaryFinding(false),
        {
            ID: "AUDIT-AUTH-001:demo",
            Rule: RuleAuthorizationBypass,
            Class: FindingProvenViolation,
            Subject: "demo",
            Summary: "ordinary configurable rule",
            Invariant: "ordinary rule remains policy-controlled",
            Evidence: []Evidence{{Kind: EvidenceCanonical, Source: "test"}},
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
        BaseRef: "HEAD",
        BaseSHA: "0123456789012345678901234567890123456789",
        Existing: []Finding{},
        New: []Finding{mandatoryBoundaryFinding(false)},
        Fixed: []Finding{},
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
        ID: "AUDIT-BOUNDARY-001:orders.list:operation_added",
        Rule: RuleOperationGrowthBoundary,
        Class: FindingProvenViolation,
        Blocking: blocking,
        Subject: "orders.list",
        Summary: "Operation Growth is not proven to remain inside the existing Service Boundary",
        Invariant: "new Operation Growth must be boundary-qualified",
        Reason: "test fixture",
        Remediation: "review boundary evidence",
        Evidence: []Evidence{
            {Kind: EvidenceGit, Source: "git.base", Detail: "0123456789012345678901234567890123456789"},
            {Kind: EvidenceCanonical, Source: "contract.source", Detail: "operation_added"},
        },
    }
}
''')

print("hardclose2 mandatory boundary blocking invariant prepared")
