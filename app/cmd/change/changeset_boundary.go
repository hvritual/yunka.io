package change

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

const ChangeSetBoundaryPolicy = "independent-canonical-additions/v1"

// CreateBoundaryProof retains the complete reviewed plan, including supporting
// AND counter-evidence. It is derived task evidence, not a writable taxonomy or
// a signature. Check must reconstruct the decision from current canonical facts.
type CreateBoundaryProof struct {
	SchemaVersion    int        `json:"schemaVersion"`
	BaseInputsDigest string     `json:"baseInputsDigest"`
	Plan             add.Report `json:"plan"`
}

type ChangeSetBoundaryEntry struct {
	OperationID    string `json:"operationId"`
	DecisionDigest string `json:"decisionDigest"`
	Outcome        string `json:"outcome"`
	Validated      bool   `json:"validated"`
}

type ChangeSetBoundaryReport struct {
	SchemaVersion        int                      `json:"schemaVersion"`
	Policy               string                   `json:"policy"`
	BaseSHA              string                   `json:"baseSha"`
	BeforeManifestDigest string                   `json:"beforeManifestDigest"`
	AfterManifestDigest  string                   `json:"afterManifestDigest"`
	InputsDigest         string                   `json:"inputsDigest"`
	Entries              []ChangeSetBoundaryEntry `json:"entries"`
	Violations           []ChangeViolation        `json:"violations"`
}

func staleBoundary(format string, args ...any) error {
	return fmt.Errorf("%w: %s", boundarycore.ErrStaleBoundaryProof, fmt.Sprintf(format, args...))
}

func isBoundaryDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// Static checks establish internal binding only. They cannot substitute for
// RevalidateAddition against independently compiled immutable/current inputs.
func validateCreateBoundaryProof(base string, value CreateOperationChange) error {
	proof := value.BoundaryProof
	if proof == nil || proof.SchemaVersion != 1 || !isBoundaryDigest(proof.BaseInputsDigest) {
		return staleBoundary("create %s requires a complete current boundary proof; replan", value.Operation.OperationID)
	}
	plan := proof.Plan
	decision := plan.BoundaryDecision
	if plan.SchemaVersion != add.OperationReportVersion || plan.Kind != "operation-plan" || plan.BaseSHA != base || !isBoundaryDigest(plan.InputsDigest) || decision == nil {
		return staleBoundary("create %s has incompatible plan/base evidence", value.Operation.OperationID)
	}
	if decision.BaseSHA != base || decision.OperationID != value.Operation.OperationID || decision.TargetApplication != value.Operation.Domain+"/"+value.Operation.Application || decision.PolicyVersion != boundarycore.AdditionPolicyVersion || decision.Authority != "read_only" || decision.Outcome != boundarycore.ReuseExistingApplication {
		return staleBoundary("create %s has missing, incompatible or non-reuse decision", value.Operation.OperationID)
	}
	expected, err := createOperationChange(plan)
	if err != nil {
		return staleBoundary("cannot reconstruct create plan binding: %v", err)
	}
	if expected.PlanDigest != value.PlanDigest || expected.Operation != value.Operation || jsonValue(expected.Expected) != jsonValue(value.Expected) || jsonValue(uniqueSorted(expected.EditablePaths)) != jsonValue(uniqueSorted(value.EditablePaths)) || jsonValue(uniqueSorted(expected.GeneratedScopes)) != jsonValue(uniqueSorted(value.GeneratedScopes)) {
		return staleBoundary("create %s differs from its bound full plan", value.Operation.OperationID)
	}
	return nil
}

