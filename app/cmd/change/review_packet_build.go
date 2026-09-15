package change

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/projectflow"
)

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
	fmt.Fprintf(&builder, "problem     %s\n", packet.Narrative.Problem)
	fmt.Fprintf(&builder, "WHY         %s\n", packet.Projection.Why)
	fmt.Fprintf(&builder, "WHAT        %s\n", packet.Projection.What)
	fmt.Fprintf(&builder, "BOUNDARY    %s\n", packet.Projection.Boundary)
	fmt.Fprintf(&builder, "behavior    %s\n", packet.BehaviorChange.State)
	fmt.Fprintf(&builder, "api         %s\n", packet.PublicAPIChange.State)
	fmt.Fprintf(&builder, "persistence %s\n", packet.PersistenceChange.State)
	fmt.Fprintf(&builder, "generated   %s\n", packet.GeneratedCodeChange.State)
	fmt.Fprintf(&builder, "verification conformant=%t gates=%d\n", packet.Verification.Conformant, len(packet.Verification.Gates))
	for _, gate := range packet.Verification.Gates {
		fmt.Fprintf(&builder, "  gate %-20s %s", gate.Name, gate.Status)
		if gate.Detail != "" {
			fmt.Fprintf(&builder, " — %s", gate.Detail)
		}
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "invariants %d risks %d unresolved %d\n", len(packet.AffectedInvariants), len(packet.Risks), len(packet.UnresolvedFindings))
	builder.WriteString("PROOF\n")
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
