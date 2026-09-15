package change

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/projectflow"
	"yunka.io/app/cmd/sourceaudit"
)

const QualityMigrationReviewSchemaVersion = 1

type QualityMigrationReviewPacket struct {
	SchemaVersion       int                          `json:"schemaVersion"`
	BaseSHA             string                       `json:"baseSha"`
	HeadSHA             string                       `json:"headSha"`
	PlanSHA256          string                       `json:"planSha256"`
	Recipes             []string                     `json:"recipes"`
	TouchedPaths        []string                     `json:"touchedPaths"`
	ChangedPaths        []string                     `json:"changedPaths"`
	BaselineFindings    []QualityMigrationFindingRef `json:"baselineFindings"`
	BehaviorChange      ReviewDelta                  `json:"behaviorChange"`
	PublicAPIChange     ReviewDelta                  `json:"publicApiChange"`
	PersistenceChange   ReviewDelta                  `json:"persistenceChange"`
	GeneratedCodeChange ReviewDelta                  `json:"generatedCodeChange"`
	QualityDebt         *ReviewQualityDebt           `json:"qualityDebt,omitempty"`
	Coverage            QualityMigrationCoverage     `json:"coverage"`
	Narrative           ReviewNarrative              `json:"narrative"`
	Projection          ReviewProjection             `json:"whyWhatBoundaryProof"`
	Conformant          bool                         `json:"conformant"`
	Violations          []string                     `json:"violations"`
	ProofSHA256         string                       `json:"proofSha256"`
}

type qualityMigrationReviewDigest struct {
	SchemaVersion       int                          `json:"schemaVersion"`
	BaseSHA             string                       `json:"baseSha"`
	HeadSHA             string                       `json:"headSha"`
	PlanSHA256          string                       `json:"planSha256"`
	Recipes             []string                     `json:"recipes"`
	TouchedPaths        []string                     `json:"touchedPaths"`
	ChangedPaths        []string                     `json:"changedPaths"`
	BaselineFindings    []QualityMigrationFindingRef `json:"baselineFindings"`
	BehaviorChange      ReviewDelta                  `json:"behaviorChange"`
	PublicAPIChange     ReviewDelta                  `json:"publicApiChange"`
	PersistenceChange   ReviewDelta                  `json:"persistenceChange"`
	GeneratedCodeChange ReviewDelta                  `json:"generatedCodeChange"`
	QualityDebt         *ReviewQualityDebt           `json:"qualityDebt,omitempty"`
	Coverage            QualityMigrationCoverage     `json:"coverage"`
	Narrative           ReviewNarrative              `json:"narrative"`
	Projection          ReviewProjection             `json:"whyWhatBoundaryProof"`
	Conformant          bool                         `json:"conformant"`
	Violations          []string                     `json:"violations"`
}