// A create proof is issued only when the immutable Git baseline really has the
// captured compiler inputs. Ignored/uncommitted inputs cannot be promoted into
// an earlier commit's architectural evidence by copying its SHA into a plan.
func bindCreateBoundaryBase(root, base string, create *CreateOperationChange) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plan := create.BoundaryProof.Plan
	options := projectflow.Options{Root: root, ProtoPaths: plan.ProtoPaths}
	before, after, err := captureBoundaryPair(ctx, options, base)
	if err != nil {
		return staleBoundary("cannot bind immutable base: %v", err)
	}
	if before.ContentDigest != after.ContentDigest || after.InputsDigest != plan.InputsDigest || boundarycore.ManifestDigest(before.Manifest) != plan.BoundaryDecision.BeforeManifestDigest {
		return staleBoundary("plan compiler inputs or canonical before-model do not equal the immutable ChangeSet base")
	}
	head, err := resolveGitBase(root, "HEAD")
	if err != nil || head != base {
		return staleBoundary("HEAD changed while binding the create proof")
	}
	create.BoundaryProof.BaseInputsDigest = before.ContentDigest
	return validateCreateBoundaryProof(base, *create)
}

func captureBoundaryPair(ctx context.Context, options projectflow.Options, base string) (projectflow.ContractInputSnapshot, projectflow.ContractInputSnapshot, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return projectflow.ContractInputSnapshot{}, projectflow.ContractInputSnapshot{}, err
	}
	options.Root = root
	baseRoot, cleanup, err := materializeSourceBase(ctx, root, base)
	if err != nil {
		return projectflow.ContractInputSnapshot{}, projectflow.ContractInputSnapshot{}, err
	}
	defer cleanup()
	baseOptions := options
	baseOptions.Root = baseRoot
	baseOptions.ProtoPaths = snapshotIncludePaths(root, baseRoot, options.ProtoPaths)
	before, err := projectflow.CaptureContractSnapshot(ctx, baseOptions)
	if err != nil {
		return before, projectflow.ContractInputSnapshot{}, fmt.Errorf("immutable boundary input: %w", err)
	}
	after, err := projectflow.CaptureContractSnapshot(ctx, options)
	if err != nil {
		return before, after, fmt.Errorf("current boundary input: %w", err)
	}
	return before, after, nil
}

// Compiler include paths are caller inputs, not commands to execute from a saved
// proof. Check requires the same ordered profile and never silently adopts an
// untrusted serialized external directory.
func boundaryIncludeIdentity(root string, paths []string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return nil, staleBoundary("blank compiler include")
		}
		absolute := path
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(root, path)
		}
		absolute = filepath.Clean(absolute)
		rel, err := filepath.Rel(root, absolute)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			result = append(result, "project/"+filepath.ToSlash(rel))
		} else {
			result = append(result, "external/"+filepath.ToSlash(absolute))
		}
	}
	return result, nil
}

