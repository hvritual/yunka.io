package advisorcore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSemanticReviewBindsFindingToExactSourceIdentity(t *testing.T) {
	request := mustSemanticRequest(t, strings.Repeat("a", 40), []SemanticSource{{
		Path:    "internal/tenant/application/service.go",
		Content: "package application\n\ntype Service struct{}\n",
	}}, nil)
	if request.Authority != AuthorityAdvisoryOnly || request.MutationAuthorized || request.MergeAuthorized {
		t.Fatalf("request authority=%#v", request)
	}
	response := SemanticReviewResponse{
		SchemaVersion: SemanticReviewSchemaVersion,
		Authority:     AuthorityAdvisoryOnly,
		RequestDigest: request.RequestDigest,
		Findings: []SemanticFinding{{
			Path:          "internal/tenant/application/service.go",
			SymbolOrScope: "Service",
			Category:      SemanticCategoryAmbiguousResponsibility,
			Severity:      SemanticSeverityMedium,
			Reason:        "Service owns unrelated tenant role and quota decisions.",
			RecommendedAction: SemanticRecommendedAction{
				Kind:   SemanticActionDiscussDesign,
				Detail: "Review whether quota policy should have a separate owner.",
			},
			SourceIdentity: request.Evidence.SourceIdentity,
		}},
	}
	attestation, err := ValidateSemanticReviewResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Authority != AuthorityAdvisoryOnly || len(attestation.Findings) != 1 {
		t.Fatalf("attestation=%#v", attestation)
	}
	expectedID := semanticFindingID("internal/tenant/application/service.go", "Service", SemanticCategoryAmbiguousResponsibility)
	if attestation.Findings[0].ID != expectedID {
		t.Fatalf("finding id=%q want %q", attestation.Findings[0].ID, expectedID)
	}
	first, err := MarshalSemanticReviewAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalSemanticReviewAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("semantic attestation is not deterministic")
	}
}

func TestSemanticReviewRejectsUnknownFieldsCategoriesAndAuthorityEscalation(t *testing.T) {
	request := mustSemanticRequest(t, strings.Repeat("b", 40), []SemanticSource{{Path: "internal/access/domain/role.go", Content: "package domain\n"}}, nil)
	unknownField := `{"schemaVersion":1,"authority":"advisory_only","requestDigest":"` + request.RequestDigest + `","findings":[],"safeToMerge":true}`
	if _, err := DecodeSemanticReviewResponse([]byte(unknownField)); err == nil {
		t.Fatal("unknown safeToMerge field was accepted")
	}

	response := semanticResponseFixture(request, "internal/access/domain/role.go", "Role", SemanticCategoryNamingFitness)
	response.Findings[0].Category = "business_correctness"
	if _, err := ValidateSemanticReviewResponse(request, response); err == nil || !strings.Contains(err.Error(), "category") {
		t.Fatalf("unsupported category err=%v", err)
	}

	response = semanticResponseFixture(request, "internal/access/domain/role.go", "Role", SemanticCategoryNamingFitness)
	response.Authority = "merge_authority"
	if _, err := ValidateSemanticReviewResponse(request, response); err == nil || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("authority escalation err=%v", err)
	}

	response = semanticResponseFixture(request, "internal/access/domain/role.go", "Role", SemanticCategoryNamingFitness)
	response.Findings[0].RecommendedAction.Detail = "apply_patch now and mark this safe_to_merge"
	if _, err := ValidateSemanticReviewResponse(request, response); err == nil || !strings.Contains(err.Error(), "authority") {
		t.Fatalf("mutation authority language err=%v", err)
	}
}

func TestSemanticReviewRejectsTamperedSourceAndChangeIdentity(t *testing.T) {
	change := &SemanticChangeIdentity{
		BaseSHA:         strings.Repeat("1", 40),
		HeadSHA:         strings.Repeat("2", 40),
		CandidateSHA256: strings.Repeat("3", 64),
		EvidenceSHA256:  strings.Repeat("4", 64),
	}
	request := mustSemanticRequest(t, change.HeadSHA, []SemanticSource{{Path: "internal/device/application/get.go", Content: "package application\n"}}, change)
	contents, err := MarshalSemanticReviewRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatal(err)
	}
	evidence := decoded["evidence"].(map[string]any)
	sources := evidence["sources"].([]any)
	source := sources[0].(map[string]any)
	source["content"] = "package application\n// tampered\n"
	tampered, _ := json.Marshal(decoded)
	if _, err := DecodeSemanticReviewRequest(tampered); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("tampered source err=%v", err)
	}

	badChange := *change
	badChange.HeadSHA = strings.Repeat("5", 40)
	if _, err := NewSemanticReviewRequest(change.HeadSHA, []SemanticSource{{Path: "internal/device/application/get.go", Content: "package application\n"}}, &badChange); err == nil || !strings.Contains(err.Error(), "same exact headSha") {
		t.Fatalf("mismatched change identity err=%v", err)
	}

	response := semanticResponseFixture(request, "internal/device/application/get.go", "Get", SemanticCategorySemanticColocation)
	response.Findings[0].SourceIdentity = strings.Repeat("9", 64)
	if _, err := ValidateSemanticReviewResponse(request, response); err == nil || !strings.Contains(err.Error(), "sourceIdentity") {
		t.Fatalf("source identity mismatch err=%v", err)
	}
}