func CheckQualityMigration(ctx context.Context, options projectflow.Options, planInput string) (QualityMigrationReviewPacket, error) {
	if ctx == nil {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: context is required")
	}
	descriptor, err := projectflow.DescribeProject(options)
	if err != nil {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: resolve project: %w", err)
	}
	plan, _, err := LoadQualityMigrationPlan(descriptor.Root, planInput)
	if err != nil {
		return QualityMigrationReviewPacket{}, err
	}
	headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
	if err != nil {
		return QualityMigrationReviewPacket{}, err
	}
	envelope := ChangeContract{
		SchemaVersion: ChangeContractSchemaVersion,
		BaseSHA:       plan.BaseSHA,
		Intent:        "quality-migration",
		Operation:     ChangeOperation{OperationID: "quality-migration"},
		EditablePaths: append([]string(nil), plan.TouchedPaths...),
	}
	normalizeChangeContract(&envelope)
	reconciliation, err := reconcileGitDeltaFiles(descriptor.Root, envelope)
	if err != nil {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: Git reconciliation: %w", err)
	}

	sourceReport, err := sourceaudit.Check(ctx, descriptor.Root, plan.Coverage.PolicyPath)
	if err != nil {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: source coverage: %w", err)
	}
	auditReport, err := audit.BuildWithBase(descriptor.Root, plan.BaseSHA)
	if err != nil {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: engineering-quality debt: %w", err)
	}
	if !auditReport.QualityPolicy.Present {
		return QualityMigrationReviewPacket{}, fmt.Errorf("quality migration check: %s disappeared from the candidate", auditcore.QualityPolicyRelativePath)
	}
	debt := *auditReport.Debt
	qualityProof, err := BuildQualityDebtProof(debt, nil, nil, plan.BaseSHA, headSHA, time.Unix(0, 0).UTC())
	if err != nil {
		return QualityMigrationReviewPacket{}, err
	}
	currentFingerprints, err := qualityMigrationFingerprints(descriptor.Root, plan.TouchedPaths)
	if err != nil {
		return QualityMigrationReviewPacket{}, err
	}

	packet := QualityMigrationReviewPacket{
		SchemaVersion:    QualityMigrationReviewSchemaVersion,
		BaseSHA:          plan.BaseSHA,
		HeadSHA:          headSHA,
		PlanSHA256:       plan.PlanSHA256,
		Recipes:          append([]string(nil), plan.Recipes...),
		TouchedPaths:     append([]string(nil), plan.TouchedPaths...),
		ChangedPaths:     migrationChangedPaths(reconciliation.Changes),
		BaselineFindings: append([]QualityMigrationFindingRef(nil), plan.BaselineFindings...),
		BehaviorChange: migrationFingerprintDelta(
			"production-syntax", plan.BaselineFingerprints.ProductionSHA256, currentFingerprints.ProductionSHA256,
		),
		PublicAPIChange: migrationFingerprintDelta(
			"public-api-syntax", plan.BaselineFingerprints.PublicAPISHA256, currentFingerprints.PublicAPISHA256,
		),
		PersistenceChange: migrationPathDelta("persistence", reconciliation.Changes, func(change FileChange) bool {
			return isPersistenceReviewPath(change.Path) || isPersistenceReviewPath(change.PreviousPath)
		}),
		GeneratedCodeChange: migrationPathDelta("generated", reconciliation.Changes, func(change FileChange) bool {
			return strings.EqualFold(strings.TrimSpace(change.Class), "generated") || migrationGeneratedPath(descriptor.GeneratedGoRoot, change.Path) || migrationGeneratedPath(descriptor.GeneratedGoRoot, change.PreviousPath)
		}),
		QualityDebt: reviewQualityDebt(&qualityProof),
		Coverage: QualityMigrationCoverage{
			PolicyPath: plan.Coverage.PolicyPath, PolicySHA256: sourceReport.PolicySHA256,
			InventorySHA256: sourceReport.Inventory.Digest, Status: sourceReport.Status,
		},
		Narrative: plan.Narrative,
		Violations: []string{},
	}

	for _, violation := range reconciliation.Violations {
		packet.Violations = append(packet.Violations, "scope:"+cleanProjectPath(violation.Path)+":"+strings.TrimSpace(violation.Kind))
	}
	if sourceReport.Status != sourceaudit.Pass || !sourceReport.Analysis.Complete || !sourceReport.SourceUnchanged {
		packet.Violations = append(packet.Violations, "coverage:candidate source coverage is not PASS and complete")
	}
	if sourceReport.PolicySHA256 != plan.Coverage.PolicySHA256 {
		packet.Violations = append(packet.Violations, "coverage:source policy changed during the migration; establish coverage separately before refactoring")
	}
	if len(qualityProof.UnwaivedBlocking) > 0 {
		for _, finding := range qualityProof.UnwaivedBlocking {
			packet.Violations = append(packet.Violations, "quality-debt:new blocking finding "+finding.ID)
		}
	}
	if packet.BehaviorChange.State != ReviewDeltaNone {
		packet.Violations = append(packet.Violations, "behavior:production syntax changed; this bounded structural migration cannot prove behavior delta NONE")
	}
	if packet.PublicAPIChange.State != ReviewDeltaNone {
		packet.Violations = append(packet.Violations, "public-api:exported syntax changed; public API delta is not NONE")
	}
	if packet.PersistenceChange.State != ReviewDeltaNone {
		packet.Violations = append(packet.Violations, "persistence:persistence paths changed")
	}
	if packet.GeneratedCodeChange.State != ReviewDeltaNone {
		packet.Violations = append(packet.Violations, "generated:generator-owned paths changed")
	}
	if len(packet.ChangedPaths) == 0 {
		packet.Violations = append(packet.Violations, "scope:no candidate migration changes were found")
	}
	if !migrationFixesOrPreservesDebt(plan, debt) {
		packet.Violations = append(packet.Violations, "quality-debt:the candidate neither fixes an inventoried finding nor preserves a documentation-only migration target")
	}

	packet.Violations = uniqueSortedReviewText(packet.Violations)
	packet.Conformant = len(packet.Violations) == 0
	packet.Projection = ReviewProjection{
		Why:      plan.Narrative.Why,
		What:     plan.Narrative.What,
		Boundary: plan.Narrative.Boundary,
		Proof: qualityMigrationProofLines(packet, currentFingerprints),
	}
	normalizeQualityMigrationReview(&packet)
	packet.ProofSHA256, err = qualityMigrationReviewSHA(packet)
	if err != nil {
		return QualityMigrationReviewPacket{}, err
	}
	if err := validateQualityMigrationReview(packet); err != nil {
		return QualityMigrationReviewPacket{}, err
	}
	return packet, nil
}

