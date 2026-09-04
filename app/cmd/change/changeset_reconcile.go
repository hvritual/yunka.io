package change

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"yunka.io/app/cmd/projectflow"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

const (
	ChangeSetCheckSchemaVersion    = 1
	ChangeSetSemanticSchemaVersion = 1
)

type ChangeSetSemanticReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Deltas        []SemanticDelta `json:"deltas"`
	Violations    []SemanticDelta `json:"violations"`
}

type ChangeSetCheckReport struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	BaseSHA        string                  `json:"baseSha"`
	Reconciliation Reconciliation          `json:"reconciliation"`
	Semantic       ChangeSetSemanticReport `json:"semantic"`
	Conformant     bool                    `json:"conformant"`
}

func ReconcileChangeSet(root string, value ChangeSet) (ChangeSetCheckReport, error) {
	if err := validateChangeSet(value); err != nil {
		return ChangeSetCheckReport{}, err
	}
	envelope := changeSetEnvelope(value)
	gitReport, err := ReconcileGitDelta(root, envelope)
	if err != nil {
		return ChangeSetCheckReport{}, err
	}
	semantic, err := ReconcileChangeSetSemantic(root, value)
	if err != nil {
		return ChangeSetCheckReport{}, err
	}
	return ChangeSetCheckReport{
		SchemaVersion:  ChangeSetCheckSchemaVersion,
		BaseSHA:        value.BaseSHA,
		Reconciliation: gitReport,
		Semantic:       semantic,
		Conformant:     len(gitReport.Violations) == 0 && len(semantic.Violations) == 0,
	}, nil
}

func changeSetEnvelope(value ChangeSet) ChangeContract {
	envelope := ChangeContract{
		SchemaVersion: ChangeContractSchemaVersion,
		BaseSHA:       value.BaseSHA,
		Intent:        "change-set",
		Operation:     ChangeOperation{OperationID: "change-set"},
	}
	for _, subject := range value.Subjects {
		if subject.Existing != nil {
			envelope.EditablePaths = append(envelope.EditablePaths, subject.Existing.EditablePaths...)
			envelope.EditableScopes = append(envelope.EditableScopes, subject.Existing.EditableScopes...)
			envelope.GeneratedPaths = append(envelope.GeneratedPaths, subject.Existing.GeneratedPaths...)
			envelope.GeneratedScopes = append(envelope.GeneratedScopes, subject.Existing.GeneratedScopes...)
		}
		if subject.Create != nil {
			envelope.EditablePaths = append(envelope.EditablePaths, subject.Create.EditablePaths...)
			envelope.GeneratedPaths = append(envelope.GeneratedPaths, subject.Create.GeneratedPaths...)
			envelope.GeneratedScopes = append(envelope.GeneratedScopes, subject.Create.GeneratedScopes...)
		}
	}
	normalizeChangeContract(&envelope)
	return envelope
}