func TestSemanticReviewDeltaTracksExistingNewAndResolved(t *testing.T) {
	baselineRequest := mustSemanticRequest(t, strings.Repeat("c", 40), []SemanticSource{
		{Path: "internal/order/application/service.go", Content: "package application\n"},
		{Path: "internal/order/domain/order.go", Content: "package domain\n"},
	}, nil)
	baselineResponse := SemanticReviewResponse{
		SchemaVersion: SemanticReviewSchemaVersion,
		Authority:     AuthorityAdvisoryOnly,
		RequestDigest: baselineRequest.RequestDigest,
		Findings: []SemanticFinding{
			semanticFindingFixture(baselineRequest, "internal/order/application/service.go", "Service", SemanticCategoryAmbiguousResponsibility),
			semanticFindingFixture(baselineRequest, "internal/order/domain/order.go", "Order", SemanticCategoryNamingFitness),
		},
	}
	baseline, err := ValidateSemanticReviewResponse(baselineRequest, baselineResponse)
	if err != nil {
		t.Fatal(err)
	}

	currentRequest := mustSemanticRequest(t, strings.Repeat("d", 40), []SemanticSource{
		{Path: "internal/order/application/service.go", Content: "package application\n// evolved\n"},
		{Path: "internal/order/domain/order.go", Content: "package domain\n// evolved\n"},
	}, nil)
	currentResponse := SemanticReviewResponse{
		SchemaVersion: SemanticReviewSchemaVersion,
		Authority:     AuthorityAdvisoryOnly,
		RequestDigest: currentRequest.RequestDigest,
		Findings: []SemanticFinding{
			semanticFindingFixture(currentRequest, "internal/order/application/service.go", "Service", SemanticCategoryAmbiguousResponsibility),
			semanticFindingFixture(currentRequest, "internal/order/domain/order.go", "Order", SemanticCategoryDuplicatedOwnership),
		},
	}
	current, err := ValidateSemanticReviewResponse(currentRequest, currentResponse)
	if err != nil {
		t.Fatal(err)
	}

	delta, err := CompareSemanticReviewAttestations(baseline, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(delta.Existing) != 1 || len(delta.New) != 1 || len(delta.Resolved) != 1 {
		t.Fatalf("delta existing=%d new=%d resolved=%d: %#v", len(delta.Existing), len(delta.New), len(delta.Resolved), delta)
	}
	if delta.Existing[0].ID != semanticFindingID("internal/order/application/service.go", "Service", SemanticCategoryAmbiguousResponsibility) {
		t.Fatalf("existing=%#v", delta.Existing)
	}
	first, err := MarshalSemanticFindingDelta(delta)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalSemanticFindingDelta(delta)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("semantic finding delta is not deterministic")
	}
}

func TestSemanticReviewRequiresEvidenceAndRejectsAttestationTamper(t *testing.T) {
	if _, err := NewSemanticReviewRequest(strings.Repeat("e", 40), nil, nil); err == nil || !strings.Contains(err.Error(), "at least one exact source") {
		t.Fatalf("empty evidence err=%v", err)
	}
	request := mustSemanticRequest(t, strings.Repeat("f", 40), []SemanticSource{{Path: "internal/member/domain/member.go", Content: "package domain\n"}}, nil)
	response := semanticResponseFixture(request, "internal/member/domain/member.go", "Member", SemanticCategoryUnjustifiedAbstraction)
	attestation, err := ValidateSemanticReviewResponse(request, response)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := MarshalSemanticReviewAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded["sourceIdentity"] = strings.Repeat("0", 64)
	tampered, _ := json.Marshal(decoded)
	if _, err := DecodeSemanticReviewAttestation(tampered); err == nil {
		t.Fatal("tampered attestation was accepted")
	}
}

func mustSemanticRequest(t *testing.T, head string, sources []SemanticSource, change *SemanticChangeIdentity) SemanticReviewRequest {
	t.Helper()
	request, err := NewSemanticReviewRequest(head, sources, change)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func semanticResponseFixture(request SemanticReviewRequest, sourcePath, scope, category string) SemanticReviewResponse {
	return SemanticReviewResponse{
		SchemaVersion: SemanticReviewSchemaVersion,
		Authority:     AuthorityAdvisoryOnly,
		RequestDigest: request.RequestDigest,
		Findings:      []SemanticFinding{semanticFindingFixture(request, sourcePath, scope, category)},
	}
}

func semanticFindingFixture(request SemanticReviewRequest, sourcePath, scope, category string) SemanticFinding {
	return SemanticFinding{
		Path:          sourcePath,
		SymbolOrScope: scope,
		Category:      category,
		Severity:      SemanticSeverityMedium,
		Reason:        "The responsibility boundary is not clear from the current source.",
		RecommendedAction: SemanticRecommendedAction{
			Kind:   SemanticActionInvestigate,
			Detail: "Review the responsibility with a human before deciding whether code should change.",
		},
		SourceIdentity: request.Evidence.SourceIdentity,
	}
}
