package advisorcore

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	SemanticReviewSchemaVersion = 1
	SemanticReviewResultValid   = "valid"

	SemanticCategoryColocation             = "semantic_colocation"
	SemanticCategoryAmbiguousResponsibility = "ambiguous_responsibility"
	SemanticCategoryUnjustifiedAbstraction = "unjustified_abstraction"
	SemanticCategoryNamingFitness          = "naming_fitness"
	SemanticCategoryDuplicatedOwnership    = "duplicated_concept_ownership"

	SemanticSeverityLow    = "low"
	SemanticSeverityMedium = "medium"
	SemanticSeverityHigh   = "high"

	SemanticActionInvestigate      = "investigate"
	SemanticActionDiscussDesign    = "discuss_design"
	SemanticActionConsiderRefactor = "consider_refactor"
	SemanticActionConsiderRename   = "consider_rename"
	SemanticActionDocumentDecision = "document_decision"
)

type SemanticSource struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Content string `json:"content"`
}

type SemanticChangeIdentity struct {
	BaseSHA         string `json:"baseSha"`
	HeadSHA         string `json:"headSha"`
	CandidateSHA256 string `json:"candidateSha256"`
	EvidenceSHA256  string `json:"evidenceSha256"`
}

type SemanticEvidence struct {
	HeadSHA        string                  `json:"headSha"`
	Sources        []SemanticSource        `json:"sources"`
	Change         *SemanticChangeIdentity `json:"change,omitempty"`
	SourceSHA256   string                  `json:"sourceSha256"`
	SourceIdentity string                  `json:"sourceIdentity"`
}

// SemanticSourceBinding preserves the exact path/content identity of reviewed
// source without copying source contents into downstream attestations.
type SemanticSourceBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// SemanticEvidenceBinding is a compact projection of the exact request
// evidence. Consumers must re-read authoritative source bytes before granting
// candidate-level meaning to the binding; the digests alone are not authority.
type SemanticEvidenceBinding struct {
	HeadSHA        string                  `json:"headSha"`
	Sources        []SemanticSourceBinding `json:"sources"`
	Change         *SemanticChangeIdentity `json:"change,omitempty"`
	SourceSHA256   string                  `json:"sourceSha256"`
	SourceIdentity string                  `json:"sourceIdentity"`
}

type SemanticReviewRequest struct {
	SchemaVersion      int              `json:"schemaVersion"`
	Authority          string           `json:"authority"`
	MutationAuthorized bool             `json:"mutationAuthorized"`
	MergeAuthorized    bool             `json:"mergeAuthorized"`
	Evidence           SemanticEvidence `json:"evidence"`
	RequestDigest      string           `json:"requestDigest"`
}

type SemanticRecommendedAction struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type SemanticFinding struct {
	ID                     string                    `json:"id"`
	Path                   string                    `json:"path"`
	SymbolOrScope          string                    `json:"symbolOrScope"`
	Category               string                    `json:"category"`
	Severity               string                    `json:"severity"`
	Reason                 string                    `json:"reason"`
	RecommendedAction      SemanticRecommendedAction `json:"recommendedAction"`
	BehaviorChangeRequired bool                      `json:"behaviorChangeRequired"`
	SourceIdentity         string                    `json:"sourceIdentity"`
}

type SemanticReviewResponse struct {
	SchemaVersion int               `json:"schemaVersion"`
	Authority     string            `json:"authority"`
	RequestDigest string            `json:"requestDigest"`
	Findings      []SemanticFinding `json:"findings"`
}

type SemanticReviewAttestation struct {
	SchemaVersion     int                      `json:"schemaVersion"`
	Authority         string                   `json:"authority"`
	RequestDigest     string                   `json:"requestDigest"`
	ResponseDigest    string                   `json:"responseDigest"`
	SourceIdentity    string                   `json:"sourceIdentity"`
	Evidence          *SemanticEvidenceBinding `json:"evidence,omitempty"`
	Findings          []SemanticFinding        `json:"findings"`
	Result            string                   `json:"result"`
	AttestationDigest string                   `json:"attestationDigest"`
}