func ReconcileChangeSetSemantic(root string, value ChangeSet) (ChangeSetSemanticReport, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return ChangeSetSemanticReport{}, fmt.Errorf("change set semantic: resolve project: %w", err)
	}
	base, err := loadBaseCanonicalFacts(descriptor, value.BaseSHA)
	if err != nil {
		return ChangeSetSemanticReport{}, err
	}
	current, err := loadCurrentCanonicalFacts(descriptor)
	if err != nil {
		return ChangeSetSemanticReport{}, err
	}

	subjects := make(map[string]ChangeSetSubject, len(value.Subjects))
	applicationAllowances := map[string]map[string]bool{}
	for _, subject := range value.Subjects {
		_, operationID := changeSetSubjectIdentity(subject)
		subjects[operationID] = subject
		if subject.Existing != nil {
			applicationID := strings.TrimSpace(subject.Existing.Operation.Domain) + "/" + strings.TrimSpace(subject.Existing.Operation.Application)
			if applicationID != "/" {
				if applicationAllowances[applicationID] == nil {
					applicationAllowances[applicationID] = map[string]bool{}
				}
				for _, category := range subject.Existing.AllowedSemantic {
					applicationAllowances[applicationID][category] = true
				}
			}
		}
	}

	beforeIndex := operationPlanIndex(base.Plans)
	afterIndex := operationPlanIndex(current.Plans)
	ids := unionKeys(beforeIndex, afterIndex)
	report := ChangeSetSemanticReport{SchemaVersion: ChangeSetSemanticSchemaVersion}
	for _, operationID := range ids {
		left, leftOK := beforeIndex[operationID]
		right, rightOK := afterIndex[operationID]
		subject, declared := subjects[operationID]
		if !declared {
			if leftOK != rightOK || (leftOK && rightOK && jsonValue(left) != jsonValue(right)) {
				report.Deltas = append(report.Deltas, SemanticDelta{Category: "scope", Subject: "operation:" + operationID, Field: "operation", Before: optionalJSON(left, leftOK), After: optionalJSON(right, rightOK), Allowed: false})
			}
			continue
		}
		if subject.Existing != nil {
			before := operationplan.Set{SchemaVersion: operationplan.SchemaVersion}
			after := operationplan.Set{SchemaVersion: operationplan.SchemaVersion}
			if leftOK {
				before.Operations = []operationplan.Plan{left}
			}
			if rightOK {
				after.Operations = []operationplan.Plan{right}
			}
			report.Deltas = append(report.Deltas, compareOperationPlans(*subject.Existing, before, after)...)
			continue
		}
		if subject.Create != nil {
			report.Deltas = append(report.Deltas, compareCreateOperation(*subject.Create, left, leftOK, right, rightOK)...)
		}
	}
	report.Deltas = append(report.Deltas, compareChangeSetApplications(base, current, applicationAllowances)...)
	for _, delta := range report.Deltas {
		if !delta.Allowed {
			report.Violations = append(report.Violations, delta)
		}
	}
	sortSemanticDeltas(report.Deltas)
	sortSemanticDeltas(report.Violations)
	if report.Deltas == nil {
		report.Deltas = []SemanticDelta{}
	}
	if report.Violations == nil {
		report.Violations = []SemanticDelta{}
	}
	return report, nil
}

func compareCreateOperation(expected CreateOperationChange, left operationplan.Plan, leftOK bool, right operationplan.Plan, rightOK bool) []SemanticDelta {
	operationID := expected.Operation.OperationID
	var deltas []SemanticDelta
	if leftOK {
		return append(deltas, SemanticDelta{Category: SemanticContract, Subject: "operation:" + operationID, Field: "base-existence", Before: "true", After: "true", Allowed: false})
	}
	if !rightOK {
		return append(deltas, SemanticDelta{Category: SemanticContract, Subject: "operation:" + operationID, Field: "existence", Before: "false", After: "false", Allowed: false})
	}
	deltas = append(deltas, SemanticDelta{Category: SemanticContract, Subject: "operation:" + operationID, Field: "existence", Before: "false", After: "true", Allowed: true})
	right = operationplan.Normalize(operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{right}}).Operations[0]
	semantics := expected.Expected.Semantics
	appendMismatch := func(category, field string, want, got interface{}) {
		leftValue := jsonValue(want)
		rightValue := jsonValue(got)
		if leftValue == rightValue {
			return
		}
		deltas = append(deltas, SemanticDelta{Category: category, Subject: "operation:" + operationID, Field: field, Before: leftValue, After: rightValue, Allowed: false})
	}
	appendMismatch(SemanticContract, "contract", struct {
		Domain       string `json:"domain"`
		Application  string `json:"application"`
		UseCase      string `json:"useCase"`
		RequestType  string `json:"requestType"`
		ResponseType string `json:"responseType"`
	}{expected.Operation.Domain, expected.Operation.Application, semantics.UseCase, expected.Expected.RequestType, expected.Expected.ResponseType}, struct {
		Domain       string `json:"domain"`
		Application  string `json:"application"`
		UseCase      string `json:"useCase"`
		RequestType  string `json:"requestType"`
		ResponseType string `json:"responseType"`
	}{right.Domain, right.Application, right.UseCase, right.RequestType, right.ResponseType})

	permissionMode := semantics.PermissionMode
	if permissionMode == "" {
		permissionMode = "all"
	}
	appendMismatch(SemanticPermission, "security.permissions", struct {
		Permissions []string `json:"permissions"`
		Mode        string   `json:"mode"`
	}{semantics.Permissions, permissionMode}, struct {
		Permissions []string `json:"permissions"`
		Mode        string   `json:"mode"`
	}{right.Security.Permissions, right.Security.PermissionMode})
	appendMismatch(SemanticTenant, "security.tenantRequired", semantics.Tenant == "required", right.Security.TenantRequired)
	appendMismatch(SemanticAuthentication, "security.authentication", struct {
		Public         bool     `json:"public"`
		Authentication []string `json:"authentication"`
	}{semantics.Access == "public", semantics.Authentication}, struct {
		Public         bool     `json:"public"`
		Authentication []string `json:"authentication"`
	}{right.Security.Public, right.Security.Authentication})
	appendMismatch(SemanticTransaction, "execution.transaction", semantics.Transaction, right.Execution.Transaction)
	appendMismatch(SemanticIdempotency, "execution.idempotency", semantics.Idempotency, right.Execution.Idempotency)
	boundary := semantics.Composition
	if boundary == "none" {
		boundary = ""
	}
	appendMismatch(SemanticComposition, "composition", struct {
		Boundary string   `json:"boundary"`
		Requires []string `json:"requires"`
	}{boundary, semantics.RequiresOperations}, struct {
		Boundary string   `json:"boundary"`
		Requires []string `json:"requires"`
	}{right.Composition.Boundary, right.Composition.RequiresOperations})

	var expectedHTTP []operationplan.HTTPBinding
	if semantics.HTTP != nil {
		expectedHTTP = []operationplan.HTTPBinding{{Method: semantics.HTTP.Method, Path: semantics.HTTP.Path, Body: semantics.HTTP.Body}}
	} else {
		expectedHTTP = []operationplan.HTTPBinding{}
	}
	appendMismatch(SemanticTransport, "bindings.http", expectedHTTP, right.Bindings.HTTP)
	if expected.Expected.Service != "" && expected.Expected.RPC != "" {
		suffix := "/" + expected.Expected.Service + "/" + expected.Expected.RPC
		if !strings.HasSuffix(right.Bindings.RPC, suffix) {
			deltas = append(deltas, SemanticDelta{Category: SemanticTransport, Subject: "operation:" + operationID, Field: "bindings.rpc", Before: jsonValue(suffix), After: jsonValue(right.Bindings.RPC), Allowed: false})
		}
	}
	return deltas
}

