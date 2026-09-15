package change

import (
	"fmt"
	"sort"
	"strings"
)

type ReviewQualityDebt struct {
	DeterministicExisting int      `json:"deterministicExisting"`
	DeterministicNew      int      `json:"deterministicNew"`
	DeterministicFixed    int      `json:"deterministicFixed"`
	BlockingNew           int      `json:"blockingNew"`
	WaivedBlocking        int      `json:"waivedBlocking"`
	UnwaivedBlocking      int      `json:"unwaivedBlocking"`
	AdvisoryExisting      int      `json:"advisoryExisting"`
	AdvisoryNew           int      `json:"advisoryNew"`
	AdvisoryResolved      int      `json:"advisoryResolved"`
	BlockingFindingIDs    []string `json:"blockingFindingIds"`
	WaivedFindingIDs      []string `json:"waivedFindingIds"`
	AdvisoryNewFindingIDs []string `json:"advisoryNewFindingIds"`
	ProofSHA256            string   `json:"proofSha256"`
}

func reviewQualityDebt(proof *QualityDebtProof) *ReviewQualityDebt {
	if proof == nil {
		return nil
	}
	result := &ReviewQualityDebt{
		DeterministicExisting: len(proof.Deterministic.Existing),
		DeterministicNew:      len(proof.Deterministic.New),
		DeterministicFixed:    len(proof.Deterministic.Fixed),
		BlockingNew:           len(proof.BlockingNew),
		WaivedBlocking:        len(proof.WaivedBlocking),
		UnwaivedBlocking:      len(proof.UnwaivedBlocking),
		BlockingFindingIDs:    []string{},
		WaivedFindingIDs:      []string{},
		AdvisoryNewFindingIDs: []string{},
		ProofSHA256:            proof.ProofSHA256,
	}
	for _, finding := range proof.BlockingNew {
		result.BlockingFindingIDs = append(result.BlockingFindingIDs, finding.ID)
	}
	for _, application := range proof.WaivedBlocking {
		result.WaivedFindingIDs = append(result.WaivedFindingIDs, application.FindingID)
	}
	if proof.Advisory != nil {
		result.AdvisoryExisting = len(proof.Advisory.Existing)
		result.AdvisoryNew = len(proof.Advisory.New)
		result.AdvisoryResolved = len(proof.Advisory.Resolved)
		for _, finding := range proof.Advisory.New {
			result.AdvisoryNewFindingIDs = append(result.AdvisoryNewFindingIDs, finding.ID)
		}
	}
	normalizeReviewQualityDebt(result)
	return result
}

func normalizeReviewQualityDebt(value *ReviewQualityDebt) {
	if value == nil {
		return
	}
	value.ProofSHA256 = strings.TrimSpace(value.ProofSHA256)
	value.BlockingFindingIDs = uniqueSortedReviewText(value.BlockingFindingIDs)
	value.WaivedFindingIDs = uniqueSortedReviewText(value.WaivedFindingIDs)
	value.AdvisoryNewFindingIDs = uniqueSortedReviewText(value.AdvisoryNewFindingIDs)
}

func validateReviewQualityDebt(value *ReviewQualityDebt) error {
	if value == nil {
		return nil
	}
	counts := []int{
		value.DeterministicExisting, value.DeterministicNew, value.DeterministicFixed,
		value.BlockingNew, value.WaivedBlocking, value.UnwaivedBlocking,
		value.AdvisoryExisting, value.AdvisoryNew, value.AdvisoryResolved,
	}
	for _, count := range counts {
		if count < 0 {
			return fmt.Errorf("change review: quality debt counts must be non-negative")
		}
	}
	if value.BlockingNew != value.WaivedBlocking+value.UnwaivedBlocking {
		return fmt.Errorf("change review: quality debt blocking partition is inconsistent")
	}
	if value.BlockingNew != len(value.BlockingFindingIDs) || value.WaivedBlocking != len(value.WaivedFindingIDs) || value.AdvisoryNew != len(value.AdvisoryNewFindingIDs) {
		return fmt.Errorf("change review: quality debt finding IDs do not match projected counts")
	}
	if !validSHA256(value.ProofSHA256) {
		return fmt.Errorf("change review: quality debt proof SHA-256 is invalid")
	}
	for _, values := range [][]string{value.BlockingFindingIDs, value.WaivedFindingIDs, value.AdvisoryNewFindingIDs} {
		copyValues := append([]string(nil), values...)
		sort.Strings(copyValues)
		for i := range copyValues {
			if copyValues[i] == "" || copyValues[i] != values[i] {
				return fmt.Errorf("change review: quality debt finding IDs must be non-empty and sorted")
			}
		}
	}
	return nil
}
