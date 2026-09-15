package change

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/advisorcore"
)

func loadAdvisoryQualityDebt(root, baselineInput, currentInput string) (*advisorcore.SemanticFindingDelta, error) {
	baselineInput = strings.TrimSpace(baselineInput)
	currentInput = strings.TrimSpace(currentInput)
	if baselineInput == "" && currentInput == "" {
		return nil, nil
	}
	if baselineInput == "" || currentInput == "" {
		return nil, fmt.Errorf("quality debt proof: semantic baseline and current attestation paths must be supplied together")
	}
	baselineBytes, err := readQualityEvidenceFile(root, baselineInput)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: read semantic baseline: %w", err)
	}
	currentBytes, err := readQualityEvidenceFile(root, currentInput)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: read semantic current: %w", err)
	}
	baseline, err := advisorcore.DecodeSemanticReviewAttestation(baselineBytes)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic baseline: %w", err)
	}
	current, err := advisorcore.DecodeSemanticReviewAttestation(currentBytes)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic current: %w", err)
	}
	delta, err := advisorcore.CompareSemanticReviewAttestations(baseline, current)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic delta: %w", err)
	}
	return &delta, nil
}

func readQualityEvidenceFile(root, input string) ([]byte, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	path := strings.TrimSpace(input)
	if !filepath.IsAbs(path) {
		path = filepath.Join(rootAbs, filepath.FromSlash(path))
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return nil, fmt.Errorf("evidence path %s is outside project root", input)
	}
	info, err := os.Lstat(pathAbs)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("evidence path %s is not a regular file", input)
	}
	return os.ReadFile(pathAbs)
}
