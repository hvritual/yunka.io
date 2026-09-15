package audit

import (
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/domain"
)

func generatedArtifactFindings(root, generatedGoRoot string) ([]auditcore.Finding, error) {
	generatedRoot := generatedGoRoot
	if !filepath.IsAbs(generatedRoot) {
		generatedRoot = filepath.Join(root, filepath.FromSlash(generatedGoRoot))
	}
	issues, err := domain.InspectGeneratedArtifacts(generatedRoot)
	if err != nil {
		return nil, err
	}
	findings := make([]auditcore.Finding, 0, len(issues))
	for _, issue := range issues {
		finding := auditcore.Finding{
			Class:   auditcore.FindingProvenViolation,
			Subject: strings.TrimSpace(issue.Domain),
			Path:    filepath.ToSlash(strings.TrimSpace(issue.Path)),
			Symbol:  filepath.Base(filepath.FromSlash(strings.TrimSpace(issue.Path))),
			Reason:  strings.TrimSpace(issue.Reason),
			Evidence: []auditcore.Evidence{
				{Kind: auditcore.EvidenceCanonical, Source: "domain.json", Detail: "managed domain=" + strings.TrimSpace(issue.Domain)},
				{Kind: auditcore.EvidenceGenerated, Source: "domain.renderer", Path: filepath.ToSlash(strings.TrimSpace(issue.Path)), Detail: strings.TrimSpace(issue.Reason)},
			},
		}
		switch issue.Kind {
		case domain.GeneratedOwnershipConflict:
			finding.Rule = auditcore.RuleGeneratedOwnershipMix
			finding.Summary = "generator-owned path contains developer-owned source"
			finding.Invariant = "a canonical generator-owned path must contain generator-owned output; developer-owned behavior must remain outside generator overwrite scope"
			finding.Remediation = "move developer-owned behavior to a developer-owned file, then regenerate the managed Domain so the canonical generated path is restored"
		case domain.GeneratedStaleArtifact:
			finding.Rule = auditcore.RuleStaleGeneratedArtifact
			finding.Summary = "stale generator-owned artifact remains in the managed Domain"
			finding.Invariant = "files marked as framework generated must correspond to the current canonical renderer output"
			finding.Remediation = "run the canonical Domain generator to remove stale generated output; do not hand-edit or manually retain obsolete generator-owned files"
		case domain.GeneratedArtifactDrift:
			finding.Rule = auditcore.RuleGeneratedArtifactDrift
			finding.Summary = "generator-owned artifact is missing or differs from canonical output"
			finding.Invariant = "generator-owned output must be byte-equivalent to the canonical renderer for the current managed Domain contract"
			finding.Remediation = "run the canonical Domain generator and review the resulting generated delta; do not repair generated output by hand"
		default:
			continue
		}
		finding.ID = finding.Rule + ":" + finding.Path
		findings = append(findings, finding)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return findings, nil
}