func compareChangeSetApplications(before, after canonicalFacts, allowances map[string]map[string]bool) []SemanticDelta {
	beforeIndex := applicationSemanticIndex(before.Manifest)
	afterIndex := applicationSemanticIndex(after.Manifest)
	ids := unionKeys(beforeIndex, afterIndex)
	var deltas []SemanticDelta
	for _, id := range ids {
		left, leftOK := beforeIndex[id]
		right, rightOK := afterIndex[id]
		if leftOK == rightOK && (!leftOK || jsonValue(left) == jsonValue(right)) {
			continue
		}
		allowed := allowances[id]
		if !leftOK || !rightOK {
			deltas = append(deltas, SemanticDelta{Category: "scope", Subject: "application:" + id, Field: "existence", Before: boolString(leftOK), After: boolString(rightOK), Allowed: false})
			continue
		}
		if jsonValue(left.Requires) != jsonValue(right.Requires) {
			deltas = append(deltas, SemanticDelta{Category: SemanticDependencies, Subject: "application:" + id, Field: "requires", Before: jsonValue(left.Requires), After: jsonValue(right.Requires), Allowed: allowed != nil && allowed[SemanticDependencies]})
		}
		if jsonValue(left.Capabilities) != jsonValue(right.Capabilities) {
			deltas = append(deltas, SemanticDelta{Category: SemanticCapabilities, Subject: "application:" + id, Field: "capabilities", Before: jsonValue(left.Capabilities), After: jsonValue(right.Capabilities), Allowed: allowed != nil && allowed[SemanticCapabilities]})
		}
	}
	return deltas
}

func RenderChangeSetCheck(report ChangeSetCheckReport, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(data, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change set check: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "change set base %s\n", report.BaseSHA)
	fmt.Fprintf(&builder, "git violations      %d\n", len(report.Reconciliation.Violations))
	fmt.Fprintf(&builder, "semantic violations %d\n", len(report.Semantic.Violations))
	fmt.Fprintf(&builder, "conformant           %t\n", report.Conformant)
	return builder.String(), nil
}

func normalizedSemanticValues(values []SemanticDelta) []SemanticDelta {
	result := append([]SemanticDelta(nil), values...)
	sortSemanticDeltas(result)
	return result
}

func stableSemanticJSON(values []SemanticDelta) string {
	data, _ := json.Marshal(normalizedSemanticValues(values))
	return string(data)
}

func sortedOperationIDs(subjects map[string]ChangeSetSubject) []string {
	ids := make([]string, 0, len(subjects))
	for id := range subjects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
