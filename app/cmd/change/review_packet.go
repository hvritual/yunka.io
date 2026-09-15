package change

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/projectflow"
)

const (
	ReviewPacketSchemaVersion = 1
	DefaultReviewPacketPath   = ".git/yunka/review-packet.json"
	ReviewDeltaNone           = "NONE"
	ReviewDeltaChanged        = "CHANGED"
)

// ReviewNarrative is declared review context. It explains intent to a human but
// never authorizes mutation, conformance, or merge. Executable facts are kept
// separately in the derived packet projections below.
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
	BaseSHA             string `json:"baseSha"`
	HeadSHA             string `json:"headSha"`
	OperationID         string `json:"operationId"`
	ContractSHA256      string `json:"contractSha256"`
	AttestationSHA256   string `json:"attestationSha256"`
	NarrativeSHA256     string `json:"narrativeSha256"`
	ChangedPathsSHA256  string `json:"changedPathsSha256"`
	CandidateSHA256     string `json:"candidateSha256"`
	EvidenceSHA256      string `json:"evidenceSha256"`
}

// ReviewPacket is a deterministic presentation artifact. Contract/attestation
// remain authoritative; this packet only projects their exact-candidate facts
// ahead of raw-diff review.
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

func BuildReviewPacket(ctx context.Context, options projectflow.Options, contractInput, attestationInput string, narrative ReviewNarrative) (ReviewPacket, string, error) {
	if ctx == nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: context is required")
	}
	descriptor, err := projectflow.DescribeProject(options)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: resolve project: %w", err)
	}
	if err := normalizeReviewNarrative(&narrative); err != nil {
		return ReviewPacket{}, "", err
	}

	contractValue, _, err := LoadChangeContract(descriptor.Root, contractInput)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: load contract: %w", err)
	}
	contractBytes, _, err := readGitPrivateState(descriptor.Root, contractInput, DefaultChangeContractPath)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: read contract evidence: %w", err)
	}
	attestation, attestationBytes, _, err := loadChangeAttestation(descriptor.Root, attestationInput)
	if err != nil {
		return ReviewPacket{}, "", err
	}
	if err := validateReviewEvidencePair(contractValue, attestation); err != nil {
		return ReviewPacket{}, "", err
	}

	headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
	if err != nil {
		return ReviewPacket{}, "", err
	}
	if headSHA != attestation.HeadSHA {
		return ReviewPacket{}, "", fmt.Errorf("change review: attestation head %s is stale for current HEAD %s", attestation.HeadSHA, headSHA)
	}

	currentOptions := options
	currentOptions.Root = descriptor.Root
	reconciliation, err := ReconcileGitDeltaWithOptions(ctx, currentOptions, contractValue)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: reconcile exact Git delta: %w", err)
	}
	if !sameJSON(reconciliation, attestation.Reconciliation) {
		return ReviewPacket{}, "", fmt.Errorf("change review: changed path set no longer matches the attestation")
	}
	semantic, err := ReconcileSemanticDelta(descriptor.Root, contractValue)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: reconcile semantic delta: %w", err)
	}
	if !sameJSON(semantic, attestation.Semantic) {
		return ReviewPacket{}, "", fmt.Errorf("change review: semantic evidence no longer matches the attestation")
	}

	changedPathsDigest, err := digestJSON(reconciliation.Changes)
	if err != nil {
		return ReviewPacket{}, "", err
	}
	candidateDigest, err := digestCandidate(descriptor.Root, contractValue.BaseSHA, headSHA, reconciliation.Changes)
	if err != nil {
		return ReviewPacket{}, "", err
	}
	narrativeDigest, err := digestJSON(narrative)
	if err != nil {
		return ReviewPacket{}, "", err
	}

	packet := ReviewPacket{
		SchemaVersion:       ReviewPacketSchemaVersion,
		Narrative:           narrative,
		BehaviorChange:      semanticReviewDelta(semantic.Deltas),
		PublicAPIChange:     publicAPIReviewDelta(semantic.Deltas),
		PersistenceChange:   persistenceReviewDelta(reconciliation.Changes),
		GeneratedCodeChange: generatedReviewDelta(reconciliation.Changes),
		Verification: ReviewVerification{
			Conformant: attestation.Conformant,
			Gates:      append([]GateResult(nil), attestation.Gates...),
		},
		AffectedInvariants: deriveAffectedInvariants(narrative, semantic.Deltas),
		Risks:              uniqueSorted(narrative.Risks),
		UnresolvedFindings: deriveUnresolvedFindings(narrative, attestation),
		Evidence: ReviewEvidenceIdentity{
			BaseSHA:            contractValue.BaseSHA,
			HeadSHA:            headSHA,
			OperationID:        contractValue.Operation.OperationID,
			ContractSHA256:     digestBytes(contractBytes),
			AttestationSHA256:  digestBytes(attestationBytes),
			NarrativeSHA256:    narrativeDigest,
			ChangedPathsSHA256: changedPathsDigest,
			CandidateSHA256:    candidateDigest,
		},
	}
	normalizeReviewPacket(&packet)
	packet.Evidence.EvidenceSHA256, err = reviewEvidenceDigest(packet.Evidence)
	if err != nil {
		return ReviewPacket{}, "", err
	}
	packet.Projection = ReviewProjection{
		Why:      packet.Narrative.Why,
		What:     packet.Narrative.What,
		Boundary: packet.Narrative.Boundary,
		Proof:    reviewProof(packet),
	}
	normalizeReviewPacket(&packet)
	if err := validateReviewPacket(packet); err != nil {
		return ReviewPacket{}, "", err
	}
	return packet, descriptor.Root, nil
}

