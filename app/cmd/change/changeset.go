package change

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

	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/projectflow"
)

const (
	ChangeSetSchemaVersion = 2
	DefaultChangeSetPath    = ".git/yunka/change-set.json"

	ChangeSubjectExistingOperation = "existing_operation"
	ChangeSubjectCreateOperation   = "create_operation"
)

type CreateOperationExpectation struct {
	Service      string                 `json:"service"`
	RPC          string                 `json:"rpc"`
	RequestType  string                 `json:"requestType"`
	ResponseType string                 `json:"responseType"`
	Semantics    add.OperationSemantics `json:"semantics"`
}

type CreateOperationChange struct {
	Operation       ChangeOperation            `json:"operation"`
	PlanDigest      string                     `json:"planDigest"`
	Expected        CreateOperationExpectation `json:"expected"`
	EditablePaths   []string                   `json:"editablePaths"`
	GeneratedPaths  []string                   `json:"generatedPaths"`
	GeneratedScopes []string                   `json:"generatedScopes"`
}

type ChangeSetSubject struct {
	Kind     string                 `json:"kind"`
	Existing *ChangeContract        `json:"existing,omitempty"`
	Create   *CreateOperationChange `json:"create,omitempty"`
}

type ChangeSet struct {
	SchemaVersion int                `json:"schemaVersion"`
	BaseSHA       string             `json:"baseSha"`
	Subjects      []ChangeSetSubject `json:"subjects"`
}

func BuildChangeSet(root, base string, existingContracts, createPlans []string) (ChangeSet, string, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: resolve project: %w", err)}
	}
	if err := ensureCleanWorktree(descriptor.Root); err != nil {
		return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
	}
	baseSHA, err := resolveGitBase(descriptor.Root, base)
	if err != nil {
		return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
	}
	if len(existingContracts) == 0 && len(createPlans) == 0 {
		return ChangeSet{}, "", &Failure{Kind: FailureIntent, Err: fmt.Errorf("change set begin: at least one --contract or --create-plan is required")}
	}
	baseFacts, err := loadBaseCanonicalFacts(descriptor, baseSHA)
	if err != nil {
		return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
	}
	baseOperations := operationPlanIndex(baseFacts.Plans)
	seenOperations := map[string]struct{}{}
	value := ChangeSet{SchemaVersion: ChangeSetSchemaVersion, BaseSHA: baseSHA}

	for _, input := range existingContracts {
		contractValue, _, err := LoadChangeContract(descriptor.Root, input)
		if err != nil {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: load existing contract %s: %w", input, err)}
		}
		if contractValue.BaseSHA != baseSHA {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: existing contract %s uses base %s; ChangeSet base is %s", input, contractValue.BaseSHA, baseSHA)}
		}
		operationID := strings.TrimSpace(contractValue.Operation.OperationID)
		if operationID == "" {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: existing contract %s has no operationId", input)}
		}
		if _, ok := baseOperations[operationID]; !ok {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: existing-operation subject %s does not exist at base %s", operationID, baseSHA)}
		}
		if err := reserveChangeSetOperation(seenOperations, operationID); err != nil {
			return ChangeSet{}, "", err
		}
		copyValue := contractValue
		value.Subjects = append(value.Subjects, ChangeSetSubject{Kind: ChangeSubjectExistingOperation, Existing: &copyValue})
	}

	for _, input := range createPlans {
		plan, err := loadOperationCreatePlan(descriptor.Root, input)
		if err != nil {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: load create plan %s: %w", input, err)}
		}
		plan, err = add.RevalidateOperationPlan(descriptor.Root, plan)
		if err != nil {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: revalidate create plan %s: %w", input, err)}
		}
		operationID := strings.TrimSpace(plan.Identity["operationId"])
		if _, ok := baseOperations[operationID]; ok {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: create-operation subject %s already exists at base %s", operationID, baseSHA)}
		}
		if err := reserveChangeSetOperation(seenOperations, operationID); err != nil {
			return ChangeSet{}, "", err
		}
		create, err := createOperationChange(plan)
		if err != nil {
			return ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
		}
		value.Subjects = append(value.Subjects, ChangeSetSubject{Kind: ChangeSubjectCreateOperation, Create: &create})
	}
	normalizeChangeSet(&value)
	return value, descriptor.Root, nil
}

