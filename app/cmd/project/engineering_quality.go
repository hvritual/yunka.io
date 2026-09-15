package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/auditcore"
)

const (
	EngineeringQualityBaselineSchemaVersion = 1
	EngineeringQualityPolicyVersion          = "yunka.engineering-quality/v1"
	EngineeringQualityProvenance             = "yunka:docs/ENGINEERING_QUALITY_RULES.md"
	EngineeringQualityBaselineRelativePath   = ".yunka/engineering-quality-baseline.json"
	EngineeringQualityRulesRelativePath      = ".yunka/ENGINEERING_QUALITY.md"
	EngineeringQualityInstructionsPath       = "AGENTS.md"
	EngineeringQualityUpgradePolicy          = "explicit-review-required"
)

// EngineeringQualityBaseline identifies the durable consumer-facing rule set.
// It intentionally does not contain framework status, task history, or a copy of
// consumer-owned source/configuration facts.
type EngineeringQualityBaseline struct {
	SchemaVersion           int    `json:"schemaVersion"`
	PolicyVersion           string `json:"policyVersion"`
	PolicyIdentity          string `json:"policyIdentity"`
	Provenance              string `json:"provenance"`
	RulesPath               string `json:"rulesPath"`
	CanonicalRulesSHA256    string `json:"canonicalRulesSha256"`
	EnforcementPolicyPath   string `json:"enforcementPolicyPath"`
	ProjectInstructionsPath string `json:"projectInstructionsPath"`
	UpgradePolicy           string `json:"upgradePolicy"`
}

type EngineeringQualityInstallReport struct {
	Baseline        string
	Rules           string
	Policy          string
	Instructions    string
	PolicyVersion   string
	PolicyIdentity  string
	UpgradeRequired string
	Skipped         string
}

func ensureEngineeringQualityBaseline(root string, hasGoModule bool) (EngineeringQualityInstallReport, error) {
	report := EngineeringQualityInstallReport{
		PolicyVersion:  EngineeringQualityPolicyVersion,
		PolicyIdentity: engineeringQualityPolicyIdentity(),
	}
	if !hasGoModule {
		report.Skipped = "go.mod is required before the engineering-quality baseline can be installed"
		return report, nil
	}
	absolute, err := absoluteRoot(root)
	if err != nil {
		return EngineeringQualityInstallReport{}, err
	}

	baselinePath := filepath.Join(absolute, filepath.FromSlash(EngineeringQualityBaselineRelativePath))
	if contents, err := os.ReadFile(baselinePath); err == nil {
		existing, decodeErr := decodeEngineeringQualityBaseline(contents)
		if decodeErr != nil {
			return EngineeringQualityInstallReport{}, fmt.Errorf("engineering quality baseline: %w", decodeErr)
		}
		report.Baseline = EngineeringQualityBaselineRelativePath
		if existing.PolicyIdentity != report.PolicyIdentity || existing.PolicyVersion != EngineeringQualityPolicyVersion {
			report.UpgradeRequired = fmt.Sprintf("existing baseline version=%s identity=%s preserved; canonical version=%s identity=%s requires explicit review", existing.PolicyVersion, existing.PolicyIdentity, EngineeringQualityPolicyVersion, report.PolicyIdentity)
			return report, nil
		}
	} else if !os.IsNotExist(err) {
		return EngineeringQualityInstallReport{}, err
	}

	rules := engineeringQualityRulesBytes()
	rulesPath := filepath.Join(absolute, filepath.FromSlash(EngineeringQualityRulesRelativePath))
	rulesInstalled, err := ensureCanonicalScaffoldFile(rulesPath, rules)
	if err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	if !rulesInstalled {
		report.Rules = EngineeringQualityRulesRelativePath
		report.UpgradeRequired = fmt.Sprintf("%s contains consumer-owned edits; preserved without replacement; explicit reconciliation with %s is required", EngineeringQualityRulesRelativePath, EngineeringQualityPolicyVersion)
		return report, nil
	}
	report.Rules = EngineeringQualityRulesRelativePath

	policyPath := filepath.Join(absolute, filepath.FromSlash(auditcore.QualityPolicyRelativePath))
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		contents, buildErr := defaultEngineeringQualityPolicyBytes()
		if buildErr != nil {
			return EngineeringQualityInstallReport{}, buildErr
		}
		if err := writeIfMissing(policyPath, contents); err != nil {
			return EngineeringQualityInstallReport{}, err
		}
	} else if err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	if _, _, err := auditcore.LoadQualityPolicy(absolute); err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	report.Policy = auditcore.QualityPolicyRelativePath

	instructionsPath := filepath.Join(absolute, EngineeringQualityInstructionsPath)
	if err := writeIfMissing(instructionsPath, engineeringQualityAgentInstructionsBytes()); err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	report.Instructions = EngineeringQualityInstructionsPath

	if report.Baseline == "" {
		contents, buildErr := engineeringQualityBaselineBytes()
		if buildErr != nil {
			return EngineeringQualityInstallReport{}, buildErr
		}
		if err := writeIfMissing(baselinePath, contents); err != nil {
			return EngineeringQualityInstallReport{}, err
		}
		report.Baseline = EngineeringQualityBaselineRelativePath
	}
	return report, nil
}