func CheckReviewPacket(ctx context.Context, options projectflow.Options, contractInput, attestationInput, packetInput string) (ReviewPacket, error) {
	descriptor, err := projectflow.DescribeProject(options)
	if err != nil {
		return ReviewPacket{}, fmt.Errorf("change review check: resolve project: %w", err)
	}
	stored, _, err := LoadReviewPacket(descriptor.Root, packetInput)
	if err != nil {
		return ReviewPacket{}, err
	}
	expected, _, err := BuildReviewPacket(ctx, options, contractInput, attestationInput, stored.Narrative)
	if err != nil {
		return ReviewPacket{}, err
	}
	if !sameJSON(stored, expected) {
		return ReviewPacket{}, fmt.Errorf("change review check: review packet is stale or tampered for the exact candidate")
	}
	return stored, nil
}

func WriteReviewPacket(root, output string, packet ReviewPacket) (string, error) {
	normalizeReviewPacket(&packet)
	if err := validateReviewPacket(packet); err != nil {
		return "", err
	}
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultReviewPacketPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	contents, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", err
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return "", err
	}
	return display, nil
}

func LoadReviewPacket(root, input string) (ReviewPacket, string, error) {
	contents, display, err := readGitPrivateState(root, input, DefaultReviewPacketPath)
	if err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: load packet: %w", err)
	}
	var packet ReviewPacket
	if err := decodeStrictJSON(contents, &packet); err != nil {
		return ReviewPacket{}, "", fmt.Errorf("change review: decode packet: %w", err)
	}
	normalizeReviewPacket(&packet)
	if err := validateReviewPacket(packet); err != nil {
		return ReviewPacket{}, "", err
	}
	return packet, display, nil
}