func migrationFixesOrPreservesDebt(plan QualityMigrationPlan, debt auditcore.DebtDelta) bool {
	baseline := map[string]struct{}{}
	for _, finding := range plan.BaselineFindings {
		baseline[finding.ID] = struct{}{}
	}
	if len(baseline) == 0 {
		for _, recipe := range plan.Recipes {
			if recipe == MigrationRecipeAbstractionSimplify {
				return true
			}
		}
		return false
	}
	for _, finding := range debt.Fixed {
		if _, ok := baseline[finding.ID]; ok {
			return true
		}
	}
	for _, recipe := range plan.Recipes {
		if recipe == MigrationRecipePackageDocumentation {
			return false
		}
	}
	// Generic-container observations are advisory and excluded from DebtDelta.
	// Their bounded improvement is proven by scope + unchanged production syntax.
	for _, finding := range plan.BaselineFindings {
		if finding.Rule == auditcore.RuleGenericContainerCohesion {
			return true
		}
	}
	return false
}

func migrationChangedPaths(values []FileChange) []string {
	result := []string{}
	for _, value := range values {
		if value.Path != "" {
			result = append(result, cleanProjectPath(value.Path))
		}
		if value.PreviousPath != "" {
			result = append(result, cleanProjectPath(value.PreviousPath))
		}
	}
	return uniqueSorted(result)
}

func migrationFingerprintDelta(kind, before, after string) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	if before != after {
		delta.Facts = append(delta.Facts, ReviewFact{Kind: kind, Detail: "sha256 " + before + " -> " + after})
	}
	normalizeReviewDelta(&delta)
	return delta
}

func migrationPathDelta(kind string, values []FileChange, matches func(FileChange) bool) ReviewDelta {
	delta := ReviewDelta{Facts: []ReviewFact{}}
	for _, value := range values {
		if !matches(value) {
			continue
		}
		delta.Facts = append(delta.Facts, ReviewFact{Kind: kind, Path: cleanProjectPath(value.Path), Detail: reviewFileChangeDetail(value)})
	}
	normalizeReviewDelta(&delta)
	return delta
}

func migrationGeneratedPath(generatedRoot, value string) bool {
	generatedRoot = cleanProjectPath(generatedRoot)
	value = cleanProjectPath(value)
	return generatedRoot != "" && value != "" && (value == generatedRoot || strings.HasPrefix(value, generatedRoot+"/"))
}