func createOperationChange(plan add.Report) (CreateOperationChange, error) {
	if plan.ExplicitSemantics == nil {
		return CreateOperationChange{}, fmt.Errorf("change set begin: create plan has no explicit semantics")
	}
	data, err := add.Render(plan, add.FormatAgentJSON)
	if err != nil {
		return CreateOperationChange{}, err
	}
	digest := sha256.Sum256([]byte(data))
	value := CreateOperationChange{
		Operation: ChangeOperation{
			OperationID: strings.TrimSpace(plan.Identity["operationId"]),
			Domain:      strings.TrimSpace(plan.Identity["domain"]),
			Application: strings.TrimSpace(plan.Identity["application"]),
		},
		PlanDigest: hex.EncodeToString(digest[:]),
		Expected: CreateOperationExpectation{
			Service:      strings.TrimSpace(plan.Identity["service"]),
			RPC:          strings.TrimSpace(plan.Identity["rpc"]),
			RequestType:  strings.TrimSpace(plan.Identity["requestType"]),
			ResponseType: strings.TrimSpace(plan.Identity["responseType"]),
			Semantics:    *plan.ExplicitSemantics,
		},
	}
	for _, mutation := range plan.Mutations {
		path := cleanProjectPath(mutation.Path)
		if path == "" {
			return CreateOperationChange{}, fmt.Errorf("change set begin: create plan contains invalid mutation path %q", mutation.Path)
		}
		value.EditablePaths = append(value.EditablePaths, path)
	}
	for _, effect := range plan.Effects {
		if path := cleanProjectPath(effect.Path); path != "" {
			value.GeneratedPaths = append(value.GeneratedPaths, path)
		}
		if scope := cleanProjectPath(effect.Scope); scope != "" {
			value.GeneratedScopes = append(value.GeneratedScopes, scope)
		}
	}
	normalizeCreateOperationChange(&value)
	return value, nil
}

func reserveChangeSetOperation(seen map[string]struct{}, operationID string) error {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: subject operationId is required")}
	}
	if _, duplicate := seen[operationID]; duplicate {
		return &Failure{Kind: FailureIntent, Err: fmt.Errorf("change set begin: duplicate subject operation %s", operationID)}
	}
	seen[operationID] = struct{}{}
	return nil
}

func normalizeChangeSet(value *ChangeSet) {
	if value == nil {
		return
	}
	value.BaseSHA = strings.TrimSpace(value.BaseSHA)
	for index := range value.Subjects {
		subject := &value.Subjects[index]
		subject.Kind = strings.TrimSpace(subject.Kind)
		if subject.Existing != nil {
			normalizeChangeContract(subject.Existing)
		}
		if subject.Create != nil {
			normalizeCreateOperationChange(subject.Create)
		}
	}
	sort.Slice(value.Subjects, func(i, j int) bool {
		leftKind, leftID := changeSetSubjectIdentity(value.Subjects[i])
		rightKind, rightID := changeSetSubjectIdentity(value.Subjects[j])
		if leftID != rightID {
			return leftID < rightID
		}
		return leftKind < rightKind
	})
	if value.Subjects == nil {
		value.Subjects = []ChangeSetSubject{}
	}
}

func normalizeCreateOperationChange(value *CreateOperationChange) {
	if value == nil {
		return
	}
	value.Operation.OperationID = strings.TrimSpace(value.Operation.OperationID)
	value.Operation.Domain = strings.TrimSpace(value.Operation.Domain)
	value.Operation.Application = strings.TrimSpace(value.Operation.Application)
	value.PlanDigest = strings.TrimSpace(value.PlanDigest)
	value.EditablePaths = uniqueSorted(value.EditablePaths)
	value.GeneratedPaths = uniqueSorted(value.GeneratedPaths)
	value.GeneratedScopes = uniqueSorted(value.GeneratedScopes)
	value.Expected.Semantics.Permissions = uniqueSorted(value.Expected.Semantics.Permissions)
	value.Expected.Semantics.Authentication = uniqueSorted(value.Expected.Semantics.Authentication)
	value.Expected.Semantics.RequiresOperations = uniqueSorted(value.Expected.Semantics.RequiresOperations)
	if value.EditablePaths == nil {
		value.EditablePaths = []string{}
	}
	if value.GeneratedPaths == nil {
		value.GeneratedPaths = []string{}
	}
	if value.GeneratedScopes == nil {
		value.GeneratedScopes = []string{}
	}
	if value.Expected.Semantics.Permissions == nil {
		value.Expected.Semantics.Permissions = []string{}
	}
	if value.Expected.Semantics.Authentication == nil {
		value.Expected.Semantics.Authentication = []string{}
	}
	if value.Expected.Semantics.RequiresOperations == nil {
		value.Expected.Semantics.RequiresOperations = []string{}
	}
}

