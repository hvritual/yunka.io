package auditcore

import (
	"strings"
	"testing"
)

func TestMarshalNormalizesFindingAndEvidenceOrderDeterministically(t *testing.T) {
	left := Report{
		SchemaVersion: SchemaVersion,
		Project:       ProjectIdentity{GoModule: " example.com/demo ", Profiled: true},
		Findings: []Finding{
			{
				ID:        "AUDIT-INFRA-001:tenant/lifecycle",
				Rule:      "AUDIT-INFRA-001",
				Class:     FindingProvenViolation,
				Subject:   " tenant/lifecycle ",
				Summary:   " direct infrastructure ownership ",
				Invariant: " infrastructure is App-owned and injected as typed capability ",
				Evidence: []Evidence{
					{Kind: EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "framework/platform"},
					{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "application declares no matching capability"},
					{Kind: EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "framework/platform"},
				},
			},
			{
				ID:      "AUDIT-OBS-001:tenant/lifecycle",
				Rule:    "AUDIT-OBS-001",
				Class:   FindingEvidenceObservation,
				Subject: "tenant/lifecycle",
				Summary: "high cross-domain dependency surface",
				Evidence: []Evidence{
					{Kind: EvidenceCanonical, Source: "application.graph", Detail: "cross-domain dependencies=4"},
				},
			},
		},
	}
	right := Report{
		SchemaVersion: SchemaVersion,
		Project:       ProjectIdentity{GoModule: "example.com/demo", Profiled: true},
		Findings: []Finding{
			left.Findings[1],
			{
				ID:        left.Findings[0].ID,
				Rule:      left.Findings[0].Rule,
				Class:     left.Findings[0].Class,
				Subject:   "tenant/lifecycle",
				Summary:   "direct infrastructure ownership",
				Invariant: "infrastructure is App-owned and injected as typed capability",
				Evidence: []Evidence{
					left.Findings[0].Evidence[1],
					left.Findings[0].Evidence[0],
				},
			},
		},
	}

	leftBytes, err := Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	rightBytes, err := Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftBytes) != string(rightBytes) {
		t.Fatalf("audit report is not byte-stable:\nleft:\n%s\nright:\n%s", leftBytes, rightBytes)
	}
	if strings.Count(string(leftBytes), "framework/platform") != 1 {
		t.Fatalf("duplicate evidence was not normalized:\n%s", leftBytes)
	}
}

func TestValidateRejectsProbabilisticFindingClasses(t *testing.T) {
	report := NewReport(ProjectIdentity{GoModule: "example.com/demo"})
	report.Findings = append(report.Findings, Finding{
		ID:      "DESIGN-001:tenant",
		Rule:    "DESIGN-001",
		Class:   FindingClass("design_hypothesis"),
		Subject: "tenant",
		Summary: "tenant may be too broad",
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: "application.graph"},
		},
	})
	Normalize(&report)
	if err := Validate(report); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported probabilistic class error, got %v", err)
	}
}

func TestValidateRequiresInvariantForProvenViolation(t *testing.T) {
	report := NewReport(ProjectIdentity{GoModule: "example.com/demo"})
	report.Findings = append(report.Findings, Finding{
		ID:      "AUDIT-APP-001:tenant/lifecycle",
		Rule:    "AUDIT-APP-001",
		Class:   FindingProvenViolation,
		Subject: "tenant/lifecycle",
		Summary: "cross-application repository bypass",
		Evidence: []Evidence{
			{Kind: EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go"},
		},
	})
	Normalize(&report)
	if err := Validate(report); err == nil || !strings.Contains(err.Error(), "invariant") {
		t.Fatalf("expected invariant validation error, got %v", err)
	}
}
