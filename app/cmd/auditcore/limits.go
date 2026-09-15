package auditcore

import (
	"fmt"
	"path"
	"sort"
)

func evaluateDeclaredLimits(snapshot SourceSnapshot, limits QualityLimits) []Finding {
	var findings []Finding
	for _, file := range snapshot.Files {
		if file.Generated || file.Test {
			continue
		}
		if limits.MaxFileLines > 0 && file.Lines > limits.MaxFileLines {
			findings = append(findings, limitFinding(
				RuleFileLineLimit,
				file,
				"source file exceeds the declared line limit",
				fmt.Sprintf("lines=%d limit=%d", file.Lines, limits.MaxFileLines),
				"split the file along existing responsibility boundaries or raise the explicit policy limit with review evidence; do not change behavior merely to reduce line count",
			))
		}
		if limits.MaxTopLevelDeclarations > 0 && file.TopLevelDeclarations > limits.MaxTopLevelDeclarations {
			findings = append(findings, limitFinding(
				RuleFileDeclarationLimit,
				file,
				"source file exceeds the declared top-level declaration limit",
				fmt.Sprintf("topLevelDeclarations=%d limit=%d", file.TopLevelDeclarations, limits.MaxTopLevelDeclarations),
				"review the file responsibility and move coherent declarations behind semantic ownership boundaries, or raise the explicit policy limit with review evidence",
			))
		}
		if limits.MaxBranchPoints > 0 && file.BranchPoints > limits.MaxBranchPoints {
			findings = append(findings, limitFinding(
				RuleFileBranchLimit,
				file,
				"source file exceeds the declared structural branch-point limit",
				fmt.Sprintf("branchPoints=%d limit=%d", file.BranchPoints, limits.MaxBranchPoints),
				"reduce structural branching through behavior-preserving decomposition where responsibility warrants it, or raise the explicit policy limit with review evidence",
			))
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	if findings == nil {
		return []Finding{}
	}
	return findings
}

func limitFinding(rule string, file GoSourceFile, summary, detail, remediation string) Finding {
	filePath := cleanSlash(file.Path)
	reason := summary + ": " + detail
	return Finding{
		ID:          rule + ":" + filePath,
		Rule:        rule,
		Class:       FindingProvenViolation,
		Subject:     filePath,
		Summary:     summary,
		Invariant:   "declared engineering-quality limits are deterministic review budgets; exceeding an enabled limit is objective debt but does not by itself prove bad domain design",
		Path:        filePath,
		Symbol:      path.Base(filePath),
		Reason:      reason,
		Remediation: remediation,
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: QualityPolicyRelativePath, Detail: detail},
			{Kind: EvidenceSource, Source: "go.ast.metrics", Path: filePath, Detail: detail},
		},
	}
}