func changeSetSubjectIdentity(subject ChangeSetSubject) (string, string) {
	if subject.Existing != nil {
		return ChangeSubjectExistingOperation, subject.Existing.Operation.OperationID
	}
	if subject.Create != nil {
		return ChangeSubjectCreateOperation, subject.Create.Operation.OperationID
	}
	return subject.Kind, ""
}

func WriteChangeSet(root, output string, value ChangeSet) (string, error) {
	normalizeChangeSet(&value)
	if err := validateChangeSet(value); err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		output = DefaultChangeSetPath
	}
	path := output
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative), nil
	}
	return filepath.ToSlash(path), nil
}

func LoadChangeSet(root, input string) (ChangeSet, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		input = DefaultChangeSetPath
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ChangeSet{}, "", err
	}
	var value ChangeSet
	if err := decodeStrictJSON(data, &value); err != nil {
		return ChangeSet{}, "", fmt.Errorf("decode change set: %w", err)
	}
	normalizeChangeSet(&value)
	if err := validateChangeSet(value); err != nil {
		return ChangeSet{}, "", err
	}
	return value, filepath.ToSlash(path), nil
}

func validateChangeSet(value ChangeSet) error {
	if value.SchemaVersion != ChangeSetSchemaVersion {
		return fmt.Errorf("unsupported change set schemaVersion %d", value.SchemaVersion)
	}
	if strings.TrimSpace(value.BaseSHA) == "" {
		return fmt.Errorf("change set: baseSha is required")
	}
	if len(value.Subjects) == 0 {
		return fmt.Errorf("change set: subjects are required")
	}
	seen := map[string]struct{}{}
	for _, subject := range value.Subjects {
		kind, operationID := changeSetSubjectIdentity(subject)
		switch kind {
		case ChangeSubjectExistingOperation:
			if subject.Existing == nil || subject.Create != nil || subject.Kind != ChangeSubjectExistingOperation {
				return fmt.Errorf("change set: existing_operation subject has invalid shape")
			}
			if subject.Existing.SchemaVersion != ChangeContractSchemaVersion || subject.Existing.BaseSHA != value.BaseSHA {
				return fmt.Errorf("change set: existing operation %s has incompatible contract/base", operationID)
			}
		case ChangeSubjectCreateOperation:
			if subject.Create == nil || subject.Existing != nil || subject.Kind != ChangeSubjectCreateOperation {
				return fmt.Errorf("change set: create_operation subject has invalid shape")
			}
			if subject.Create.PlanDigest == "" || subject.Create.Expected.Semantics.UseCase == "" || len(subject.Create.EditablePaths) == 0 {
				return fmt.Errorf("change set: create operation %s is missing plan evidence", operationID)
			}
		default:
			return fmt.Errorf("change set: unsupported subject kind %q", subject.Kind)
		}
		if operationID == "" {
			return fmt.Errorf("change set: subject operationId is required")
		}
		if _, duplicate := seen[operationID]; duplicate {
			return fmt.Errorf("change set: duplicate operation %s", operationID)
		}
		seen[operationID] = struct{}{}
	}
	return nil
}

func loadOperationCreatePlan(root, input string) (add.Report, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return add.Report{}, fmt.Errorf("create plan path is required")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return add.Report{}, err
	}
	var value add.Report
	if err := decodeStrictJSON(data, &value); err != nil {
		return add.Report{}, fmt.Errorf("decode operation create plan: %w", err)
	}
	return value, nil
}

func decodeStrictJSON(data []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value is not allowed")
		}
		return err
	}
	return nil
}

func RenderChangeSet(value ChangeSet, path, format string) (string, error) {
	normalizeChangeSet(&value)
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path      string    `json:"path"`
			ChangeSet ChangeSet `json:"changeSet"`
		}{Path: path, ChangeSet: value}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(data, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change set: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "change set %s\n", path)
	fmt.Fprintf(&builder, "base       %s\n", value.BaseSHA)
	fmt.Fprintf(&builder, "subjects   %d\n", len(value.Subjects))
	for _, subject := range value.Subjects {
		kind, operationID := changeSetSubjectIdentity(subject)
		fmt.Fprintf(&builder, "  %-18s %s\n", kind, operationID)
	}
	builder.WriteString("next       apply bounded edits, run `yunka generate`, then `yunka change set check`\n")
	return builder.String(), nil
}
