package change

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"yunka.io/app/cmd/advisorcore"
)

func loadAdvisoryQualityDebt(ctx context.Context, root, baselineInput, currentInput, expectedBaseSHA, expectedHeadSHA, expectedCandidateSHA string) (*advisorcore.SemanticFindingDelta, error) {
	baselineInput = strings.TrimSpace(baselineInput)
	currentInput = strings.TrimSpace(currentInput)
	if baselineInput == "" && currentInput == "" {
		return nil, nil
	}
	if ctx == nil {
		return nil, fmt.Errorf("quality debt proof: context is required for semantic evidence binding")
	}
	if baselineInput == "" || currentInput == "" {
		return nil, fmt.Errorf("quality debt proof: semantic baseline and current attestation paths must be supplied together")
	}
	expectedBaseSHA = strings.TrimSpace(expectedBaseSHA)
	expectedHeadSHA = strings.TrimSpace(expectedHeadSHA)
	expectedCandidateSHA = strings.TrimSpace(expectedCandidateSHA)
	if expectedBaseSHA == "" || expectedHeadSHA == "" || expectedCandidateSHA == "" {
		return nil, fmt.Errorf("quality debt proof: exact base, head, and candidate identities are required for semantic evidence")
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

	baseRoot, cleanup, err := materializeSourceBase(ctx, root, expectedBaseSHA)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: materialize semantic baseline %s: %w", expectedBaseSHA, err)
	}
	defer cleanup()
	if err := revalidateSemanticAttestationEvidence(baseRoot, baseline, expectedBaseSHA, "", ""); err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic baseline evidence: %w", err)
	}
	if err := revalidateSemanticAttestationEvidence(root, current, expectedHeadSHA, expectedBaseSHA, expectedCandidateSHA); err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic current evidence: %w", err)
	}

	delta, err := advisorcore.CompareSemanticReviewAttestations(baseline, current)
	if err != nil {
		return nil, fmt.Errorf("quality debt proof: semantic delta: %w", err)
	}
	return &delta, nil
}

func revalidateSemanticAttestationEvidence(root string, attestation advisorcore.SemanticReviewAttestation, expectedHeadSHA, expectedBaseSHA, expectedCandidateSHA string) error {
	if attestation.Evidence == nil {
		return fmt.Errorf("exact evidence binding is missing; regenerate the semantic attestation from its validated request")
	}
	binding := *attestation.Evidence
	if binding.HeadSHA != expectedHeadSHA {
		return fmt.Errorf("review head %s is stale for expected head %s", binding.HeadSHA, expectedHeadSHA)
	}
	if expectedCandidateSHA != "" {
		if binding.Change == nil {
			return fmt.Errorf("current semantic review is not bound to an exact Change candidate")
		}
		if binding.Change.BaseSHA != expectedBaseSHA || binding.Change.HeadSHA != expectedHeadSHA {
			return fmt.Errorf("semantic Change identity base/head does not match the active Change candidate")
		}
		if binding.Change.CandidateSHA256 != expectedCandidateSHA {
			return fmt.Errorf("semantic Change candidate %s is stale for active candidate %s", binding.Change.CandidateSHA256, expectedCandidateSHA)
		}
	}

	sources, err := readBoundSemanticSources(root, binding.Sources)
	if err != nil {
		return err
	}
	request, err := advisorcore.NewSemanticReviewRequest(binding.HeadSHA, sources, binding.Change)
	if err != nil {
		return err
	}
	if request.RequestDigest != attestation.RequestDigest {
		return fmt.Errorf("request digest does not match authoritative source/change evidence")
	}
	if request.Evidence.SourceIdentity != attestation.SourceIdentity || request.Evidence.SourceIdentity != binding.SourceIdentity {
		return fmt.Errorf("source identity does not match authoritative source/change evidence")
	}
	if request.Evidence.SourceSHA256 != binding.SourceSHA256 {
		return fmt.Errorf("source digest does not match authoritative source bytes")
	}
	if len(request.Evidence.Sources) != len(binding.Sources) {
		return fmt.Errorf("source scope changed since semantic review")
	}
	for index := range request.Evidence.Sources {
		if request.Evidence.Sources[index].Path != binding.Sources[index].Path || request.Evidence.Sources[index].SHA256 != binding.Sources[index].SHA256 {
			return fmt.Errorf("source evidence for %s is stale", binding.Sources[index].Path)
		}
	}
	return nil
}

func readBoundSemanticSources(root string, bindings []advisorcore.SemanticSourceBinding) ([]advisorcore.SemanticSource, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	sources := make([]advisorcore.SemanticSource, 0, len(bindings))
	for _, binding := range bindings {
		path := strings.TrimSpace(binding.Path)
		if path == "" || filepath.IsAbs(filepath.FromSlash(path)) {
			return nil, fmt.Errorf("bound semantic source path %q is invalid", path)
		}
		absolute := filepath.Clean(filepath.Join(rootAbs, filepath.FromSlash(path)))
		relative, err := filepath.Rel(rootAbs, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("bound semantic source path %q escapes project root", path)
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, fmt.Errorf("read bound semantic source %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("bound semantic source %s must be a regular non-symlink file", path)
		}
		contents, err := os.ReadFile(absolute)
		if err != nil {
			return nil, fmt.Errorf("read bound semantic source %s: %w", path, err)
		}
		if !utf8.Valid(contents) {
			return nil, fmt.Errorf("bound semantic source %s is not UTF-8 text", path)
		}
		sources = append(sources, advisorcore.SemanticSource{Path: filepath.ToSlash(relative), Content: string(contents)})
	}
	return sources, nil
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
