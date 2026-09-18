package audit

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

const RuleOperationGrowthBoundary = auditcore.RuleOperationGrowthBoundary

func boundaryGrowthFindings(currentOptions projectflow.Options, baselineRoot, baseSHA string) ([]auditcore.Finding, error) {
	currentRoot, err := filepath.Abs(strings.TrimSpace(currentOptions.Root))
	if err != nil {
		return nil, fmt.Errorf("audit boundary growth: current root: %w", err)
	}
	baselineOptions, err := rebaseBoundaryCompilerOptions(currentOptions, currentRoot, baselineRoot)
	if err != nil {
		return nil, err
	}
	currentOptions.Root = currentRoot

	baseline, err := projectflow.DescribeContractSourceSnapshot(context.Background(), baselineOptions)
	if err != nil {
		return nil, fmt.Errorf("audit boundary growth: compile immutable baseline %s: %w", baseSHA, err)
	}
	current, err := projectflow.DescribeContractSourceSnapshot(context.Background(), currentOptions)
	if err != nil {
		return nil, fmt.Errorf("audit boundary growth: compile current canonical source: %w", err)
	}
	events := boundarycore.EvaluateGrowth(baseSHA, baseline.Manifest, current.Manifest)
	result := []auditcore.Finding{}
	for _, event := range events {
		if event.Outcome == boundarycore.ReuseExistingService {
			continue
		}
		result = append(result, auditcore.Finding{
			ID:   RuleOperationGrowthBoundary + ":" + event.OperationID + ":" + event.Kind,
			Rule: RuleOperationGrowthBoundary, Class: auditcore.FindingProvenViolation, Blocking: true,
			Subject:     event.OperationID,
			Summary:     "Operation Growth is not proven to remain inside the existing Service Boundary",
			Invariant:   "new or boundary-changing Operations must recompute canonical base/current Service Boundary evidence; non-reuse outcomes cannot enter silently",
			Reason:      event.Reason + "; outcome=" + event.Outcome,
			Remediation: "review the boundary decision; split the Application/Service projection when contradicted, or supply explicit compatible canonical boundary evidence before growth",
			Evidence: []auditcore.Evidence{
				{Kind: auditcore.EvidenceGit, Source: "git.base", Detail: baseSHA},
				{Kind: auditcore.EvidenceCanonical, Source: "contract.source", Detail: event.Kind + " outcome=" + event.Outcome},
			},
		})
	}
	return result, nil
}

func rebaseBoundaryCompilerOptions(current projectflow.Options, currentRoot, baselineRoot string) (projectflow.Options, error) {
	baselineAbs, err := filepath.Abs(baselineRoot)
	if err != nil {
		return projectflow.Options{}, fmt.Errorf("audit boundary growth: baseline root: %w", err)
	}
	currentRoot = filepath.Clean(currentRoot)
	result := projectflow.Options{Root: baselineAbs, Protoc: current.Protoc, ProtoPaths: make([]string, 0, len(current.ProtoPaths))}
	for _, raw := range current.ProtoPaths {
		value := strings.TrimSpace(raw)
		if value == "" {
			return projectflow.Options{}, fmt.Errorf("audit boundary growth: proto-path must not be blank")
		}
		absolute := value
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(currentRoot, absolute)
		}
		absolute = filepath.Clean(absolute)
		rel, relErr := filepath.Rel(currentRoot, absolute)
		inside := relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		if inside {
			result.ProtoPaths = append(result.ProtoPaths, filepath.Join(baselineAbs, rel))
		} else {
			result.ProtoPaths = append(result.ProtoPaths, absolute)
		}
	}
	return result, nil
}
