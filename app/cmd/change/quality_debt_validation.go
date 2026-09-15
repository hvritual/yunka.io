package change

import (
	"fmt"
	"sort"
	"strings"

	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/auditcore"
)

func ValidateQualityDebtProof(proof QualityDebtProof) error {
	if proof.SchemaVersion != QualityDebtSchemaVersion {
		return fmt.Errorf("quality debt proof: unsupported schemaVersion %d", proof.SchemaVersion)
	}
	if strings.TrimSpace(proof.Deterministic.BaseSHA) == "" {
		return fmt.Errorf("quality debt proof: deterministic baseline SHA is required")
	}
	if proof.Advisory != nil {
		if _, err := advisorcore.MarshalSemanticFindingDelta(*proof.Advisory); err != nil {
			return fmt.Errorf("quality debt proof: advisory delta: %w", err)
		}
	}
	expectedBlocking := blockingNewFindings(proof.Deterministic.New)
	if !sameAuditFindingIDs(expectedBlocking, proof.BlockingNew) {
		return fmt.Errorf("quality debt proof: blockingNew does not match deterministic blocking policy")
	}
	blockingIndex := make(map[string]auditcore.Finding, len(expectedBlocking))
	for _, finding := range expectedBlocking {
		blockingIndex[finding.ID] = finding
	}
	waived := map[string]struct{}{}
	for _, application := range proof.WaivedBlocking {
		application.WaiverID = strings.TrimSpace(application.WaiverID)
		application.FindingID = strings.TrimSpace(application.FindingID)
		application.Owner = strings.TrimSpace(application.Owner)
		application.Reason = strings.TrimSpace(application.Reason)
		application.ExpiresAt = strings.TrimSpace(application.ExpiresAt)
		application.ReviewCondition = strings.TrimSpace(application.ReviewCondition)
		if application.WaiverID == "" || application.FindingID == "" || application.Owner == "" || application.Reason == "" || application.ExpiresAt == "" || application.ReviewCondition == "" {
			return fmt.Errorf("quality debt proof: waiver application fields are incomplete")
		}
		if _, ok := blockingIndex[application.FindingID]; !ok {
			return fmt.Errorf("quality debt proof: waiver %s targets non-blocking finding %s", application.WaiverID, application.FindingID)
		}
		if _, duplicate := waived[application.FindingID]; duplicate {
			return fmt.Errorf("quality debt proof: multiple waiver applications target finding %s", application.FindingID)
		}
		waived[application.FindingID] = struct{}{}
	}
	unwaived := map[string]struct{}{}
	for _, finding := range proof.UnwaivedBlocking {
		if _, ok := blockingIndex[finding.ID]; !ok {
			return fmt.Errorf("quality debt proof: unwaived finding %s is not blocking new debt", finding.ID)
		}
		if _, duplicate := unwaived[finding.ID]; duplicate {
			return fmt.Errorf("quality debt proof: duplicate unwaived finding %s", finding.ID)
		}
		unwaived[finding.ID] = struct{}{}
	}
	for id := range blockingIndex {
		_, isWaived := waived[id]
		_, isUnwaived := unwaived[id]
		if isWaived == isUnwaived {
			return fmt.Errorf("quality debt proof: blocking finding %s must appear in exactly one waived/unwaived partition", id)
		}
	}
	if proof.WaiverSetSHA256 != "" && !validSHA256(proof.WaiverSetSHA256) {
		return fmt.Errorf("quality debt proof: waiverSetSha256 is invalid")
	}
	if !validSHA256(proof.ProofSHA256) {
		return fmt.Errorf("quality debt proof: proofSha256 is invalid")
	}
	copyValue := proof
	normalizeQualityDebtProof(&copyValue)
	digest, err := qualityDebtProofDigest(copyValue)
	if err != nil {
		return err
	}
	if digest != proof.ProofSHA256 {
		return fmt.Errorf("quality debt proof: proofSha256 mismatch")
	}
	return nil
}

func sameAuditFindingIDs(left, right []auditcore.Finding) bool {
	leftIDs := make([]string, 0, len(left))
	rightIDs := make([]string, 0, len(right))
	for _, finding := range left {
		leftIDs = append(leftIDs, finding.ID)
	}
	for _, finding := range right {
		rightIDs = append(rightIDs, finding.ID)
	}
	sort.Strings(leftIDs)
	sort.Strings(rightIDs)
	if len(leftIDs) != len(rightIDs) {
		return false
	}
	for i := range leftIDs {
		if leftIDs[i] != rightIDs[i] {
			return false
		}
	}
	return true
}