func engineeringQualityBaselineBytes() ([]byte, error) {
	baseline := EngineeringQualityBaseline{
		SchemaVersion:           EngineeringQualityBaselineSchemaVersion,
		PolicyVersion:           EngineeringQualityPolicyVersion,
		PolicyIdentity:          engineeringQualityPolicyIdentity(),
		Provenance:              EngineeringQualityProvenance,
		RulesPath:               EngineeringQualityRulesRelativePath,
		CanonicalRulesSHA256:    engineeringQualityDigest(engineeringQualityRulesBytes()),
		EnforcementPolicyPath:   auditcore.QualityPolicyRelativePath,
		ProjectInstructionsPath: EngineeringQualityInstructionsPath,
		UpgradePolicy:           EngineeringQualityUpgradePolicy,
	}
	contents, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func decodeEngineeringQualityBaseline(contents []byte) (EngineeringQualityBaseline, error) {
	var baseline EngineeringQualityBaseline
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&baseline); err != nil {
		return EngineeringQualityBaseline{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return EngineeringQualityBaseline{}, fmt.Errorf("trailing JSON value is not allowed")
		}
		return EngineeringQualityBaseline{}, err
	}
	if baseline.SchemaVersion != EngineeringQualityBaselineSchemaVersion {
		return EngineeringQualityBaseline{}, fmt.Errorf("unsupported schemaVersion %d", baseline.SchemaVersion)
	}
	if strings.TrimSpace(baseline.PolicyVersion) == "" || strings.TrimSpace(baseline.PolicyIdentity) == "" || strings.TrimSpace(baseline.Provenance) == "" {
		return EngineeringQualityBaseline{}, fmt.Errorf("policyVersion, policyIdentity, and provenance are required")
	}
	return baseline, nil
}

func engineeringQualityPolicyIdentity() string {
	payload := strings.Join([]string{
		EngineeringQualityPolicyVersion,
		EngineeringQualityProvenance,
		engineeringQualityDigest(engineeringQualityRulesBytes()),
	}, "\n")
	return engineeringQualityDigest([]byte(payload))
}

