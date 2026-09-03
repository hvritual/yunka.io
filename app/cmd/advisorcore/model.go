package advisorcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"yunka.io/app/cmd/auditcore"
)

const (
	SchemaVersion         = 1
	AuthorityAdvisoryOnly = "advisory_only"
	ResultValid           = "valid"
)

type RecommendationKind string

const (
	KindRemediationRecommendation RecommendationKind = "remediation_recommendation"
	KindInvestigationHypothesis   RecommendationKind = "investigation_hypothesis"
	KindDesignQuestion            RecommendationKind = "design_question"
)

type Policy struct {
	AllowedRecommendationKinds      []RecommendationKind `json:"allowedRecommendationKinds"`
	MutationAuthorized              bool                 `json:"mutationAuthorized"`
	AuthorityExpansionRequiresHuman bool                 `json:"authorityExpansionRequiresHuman"`
}

type Request struct {
	SchemaVersion int              `json:"schemaVersion"`
	Authority     string           `json:"authority"`
	Policy        Policy           `json:"policy"`
	AuditDigest   string           `json:"auditDigest"`
	RequestDigest string           `json:"requestDigest"`
	Audit         auditcore.Report `json:"audit"`
}

type Recommendation struct {
	ID         string             `json:"id"`
	Kind       RecommendationKind `json:"kind"`
	Title      string             `json:"title"`
	Rationale  string             `json:"rationale"`
	FindingIDs []string           `json:"findingIds"`
}

type Response struct {
	SchemaVersion   int              `json:"schemaVersion"`
	RequestDigest   string           `json:"requestDigest"`
	Recommendations []Recommendation `json:"recommendations"`
}

type EvidenceBinding struct {
	RecommendationID string   `json:"recommendationId"`
	FindingIDs       []string `json:"findingIds"`
}

type Attestation struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Authority      string            `json:"authority"`
	RequestDigest  string            `json:"requestDigest"`
	ResponseDigest string            `json:"responseDigest"`
	Bindings       []EvidenceBinding `json:"bindings"`
	Result         string            `json:"result"`
}

type requestPayload struct {
	SchemaVersion int              `json:"schemaVersion"`
	Authority     string           `json:"authority"`
	Policy        Policy           `json:"policy"`
	AuditDigest   string           `json:"auditDigest"`
	Audit         auditcore.Report `json:"audit"`
}

func NewRequest(report auditcore.Report) (Request, error) {
	request := Request{
		SchemaVersion: SchemaVersion,
		Authority:     AuthorityAdvisoryOnly,
		Policy:        canonicalPolicy(),
		Audit:         report,
	}
	return canonicalRequest(request, false)
}

func DecodeRequest(contents []byte) (Request, error) {
	var request Request
	if err := decodeStrict(contents, &request); err != nil {
		return Request{}, fmt.Errorf("advisor request: %w", err)
	}
	return canonicalRequest(request, true)
}