func qualityMigrationProofLines(packet QualityMigrationReviewPacket, current QualityMigrationFingerprints) []string {
	proof := []string{
		"base=" + packet.BaseSHA,
		"head=" + packet.HeadSHA,
		"plan-sha256=" + packet.PlanSHA256,
		"coverage-policy-sha256=" + packet.Coverage.PolicySHA256,
		"coverage-inventory-sha256=" + packet.Coverage.InventorySHA256,
		"production-syntax-sha256=" + current.ProductionSHA256,
		"public-api-sha256=" + current.PublicAPISHA256,
		fmt.Sprintf("debt:existing=%d new=%d fixed=%d blocking-new=%d", packet.QualityDebt.DeterministicExisting, packet.QualityDebt.DeterministicNew, packet.QualityDebt.DeterministicFixed, packet.QualityDebt.BlockingNew),
	}
	return uniqueSortedReviewText(proof)
}

func normalizeQualityMigrationReview(packet *QualityMigrationReviewPacket) {
	if packet == nil {
		return
	}
	packet.BaseSHA = strings.TrimSpace(packet.BaseSHA)
	packet.HeadSHA = strings.TrimSpace(packet.HeadSHA)
	packet.PlanSHA256 = strings.TrimSpace(packet.PlanSHA256)
	packet.Recipes = uniqueSorted(packet.Recipes)
	packet.TouchedPaths = uniqueSorted(packet.TouchedPaths)
	packet.ChangedPaths = uniqueSorted(packet.ChangedPaths)
	normalizeReviewDelta(&packet.BehaviorChange)
	normalizeReviewDelta(&packet.PublicAPIChange)
	normalizeReviewDelta(&packet.PersistenceChange)
	normalizeReviewDelta(&packet.GeneratedCodeChange)
	normalizeReviewQualityDebt(packet.QualityDebt)
	_ = normalizeReviewNarrative(&packet.Narrative)
	packet.Projection.Why = strings.TrimSpace(packet.Projection.Why)
	packet.Projection.What = strings.TrimSpace(packet.Projection.What)
	packet.Projection.Boundary = strings.TrimSpace(packet.Projection.Boundary)
	packet.Projection.Proof = uniqueSortedReviewText(packet.Projection.Proof)
	packet.Violations = uniqueSortedReviewText(packet.Violations)
	packet.ProofSHA256 = strings.TrimSpace(packet.ProofSHA256)
}

func validateQualityMigrationReview(packet QualityMigrationReviewPacket) error {
	if packet.SchemaVersion != QualityMigrationReviewSchemaVersion {
		return fmt.Errorf("quality migration review: unsupported schemaVersion %d", packet.SchemaVersion)
	}
	if packet.BaseSHA == "" || packet.HeadSHA == "" || !validSHA256(packet.PlanSHA256) {
		return fmt.Errorf("quality migration review: exact candidate identity is incomplete")
	}
	if packet.Projection.Why != packet.Narrative.Why || packet.Projection.What != packet.Narrative.What || packet.Projection.Boundary != packet.Narrative.Boundary {
		return fmt.Errorf("quality migration review: WHY/WHAT/BOUNDARY projection differs from plan narrative")
	}
	if err := validateReviewQualityDebt(packet.QualityDebt); err != nil {
		return err
	}
	for name, delta := range map[string]ReviewDelta{
		"behavior": packet.BehaviorChange, "publicApi": packet.PublicAPIChange,
		"persistence": packet.PersistenceChange, "generated": packet.GeneratedCodeChange,
	} {
		if delta.State != ReviewDeltaNone && delta.State != ReviewDeltaChanged {
			return fmt.Errorf("quality migration review: %s delta state %q is unsupported", name, delta.State)
		}
	}
	expected, err := qualityMigrationReviewSHA(packet)
	if err != nil {
		return err
	}
	if packet.ProofSHA256 != expected {
		return fmt.Errorf("quality migration review: proofSha256 mismatch")
	}
	return nil
}

