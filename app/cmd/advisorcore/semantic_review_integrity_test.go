package advisorcore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSemanticReviewAttestationRejectsChangedFindingEvenWithRecomputedOuterDigest(t *testing.T) {
	request := mustSemanticRequest(t, strings.Repeat("a", 40), []SemanticSource{{
		Path:    "internal/tenant/application/service.go",
		Content: "package application\n",
	}}, nil)
	response := semanticResponseFixture(request, "internal/tenant/application/service.go", "Service", SemanticCategoryAmbiguousResponsibility)
	attestation, err := ValidateSemanticReviewResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}

	attestation.Findings[0].Reason = "tampered finding content"
	payload, err := json.Marshal(semanticAttestationPayload{
		SchemaVersion:  attestation.SchemaVersion,
		Authority:      attestation.Authority,
		RequestDigest:  attestation.RequestDigest,
		ResponseDigest: attestation.ResponseDigest,
		SourceIdentity: attestation.SourceIdentity,
		Findings:       attestation.Findings,
		Result:         attestation.Result,
	})
	if err != nil {
		t.Fatal(err)
	}
	attestation.AttestationDigest = digest(payload)

	contents, err := json.Marshal(attestation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSemanticReviewAttestation(contents); err == nil || !strings.Contains(err.Error(), "responseDigest") {
		t.Fatalf("expected response digest rejection, got %v", err)
	}
}