func RenderReviewPacket(packet ReviewPacket, path, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path   string       `json:"path"`
			Packet ReviewPacket `json:"packet"`
		}{Path: path, Packet: packet}
		contents, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change review: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "human review packet %s\n", path)
	fmt.Fprintf(&builder, "problem   %s\n", packet.Narrative.Problem)
	fmt.Fprintf(&builder, "WHY       %s\n", packet.Projection.Why)
	fmt.Fprintf(&builder, "WHAT      %s\n", packet.Projection.What)
	fmt.Fprintf(&builder, "BOUNDARY  %s\n", packet.Projection.Boundary)
	fmt.Fprintf(&builder, "behavior  %s\n", packet.BehaviorChange.State)
	fmt.Fprintf(&builder, "api       %s\n", packet.PublicAPIChange.State)
	fmt.Fprintf(&builder, "persistence %s\n", packet.PersistenceChange.State)
	fmt.Fprintf(&builder, "generated %s\n", packet.GeneratedCodeChange.State)
	fmt.Fprintf(&builder, "verification conformant=%t gates=%d\n", packet.Verification.Conformant, len(packet.Verification.Gates))
	for _, gate := range packet.Verification.Gates {
		fmt.Fprintf(&builder, "  gate %-20s %s", gate.Name, gate.Status)
		if gate.Detail != "" {
			fmt.Fprintf(&builder, " — %s", gate.Detail)
		}
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "invariants %d risks %d unresolved %d\n", len(packet.AffectedInvariants), len(packet.Risks), len(packet.UnresolvedFindings))
	fmt.Fprintf(&builder, "PROOF\n")
	for _, proof := range packet.Projection.Proof {
		fmt.Fprintf(&builder, "  %s\n", proof)
	}
	return builder.String(), nil
}

func loadChangeAttestation(root, input string) (ChangeAttestation, []byte, string, error) {
	contents, display, err := readGitPrivateState(root, input, DefaultChangeAttestationPath)
	if err != nil {
		return ChangeAttestation{}, nil, "", fmt.Errorf("change review: load attestation: %w", err)
	}
	var value ChangeAttestation
	if err := decodeStrictJSON(contents, &value); err != nil {
		return ChangeAttestation{}, nil, "", fmt.Errorf("change review: decode attestation: %w", err)
	}
	if value.SchemaVersion != ChangeAttestationSchemaVersion {
		return ChangeAttestation{}, nil, "", fmt.Errorf("change review: unsupported attestation schemaVersion %d", value.SchemaVersion)
	}
	if strings.TrimSpace(value.BaseSHA) == "" || strings.TrimSpace(value.HeadSHA) == "" || strings.TrimSpace(value.OperationID) == "" {
		return ChangeAttestation{}, nil, "", fmt.Errorf("change review: attestation baseSha, headSha and operationId are required")
	}
	return value, contents, display, nil
}