// Every declared addition is proved independently against the SAME immutable
// base. Only other declared, base-absent Operations may be projected out; their
// proofs must also pass. Existing declarations, files and DTOs are never erased.
// This is not authorization for arbitrary batches, peer edits or migrations.
func reconcileChangeSetBoundaries(ctx context.Context, options projectflow.Options, value ChangeSet) (*ChangeSetBoundaryReport, error) {
	creates := []CreateOperationChange{}
	for _, subject := range value.Subjects {
		if subject.Create != nil {
			creates = append(creates, *subject.Create)
		}
	}
	if len(creates) == 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for _, create := range creates {
		if err := validateCreateBoundaryProof(value.BaseSHA, create); err != nil {
			return nil, err
		}
		want, err := boundaryIncludeIdentity(options.Root, create.BoundaryProof.Plan.ProtoPaths)
		if err != nil {
			return nil, err
		}
		got, err := boundaryIncludeIdentity(options.Root, options.ProtoPaths)
		if err != nil {
			return nil, err
		}
		if jsonValue(want) != jsonValue(got) {
			return nil, staleBoundary("check compiler include profile differs from create plan %s; supply the same explicit includes", create.Operation.OperationID)
		}
	}
	head, err := resolveGitBase(options.Root, "HEAD")
	if err != nil {
		return nil, err
	}
	before, after, err := captureBoundaryPair(ctx, options, value.BaseSHA)
	if err != nil {
		return nil, staleBoundary("cannot reconstruct canonical proof inputs: %v", err)
	}
	report := &ChangeSetBoundaryReport{
		SchemaVersion: 1, Policy: ChangeSetBoundaryPolicy, BaseSHA: value.BaseSHA,
		BeforeManifestDigest: boundarycore.ManifestDigest(before.Manifest), AfterManifestDigest: boundarycore.ManifestDigest(after.Manifest),
		InputsDigest: after.InputsDigest, Entries: []ChangeSetBoundaryEntry{}, Violations: []ChangeViolation{},
	}
	basePlans, err := contract.CompileOperationPlans(before.Manifest)
	if err != nil {
		return nil, err
	}
	baseIDs := operationPlanIndex(basePlans)
	declared := map[string]bool{}
	for _, create := range creates {
		if _, exists := baseIDs[create.Operation.OperationID]; exists {
			return nil, staleBoundary("create subject %s already exists at the immutable base", create.Operation.OperationID)
		}
		declared[create.Operation.OperationID] = true
	}
	for _, create := range creates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		proof := create.BoundaryProof
		decision := proof.Plan.BoundaryDecision
		entry := ChangeSetBoundaryEntry{OperationID: create.Operation.OperationID, DecisionDigest: decision.DecisionDigest, Outcome: decision.Outcome}
		var validation error
		if proof.BaseInputsDigest != before.ContentDigest {
			validation = staleBoundary("immutable base compiler inputs changed, including explicit includes or metadata")
		} else {
			projected, err := projectIndependentAddition(after.Manifest, declared, entry.OperationID)
			if err != nil {
				return nil, err
			}
			request := boundarycore.AdditionRequest{BaseSHA: value.BaseSHA, Application: create.Operation.Domain + "/" + create.Operation.Application, OperationID: entry.OperationID}
			validation = boundarycore.RevalidateAddition(request, before.Manifest, projected, *decision)
		}
		entry.Validated = validation == nil
		if validation != nil {
			report.Violations = append(report.Violations, ChangeViolation{Kind: "boundary-proof", Path: entry.OperationID, Detail: validation.Error()})
		}
		report.Entries = append(report.Entries, entry)
	}
	currentHead, err := resolveGitBase(options.Root, "HEAD")
	if err != nil {
		return nil, err
	}
	digest, err := projectflow.ContractInputsDigest(ctx, options)
	if err != nil {
		return nil, err
	}
	if currentHead != head || digest != after.InputsDigest {
		return nil, staleBoundary("HEAD or compiler inputs changed during ChangeSet boundary check")
	}
	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].OperationID < report.Entries[j].OperationID })
	sortChangeViolations(report.Violations)
	return report, nil
}

func projectIndependentAddition(current contract.Manifest, declared map[string]bool, target string) (contract.Manifest, error) {
	data, err := json.Marshal(current)
	if err != nil {
		return contract.Manifest{}, err
	}
	var projected contract.Manifest
	if err := json.Unmarshal(data, &projected); err != nil {
		return contract.Manifest{}, err
	}
	for i := range projected.Services {
		service := &projected.Services[i]
		methods := []contract.Method(nil)
		for _, method := range service.Methods {
			if method.Operation == nil || method.Operation.ID == target || !declared[method.Operation.ID] {
				methods = append(methods, method)
			}
		}
		service.Methods = methods
		if service.Application != nil {
			operations := []contract.OperationDeclaration(nil)
			for _, operation := range service.Application.Operations {
				if operation.ID == target || !declared[operation.ID] {
					operations = append(operations, operation)
				}
			}
			service.Application.Operations = operations
		}
	}
	projected.Normalize()
	return projected, nil
}

func validateCreateProfiles(root string, value ChangeSet) error {
	var profile []string
	initialized := false
	for _, subject := range value.Subjects {
		if subject.Create == nil {
			continue
		}
		if subject.Create.BoundaryProof == nil {
			return staleBoundary("create subject is missing a compiler profile proof")
		}
		next, err := boundaryIncludeIdentity(root, subject.Create.BoundaryProof.Plan.ProtoPaths)
		if err != nil {
			return err
		}
		if initialized && jsonValue(profile) != jsonValue(next) {
			return staleBoundary("create subjects use incompatible compiler include profiles; replan against one explicit profile")
		}
		profile, initialized = next, true
	}
	return nil
}
