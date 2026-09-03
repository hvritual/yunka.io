package advisorcore

import (
	"strings"
	"testing"

	"yunka.io/app/cmd/auditcore"
)

func TestRequestAndAttestationAreDeterministicAndEvidenceBound(t *testing.T) {
	report := testAuditReport()
	first, err := NewRequest(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRequest(report)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := MarshalRequest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := MarshalRequest(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("request is not deterministic:\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
	decoded, err := DecodeRequest(firstJSON)
	if err != nil {
		t.Fatal(err)
	}
	response := Response{
		SchemaVersion: SchemaVersion,
		RequestDigest: decoded.RequestDigest,
		Recommendations: []Recommendation{
			{
				ID:         "R-2",
				Kind:       KindDesignQuestion,
				Title:      "Confirm intended boundary",
				Rationale:  "The deterministic observation needs human architectural context before any mutation is proposed.",
				FindingIDs: []string{"OBS-1"},
			},
			{
				ID:         "R-1",
				Kind:       KindRemediationRecommendation,
				Title:      "Remove direct provider dependency",
				Rationale:  "The proven violation bypasses the typed capability constructor boundary.",
				FindingIDs: []string{"PROVEN-1"},
			},
		},
	}
	attestation, err := ValidateResponse(decoded, response)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Authority != AuthorityAdvisoryOnly || attestation.Result != ResultValid {
		t.Fatalf("unexpected attestation: %#v", attestation)
	}
	if len(attestation.Bindings) != 2 || attestation.Bindings[0].RecommendationID != "R-1" || attestation.Bindings[1].RecommendationID != "R-2" {
		t.Fatalf("bindings are not normalized: %#v", attestation.Bindings)
	}
	firstAttestation, err := MarshalAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	secondAttestation, err := MarshalAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstAttestation) != string(secondAttestation) {
		t.Fatal("attestation is not deterministic")
	}
}

func TestResponseFailsClosedForUnknownFindingAndDigestMismatch(t *testing.T) {
	request, err := NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	response := Response{
		SchemaVersion: SchemaVersion,
		RequestDigest: request.RequestDigest,
		Recommendations: []Recommendation{{
			ID:         "R-1",
			Kind:       KindInvestigationHypothesis,
			Title:      "Investigate",
			Rationale:  "Evidence must exist.",
			FindingIDs: []string{"MISSING"},
		}},
	}
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "unknown current finding") {
		t.Fatalf("expected unknown finding failure, got %v", err)
	}
	response.Recommendations[0].FindingIDs = []string{"PROVEN-1"}
	response.RequestDigest = "tampered"
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "does not match request") {
		t.Fatalf("expected request digest failure, got %v", err)
	}
}

func TestRemediationCannotPromoteObservationToProvenViolation(t *testing.T) {
	request, err := NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	response := Response{
		SchemaVersion: SchemaVersion,
		RequestDigest: request.RequestDigest,
		Recommendations: []Recommendation{{
			ID:         "R-1",
			Kind:       KindRemediationRecommendation,
			Title:      "Change architecture",
			Rationale:  "This must not be allowed from observation-only evidence.",
			FindingIDs: []string{"OBS-1"},
		}},
	}
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "only proven_violation") {
		t.Fatalf("expected observation remediation failure, got %v", err)
	}
}

func TestStrictResponseDecoderRejectsAuthorityInjectionAndTrailingJSON(t *testing.T) {
	request, err := NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	injected := `{"schemaVersion":1,"requestDigest":"` + request.RequestDigest + `","authority":"safe_to_merge","recommendations":[]}`
	if _, err := DecodeResponse([]byte(injected)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown authority field rejection, got %v", err)
	}
	trailing := `{"schemaVersion":1,"requestDigest":"` + request.RequestDigest + `","recommendations":[]} {}`
	if _, err := DecodeResponse([]byte(trailing)); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("expected trailing JSON rejection, got %v", err)
	}
}

