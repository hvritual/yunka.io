package audit

import (
	"strings"
	"testing"

	"yunka.io/app/cmd/auditcore"
)

func TestRenderAuditReportIsStableAndNonBlocking(t *testing.T) {
	report := auditcore.NewReport(auditcore.ProjectIdentity{GoModule: "example.com/demo", Profiled: true})
	report.Source = auditcore.SourceSnapshot{
		SourceRoot: "internal",
		Files: []auditcore.GoSourceFile{{
			Path:    "internal/tenant/application/service.go",
			Package: "application",
			Imports: []string{"github.com/hvritual/yunka.io/framework/platform"},
		}},
	}
	report.Findings = []auditcore.Finding{{
		ID:        "AUDIT-INFRA-001:internal/tenant/application/service.go:github.com/hvritual/yunka.io/framework/platform",
		Rule:      auditcore.RulePlatformProviderBypass,
		Class:     auditcore.FindingProvenViolation,
		Subject:   "tenant",
		Summary:   "application imports the framework platform provider directly",
		Invariant: "process infrastructure is App-owned",
		Evidence: []auditcore.Evidence{
			{Kind: auditcore.EvidenceCanonical, Source: "contract.manifest", Detail: "declared application domain=tenant"},
			{Kind: auditcore.EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "github.com/hvritual/yunka.io/framework/platform"},
		},
	}}

	text, err := Render(report, "text")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "FINDINGS 1") || !strings.Contains(text, auditcore.RulePlatformProviderBypass) {
		t.Fatalf("unexpected text output:\n%s", text)
	}
	jsonOutput, err := Render(report, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOutput, `"class": "proven_violation"`) || !strings.Contains(jsonOutput, `"sourceRoot": "internal"`) {
		t.Fatalf("unexpected agent output:\n%s", jsonOutput)
	}
}

func TestRenderAuditRejectsUnsupportedFormat(t *testing.T) {
	_, err := Render(auditcore.NewReport(auditcore.ProjectIdentity{}), "yaml")
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}