func MarshalRequest(request Request) ([]byte, error) {
	normalized, err := canonicalRequest(request, true)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func DecodeResponse(contents []byte) (Response, error) {
	var response Response
	if err := decodeStrict(contents, &response); err != nil {
		return Response{}, fmt.Errorf("advisor response: %w", err)
	}
	return canonicalResponse(response)
}

func MarshalResponse(response Response) ([]byte, error) {
	normalized, err := canonicalResponse(response)
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func ValidateResponse(request Request, response Response) (Attestation, error) {
	normalizedRequest, err := canonicalRequest(request, true)
	if err != nil {
		return Attestation{}, err
	}
	normalizedResponse, err := canonicalResponse(response)
	if err != nil {
		return Attestation{}, err
	}
	if normalizedResponse.RequestDigest != normalizedRequest.RequestDigest {
		return Attestation{}, fmt.Errorf("advisor response: requestDigest %q does not match request %q", normalizedResponse.RequestDigest, normalizedRequest.RequestDigest)
	}
	if len(normalizedRequest.Audit.Findings) == 0 && len(normalizedResponse.Recommendations) != 0 {
		return Attestation{}, fmt.Errorf("advisor response: recommendations require deterministic Audit findings")
	}

	findings := make(map[string]auditcore.Finding, len(normalizedRequest.Audit.Findings))
	for _, finding := range normalizedRequest.Audit.Findings {
		findings[finding.ID] = finding
	}
	bindings := make([]EvidenceBinding, 0, len(normalizedResponse.Recommendations))
	for _, recommendation := range normalizedResponse.Recommendations {
		for _, findingID := range recommendation.FindingIDs {
			finding, ok := findings[findingID]
			if !ok {
				return Attestation{}, fmt.Errorf("advisor response: recommendation %s references unknown current finding %q", recommendation.ID, findingID)
			}
			if recommendation.Kind == KindRemediationRecommendation && finding.Class != auditcore.FindingProvenViolation {
				return Attestation{}, fmt.Errorf("advisor response: remediation recommendation %s may reference only proven_violation findings; %s is %s", recommendation.ID, findingID, finding.Class)
			}
		}
		bindings = append(bindings, EvidenceBinding{RecommendationID: recommendation.ID, FindingIDs: append([]string(nil), recommendation.FindingIDs...)})
	}

	responseBytes, err := json.Marshal(normalizedResponse)
	if err != nil {
		return Attestation{}, err
	}
	return Attestation{
		SchemaVersion:  SchemaVersion,
		Authority:      AuthorityAdvisoryOnly,
		RequestDigest:  normalizedRequest.RequestDigest,
		ResponseDigest: digest(responseBytes),
		Bindings:       bindings,
		Result:         ResultValid,
	}, nil
}

func MarshalAttestation(attestation Attestation) ([]byte, error) {
	if attestation.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("advisor attestation: unsupported schemaVersion %d", attestation.SchemaVersion)
	}
	if strings.TrimSpace(attestation.Authority) != AuthorityAdvisoryOnly {
		return nil, fmt.Errorf("advisor attestation: authority must be %q", AuthorityAdvisoryOnly)
	}
	attestation.RequestDigest = strings.TrimSpace(attestation.RequestDigest)
	attestation.ResponseDigest = strings.TrimSpace(attestation.ResponseDigest)
	attestation.Result = strings.TrimSpace(attestation.Result)
	if attestation.RequestDigest == "" || attestation.ResponseDigest == "" || attestation.Result != ResultValid {
		return nil, fmt.Errorf("advisor attestation: requestDigest, responseDigest, and valid result are required")
	}
	for index := range attestation.Bindings {
		binding := &attestation.Bindings[index]
		binding.RecommendationID = strings.TrimSpace(binding.RecommendationID)
		binding.FindingIDs = append([]string(nil), binding.FindingIDs...)
		sort.Strings(binding.FindingIDs)
	}
	sort.Slice(attestation.Bindings, func(i, j int) bool {
		return attestation.Bindings[i].RecommendationID < attestation.Bindings[j].RecommendationID
	})
	if attestation.Bindings == nil {
		attestation.Bindings = []EvidenceBinding{}
	}
	contents, err := json.MarshalIndent(attestation, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func canonicalRequest(request Request, verifyDigests bool) (Request, error) {
	request.Authority = strings.TrimSpace(request.Authority)
	request.AuditDigest = strings.TrimSpace(request.AuditDigest)
	request.RequestDigest = strings.TrimSpace(request.RequestDigest)
	auditcore.Normalize(&request.Audit)
	if request.SchemaVersion != SchemaVersion {
		return Request{}, fmt.Errorf("advisor request: unsupported schemaVersion %d", request.SchemaVersion)
	}
	if request.Authority != AuthorityAdvisoryOnly {
		return Request{}, fmt.Errorf("advisor request: authority must be %q", AuthorityAdvisoryOnly)
	}
	if err := validatePolicy(request.Policy); err != nil {
		return Request{}, err
	}
	request.Policy = canonicalPolicy()
	if err := auditcore.Validate(request.Audit); err != nil {
		return Request{}, fmt.Errorf("advisor request: invalid Audit evidence: %w", err)
	}
	auditBytes, err := auditcore.Marshal(request.Audit)
	if err != nil {
		return Request{}, fmt.Errorf("advisor request: canonical Audit evidence: %w", err)
	}
	auditDigest := digest(auditBytes)
	if verifyDigests && request.AuditDigest != auditDigest {
		return Request{}, fmt.Errorf("advisor request: auditDigest %q does not match canonical Audit evidence %q", request.AuditDigest, auditDigest)
	}
	request.AuditDigest = auditDigest
	payloadBytes, err := json.Marshal(requestPayload{
		SchemaVersion: request.SchemaVersion,
		Authority:     request.Authority,
		Policy:        request.Policy,
		AuditDigest:   request.AuditDigest,
		Audit:         request.Audit,
	})
	if err != nil {
		return Request{}, err
	}
	requestDigest := digest(payloadBytes)
	if verifyDigests && request.RequestDigest != requestDigest {
		return Request{}, fmt.Errorf("advisor request: requestDigest %q does not match canonical request %q", request.RequestDigest, requestDigest)
	}
	request.RequestDigest = requestDigest
	return request, nil
}

func canonicalResponse(response Response) (Response, error) {
	response.RequestDigest = strings.TrimSpace(response.RequestDigest)
	if response.SchemaVersion != SchemaVersion {
		return Response{}, fmt.Errorf("advisor response: unsupported schemaVersion %d", response.SchemaVersion)
	}
	if response.RequestDigest == "" {
		return Response{}, fmt.Errorf("advisor response: requestDigest is required")
	}
	seenRecommendations := make(map[string]struct{}, len(response.Recommendations))
	for index := range response.Recommendations {
		recommendation := &response.Recommendations[index]
		recommendation.ID = strings.TrimSpace(recommendation.ID)
		recommendation.Kind = RecommendationKind(strings.TrimSpace(string(recommendation.Kind)))
		recommendation.Title = strings.TrimSpace(recommendation.Title)
		recommendation.Rationale = strings.TrimSpace(recommendation.Rationale)
		if recommendation.ID == "" || recommendation.Title == "" || recommendation.Rationale == "" {
			return Response{}, fmt.Errorf("advisor response: recommendation id, title, and rationale are required")
		}
		if _, duplicate := seenRecommendations[recommendation.ID]; duplicate {
			return Response{}, fmt.Errorf("advisor response: duplicate recommendation id %q", recommendation.ID)
		}
		seenRecommendations[recommendation.ID] = struct{}{}
		if !supportedRecommendationKind(recommendation.Kind) {
			return Response{}, fmt.Errorf("advisor response: recommendation %s kind %q is unsupported", recommendation.ID, recommendation.Kind)
		}
		if len(recommendation.FindingIDs) == 0 {
			return Response{}, fmt.Errorf("advisor response: recommendation %s must reference at least one Audit finding", recommendation.ID)
		}
		seenFindings := make(map[string]struct{}, len(recommendation.FindingIDs))
		for findingIndex := range recommendation.FindingIDs {
			findingID := strings.TrimSpace(recommendation.FindingIDs[findingIndex])
			if findingID == "" {
				return Response{}, fmt.Errorf("advisor response: recommendation %s contains an empty finding id", recommendation.ID)
			}
			if _, duplicate := seenFindings[findingID]; duplicate {
				return Response{}, fmt.Errorf("advisor response: recommendation %s repeats finding id %q", recommendation.ID, findingID)
			}
			seenFindings[findingID] = struct{}{}
			recommendation.FindingIDs[findingIndex] = findingID
		}
		sort.Strings(recommendation.FindingIDs)
	}
	sort.Slice(response.Recommendations, func(i, j int) bool { return response.Recommendations[i].ID < response.Recommendations[j].ID })
	if response.Recommendations == nil {
		response.Recommendations = []Recommendation{}
	}
	return response, nil
}

func canonicalPolicy() Policy {
	return Policy{
		AllowedRecommendationKinds: []RecommendationKind{
			KindDesignQuestion,
			KindInvestigationHypothesis,
			KindRemediationRecommendation,
		},
		MutationAuthorized:              false,
		AuthorityExpansionRequiresHuman: true,
	}
}

func validatePolicy(policy Policy) error {
	expected := canonicalPolicy()
	got := append([]RecommendationKind(nil), policy.AllowedRecommendationKinds...)
	for index := range got {
		got[index] = RecommendationKind(strings.TrimSpace(string(got[index])))
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(got) != len(expected.AllowedRecommendationKinds) {
		return fmt.Errorf("advisor request: policy allowed recommendation kinds were modified")
	}
	for index := range got {
		if got[index] != expected.AllowedRecommendationKinds[index] {
			return fmt.Errorf("advisor request: policy allowed recommendation kinds were modified")
		}
	}
	if policy.MutationAuthorized || !policy.AuthorityExpansionRequiresHuman {
		return fmt.Errorf("advisor request: policy cannot authorize mutation or authority expansion")
	}
	return nil
}

func supportedRecommendationKind(kind RecommendationKind) bool {
	switch kind {
	case KindRemediationRecommendation, KindInvestigationHypothesis, KindDesignQuestion:
		return true
	default:
		return false
	}
}

func decodeStrict(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value is not allowed")
		}
		return err
	}
	return nil
}

func digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