type SemanticFindingDelta struct {
	SchemaVersion          int                      `json:"schemaVersion"`
	BaselineSourceIdentity string                   `json:"baselineSourceIdentity"`
	CurrentSourceIdentity  string                   `json:"currentSourceIdentity"`
	BaselineEvidence       *SemanticEvidenceBinding `json:"baselineEvidence,omitempty"`
	CurrentEvidence        *SemanticEvidenceBinding `json:"currentEvidence,omitempty"`
	Existing               []SemanticFinding        `json:"existing"`
	New                    []SemanticFinding        `json:"new"`
	Resolved               []SemanticFinding        `json:"resolved"`
	DeltaDigest            string                   `json:"deltaDigest"`
}

type semanticRequestPayload struct {
	SchemaVersion      int              `json:"schemaVersion"`
	Authority          string           `json:"authority"`
	MutationAuthorized bool             `json:"mutationAuthorized"`
	MergeAuthorized    bool             `json:"mergeAuthorized"`
	Evidence           SemanticEvidence `json:"evidence"`
}

type semanticAttestationPayload struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	Authority      string                   `json:"authority"`
	RequestDigest  string                   `json:"requestDigest"`
	ResponseDigest string                   `json:"responseDigest"`
	SourceIdentity string                   `json:"sourceIdentity"`
	Evidence       *SemanticEvidenceBinding `json:"evidence,omitempty"`
	Findings       []SemanticFinding        `json:"findings"`
	Result         string                   `json:"result"`
}

type semanticDeltaPayload struct {
	SchemaVersion          int                      `json:"schemaVersion"`
	BaselineSourceIdentity string                   `json:"baselineSourceIdentity"`
	CurrentSourceIdentity  string                   `json:"currentSourceIdentity"`
	BaselineEvidence       *SemanticEvidenceBinding `json:"baselineEvidence,omitempty"`
	CurrentEvidence        *SemanticEvidenceBinding `json:"currentEvidence,omitempty"`
	Existing               []SemanticFinding        `json:"existing"`
	New                    []SemanticFinding        `json:"new"`
	Resolved               []SemanticFinding        `json:"resolved"`
}

func NewSemanticReviewRequest(headSHA string, sources []SemanticSource, change *SemanticChangeIdentity) (SemanticReviewRequest, error) {
	request := SemanticReviewRequest{
		SchemaVersion:      SemanticReviewSchemaVersion,
		Authority:          AuthorityAdvisoryOnly,
		MutationAuthorized: false,
		MergeAuthorized:    false,
		Evidence: SemanticEvidence{
			HeadSHA: strings.TrimSpace(headSHA),
			Sources: append([]SemanticSource(nil), sources...),
			Change:  cloneSemanticChangeIdentity(change),
		},
	}
	return canonicalSemanticRequest(request, false)
}

func DecodeSemanticReviewRequest(contents []byte) (SemanticReviewRequest, error) {
	var request SemanticReviewRequest
	if err := decodeStrict(contents, &request); err != nil {
		return SemanticReviewRequest{}, fmt.Errorf("semantic review request: %w", err)
	}
	return canonicalSemanticRequest(request, true)
}

