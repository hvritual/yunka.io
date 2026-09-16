package change

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
	"yunka.io/app/cmd/sourceaudit"
)

const (
	QualityMigrationPlanSchemaVersion = 1
	DefaultQualityMigrationPlanPath   = ".git/yunka/quality-migration-plan.json"

	MigrationRecipeGenericContainerSplit = "generic-container-split"
	MigrationRecipeDurableTestRename     = "durable-test-rename"
	MigrationRecipePackageDocumentation  = "package-documentation"
	MigrationRecipeAbstractionSimplify   = "abstraction-simplification"
)

var supportedQualityMigrationRecipes = map[string]struct{}{
	MigrationRecipeGenericContainerSplit: {},
	MigrationRecipeDurableTestRename:     {},
	MigrationRecipePackageDocumentation:  {},
	MigrationRecipeAbstractionSimplify:   {},
}

type QualityMigrationFindingRef struct {
	ID      string                 `json:"id"`
	Rule    string                 `json:"rule"`
	Class   auditcore.FindingClass `json:"class"`
	Path    string                 `json:"path"`
	Symbol  string                 `json:"symbol,omitempty"`
	Summary string                 `json:"summary"`
}

type QualityMigrationCoverage struct {
	ProjectPath     string `json:"projectPath"`
	PolicyPath      string `json:"policyPath"`
	PolicySHA256    string `json:"policySha256"`
	InventorySHA256 string `json:"inventorySha256"`
	Status          string `json:"status"`
}

type QualityMigrationFingerprints struct {
	ProductionSHA256 string `json:"productionSha256"`
	PublicAPISHA256  string `json:"publicApiSha256"`
}

// QualityMigrationPlan is a bounded structural-refactor intent. It references
// existing Audit/source-policy facts and never becomes mutation or merge authority.
type QualityMigrationPlan struct {
	SchemaVersion        int                          `json:"schemaVersion"`
	BaseSHA              string                       `json:"baseSha"`
	Recipes              []string                     `json:"recipes"`
	TouchedPaths         []string                     `json:"touchedPaths"`
	BaselineFindings     []QualityMigrationFindingRef `json:"baselineFindings"`
	Coverage             QualityMigrationCoverage     `json:"coverage"`
	BaselineFingerprints QualityMigrationFingerprints `json:"baselineFingerprints"`
	Narrative            ReviewNarrative              `json:"narrative"`
	PlanSHA256           string                       `json:"planSha256"`
}

type qualityMigrationPlanDigest struct {
	SchemaVersion        int                          `json:"schemaVersion"`
	BaseSHA              string                       `json:"baseSha"`
	Recipes              []string                     `json:"recipes"`
	TouchedPaths         []string                     `json:"touchedPaths"`
	BaselineFindings     []QualityMigrationFindingRef `json:"baselineFindings"`
	Coverage             QualityMigrationCoverage     `json:"coverage"`
	BaselineFingerprints QualityMigrationFingerprints `json:"baselineFingerprints"`
	Narrative            ReviewNarrative              `json:"narrative"`
}

func BuildQualityMigrationPlan(ctx context.Context, options projectflow.Options, base string, recipes, touchedPaths []string, narrative ReviewNarrative) (QualityMigrationPlan, string, error) {
	return BuildQualityMigrationPlanWithCoverage(ctx, options, "", base, recipes, touchedPaths, narrative)
}