func defaultEngineeringQualityPolicyBytes() ([]byte, error) {
	blocking := []string{
		auditcore.RuleHistoricalSourceIdentity,
		auditcore.RuleMissingPackageDocumentation,
		auditcore.RuleMissingContractDocumentation,
		auditcore.RuleCrossDomainRepositoryBypass,
		auditcore.RulePlatformProviderBypass,
		auditcore.RuleAuthorizationBypass,
		auditcore.RuleGeneratedOwnershipMix,
		auditcore.RuleStaleGeneratedArtifact,
		auditcore.RuleGeneratedArtifactDrift,
	}
	sort.Strings(blocking)
	policy := auditcore.QualityPolicy{
		SchemaVersion: auditcore.QualityPolicySchemaVersion,
		BlockingRules: blocking,
	}
	if err := auditcore.ValidateQualityPolicy(policy); err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func ensureCanonicalScaffoldFile(path string, canonical []byte) (bool, error) {
	contents, err := os.ReadFile(path)
	if err == nil {
		return bytes.Equal(contents, canonical), nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	if err := writeIfMissing(path, canonical); err != nil {
		return false, err
	}
	contents, err = os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(contents, canonical), nil
}

func engineeringQualityDigest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func engineeringQualityRulesBytes() []byte {
	return []byte(`# Yunka Engineering Quality Baseline

Policy version: yunka.engineering-quality/v1
Provenance: yunka:docs/ENGINEERING_QUALITY_RULES.md

This is the self-contained consumer rule baseline installed by Yunka. Framework
release status, issue history, delivery waves and task identifiers are not policy
sources for this project.

## Normative rules

1. Production package, file, symbol and durable test identities describe domain or technical responsibility, not delivery history.
2. Core packages document responsibility, important invariants and intentional exclusions. Comments explain constraints, side effects and non-obvious decisions instead of restating syntax.
3. Generated and developer-owned semantics remain separate. Generated output is repaired through its canonical generator, never by hiding handwritten behavior in generator scope.
4. New abstractions require a real boundary, domain meaning, dependency-isolation purpose, independent lifecycle/policy, or multiple implementations.
5. Non-trivial AI changes expose Problem, current responsibility, desired ownership, behavior/API/persistence/generated deltas, verification, and WHY / WHAT / BOUNDARY / PROOF before human approval.
6. Deterministic checks own objectively provable facts. Semantic AI review is advisory and cannot grant mutation, merge, deployment or business-correctness authority.
7. Engineering-quality findings are compared against an immutable baseline. Historical debt stays visible; only explicitly blocking new deterministic debt may fail conformance, subject only to an exact, owned, expiring waiver.
8. Source ownership, project configuration, canonical contracts and generated ownership remain the enforcement inputs. This baseline does not create a parallel business or architecture source of truth.

## Local enforcement

- .yunka/engineering-quality.json configures accepted deterministic blocking rules and optional review budgets.
- yunka audit --base <immutable-ref> reports deterministic existing/new/fixed debt.
- yunka change review projects exact-candidate WHY / WHAT / BOUNDARY / PROOF and quality debt.
- yunka advisor semantic validates evidence-bound semantic findings as advisory-only.

## Ownership and upgrades

Files created by yunka init are never silently replaced on a later init. After
creation, project-specific enforcement configuration is developer-owned. A newer
Yunka policy version requires explicit review/reconciliation; it must not silently
overwrite project instructions, rule text, or enforcement policy.
`)
}

func engineeringQualityAgentInstructionsBytes() []byte {
	return []byte(`# Repository Instructions

Before creating, renaming, refactoring, reviewing, or generating source code or durable tests:

1. Read .yunka/ENGINEERING_QUALITY.md completely.
2. Treat .yunka/engineering-quality-baseline.json as the policy version/provenance identity.
3. Preserve existing project configuration, source ownership, canonical contracts and generated ownership as enforcement inputs.
4. Do not infer current project requirements from Yunka framework issue history, release status, delivery waves or task identifiers.
5. For non-trivial AI changes, make WHY / WHAT / BOUNDARY / PROOF and behavior/API/persistence/generated deltas reviewable before approval.

.yunka/ENGINEERING_QUALITY.md is normative engineering guidance for this consumer. Framework implementation history is not consumer policy.
`)
}