func MarshalSemanticReviewRequest(request SemanticReviewRequest) ([]byte, error) {
	normalized, err := canonicalSemanticRequest(request, true)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func DecodeSemanticReviewResponse(contents []byte) (SemanticReviewResponse, error) {
	var response SemanticReviewResponse
	if err := decodeStrict(contents, &response); err != nil {
		return SemanticReviewResponse{}, fmt.Errorf("semantic review response: %w", err)
	}
	return canonicalSemanticResponse(response)
}

func MarshalSemanticReviewResponse(response SemanticReviewResponse) ([]byte, error) {
	normalized, err := canonicalSemanticResponse(response)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func ValidateSemanticReviewResponse(request SemanticReviewRequest, response SemanticReviewResponse) (SemanticReviewAttestation, error) {
	normalizedRequest, err := canonicalSemanticRequest(request, true)
	if err != nil {
		return SemanticReviewAttestation{}, err
	}
	normalizedResponse, err := canonicalSemanticResponse(response)
	if err != nil {
		return SemanticReviewAttestation{}, err
	}
	if normalizedResponse.RequestDigest != normalizedRequest.RequestDigest {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review response: requestDigest %q does not match request %q", normalizedResponse.RequestDigest, normalizedRequest.RequestDigest)
	}
	if len(normalizedRequest.Evidence.Sources) == 0 && len(normalizedResponse.Findings) != 0 {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review response: findings require non-empty source evidence")
	}
	sourcePaths := make(map[string]struct{}, len(normalizedRequest.Evidence.Sources))
	for _, source := range normalizedRequest.Evidence.Sources {
		sourcePaths[source.Path] = struct{}{}
	}
	for index := range normalizedResponse.Findings {
		finding := &normalizedResponse.Findings[index]
		if finding.SourceIdentity != normalizedRequest.Evidence.SourceIdentity {
			return SemanticReviewAttestation{}, fmt.Errorf("semantic review response: finding %s sourceIdentity does not match exact request evidence", finding.ID)
		}
		if _, ok := sourcePaths[finding.Path]; !ok {
			return SemanticReviewAttestation{}, fmt.Errorf("semantic review response: finding %s path %q is outside the exact source evidence", finding.ID, finding.Path)
		}
	}
	responseBytes, err := json.Marshal(normalizedResponse)
	if err != nil {
		return SemanticReviewAttestation{}, err
	}
	evidence := semanticEvidenceBinding(normalizedRequest.Evidence)
	attestation := SemanticReviewAttestation{
		SchemaVersion:  SemanticReviewSchemaVersion,
		Authority:      AuthorityAdvisoryOnly,
		RequestDigest:  normalizedRequest.RequestDigest,
		ResponseDigest: digest(responseBytes),
		SourceIdentity: normalizedRequest.Evidence.SourceIdentity,
		Evidence:       &evidence,
		Findings:       append([]SemanticFinding(nil), normalizedResponse.Findings...),
		Result:         SemanticReviewResultValid,
	}
	return canonicalSemanticAttestation(attestation, false)
}

func DecodeSemanticReviewAttestation(contents []byte) (SemanticReviewAttestation, error) {
	var attestation SemanticReviewAttestation
	if err := decodeStrict(contents, &attestation); err != nil {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: %w", err)
	}
	return canonicalSemanticAttestation(attestation, true)
}

func MarshalSemanticReviewAttestation(attestation SemanticReviewAttestation) ([]byte, error) {
	normalized, err := canonicalSemanticAttestation(attestation, true)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func CompareSemanticReviewAttestations(baseline, current SemanticReviewAttestation) (SemanticFindingDelta, error) {
	left, err := canonicalSemanticAttestation(baseline, true)
	if err != nil {
		return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: baseline: %w", err)
	}
	right, err := canonicalSemanticAttestation(current, true)
	if err != nil {
		return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: current: %w", err)
	}
	leftIndex := make(map[string]SemanticFinding, len(left.Findings))
	rightIndex := make(map[string]SemanticFinding, len(right.Findings))
	for _, finding := range left.Findings {
		leftIndex[finding.ID] = finding
	}
	for _, finding := range right.Findings {
		rightIndex[finding.ID] = finding
	}
	delta := SemanticFindingDelta{
		SchemaVersion:          SemanticReviewSchemaVersion,
		BaselineSourceIdentity: left.SourceIdentity,
		CurrentSourceIdentity:  right.SourceIdentity,
		BaselineEvidence:       cloneSemanticEvidenceBinding(left.Evidence),
		CurrentEvidence:        cloneSemanticEvidenceBinding(right.Evidence),
	}
	for _, finding := range right.Findings {
		if _, ok := leftIndex[finding.ID]; ok {
			delta.Existing = append(delta.Existing, finding)
		} else {
			delta.New = append(delta.New, finding)
		}
	}
	for _, finding := range left.Findings {
		if _, ok := rightIndex[finding.ID]; !ok {
			delta.Resolved = append(delta.Resolved, finding)
		}
	}
	return canonicalSemanticDelta(delta, false)
}

func DecodeSemanticFindingDelta(contents []byte) (SemanticFindingDelta, error) {
	var delta SemanticFindingDelta
	if err := decodeStrict(contents, &delta); err != nil {
		return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: %w", err)
	}
	return canonicalSemanticDelta(delta, true)
}

func MarshalSemanticFindingDelta(delta SemanticFindingDelta) ([]byte, error) {
	normalized, err := canonicalSemanticDelta(delta, true)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func canonicalSemanticRequest(request SemanticReviewRequest, verifyDigests bool) (SemanticReviewRequest, error) {
	request.Authority = strings.TrimSpace(request.Authority)
	request.RequestDigest = strings.TrimSpace(request.RequestDigest)
	if request.SchemaVersion != SemanticReviewSchemaVersion {
		return SemanticReviewRequest{}, fmt.Errorf("semantic review request: unsupported schemaVersion %d", request.SchemaVersion)
	}
	if request.Authority != AuthorityAdvisoryOnly || request.MutationAuthorized || request.MergeAuthorized {
		return SemanticReviewRequest{}, fmt.Errorf("semantic review request: authority must remain advisory_only with mutationAuthorized=false and mergeAuthorized=false")
	}
	evidence, err := canonicalSemanticEvidence(request.Evidence, verifyDigests)
	if err != nil {
		return SemanticReviewRequest{}, err
	}
	request.Evidence = evidence
	payloadBytes, err := json.Marshal(semanticRequestPayload{
		SchemaVersion:      request.SchemaVersion,
		Authority:          request.Authority,
		MutationAuthorized: request.MutationAuthorized,
		MergeAuthorized:    request.MergeAuthorized,
		Evidence:           request.Evidence,
	})
	if err != nil {
		return SemanticReviewRequest{}, err
	}
	requestDigest := digest(payloadBytes)
	if verifyDigests && request.RequestDigest != requestDigest {
		return SemanticReviewRequest{}, fmt.Errorf("semantic review request: requestDigest %q does not match canonical request %q", request.RequestDigest, requestDigest)
	}
	request.RequestDigest = requestDigest
	return request, nil
}

func canonicalSemanticEvidence(evidence SemanticEvidence, verifyDigests bool) (SemanticEvidence, error) {
	evidence.HeadSHA = strings.TrimSpace(evidence.HeadSHA)
	evidence.SourceSHA256 = strings.TrimSpace(evidence.SourceSHA256)
	evidence.SourceIdentity = strings.TrimSpace(evidence.SourceIdentity)
	if !validGitIdentity(evidence.HeadSHA) {
		return SemanticEvidence{}, fmt.Errorf("semantic review request: headSha must be a 40- or 64-character hexadecimal Git identity")
	}
	if len(evidence.Sources) == 0 {
		return SemanticEvidence{}, fmt.Errorf("semantic review request: at least one exact source is required")
	}
	seen := make(map[string]struct{}, len(evidence.Sources))
	for index := range evidence.Sources {
		source := &evidence.Sources[index]
		source.Path = cleanSemanticPath(source.Path)
		source.SHA256 = strings.TrimSpace(source.SHA256)
		if source.Path == "" {
			return SemanticEvidence{}, fmt.Errorf("semantic review request: source path is invalid")
		}
		if _, duplicate := seen[source.Path]; duplicate {
			return SemanticEvidence{}, fmt.Errorf("semantic review request: duplicate source path %q", source.Path)
		}
		seen[source.Path] = struct{}{}
		contentDigest := digest([]byte(source.Content))
		if verifyDigests && source.SHA256 != contentDigest {
			return SemanticEvidence{}, fmt.Errorf("semantic review request: source %s sha256 does not match content", source.Path)
		}
		source.SHA256 = contentDigest
	}
	sort.Slice(evidence.Sources, func(i, j int) bool { return evidence.Sources[i].Path < evidence.Sources[j].Path })
	if evidence.Change != nil {
		change := cloneSemanticChangeIdentity(evidence.Change)
		change.BaseSHA = strings.TrimSpace(change.BaseSHA)
		change.HeadSHA = strings.TrimSpace(change.HeadSHA)
		change.CandidateSHA256 = strings.TrimSpace(change.CandidateSHA256)
		change.EvidenceSHA256 = strings.TrimSpace(change.EvidenceSHA256)
		if !validGitIdentity(change.BaseSHA) || !validGitIdentity(change.HeadSHA) || change.HeadSHA != evidence.HeadSHA {
			return SemanticEvidence{}, fmt.Errorf("semantic review request: change identity must bind the same exact headSha and a valid baseSha")
		}
		if !validSHA256Digest(change.CandidateSHA256) || !validSHA256Digest(change.EvidenceSHA256) {
			return SemanticEvidence{}, fmt.Errorf("semantic review request: change candidate/evidence digests must be valid SHA-256 values")
		}
		evidence.Change = change
	}
	sourceBytes, err := json.Marshal(evidence.Sources)
	if err != nil {
		return SemanticEvidence{}, err
	}
	sourceDigest := digest(sourceBytes)
	if verifyDigests && evidence.SourceSHA256 != sourceDigest {
		return SemanticEvidence{}, fmt.Errorf("semantic review request: sourceSha256 does not match exact source evidence")
	}
	evidence.SourceSHA256 = sourceDigest
	identityPayload := struct {
		Version      int                     `json:"version"`
		HeadSHA      string                  `json:"headSha"`
		SourceSHA256 string                  `json:"sourceSha256"`
		Change       *SemanticChangeIdentity `json:"change,omitempty"`
	}{Version: SemanticReviewSchemaVersion, HeadSHA: evidence.HeadSHA, SourceSHA256: sourceDigest, Change: evidence.Change}
	identityBytes, err := json.Marshal(identityPayload)
	if err != nil {
		return SemanticEvidence{}, err
	}
	identity := digest(identityBytes)
	if verifyDigests && evidence.SourceIdentity != identity {
		return SemanticEvidence{}, fmt.Errorf("semantic review request: sourceIdentity does not match exact source/change evidence")
	}
	evidence.SourceIdentity = identity
	return evidence, nil
}

func canonicalSemanticResponse(response SemanticReviewResponse) (SemanticReviewResponse, error) {
	response.Authority = strings.TrimSpace(response.Authority)
	response.RequestDigest = strings.TrimSpace(response.RequestDigest)
	if response.SchemaVersion != SemanticReviewSchemaVersion {
		return SemanticReviewResponse{}, fmt.Errorf("semantic review response: unsupported schemaVersion %d", response.SchemaVersion)
	}
	if response.Authority != AuthorityAdvisoryOnly {
		return SemanticReviewResponse{}, fmt.Errorf("semantic review response: authority must be %q", AuthorityAdvisoryOnly)
	}
	if !validSHA256Digest(response.RequestDigest) {
		return SemanticReviewResponse{}, fmt.Errorf("semantic review response: requestDigest must be a SHA-256 value")
	}
	seen := make(map[string]struct{}, len(response.Findings))
	for index := range response.Findings {
		finding, err := canonicalSemanticFinding(response.Findings[index])
		if err != nil {
			return SemanticReviewResponse{}, fmt.Errorf("semantic review response: finding[%d]: %w", index, err)
		}
		if _, duplicate := seen[finding.ID]; duplicate {
			return SemanticReviewResponse{}, fmt.Errorf("semantic review response: duplicate finding id %q", finding.ID)
		}
		seen[finding.ID] = struct{}{}
		response.Findings[index] = finding
	}
	sort.Slice(response.Findings, func(i, j int) bool { return response.Findings[i].ID < response.Findings[j].ID })
	if response.Findings == nil {
		response.Findings = []SemanticFinding{}
	}
	return response, nil
}

func canonicalSemanticFinding(finding SemanticFinding) (SemanticFinding, error) {
	finding.ID = strings.TrimSpace(finding.ID)
	finding.Path = cleanSemanticPath(finding.Path)
	finding.SymbolOrScope = strings.TrimSpace(finding.SymbolOrScope)
	finding.Category = strings.TrimSpace(finding.Category)
	finding.Severity = strings.TrimSpace(finding.Severity)
	finding.Reason = strings.TrimSpace(finding.Reason)
	finding.RecommendedAction.Kind = strings.TrimSpace(finding.RecommendedAction.Kind)
	finding.RecommendedAction.Detail = strings.TrimSpace(finding.RecommendedAction.Detail)
	finding.SourceIdentity = strings.TrimSpace(finding.SourceIdentity)
	if finding.Path == "" || finding.SymbolOrScope == "" || finding.Reason == "" || finding.RecommendedAction.Detail == "" {
		return SemanticFinding{}, fmt.Errorf("path, symbolOrScope, reason, and recommendedAction.detail are required")
	}
	if !supportedSemanticCategory(finding.Category) {
		return SemanticFinding{}, fmt.Errorf("category %q is unsupported", finding.Category)
	}
	if !supportedSemanticSeverity(finding.Severity) {
		return SemanticFinding{}, fmt.Errorf("severity %q is unsupported", finding.Severity)
	}
	if !supportedSemanticAction(finding.RecommendedAction.Kind) {
		return SemanticFinding{}, fmt.Errorf("recommendedAction.kind %q is unsupported", finding.RecommendedAction.Kind)
	}
	if forbiddenAuthorityLanguage(finding.RecommendedAction.Detail) {
		return SemanticFinding{}, fmt.Errorf("recommendedAction.detail attempts to grant mutation or merge authority")
	}
	if !validSHA256Digest(finding.SourceIdentity) {
		return SemanticFinding{}, fmt.Errorf("sourceIdentity must be a SHA-256 value")
	}
	expectedID := semanticFindingID(finding.Path, finding.SymbolOrScope, finding.Category)
	if finding.ID != "" && finding.ID != expectedID {
		return SemanticFinding{}, fmt.Errorf("id %q does not match stable semantic identity %q", finding.ID, expectedID)
	}
	finding.ID = expectedID
	return finding, nil
}

func canonicalSemanticAttestation(attestation SemanticReviewAttestation, verifyDigest bool) (SemanticReviewAttestation, error) {
	attestation.Authority = strings.TrimSpace(attestation.Authority)
	attestation.RequestDigest = strings.TrimSpace(attestation.RequestDigest)
	attestation.ResponseDigest = strings.TrimSpace(attestation.ResponseDigest)
	attestation.SourceIdentity = strings.TrimSpace(attestation.SourceIdentity)
	attestation.Result = strings.TrimSpace(attestation.Result)
	attestation.AttestationDigest = strings.TrimSpace(attestation.AttestationDigest)
	if attestation.SchemaVersion != SemanticReviewSchemaVersion || attestation.Authority != AuthorityAdvisoryOnly || attestation.Result != SemanticReviewResultValid {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: invalid schema, authority, or result")
	}
	if !validSHA256Digest(attestation.RequestDigest) || !validSHA256Digest(attestation.ResponseDigest) || !validSHA256Digest(attestation.SourceIdentity) {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: request/response/source identities must be SHA-256 values")
	}
	if attestation.Evidence != nil {
		evidence, err := canonicalSemanticEvidenceBinding(*attestation.Evidence)
		if err != nil {
			return SemanticReviewAttestation{}, err
		}
		if evidence.SourceIdentity != attestation.SourceIdentity {
			return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: evidence sourceIdentity mismatch")
		}
		attestation.Evidence = &evidence
	}
	for index := range attestation.Findings {
		finding, err := canonicalSemanticFinding(attestation.Findings[index])
		if err != nil {
			return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: finding[%d]: %w", index, err)
		}
		if finding.SourceIdentity != attestation.SourceIdentity {
			return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: finding %s sourceIdentity mismatch", finding.ID)
		}
		attestation.Findings[index] = finding
	}
	sort.Slice(attestation.Findings, func(i, j int) bool { return attestation.Findings[i].ID < attestation.Findings[j].ID })
	if attestation.Findings == nil {
		attestation.Findings = []SemanticFinding{}
	}
	payloadBytes, err := json.Marshal(semanticAttestationPayload{
		SchemaVersion:  attestation.SchemaVersion,
		Authority:      attestation.Authority,
		RequestDigest:  attestation.RequestDigest,
		ResponseDigest: attestation.ResponseDigest,
		SourceIdentity: attestation.SourceIdentity,
		Evidence:       attestation.Evidence,
		Findings:       attestation.Findings,
		Result:         attestation.Result,
	})
	if err != nil {
		return SemanticReviewAttestation{}, err
	}
	attestationDigest := digest(payloadBytes)
	if verifyDigest && attestation.AttestationDigest != attestationDigest {
		return SemanticReviewAttestation{}, fmt.Errorf("semantic review attestation: attestationDigest mismatch")
	}
	attestation.AttestationDigest = attestationDigest
	return attestation, nil
}

func canonicalSemanticDelta(delta SemanticFindingDelta, verifyDigest bool) (SemanticFindingDelta, error) {
	delta.BaselineSourceIdentity = strings.TrimSpace(delta.BaselineSourceIdentity)
	delta.CurrentSourceIdentity = strings.TrimSpace(delta.CurrentSourceIdentity)
	delta.DeltaDigest = strings.TrimSpace(delta.DeltaDigest)
	if delta.SchemaVersion != SemanticReviewSchemaVersion || !validSHA256Digest(delta.BaselineSourceIdentity) || !validSHA256Digest(delta.CurrentSourceIdentity) {
		return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: invalid schema or source identity")
	}
	if delta.BaselineEvidence != nil {
		evidence, err := canonicalSemanticEvidenceBinding(*delta.BaselineEvidence)
		if err != nil {
			return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: baseline evidence: %w", err)
		}
		if evidence.SourceIdentity != delta.BaselineSourceIdentity {
			return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: baseline evidence sourceIdentity mismatch")
		}
		delta.BaselineEvidence = &evidence
	}
	if delta.CurrentEvidence != nil {
		evidence, err := canonicalSemanticEvidenceBinding(*delta.CurrentEvidence)
		if err != nil {
			return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: current evidence: %w", err)
		}
		if evidence.SourceIdentity != delta.CurrentSourceIdentity {
			return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: current evidence sourceIdentity mismatch")
		}
		delta.CurrentEvidence = &evidence
	}
	if delta.BaselineEvidence != nil && delta.CurrentEvidence != nil && delta.CurrentEvidence.Change != nil {
		if delta.CurrentEvidence.Change.BaseSHA != delta.BaselineEvidence.HeadSHA || delta.CurrentEvidence.Change.HeadSHA != delta.CurrentEvidence.HeadSHA {
			return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: current change identity does not bind the baseline/current review heads")
		}
	}
	for _, group := range []*[]SemanticFinding{&delta.Existing, &delta.New, &delta.Resolved} {
		for index := range *group {
			finding, err := canonicalSemanticFinding((*group)[index])
			if err != nil {
				return SemanticFindingDelta{}, err
			}
			(*group)[index] = finding
		}
		sort.Slice(*group, func(i, j int) bool { return (*group)[i].ID < (*group)[j].ID })
		if *group == nil {
			*group = []SemanticFinding{}
		}
	}
	payloadBytes, err := json.Marshal(semanticDeltaPayload{
		SchemaVersion:          delta.SchemaVersion,
		BaselineSourceIdentity: delta.BaselineSourceIdentity,
		CurrentSourceIdentity:  delta.CurrentSourceIdentity,
		BaselineEvidence:       delta.BaselineEvidence,
		CurrentEvidence:        delta.CurrentEvidence,
		Existing:               delta.Existing,
		New:                    delta.New,
		Resolved:               delta.Resolved,
	})
	if err != nil {
		return SemanticFindingDelta{}, err
	}
	deltaDigest := digest(payloadBytes)
	if verifyDigest && delta.DeltaDigest != deltaDigest {
		return SemanticFindingDelta{}, fmt.Errorf("semantic review delta: deltaDigest mismatch")
	}
	delta.DeltaDigest = deltaDigest
	return delta, nil
}

func semanticEvidenceBinding(evidence SemanticEvidence) SemanticEvidenceBinding {
	binding := SemanticEvidenceBinding{
		HeadSHA:        evidence.HeadSHA,
		Change:         cloneSemanticChangeIdentity(evidence.Change),
		SourceSHA256:   evidence.SourceSHA256,
		SourceIdentity: evidence.SourceIdentity,
		Sources:        make([]SemanticSourceBinding, 0, len(evidence.Sources)),
	}
	for _, source := range evidence.Sources {
		binding.Sources = append(binding.Sources, SemanticSourceBinding{Path: source.Path, SHA256: source.SHA256})
	}
	return binding
}

func canonicalSemanticEvidenceBinding(binding SemanticEvidenceBinding) (SemanticEvidenceBinding, error) {
	binding.HeadSHA = strings.TrimSpace(binding.HeadSHA)
	binding.SourceSHA256 = strings.TrimSpace(binding.SourceSHA256)
	binding.SourceIdentity = strings.TrimSpace(binding.SourceIdentity)
	if !validGitIdentity(binding.HeadSHA) || !validSHA256Digest(binding.SourceSHA256) || !validSHA256Digest(binding.SourceIdentity) {
		return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: invalid head/source identity")
	}
	if len(binding.Sources) == 0 {
		return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: at least one exact source identity is required")
	}
	seen := make(map[string]struct{}, len(binding.Sources))
	for index := range binding.Sources {
		source := &binding.Sources[index]
		source.Path = cleanSemanticPath(source.Path)
		source.SHA256 = strings.TrimSpace(source.SHA256)
		if source.Path == "" || !validSHA256Digest(source.SHA256) {
			return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: source path/hash is invalid")
		}
		if _, duplicate := seen[source.Path]; duplicate {
			return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: duplicate source path %q", source.Path)
		}
		seen[source.Path] = struct{}{}
	}
	sort.Slice(binding.Sources, func(i, j int) bool { return binding.Sources[i].Path < binding.Sources[j].Path })
	if binding.Change != nil {
		change := cloneSemanticChangeIdentity(binding.Change)
		change.BaseSHA = strings.TrimSpace(change.BaseSHA)
		change.HeadSHA = strings.TrimSpace(change.HeadSHA)
		change.CandidateSHA256 = strings.TrimSpace(change.CandidateSHA256)
		change.EvidenceSHA256 = strings.TrimSpace(change.EvidenceSHA256)
		if !validGitIdentity(change.BaseSHA) || !validGitIdentity(change.HeadSHA) || change.HeadSHA != binding.HeadSHA {
			return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: change identity does not bind the evidence head")
		}
		if !validSHA256Digest(change.CandidateSHA256) || !validSHA256Digest(change.EvidenceSHA256) {
			return SemanticEvidenceBinding{}, fmt.Errorf("semantic review evidence binding: change candidate/evidence digest is invalid")
		}
		binding.Change = change
	}
	return binding, nil
}

func cloneSemanticEvidenceBinding(value *SemanticEvidenceBinding) *SemanticEvidenceBinding {
	if value == nil {
		return nil
	}
	copyValue := *value
	copyValue.Change = cloneSemanticChangeIdentity(value.Change)
	copyValue.Sources = append([]SemanticSourceBinding(nil), value.Sources...)
	return &copyValue
}

func semanticFindingID(sourcePath, symbolOrScope, category string) string {
	identity := strings.Join([]string{cleanSemanticPath(sourcePath), strings.TrimSpace(symbolOrScope), strings.TrimSpace(category)}, "\x00")
	return "semantic-" + digest([]byte(identity))[:20]
}

func cleanSemanticPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") {
		return ""
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, ":") {
		return ""
	}
	return cleaned
}

func supportedSemanticCategory(value string) bool {
	switch value {
	case SemanticCategoryColocation, SemanticCategoryAmbiguousResponsibility, SemanticCategoryUnjustifiedAbstraction, SemanticCategoryNamingFitness, SemanticCategoryDuplicatedOwnership:
		return true
	default:
		return false
	}
}

func supportedSemanticSeverity(value string) bool {
	switch value {
	case SemanticSeverityLow, SemanticSeverityMedium, SemanticSeverityHigh:
		return true
	default:
		return false
	}
}

func supportedSemanticAction(value string) bool {
	switch value {
	case SemanticActionInvestigate, SemanticActionDiscussDesign, SemanticActionConsiderRefactor, SemanticActionConsiderRename, SemanticActionDocumentDecision:
		return true
	default:
		return false
	}
}

func forbiddenAuthorityLanguage(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, forbidden := range []string{
		"safe_to_merge", "safe to merge", "apply_patch", "apply patch", "auto-merge", "automerge",
		"merge approved", "approve merge", "mutation authorized", "write changes automatically", "execute patch",
	} {
		if strings.Contains(value, forbidden) {
			return true
		}
	}
	return false
}

func validSHA256Digest(value string) bool {
	return len(value) == 64 && validHex(value)
}

func validGitIdentity(value string) bool {
	return (len(value) == 40 || len(value) == 64) && validHex(value)
}

func validHex(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func cloneSemanticChangeIdentity(value *SemanticChangeIdentity) *SemanticChangeIdentity {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
