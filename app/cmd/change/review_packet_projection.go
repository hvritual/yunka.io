package change

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/auditcore"
)

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
	return uniqueSortedReviewText(result)
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
	if attestation.QualityDebt != nil {
		result = appendAuditFindingProjections(result, attestation.QualityDebt.Deterministic.Existing)
		result = appendAuditFindingProjections(result, attestation.QualityDebt.Deterministic.New)
		for _, finding := range attestation.QualityDebt.AdvisoryFindingProjections() {
			result = append(result, finding)
		}
		for _, waiver := range attestation.QualityDebt.WaivedBlocking {
			result = append(result, fmt.Sprintf("waived:%s owner=%s expires=%s review=%s", waiver.FindingID, waiver.Owner, waiver.ExpiresAt, waiver.ReviewCondition))
		}
	} else if attestation.ArchitectureDebt != nil {
		result = appendAuditFindingProjections(result, attestation.ArchitectureDebt.Existing)
		result = appendAuditFindingProjections(result, attestation.ArchitectureDebt.New)
	}
	return uniqueSortedReviewText(result)
}

func (proof QualityDebtProof) AdvisoryFindingProjections() []string {
	if proof.Advisory == nil {
		return []string{}
	}
	result := make([]string, 0, len(proof.Advisory.Existing)+len(proof.Advisory.New))
	for _, finding := range proof.Advisory.Existing {
		result = append(result, fmt.Sprintf("advisory-existing:%s:%s:%s", finding.ID, finding.Category, strings.TrimSpace(finding.Reason)))
	}
	for _, finding := range proof.Advisory.New {
		result = append(result, fmt.Sprintf("advisory-new:%s:%s:%s", finding.ID, finding.Category, strings.TrimSpace(finding.Reason)))
	}
	return uniqueSortedReviewText(result)
}

func appendAuditFindingProjections(result []string, values []auditcore.Finding) []string {
	for _, finding := range values {
		result = append(result, strings.TrimSpace(finding.ID)+":"+strings.TrimSpace(finding.Summary))
	}
	return result
}

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
	if packet.QualityDebt != nil {
		proof = append(proof, "quality-debt-sha256="+packet.QualityDebt.ProofSHA256)
	}
	for _, gate := range packet.Verification.Gates {
		proof = append(proof, "gate:"+strings.TrimSpace(gate.Name)+"="+strings.TrimSpace(gate.Status))
	}
	return uniqueSortedReviewText(proof)
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

func reviewValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<none>"
	}
	return strings.TrimSpace(value)
}