func readGitPrivateState(root, input, defaultPath string) ([]byte, string, error) {
	path, display, err := resolveGitPrivateStatePath(root, input, defaultPath)
	if err != nil {
		return nil, "", err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return contents, display, nil
}

func validateReviewEvidencePair(contractValue ChangeContract, attestation ChangeAttestation) error {
	if contractValue.BaseSHA != attestation.BaseSHA {
		return fmt.Errorf("change review: contract base %s differs from attestation base %s", contractValue.BaseSHA, attestation.BaseSHA)
	}
	if contractValue.Operation.OperationID != attestation.OperationID {
		return fmt.Errorf("change review: contract operation %s differs from attestation operation %s", contractValue.Operation.OperationID, attestation.OperationID)
	}
	if attestation.Reconciliation.SchemaVersion != ChangeReconciliationSchemaVersion || attestation.Reconciliation.BaseSHA != contractValue.BaseSHA || attestation.Reconciliation.OperationID != contractValue.Operation.OperationID {
		return fmt.Errorf("change review: attestation reconciliation identity is inconsistent with the change contract")
	}
	if attestation.Semantic.SchemaVersion != SemanticReportSchemaVersion || attestation.Semantic.OperationID != contractValue.Operation.OperationID {
		return fmt.Errorf("change review: attestation semantic identity is inconsistent with the change contract")
	}
	return nil
}

func normalizeReviewNarrative(value *ReviewNarrative) error {
	if value == nil {
		return fmt.Errorf("change review: narrative is required")
	}
	value.Problem = strings.TrimSpace(value.Problem)
	value.Why = strings.TrimSpace(value.Why)
	value.What = strings.TrimSpace(value.What)
	value.Boundary = strings.TrimSpace(value.Boundary)
	value.CurrentConcepts = uniqueSorted(value.CurrentConcepts)
	value.DesiredOwnership = uniqueSorted(value.DesiredOwnership)
	value.AffectedInvariants = uniqueSorted(value.AffectedInvariants)
	value.Risks = uniqueSorted(value.Risks)
	value.UnresolvedFindings = uniqueSorted(value.UnresolvedFindings)
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
	packet.AffectedInvariants = uniqueSorted(packet.AffectedInvariants)
	packet.Risks = uniqueSorted(packet.Risks)
	packet.UnresolvedFindings = uniqueSorted(packet.UnresolvedFindings)
	packet.Projection.Why = strings.TrimSpace(packet.Projection.Why)
	packet.Projection.What = strings.TrimSpace(packet.Projection.What)
	packet.Projection.Boundary = strings.TrimSpace(packet.Projection.Boundary)
	packet.Projection.Proof = uniqueSorted(packet.Projection.Proof)
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

func semanticReviewDelta(values []SemanticDelta) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	for _, value := range values {
		delta.Facts = append(delta.Facts, semanticReviewFact(value))
	}
	normalizeReviewDelta(&delta)
	return delta
}

func publicAPIReviewDelta(values []SemanticDelta) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	for _, value := range values {
		if value.Category != SemanticContract && value.Category != SemanticTransport {
			continue
		}
		delta.Facts = append(delta.Facts, semanticReviewFact(value))
	}
	normalizeReviewDelta(&delta)
	return delta
}

func semanticReviewFact(value SemanticDelta) ReviewFact {
	return ReviewFact{
		Kind:    "semantic." + strings.TrimSpace(value.Category),
		Subject: strings.TrimSpace(value.Subject),
		Detail:  fmt.Sprintf("%s: %s -> %s (allowed=%t)", strings.TrimSpace(value.Field), reviewValue(value.Before), reviewValue(value.After), value.Allowed),
	}
}

func persistenceReviewDelta(values []FileChange) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	for _, value := range values {
		if !isPersistenceReviewPath(value.Path) && !isPersistenceReviewPath(value.PreviousPath) {
			continue
		}
		delta.Facts = append(delta.Facts, ReviewFact{Kind: "source.persistence", Path: value.Path, Detail: reviewFileChangeDetail(value)})
	}
	normalizeReviewDelta(&delta)
	return delta
}

func generatedReviewDelta(values []FileChange) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	for _, value := range values {
		if value.Class != "generated" {
			continue
		}
		delta.Facts = append(delta.Facts, ReviewFact{Kind: "source.generated", Path: value.Path, Detail: reviewFileChangeDetail(value)})
	}
	normalizeReviewDelta(&delta)
	return delta
}

func reviewFileChangeDetail(value FileChange) string {
	detail := strings.TrimSpace(value.Status) + " class=" + strings.TrimSpace(value.Class)
	if value.Owner != "" {
		detail += " owner=" + strings.TrimSpace(value.Owner)
	}
	if value.PreviousPath != "" {
		detail += " previous=" + cleanProjectPath(value.PreviousPath)
	}
	return detail
}

func isPersistenceReviewPath(value string) bool {
	value = strings.ToLower(cleanProjectPath(value))
	if value == "" {
		return false
	}
	return strings.Contains(value, "/infrastructure/persistence/") || strings.HasPrefix(value, "migrations/") || strings.Contains(value, "/migrations/") || strings.HasSuffix(value, ".sql")
}

