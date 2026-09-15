package change

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	ReviewPacketSchemaVersion = 1
	DefaultReviewPacketPath   = ".git/yunka/review-packet.json"
	ReviewDeltaNone           = "NONE"
	ReviewDeltaChanged        = "CHANGED"
)

// ReviewNarrative is declared review context. It explains intent to a human but
// never authorizes mutation, conformance, or merge.
type ReviewNarrative struct {
	Problem            string   `json:"problem"`
	CurrentConcepts    []string `json:"currentConcepts"`
	DesiredOwnership   []string `json:"desiredOwnership"`
	Why                string   `json:"why"`
	What               string   `json:"what"`
	Boundary           string   `json:"boundary"`
	AffectedInvariants []string `json:"affectedInvariants"`
	Risks              []string `json:"risks"`
	UnresolvedFindings []string `json:"unresolvedFindings"`
}

type ReviewFact struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject,omitempty"`
	Path    string `json:"path,omitempty"`
	Detail  string `json:"detail"`
}

type ReviewDelta struct {
	State string       `json:"state"`
	Facts []ReviewFact `json:"facts"`
}

type ReviewVerification struct {
	Conformant bool         `json:"conformant"`
	Gates      []GateResult `json:"gates"`
}

type ReviewProjection struct {
	Why      string   `json:"why"`
	What     string   `json:"what"`
	Boundary string   `json:"boundary"`
	Proof    []string `json:"proof"`
}

type ReviewEvidenceIdentity struct {
	BaseSHA            string `json:"baseSha"`
	HeadSHA            string `json:"headSha"`
	OperationID        string `json:"operationId"`
	ContractSHA256     string `json:"contractSha256"`
	AttestationSHA256  string `json:"attestationSha256"`
	NarrativeSHA256    string `json:"narrativeSha256"`
	ChangedPathsSHA256 string `json:"changedPathsSha256"`
	CandidateSHA256    string `json:"candidateSha256"`
	EvidenceSHA256     string `json:"evidenceSha256"`
}

// ReviewPacket is a deterministic presentation artifact. Change Contract and
// Change Attestation remain authoritative evidence; the packet only projects
// their exact-candidate facts before raw-diff review.
type ReviewPacket struct {
	SchemaVersion       int                    `json:"schemaVersion"`
	Narrative           ReviewNarrative        `json:"narrative"`
	BehaviorChange      ReviewDelta            `json:"behaviorChange"`
	PublicAPIChange     ReviewDelta            `json:"publicApiChange"`
	PersistenceChange   ReviewDelta            `json:"persistenceChange"`
	GeneratedCodeChange ReviewDelta            `json:"generatedCodeChange"`
	Verification        ReviewVerification     `json:"verification"`
	AffectedInvariants  []string               `json:"affectedInvariants"`
	Risks               []string               `json:"risks"`
	UnresolvedFindings  []string               `json:"unresolvedFindings"`
	Projection          ReviewProjection       `json:"whyWhatBoundaryProof"`
	Evidence            ReviewEvidenceIdentity `json:"evidence"`
}

func normalizeReviewNarrative(value *ReviewNarrative) error {
	if value == nil {
		return fmt.Errorf("change review: narrative is required")
	}
	value.Problem = strings.TrimSpace(value.Problem)
	value.Why = strings.TrimSpace(value.Why)
	value.What = strings.TrimSpace(value.What)
	value.Boundary = strings.TrimSpace(value.Boundary)
	value.CurrentConcepts = uniqueSortedReviewText(value.CurrentConcepts)
	value.DesiredOwnership = uniqueSortedReviewText(value.DesiredOwnership)
	value.AffectedInvariants = uniqueSortedReviewText(value.AffectedInvariants)
	value.Risks = uniqueSortedReviewText(value.Risks)
	value.UnresolvedFindings = uniqueSortedReviewText(value.UnresolvedFindings)
	if value.Problem == "" || value.Why == "" || value.What == "" || value.Boundary == "" {
		return fmt.Errorf("change review: problem, why, what and boundary are required")
	}
	if len(value.CurrentConcepts) == 0 || len(value.DesiredOwnership) == 0 {
		return fmt.Errorf("change review: current concepts and desired ownership are required")
	}
	return nil
}