func qualityMigrationReviewSHA(packet QualityMigrationReviewPacket) (string, error) {
	payload := qualityMigrationReviewDigest{
		SchemaVersion: packet.SchemaVersion, BaseSHA: packet.BaseSHA, HeadSHA: packet.HeadSHA,
		PlanSHA256: packet.PlanSHA256, Recipes: packet.Recipes, TouchedPaths: packet.TouchedPaths,
		ChangedPaths: packet.ChangedPaths, BaselineFindings: packet.BaselineFindings,
		BehaviorChange: packet.BehaviorChange, PublicAPIChange: packet.PublicAPIChange,
		PersistenceChange: packet.PersistenceChange, GeneratedCodeChange: packet.GeneratedCodeChange,
		QualityDebt: packet.QualityDebt, Coverage: packet.Coverage, Narrative: packet.Narrative,
		Projection: packet.Projection, Conformant: packet.Conformant, Violations: packet.Violations,
	}
	contents, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}

func qualityMigrationFingerprints(root string, touchedPaths []string) (QualityMigrationFingerprints, error) {
	directories := map[string]struct{}{}
	for _, path := range touchedPaths {
		directories[filepath.ToSlash(filepath.Dir(cleanProjectPath(path)))] = struct{}{}
	}
	var production, publicAPI []string
	for directory := range directories {
		absolute := filepath.Join(root, filepath.FromSlash(directory))
		entries, err := os.ReadDir(absolute)
		if err != nil {
			return QualityMigrationFingerprints{}, fmt.Errorf("quality migration fingerprint: read %s: %w", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(absolute, entry.Name())
			contents, err := os.ReadFile(path)
			if err != nil {
				return QualityMigrationFingerprints{}, err
			}
			withComments, err := parser.ParseFile(token.NewFileSet(), path, contents, parser.ParseComments)
			if err != nil {
				return QualityMigrationFingerprints{}, fmt.Errorf("quality migration fingerprint: parse %s: %w", path, err)
			}
			if ast.IsGenerated(withComments) || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path, contents, 0)
			if err != nil {
				return QualityMigrationFingerprints{}, err
			}
			for _, importSpec := range file.Imports {
				value, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil {
					return QualityMigrationFingerprints{}, err
				}
				production = append(production, "import:"+value)
			}
			for _, declaration := range file.Decls {
				rendered, err := renderMigrationDeclaration(set, declaration, false)
				if err != nil {
					return QualityMigrationFingerprints{}, err
				}
				production = append(production, file.Name.Name+":"+rendered)
				if migrationDeclarationExported(declaration) {
					signature, err := renderMigrationDeclaration(set, declaration, true)
					if err != nil {
						return QualityMigrationFingerprints{}, err
					}
					publicAPI = append(publicAPI, file.Name.Name+":"+signature)
				}
			}
		}
	}
	return QualityMigrationFingerprints{
		ProductionSHA256: migrationStringsSHA(production),
		PublicAPISHA256:  migrationStringsSHA(publicAPI),
	}, nil
}

func renderMigrationDeclaration(set *token.FileSet, declaration ast.Decl, signatureOnly bool) (string, error) {
	var node any = declaration
	if signatureOnly {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			copyFunction := *function
			copyFunction.Body = nil
			copyFunction.Doc = nil
			node = &copyFunction
		}
	}
	var builder strings.Builder
	if err := printer.Fprint(&builder, set, node); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func migrationDeclarationExported(declaration ast.Decl) bool {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		return value.Name != nil && value.Name.IsExported()
	case *ast.GenDecl:
		for _, spec := range value.Specs {
			switch item := spec.(type) {
			case *ast.TypeSpec:
				if item.Name != nil && item.Name.IsExported() {
					return true
				}
			case *ast.ValueSpec:
				for _, name := range item.Names {
					if name != nil && name.IsExported() {
						return true
					}
				}
			}
		}
	}
	return false
}

func migrationStringsSHA(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	hash := sha256.New()
	for _, value := range values {
		hash.Write([]byte(value))
		hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