func deriveAffectedInvariants(narrative ReviewNarrative, values []SemanticDelta) []string {
	result := append([]string(nil), narrative.AffectedInvariants...)
	for _, value := range values {
		switch value.Category {
		case SemanticPermission, SemanticTenant, SemanticAuthentication, SemanticTransaction, SemanticIdempotency, SemanticComposition, SemanticDependencies, SemanticCapabilities:
			result = append(result, strings.TrimSpace(value.Subject)+":"+strings.TrimSpace(value.Field))
		}
	}
	return uniqueSorted(result)
}

func deriveUnresolvedFindings(narrative ReviewNarrative, attestation ChangeAttestation) []string {
	result := append([]string(nil), narrative.UnresolvedFindings...)
	for _, item := range attestation.Diagnostics {
		detail := strings.TrimSpace(item.Detail)
		if detail == "" {
			detail = strings.TrimSpace(item.Summary)
		}
		result = append(result, strings.TrimSpace(item.Code)+":"+detail)
	}
	if attestation.ArchitectureDebt != nil {
		for _, finding := range append(append([]auditFindingProjection(nil), projectAuditFindings(attestation.ArchitectureDebt.Existing)...), projectAuditFindings(attestation.ArchitectureDebt.New)...) {
			result = append(result, finding.ID+":"+finding.Summary)
		}
	}
	return uniqueSorted(result)
}

// auditFindingProjection avoids giving review presentation code any mutation or
// policy authority over audit findings.
type auditFindingProjection struct {
	ID      string
	Summary string
}

func projectAuditFindings(values []struct{}) []auditFindingProjection { return nil }

func reviewProof(packet ReviewPacket) []string {
	proof := []string{
		"base=" + packet.Evidence.BaseSHA,
		"head=" + packet.Evidence.HeadSHA,
		"contract-sha256=" + packet.Evidence.ContractSHA256,
		"attestation-sha256=" + packet.Evidence.AttestationSHA256,
		"changed-paths-sha256=" + packet.Evidence.ChangedPathsSHA256,
		"candidate-sha256=" + packet.Evidence.CandidateSHA256,
		"evidence-sha256=" + packet.Evidence.EvidenceSHA256,
	}
	for _, gate := range packet.Verification.Gates {
		proof = append(proof, "gate:"+strings.TrimSpace(gate.Name)+"="+strings.TrimSpace(gate.Status))
	}
	return uniqueSorted(proof)
}

func digestCandidate(root, baseSHA, headSHA string, changes []FileChange) (string, error) {
	hash := sha256.New()
	fmt.Fprintf(hash, "base\x00%s\nhead\x00%s\n", strings.TrimSpace(baseSHA), strings.TrimSpace(headSHA))
	for _, change := range changes {
		encoded, err := json.Marshal(change)
		if err != nil {
			return "", err
		}
		hash.Write(encoded)
		hash.Write([]byte{'\n'})
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(change.Status)), "D") {
			hash.Write([]byte("<deleted>\n"))
			continue
		}
		path := cleanProjectPath(change.Path)
		if path == "" {
			return "", fmt.Errorf("change review: candidate contains invalid path %q", change.Path)
		}
		absolute := filepath.Join(root, filepath.FromSlash(path))
		relative, err := filepath.Rel(root, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", fmt.Errorf("change review: candidate path %s escaped project root", path)
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			return "", fmt.Errorf("change review: read candidate path %s: %w", path, err)
		}
		switch {
		case info.Mode().IsRegular():
			contents, err := os.ReadFile(absolute)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(hash, "mode\x00%o\n", info.Mode().Perm())
			hash.Write(contents)
			hash.Write([]byte{'\n'})
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(absolute)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(hash, "symlink\x00%s\n", filepath.ToSlash(target))
		default:
			return "", fmt.Errorf("change review: candidate path %s is not a regular file or symlink", path)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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

func reviewValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<none>"
	}
	return strings.TrimSpace(value)
}