// BuildQualityMigrationPlanWithCoverage allows source-policy inventory to run
// from an explicit ancestor of the Yunka project root. This is required for
// consumers whose go.mod intentionally references sibling local modules. The
// plan stores only the project-relative location inside that source root, never
// an absolute workstation path.
func BuildQualityMigrationPlanWithCoverage(ctx context.Context, options projectflow.Options, coverageRoot, base string, recipes, touchedPaths []string, narrative ReviewNarrative) (QualityMigrationPlan, string, error) {
	if ctx == nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: context is required")
	}
	descriptor, err := projectflow.DescribeProject(options)
	if err != nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: resolve project: %w", err)
	}
	if err := ensureCleanWorktree(descriptor.Root); err != nil {
		return QualityMigrationPlan{}, "", err
	}
	baseSHA, err := resolveGitBase(descriptor.Root, base)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	if baseSHA != headSHA {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: base %s must equal clean current HEAD %s; start from the exact migration baseline", baseSHA, headSHA)
	}
	if err := normalizeReviewNarrative(&narrative); err != nil {
		return QualityMigrationPlan{}, "", err
	}
	recipes, err = normalizeQualityMigrationRecipes(recipes)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	paths, err := validateQualityMigrationPaths(descriptor.Root, touchedPaths)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}

	sourceRoot, sourceProjectPath, sourcePolicyPath, err := resolveQualityMigrationCoverage(descriptor.Root, coverageRoot)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	sourceReport, err := sourceaudit.Check(ctx, sourceRoot, sourcePolicyPath)
	if err != nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: source coverage: %w", err)
	}
	if sourceReport.Status != sourceaudit.Pass || !sourceReport.Analysis.Complete || !sourceReport.SourceUnchanged {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: source coverage must be explicit and PASS before structural migration; status=%s", sourceReport.Status)
	}
	auditReport, err := audit.Build(descriptor.Root)
	if err != nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: audit baseline: %w", err)
	}
	if !auditReport.QualityPolicy.Present {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: %s is required; establish the versioned engineering-quality baseline before migration", auditcore.QualityPolicyRelativePath)
	}
	findings := qualityMigrationFindingRefs(auditReport.Findings, paths)
	if err := validateRecipeEvidence(recipes, findings); err != nil {
		return QualityMigrationPlan{}, "", err
	}
	fingerprints, err := qualityMigrationFingerprints(descriptor.Root, paths)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	plan := QualityMigrationPlan{
		SchemaVersion:    QualityMigrationPlanSchemaVersion,
		BaseSHA:          baseSHA,
		Recipes:          recipes,
		TouchedPaths:     paths,
		BaselineFindings: findings,
		Coverage: QualityMigrationCoverage{
			ProjectPath:     sourceProjectPath,
			PolicyPath:      sourcePolicyPath,
			PolicySHA256:    sourceReport.PolicySHA256,
			InventorySHA256: sourceReport.Inventory.Digest,
			Status:          sourceReport.Status,
		},
		BaselineFingerprints: fingerprints,
		Narrative:            narrative,
	}
	normalizeQualityMigrationPlan(&plan)
	plan.PlanSHA256, err = qualityMigrationPlanSHA(plan)
	if err != nil {
		return QualityMigrationPlan{}, "", err
	}
	if err := validateQualityMigrationPlan(plan); err != nil {
		return QualityMigrationPlan{}, "", err
	}
	return plan, descriptor.Root, nil
}

func resolveQualityMigrationCoverage(projectRoot, requestedRoot string) (string, string, string, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", "", err
	}
	sourceRoot := strings.TrimSpace(requestedRoot)
	if sourceRoot == "" {
		sourceRoot = projectRoot
	} else if !filepath.IsAbs(sourceRoot) {
		sourceRoot = filepath.Join(projectRoot, filepath.FromSlash(sourceRoot))
	}
	sourceRoot, err = filepath.Abs(sourceRoot)
	if err != nil {
		return "", "", "", err
	}
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("quality migration plan: coverage root: %w", err)
	}
	if !info.IsDir() {
		return "", "", "", fmt.Errorf("quality migration plan: coverage root %s is not a directory", sourceRoot)
	}
	projectPath, err := filepath.Rel(sourceRoot, projectRoot)
	if err != nil {
		return "", "", "", err
	}
	projectPath = filepath.ToSlash(filepath.Clean(projectPath))
	if projectPath == ".." || strings.HasPrefix(projectPath, "../") || filepath.IsAbs(filepath.FromSlash(projectPath)) {
		return "", "", "", fmt.Errorf("quality migration plan: coverage root must contain the project root")
	}
	if projectPath == "" {
		projectPath = "."
	}
	policyAbsolute := filepath.Join(projectRoot, ".yunka", "source-policy.json")
	policyPath, err := filepath.Rel(sourceRoot, policyAbsolute)
	if err != nil {
		return "", "", "", err
	}
	policyPath = filepath.ToSlash(filepath.Clean(policyPath))
	if policyPath == ".." || strings.HasPrefix(policyPath, "../") || filepath.IsAbs(filepath.FromSlash(policyPath)) {
		return "", "", "", fmt.Errorf("quality migration plan: source policy must remain inside the selected coverage root")
	}
	return sourceRoot, normalizeQualityMigrationProjectPath(projectPath), cleanProjectPath(policyPath), nil
}

func qualityMigrationCoverageRoot(projectRoot, projectPath string) (string, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	projectPath = normalizeQualityMigrationProjectPath(projectPath)
	if !validQualityMigrationProjectPath(projectPath) {
		return "", fmt.Errorf("quality migration coverage: invalid projectPath %q", projectPath)
	}
	if projectPath == "." {
		return projectRoot, nil
	}
	root := projectRoot
	parts := strings.Split(projectPath, "/")
	for range parts {
		root = filepath.Dir(root)
	}
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(projectPath)))
	if candidate != filepath.Clean(projectRoot) {
		return "", fmt.Errorf("quality migration coverage: projectPath %q does not resolve to the current project root", projectPath)
	}
	return root, nil
}

