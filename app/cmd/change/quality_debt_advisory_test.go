package change

import (
	"strings"
	"testing"
	"time"

	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/auditcore"
)

func TestQualityDebtAdvisoryNewFindingRemainsVisibleAndNonBlocking(t *testing.T) {
	baselineRequest := semanticQualityRequest(t, strings.Repeat("1", 40), "package application\n")
	baselineResponse := advisorcore.SemanticReviewResponse{
		SchemaVersion: advisorcore.SemanticReviewSchemaVersion,
		Authority:     advisorcore.AuthorityAdvisoryOnly,
		RequestDigest: baselineRequest.RequestDigest,
		Findings:      []advisorcore.SemanticFinding{},
	}
	baseline, err := advisorcore.ValidateSemanticReviewResponse(baselineRequest, baselineResponse)
	if err != nil {
		t.Fatal(err)
	}

	currentRequest := semanticQualityRequest(t, strings.Repeat("2", 40), "package application\n// current responsibility is ambiguous\n")
	currentResponse := advisorcore.SemanticReviewResponse{
		SchemaVersion: advisorcore.SemanticReviewSchemaVersion,
		Authority:     advisorcore.AuthorityAdvisoryOnly,
		RequestDigest: currentRequest.RequestDigest,
		Findings: []advisorcore.SemanticFinding{{
			Path:          "internal/tenant/application/service.go",
			SymbolOrScope: "Service",
			Category:      advisorcore.SemanticCategoryAmbiguousResponsibility,
			Severity:      advisorcore.SemanticSeverityHigh,
			Reason:        "Service appears to own unrelated responsibilities.",
			RecommendedAction: advisorcore.SemanticRecommendedAction{
				Kind:   advisorcore.SemanticActionDiscussDesign,
				Detail: "Review the responsibility boundary with a human before changing code.",
			},
			BehaviorChangeRequired: false,
			SourceIdentity:         currentRequest.Evidence.SourceIdentity,
		}},
	}
	current, err := advisorcore.ValidateSemanticReviewResponse(currentRequest, currentResponse)
	if err != nil {
		t.Fatal(err)
	}
	delta, err := advisorcore.CompareSemanticReviewAttestations(baseline, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(delta.New) != 1 {
		t.Fatalf("advisory delta=%#v", delta)
	}

	baseSHA := strings.Repeat("3", 40)
	proof, err := BuildQualityDebtProof(
		auditcore.DebtDelta{BaseRef: "base", BaseSHA: baseSHA, Existing: []auditcore.Finding{}, New: []auditcore.Finding{}, Fixed: []auditcore.Finding{}},
		&delta,
		nil,
		baseSHA,
		strings.Repeat("4", 40),
		time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Advisory == nil || len(proof.Advisory.New) != 1 {
		t.Fatalf("proof advisory=%#v", proof.Advisory)
	}
	if len(proof.BlockingNew) != 0 || len(proof.UnwaivedBlocking) != 0 {
		t.Fatalf("advisory finding gained blocking authority: %#v", proof)
	}
}

func semanticQualityRequest(t *testing.T, headSHA, content string) advisorcore.SemanticReviewRequest {
	t.Helper()
	request, err := advisorcore.NewSemanticReviewRequest(headSHA, []advisorcore.SemanticSource{{
		Path:    "internal/tenant/application/service.go",
		Content: content,
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
