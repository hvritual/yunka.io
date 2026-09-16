package change

import (
	"fmt"
	"strings"
)

// validateQualityDebtCandidateBinding establishes correspondence between a
// persisted advisory debt proof and the active Change candidate. Structural
// proof validation is necessary but not sufficient: the candidate digest is
// re-derived from authoritative Git/source state by the caller.
func validateQualityDebtCandidateBinding(proof *QualityDebtProof, baseSHA, headSHA, candidateSHA string) error {
	if proof == nil || proof.Advisory == nil {
		return nil
	}
	baseSHA = strings.TrimSpace(baseSHA)
	headSHA = strings.TrimSpace(headSHA)
	candidateSHA = strings.TrimSpace(candidateSHA)
	if baseSHA == "" || headSHA == "" || !validSHA256(candidateSHA) {
		return fmt.Errorf("quality debt proof: active candidate identity is incomplete")
	}
	baseline := proof.Advisory.BaselineEvidence
	current := proof.Advisory.CurrentEvidence
	if baseline == nil || current == nil || current.Change == nil {
		return fmt.Errorf("quality debt proof: advisory evidence is not bound to an exact Change candidate")
	}
	if baseline.HeadSHA != baseSHA {
		return fmt.Errorf("quality debt proof: advisory baseline %s differs from active Change base %s", baseline.HeadSHA, baseSHA)
	}
	if current.HeadSHA != headSHA || current.Change.HeadSHA != headSHA || current.Change.BaseSHA != baseSHA {
		return fmt.Errorf("quality debt proof: advisory review base/head is stale for the active Change candidate")
	}
	if current.Change.CandidateSHA256 != candidateSHA {
		return fmt.Errorf("quality debt proof: advisory candidate %s is stale for active candidate %s", current.Change.CandidateSHA256, candidateSHA)
	}
	return nil
}