func normalizeQualityMigrationProjectPath(value string) string {
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if value == "" || value == "." {
		return "."
	}
	return strings.TrimPrefix(value, "./")
}

func validQualityMigrationProjectPath(value string) bool {
	value = normalizeQualityMigrationProjectPath(value)
	return value == "." || (value != "" && value != ".." && !strings.HasPrefix(value, "../") && !filepath.IsAbs(filepath.FromSlash(value)))
}

func normalizeQualityMigrationRecipes(values []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := supportedQualityMigrationRecipes[value]; !ok {
			return nil, fmt.Errorf("quality migration plan: unsupported recipe %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("quality migration plan: at least one migration recipe is required")
	}
	sort.Strings(result)
	return result, nil
}

func validateQualityMigrationPaths(root string, values []string) ([]string, error) {
	paths := uniqueSorted(values)
	if len(paths) == 0 {
		return nil, fmt.Errorf("quality migration plan: at least one touched path is required")
	}
	existingParents := map[string]bool{}
	for _, path := range paths {
		path = cleanProjectPath(path)
		if path == "" || path == "." || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
			return nil, fmt.Errorf("quality migration plan: touched path %q is invalid", path)
		}
		report, ownErr := ownership.Build(root, []string{path})
		if ownErr != nil || len(report.Decisions) != 1 || !report.Decisions[0].SafeAutoEdit {
			return nil, fmt.Errorf("quality migration plan: ownership does not prove %s safe for developer editing", path)
		}
		absolute := filepath.Join(root, filepath.FromSlash(path))
		info, err := os.Lstat(absolute)
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("quality migration plan: existing target %s is not a regular file", path)
			}
			existingParents[filepath.ToSlash(filepath.Dir(path))] = true
			continue
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, path := range paths {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if _, err := os.Stat(absolute); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		parent := filepath.ToSlash(filepath.Dir(path))
		if !existingParents[parent] {
			return nil, fmt.Errorf("quality migration plan: new target %s must share a package directory with an existing developer-owned migration target", path)
		}
	}
	return paths, nil
}