func TestRequestDigestDetectsEvidenceOrPolicyTampering(t *testing.T) {
	request, err := NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	request.Audit.Findings[0].Summary = "tampered"
	if _, err := MarshalRequest(request); err == nil || !strings.Contains(err.Error(), "auditDigest") {
		t.Fatalf("expected evidence digest failure, got %v", err)
	}
	request, err = NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	request.Policy.MutationAuthorized = true
	if _, err := MarshalRequest(request); err == nil || !strings.Contains(err.Error(), "cannot authorize mutation") {
		t.Fatalf("expected policy authority failure, got %v", err)
	}
}

func TestZeroFindingRequestAllowsOnlyEmptyAdvice(t *testing.T) {
	request, err := NewRequest(auditcore.NewReport(auditcore.ProjectIdentity{GoModule: "example.com/demo"}))
	if err != nil {
		t.Fatal(err)
	}
	empty := Response{SchemaVersion: SchemaVersion, RequestDigest: request.RequestDigest, Recommendations: []Recommendation{}}
	if _, err := ValidateResponse(request, empty); err != nil {
		t.Fatalf("empty advisory response should be valid: %v", err)
	}
	nonempty := Response{
		SchemaVersion: SchemaVersion,
		RequestDigest: request.RequestDigest,
		Recommendations: []Recommendation{{
			ID:         "R-1",
			Kind:       KindDesignQuestion,
			Title:      "Invent a question",
			Rationale:  "There is no deterministic evidence to bind this to.",
			FindingIDs: []string{"invented"},
		}},
	}
	if _, err := ValidateResponse(request, nonempty); err == nil || !strings.Contains(err.Error(), "require deterministic Audit findings") {
		t.Fatalf("expected zero-finding failure, got %v", err)
	}
}

func TestResponseShapeRejectsDuplicatesAndUnsupportedKinds(t *testing.T) {
	request, err := NewRequest(testAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	response := Response{
		SchemaVersion: SchemaVersion,
		RequestDigest: request.RequestDigest,
		Recommendations: []Recommendation{
			{ID: "R", Kind: KindDesignQuestion, Title: "One", Rationale: "One", FindingIDs: []string{"PROVEN-1"}},
			{ID: "R", Kind: KindDesignQuestion, Title: "Two", Rationale: "Two", FindingIDs: []string{"OBS-1"}},
		},
	}
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "duplicate recommendation") {
		t.Fatalf("expected duplicate recommendation failure, got %v", err)
	}
	response.Recommendations = []Recommendation{{
		ID: "R", Kind: RecommendationKind("apply_patch"), Title: "Patch", Rationale: "Not an advisory kind", FindingIDs: []string{"PROVEN-1"},
	}}
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported kind failure, got %v", err)
	}
	response.Recommendations = []Recommendation{{
		ID: "R", Kind: KindDesignQuestion, Title: "Duplicate evidence", Rationale: "Duplicate refs are ambiguous", FindingIDs: []string{"PROVEN-1", "PROVEN-1"},
	}}
	if _, err := ValidateResponse(request, response); err == nil || !strings.Contains(err.Error(), "repeats finding id") {
		t.Fatalf("expected duplicate finding failure, got %v", err)
	}
}

func testAuditReport() auditcore.Report {
	report := auditcore.NewReport(auditcore.ProjectIdentity{GoModule: "example.com/demo"})
	report.Source = auditcore.SourceSnapshot{SourceRoot: "internal", Files: []auditcore.GoSourceFile{{Path: "internal/tenant/application/service.go", Package: "application"}}}
	report.Findings = []auditcore.Finding{
		{
			ID:        "PROVEN-1",
			Rule:      auditcore.RulePlatformProviderBypass,
			Class:     auditcore.FindingProvenViolation,
			Subject:   "tenant",
			Summary:   "application imports the framework platform provider directly",
			Invariant: "business Applications receive declared typed capabilities",
			Evidence:  []auditcore.Evidence{{Kind: auditcore.EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "github.com/hvritual/yunka.io/framework/platform"}},
		},
		{
			ID:       "OBS-1",
			Rule:     "AUDIT-OBS-001",
			Class:    auditcore.FindingEvidenceObservation,
			Subject:  "tenant",
			Summary:  "deterministic observation for human investigation",
			Evidence: []auditcore.Evidence{{Kind: auditcore.EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "example.com/observation"}},
		},
	}
	auditcore.Normalize(&report)
	return report
}