func normalizeReviewPacket(packet *ReviewPacket) {
	if packet == nil {
		return
	}
	_ = normalizeReviewNarrative(&packet.Narrative)
	normalizeReviewDelta(&packet.BehaviorChange)
	normalizeReviewDelta(&packet.PublicAPIChange)
	normalizeReviewDelta(&packet.PersistenceChange)
	normalizeReviewDelta(&packet.GeneratedCodeChange)
	packet.AffectedInvariants = uniqueSortedReviewText(packet.AffectedInvariants)
	packet.Risks = uniqueSortedReviewText(packet.Risks)
	packet.UnresolvedFindings = uniqueSortedReviewText(packet.UnresolvedFindings)
	packet.Projection.Why = strings.TrimSpace(packet.Projection.Why)
	packet.Projection.What = strings.TrimSpace(packet.Projection.What)
	packet.Projection.Boundary = strings.TrimSpace(packet.Projection.Boundary)
	packet.Projection.Proof = uniqueSortedReviewText(packet.Projection.Proof)
	packet.Evidence.BaseSHA = strings.TrimSpace(packet.Evidence.BaseSHA)
	packet.Evidence.HeadSHA = strings.TrimSpace(packet.Evidence.HeadSHA)
	packet.Evidence.OperationID = strings.TrimSpace(packet.Evidence.OperationID)
	packet.Evidence.ContractSHA256 = strings.TrimSpace(packet.Evidence.ContractSHA256)
	packet.Evidence.AttestationSHA256 = strings.TrimSpace(packet.Evidence.AttestationSHA256)
	packet.Evidence.NarrativeSHA256 = strings.TrimSpace(packet.Evidence.NarrativeSHA256)
	packet.Evidence.ChangedPathsSHA256 = strings.TrimSpace(packet.Evidence.ChangedPathsSHA256)
	packet.Evidence.CandidateSHA256 = strings.TrimSpace(packet.Evidence.CandidateSHA256)
	packet.Evidence.EvidenceSHA256 = strings.TrimSpace(packet.Evidence.EvidenceSHA256)
}

func normalizeReviewDelta(delta *ReviewDelta) {
	if delta == nil {
		return
	}
	for index := range delta.Facts {
		delta.Facts[index].Kind = strings.TrimSpace(delta.Facts[index].Kind)
		delta.Facts[index].Subject = strings.TrimSpace(delta.Facts[index].Subject)
		delta.Facts[index].Path = cleanProjectPath(delta.Facts[index].Path)
		delta.Facts[index].Detail = strings.TrimSpace(delta.Facts[index].Detail)
	}
	sort.Slice(delta.Facts, func(i, j int) bool {
		left, right := delta.Facts[i], delta.Facts[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Detail < right.Detail
	})
	if delta.Facts == nil {
		delta.Facts = []ReviewFact{}
	}
	if len(delta.Facts) == 0 {
		delta.State = ReviewDeltaNone
	} else {
		delta.State = ReviewDeltaChanged
	}
}

func validateReviewPacket(packet ReviewPacket) error {
	if packet.SchemaVersion != ReviewPacketSchemaVersion {
		return fmt.Errorf("change review: unsupported packet schemaVersion %d", packet.SchemaVersion)
	}
	narrative := packet.Narrative
	if err := normalizeReviewNarrative(&narrative); err != nil {
		return err
	}
	for name, delta := range map[string]ReviewDelta{
		"behavior": packet.BehaviorChange, "publicApi": packet.PublicAPIChange,
		"persistence": packet.PersistenceChange, "generated": packet.GeneratedCodeChange,
	} {
		if delta.State != ReviewDeltaNone && delta.State != ReviewDeltaChanged {
			return fmt.Errorf("change review: %s delta state %q is unsupported", name, delta.State)
		}
		if (delta.State == ReviewDeltaNone) != (len(delta.Facts) == 0) {
			return fmt.Errorf("change review: %s delta state/facts are inconsistent", name)
		}
		for _, fact := range delta.Facts {
			if fact.Kind == "" || fact.Detail == "" {
				return fmt.Errorf("change review: %s delta facts require kind and detail", name)
			}
		}
	}
	if packet.Evidence.BaseSHA == "" || packet.Evidence.HeadSHA == "" || packet.Evidence.OperationID == "" {
		return fmt.Errorf("change review: evidence identity is incomplete")
	}
	for _, digest := range []string{packet.Evidence.ContractSHA256, packet.Evidence.AttestationSHA256, packet.Evidence.NarrativeSHA256, packet.Evidence.ChangedPathsSHA256, packet.Evidence.CandidateSHA256, packet.Evidence.EvidenceSHA256} {
		if !validSHA256(digest) {
			return fmt.Errorf("change review: evidence contains an invalid SHA-256 digest")
		}
	}
	narrativeDigest, err := digestJSON(narrative)
	if err != nil || narrativeDigest != packet.Evidence.NarrativeSHA256 {
		return fmt.Errorf("change review: narrative digest mismatch")
	}
	evidenceDigest, err := reviewEvidenceDigest(packet.Evidence)
	if err != nil || evidenceDigest != packet.Evidence.EvidenceSHA256 {
		return fmt.Errorf("change review: evidence digest mismatch")
	}
	if packet.Projection.Why != packet.Narrative.Why || packet.Projection.What != packet.Narrative.What || packet.Projection.Boundary != packet.Narrative.Boundary {
		return fmt.Errorf("change review: WHY/WHAT/BOUNDARY projection differs from the declared narrative")
	}
	return nil
}

func uniqueSortedReviewText(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	if result == nil {
		return []string{}
	}
	return result
}

func digestBytes(contents []byte) string {
	value := sha256.Sum256(contents)
	return hex.EncodeToString(value[:])
}

func digestJSON(value interface{}) (string, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(contents), nil
}

func reviewEvidenceDigest(value ReviewEvidenceIdentity) (string, error) {
	value.EvidenceSHA256 = ""
	return digestJSON(value)
}

func sameJSON(left, right interface{}) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