func qualityMigrationFindingRefs(findings []auditcore.Finding, touchedPaths []string) []QualityMigrationFindingRef {
	pathSet := make(map[string]struct{}, len(touchedPaths))
	for _, path := range touchedPaths {
		pathSet[cleanProjectPath(path)] = struct{}{}
	}
	result := []QualityMigrationFindingRef{}
	for _, finding := range findings {
		path := cleanProjectPath(finding.Path)
		if path == "" {
			for _, evidence := range finding.Evidence {
				if evidence.Path != "" {
					path = cleanProjectPath(evidence.Path)
					break
				}
		}
		}
		if _, ok := pathSet[path]; !ok {
			continue
		}
		result = append(result, QualityMigrationFindingRef{
			ID: finding.ID, Rule: finding.Rule, Class: finding.Class, Path: path,
			Symbol: finding.Symbol, Summary: finding.Summary,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func validateRecipeEvidence(recipes []string, findings []QualityMigrationFindingRef) error {
	rules := map[string]bool{}
	for _, finding := range findings {
		rules[finding.Rule] = true
	}
	for _, recipe := range recipes {
		switch recipe {
		case MigrationRecipeGenericContainerSplit:
			if !rules[auditcore.RuleGenericContainerCohesion] {
				return fmt.Errorf("quality migration plan: %s requires an in-scope %s baseline observation", recipe, auditcore.RuleGenericContainerCohesion)
			}
		case MigrationRecipeDurableTestRename:
			if !rules[auditcore.RuleHistoricalSourceIdentity] {
				return fmt.Errorf("quality migration plan: %s requires an in-scope %s baseline finding", recipe, auditcore.RuleHistoricalSourceIdentity)
			}
		case MigrationRecipePackageDocumentation:
			if !rules[auditcore.RuleMissingPackageDocumentation] && !rules[auditcore.RuleMissingContractDocumentation] {
				return fmt.Errorf("quality migration plan: %s requires an in-scope documentation baseline finding", recipe)
			}
		case MigrationRecipeAbstractionSimplify:
			// Semantic justification is intentionally not inferred here. The review
			// packet remains advisory and candidate proof must still preserve the
			// declared NONE boundaries or fail closed.
		}
	}
	return nil
}

func normalizeQualityMigrationPlan(plan *QualityMigrationPlan) {
	if plan == nil {
		return
	}
	plan.BaseSHA = strings.TrimSpace(plan.BaseSHA)
	plan.Recipes = uniqueSorted(plan.Recipes)
	plan.TouchedPaths = uniqueSorted(plan.TouchedPaths)
	for i := range plan.BaselineFindings {
		item := &plan.BaselineFindings[i]
		item.ID = strings.TrimSpace(item.ID)
		item.Rule = strings.TrimSpace(item.Rule)
		item.Path = cleanProjectPath(item.Path)
		item.Symbol = strings.TrimSpace(item.Symbol)
		item.Summary = strings.TrimSpace(item.Summary)
	}
	sort.Slice(plan.BaselineFindings, func(i, j int) bool { return plan.BaselineFindings[i].ID < plan.BaselineFindings[j].ID })
	plan.Coverage.ProjectPath = normalizeQualityMigrationProjectPath(plan.Coverage.ProjectPath)
	plan.Coverage.PolicyPath = cleanProjectPath(plan.Coverage.PolicyPath)
	plan.Coverage.PolicySHA256 = strings.TrimSpace(plan.Coverage.PolicySHA256)
	plan.Coverage.InventorySHA256 = strings.TrimSpace(plan.Coverage.InventorySHA256)
	plan.Coverage.Status = strings.TrimSpace(plan.Coverage.Status)
	plan.BaselineFingerprints.ProductionSHA256 = strings.TrimSpace(plan.BaselineFingerprints.ProductionSHA256)
	plan.BaselineFingerprints.PublicAPISHA256 = strings.TrimSpace(plan.BaselineFingerprints.PublicAPISHA256)
	_ = normalizeReviewNarrative(&plan.Narrative)
	plan.PlanSHA256 = strings.TrimSpace(plan.PlanSHA256)
}

func validateQualityMigrationPlan(plan QualityMigrationPlan) error {
	if plan.SchemaVersion != QualityMigrationPlanSchemaVersion {
		return fmt.Errorf("quality migration plan: unsupported schemaVersion %d", plan.SchemaVersion)
	}
	if plan.BaseSHA == "" || len(plan.Recipes) == 0 || len(plan.TouchedPaths) == 0 {
		return fmt.Errorf("quality migration plan: baseSha, recipes and touchedPaths are required")
	}
	if !validQualityMigrationProjectPath(plan.Coverage.ProjectPath) {
		return fmt.Errorf("quality migration plan: valid source coverage projectPath is required")
	}
	expectedPolicyPath := ".yunka/source-policy.json"
	if plan.Coverage.ProjectPath != "." {
		expectedPolicyPath = filepath.ToSlash(filepath.Join(filepath.FromSlash(plan.Coverage.ProjectPath), ".yunka", "source-policy.json"))
	}
	if plan.Coverage.Status != sourceaudit.Pass || plan.Coverage.PolicyPath != expectedPolicyPath || !validSHA256(plan.Coverage.PolicySHA256) || !validSHA256(plan.Coverage.InventorySHA256) {
		return fmt.Errorf("quality migration plan: complete PASS source coverage identity is required")
	}
	if !validSHA256(plan.BaselineFingerprints.ProductionSHA256) || !validSHA256(plan.BaselineFingerprints.PublicAPISHA256) {
		return fmt.Errorf("quality migration plan: baseline fingerprints are invalid")
	}
	narrative := plan.Narrative
	if err := normalizeReviewNarrative(&narrative); err != nil {
		return err
	}
	expected, err := qualityMigrationPlanSHA(plan)
	if err != nil {
		return err
	}
	if plan.PlanSHA256 != expected {
		return fmt.Errorf("quality migration plan: planSha256 mismatch")
	}
	return nil
}

func qualityMigrationPlanSHA(plan QualityMigrationPlan) (string, error) {
	payload := qualityMigrationPlanDigest{
		SchemaVersion: plan.SchemaVersion, BaseSHA: plan.BaseSHA, Recipes: plan.Recipes,
		TouchedPaths: plan.TouchedPaths, BaselineFindings: plan.BaselineFindings,
		Coverage: plan.Coverage, BaselineFingerprints: plan.BaselineFingerprints, Narrative: plan.Narrative,
	}
	contents, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}

func WriteQualityMigrationPlan(root, output string, plan QualityMigrationPlan) (string, error) {
	normalizeQualityMigrationPlan(&plan)
	if err := validateQualityMigrationPlan(plan); err != nil {
		return "", err
	}
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultQualityMigrationPlanPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	contents, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		return "", err
	}
	return display, nil
}

func LoadQualityMigrationPlan(root, input string) (QualityMigrationPlan, string, error) {
	contents, display, err := readGitPrivateState(root, input, DefaultQualityMigrationPlanPath)
	if err != nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: load: %w", err)
	}
	var plan QualityMigrationPlan
	if err := decodeStrictJSON(contents, &plan); err != nil {
		return QualityMigrationPlan{}, "", fmt.Errorf("quality migration plan: decode: %w", err)
	}
	normalizeQualityMigrationPlan(&plan)
	if err := validateQualityMigrationPlan(plan); err != nil {
		return QualityMigrationPlan{}, "", err
	}
	return plan, display, nil
}
