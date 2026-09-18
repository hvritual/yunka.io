package auditcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	QualityPolicySchemaVersion = 1
	QualityPolicyRelativePath  = ".yunka/engineering-quality.json"

	RuleGeneratedOwnershipMix  = "AUDIT-GEN-001"
	RuleStaleGeneratedArtifact = "AUDIT-GEN-002"
	RuleGeneratedArtifactDrift = "AUDIT-GEN-003"
	RuleFileLineLimit          = "AUDIT-SIZE-001"
	RuleFileDeclarationLimit   = "AUDIT-SIZE-002"
	RuleFileBranchLimit        = "AUDIT-COMPLEXITY-001"

	// RuleOperationGrowthBoundary is mandatory architecture correctness. It is
	// intentionally excluded from configurable QualityPolicy.blockingRules.
	RuleOperationGrowthBoundary = "AUDIT-BOUNDARY-001"
)

type QualityLimits struct {
	MaxFileLines            int `json:"maxFileLines,omitempty"`
	MaxTopLevelDeclarations int `json:"maxTopLevelDeclarations,omitempty"`
	MaxBranchPoints         int `json:"maxBranchPoints,omitempty"`
}

type QualityPolicy struct {
	SchemaVersion int           `json:"schemaVersion"`
	Limits        QualityLimits `json:"limits,omitempty"`
	BlockingRules []string      `json:"blockingRules,omitempty"`
}

type QualityPolicyEvidence struct {
	Path          string        `json:"path"`
	Present       bool          `json:"present"`
	SHA256        string        `json:"sha256,omitempty"`
	Limits        QualityLimits `json:"limits,omitempty"`
	BlockingRules []string      `json:"blockingRules"`
}

func LoadQualityPolicy(root string) (QualityPolicy, QualityPolicyEvidence, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	policy := QualityPolicy{SchemaVersion: QualityPolicySchemaVersion, BlockingRules: []string{}}
	evidence := QualityPolicyEvidence{Path: QualityPolicyRelativePath, BlockingRules: []string{}}
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(QualityPolicyRelativePath)))
	if os.IsNotExist(err) {
		return policy, evidence, nil
	}
	if err != nil {
		return QualityPolicy{}, QualityPolicyEvidence{}, fmt.Errorf("audit quality policy: read %s: %w", QualityPolicyRelativePath, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return QualityPolicy{}, QualityPolicyEvidence{}, fmt.Errorf("audit quality policy: decode %s: %w", QualityPolicyRelativePath, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected trailing JSON value")
		}
		return QualityPolicy{}, QualityPolicyEvidence{}, fmt.Errorf("audit quality policy: decode %s: %w", QualityPolicyRelativePath, err)
	}
	if err := ValidateQualityPolicy(policy); err != nil {
		return QualityPolicy{}, QualityPolicyEvidence{}, err
	}
	policy.BlockingRules = uniqueStrings(policy.BlockingRules)
	digest := sha256.Sum256(contents)
	evidence.Present = true
	evidence.SHA256 = hex.EncodeToString(digest[:])
	evidence.Limits = policy.Limits
	evidence.BlockingRules = append([]string(nil), policy.BlockingRules...)
	return policy, evidence, nil
}

func ValidateQualityPolicy(policy QualityPolicy) error {
	if policy.SchemaVersion != QualityPolicySchemaVersion {
		return fmt.Errorf("audit quality policy: %s schemaVersion=%d is unsupported", QualityPolicyRelativePath, policy.SchemaVersion)
	}
	if policy.Limits.MaxFileLines < 0 || policy.Limits.MaxTopLevelDeclarations < 0 || policy.Limits.MaxBranchPoints < 0 {
		return fmt.Errorf("audit quality policy: limits must be non-negative")
	}
	allowed := supportedBlockingRuleSet()
	seen := map[string]struct{}{}
	for _, rule := range policy.BlockingRules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			return fmt.Errorf("audit quality policy: blockingRules contains an empty rule")
		}
		if _, ok := allowed[rule]; !ok {
			return fmt.Errorf("audit quality policy: blocking rule %q is unsupported or advisory-only", rule)
		}
		if _, duplicate := seen[rule]; duplicate {
			return fmt.Errorf("audit quality policy: blocking rule %q is duplicated", rule)
		}
		seen[rule] = struct{}{}
	}
	return nil
}

func supportedBlockingRuleSet() map[string]struct{} {
	return map[string]struct{}{
		RuleHistoricalSourceIdentity:     {},
		RuleMissingPackageDocumentation:  {},
		RuleMissingContractDocumentation: {},
		RuleCrossDomainRepositoryBypass:  {},
		RulePlatformProviderBypass:       {},
		RuleAuthorizationBypass:          {},
		RuleGeneratedOwnershipMix:        {},
		RuleStaleGeneratedArtifact:       {},
		RuleGeneratedArtifactDrift:       {},
		RuleFileLineLimit:                {},
		RuleFileDeclarationLimit:         {},
		RuleFileBranchLimit:              {},
	}
}

func IsMandatoryBlockingRule(rule string) bool {
	switch strings.TrimSpace(rule) {
	case RuleOperationGrowthBoundary:
		return true
	default:
		return false
	}
}

func EnforceMandatoryBlocking(findings []Finding) {
	for index := range findings {
		finding := &findings[index]
		if finding.Class == FindingProvenViolation && IsMandatoryBlockingRule(finding.Rule) {
			finding.Blocking = true
		}
	}
}

func ApplyBlockingPolicy(findings []Finding, policy QualityPolicy) {
	blocking := stringSet(policy.BlockingRules)
	for index := range findings {
		finding := &findings[index]
		_, enabled := blocking[finding.Rule]
		finding.Blocking = finding.Class == FindingProvenViolation && (enabled || IsMandatoryBlockingRule(finding.Rule))
	}
}

func BlockingNewFindings(report Report) []Finding {
	if report.Debt == nil {
		return []Finding{}
	}
	result := make([]Finding, 0, len(report.Debt.New))
	for _, finding := range report.Debt.New {
		if finding.Blocking {
			result = append(result, cloneFinding(finding))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
