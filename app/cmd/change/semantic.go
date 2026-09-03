package change

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/projectflow"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

const SemanticReportSchemaVersion = 1

type SemanticDelta struct {
	Category string `json:"category"`
	Subject  string `json:"subject"`
	Field    string `json:"field"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
	Allowed  bool   `json:"allowed"`
}

type SemanticReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	OperationID   string          `json:"operationId"`
	Deltas        []SemanticDelta `json:"deltas"`
	Violations    []SemanticDelta `json:"violations"`
}

type canonicalFacts struct {
	Manifest contract.Manifest
	Plans    operationplan.Set
}

type applicationSemantic struct {
	ID           string                           `json:"id"`
	Requires     []string                         `json:"requires"`
	Capabilities []contract.CapabilityRequirement `json:"capabilities"`
}

func ReconcileSemanticDelta(root string, contractValue ChangeContract) (SemanticReport, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: root})
	if err != nil {
		return SemanticReport{}, fmt.Errorf("change semantic: resolve project: %w", err)
	}
	base, err := loadBaseCanonicalFacts(descriptor, contractValue.BaseSHA)
	if err != nil {
		return SemanticReport{}, err
	}
	current, err := loadCurrentCanonicalFacts(descriptor)
	if err != nil {
		return SemanticReport{}, err
	}
	report := SemanticReport{SchemaVersion: SemanticReportSchemaVersion, OperationID: contractValue.Operation.OperationID}
	report.Deltas = append(report.Deltas, compareOperationPlans(contractValue, base.Plans, current.Plans)...)
	report.Deltas = append(report.Deltas, compareApplicationSemantics(contractValue, base.Manifest, current.Manifest)...)
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

func loadCurrentCanonicalFacts(project projectflow.ProjectDescriptor) (canonicalFacts, error) {
	generated := projectflow.ResolveDescriptorPath(project, project.ContractGenerated)
	manifest, err := contract.LoadManifest(filepath.Join(generated, contract.ManifestFilename))
	if err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: load current manifest: %w", err)
	}
	plans, err := operationplan.Load(filepath.Join(generated, contract.OperationPlansFilename))
	if err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: load current operation plans: %w", err)
	}
	return canonicalFacts{Manifest: manifest, Plans: plans}, nil
}

func loadBaseCanonicalFacts(project projectflow.ProjectDescriptor, baseSHA string) (canonicalFacts, error) {
	manifestPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(project.ContractGenerated), contract.ManifestFilename))
	plansPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(project.ContractGenerated), contract.OperationPlansFilename))
	manifestBytes, err := gitShow(project.Root, baseSHA, manifestPath)
	if err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: load base manifest %s: %w", manifestPath, err)
	}
	plansBytes, err := gitShow(project.Root, baseSHA, plansPath)
	if err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: load base operation plans %s: %w", plansPath, err)
	}
	var manifest contract.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: decode base manifest: %w", err)
	}
	manifest.Normalize()
	plans, err := operationplan.Decode(bytes.NewReader(plansBytes))
	if err != nil {
		return canonicalFacts{}, fmt.Errorf("change semantic: decode base operation plans: %w", err)
	}
	return canonicalFacts{Manifest: manifest, Plans: plans}, nil
}

func gitShow(root, baseSHA, path string) ([]byte, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	path = cleanProjectPath(path)
	if baseSHA == "" || path == "" {
		return nil, fmt.Errorf("base SHA and canonical path are required")
	}
	return runGitBytes(root, "show", baseSHA+":"+path)
}

func compareOperationPlans(contractValue ChangeContract, before, after operationplan.Set) []SemanticDelta {
	before = operationplan.Normalize(before)
	after = operationplan.Normalize(after)
	beforeIndex := operationPlanIndex(before)
	afterIndex := operationPlanIndex(after)
	ids := unionKeys(beforeIndex, afterIndex)
	var deltas []SemanticDelta
	for _, id := range ids {
		left, leftOK := beforeIndex[id]
		right, rightOK := afterIndex[id]
		if id != contractValue.Operation.OperationID {
			if !leftOK || !rightOK || jsonValue(left) != jsonValue(right) {
				deltas = append(deltas, SemanticDelta{Category: "scope", Subject: "operation:" + id, Field: "operation", Before: optionalJSON(left, leftOK), After: optionalJSON(right, rightOK), Allowed: false})
			}
			continue
		}
		if !leftOK || !rightOK {
			deltas = append(deltas, SemanticDelta{Category: SemanticContract, Subject: "operation:" + id, Field: "existence", Before: boolString(leftOK), After: boolString(rightOK), Allowed: semanticAllowed(contractValue, SemanticContract)})
			continue
		}
		deltas = appendIfChanged(deltas, contractValue, SemanticContract, id, "contract", struct {
			Domain       string `json:"domain"`
			Application  string `json:"application"`
			UseCase      string `json:"useCase"`
			RequestType  string `json:"requestType"`
			ResponseType string `json:"responseType"`
		}{left.Domain, left.Application, left.UseCase, left.RequestType, left.ResponseType}, struct {
			Domain       string `json:"domain"`
			Application  string `json:"application"`
			UseCase      string `json:"useCase"`
			RequestType  string `json:"requestType"`
			ResponseType string `json:"responseType"`
		}{right.Domain, right.Application, right.UseCase, right.RequestType, right.ResponseType})
		deltas = appendIfChanged(deltas, contractValue, SemanticPermission, id, "security.permissions", struct {
			Permissions []string `json:"permissions"`
			Mode        string   `json:"mode"`
		}{left.Security.Permissions, left.Security.PermissionMode}, struct {
			Permissions []string `json:"permissions"`
			Mode        string   `json:"mode"`
		}{right.Security.Permissions, right.Security.PermissionMode})
		deltas = appendIfChanged(deltas, contractValue, SemanticTenant, id, "security.tenantRequired", left.Security.TenantRequired, right.Security.TenantRequired)
		deltas = appendIfChanged(deltas, contractValue, SemanticAuthentication, id, "security.authentication", struct {
			Public         bool     `json:"public"`
			Authentication []string `json:"authentication"`
		}{left.Security.Public, left.Security.Authentication}, struct {
			Public         bool     `json:"public"`
			Authentication []string `json:"authentication"`
		}{right.Security.Public, right.Security.Authentication})
		deltas = appendIfChanged(deltas, contractValue, SemanticTransaction, id, "execution.transaction", left.Execution.Transaction, right.Execution.Transaction)
		deltas = appendIfChanged(deltas, contractValue, SemanticIdempotency, id, "execution.idempotency", left.Execution.Idempotency, right.Execution.Idempotency)
		deltas = appendIfChanged(deltas, contractValue, SemanticComposition, id, "composition", struct {
			Boundary          string   `json:"boundary"`
			Requires          []string `json:"requires"`
			PermissionClosure []string `json:"permissionClosure"`
		}{left.Composition.Boundary, left.Composition.RequiresOperations, left.Composition.PermissionClosure}, struct {
			Boundary          string   `json:"boundary"`
			Requires          []string `json:"requires"`
			PermissionClosure []string `json:"permissionClosure"`
		}{right.Composition.Boundary, right.Composition.RequiresOperations, right.Composition.PermissionClosure})
		deltas = appendIfChanged(deltas, contractValue, SemanticDependencies, id, "applicationRequires", left.ApplicationRequires, right.ApplicationRequires)
		deltas = appendIfChanged(deltas, contractValue, SemanticTransport, id, "bindings", left.Bindings, right.Bindings)
	}
	return deltas
}

func compareApplicationSemantics(contractValue ChangeContract, before, after contract.Manifest) []SemanticDelta {
	beforeIndex := applicationSemanticIndex(before)
	afterIndex := applicationSemanticIndex(after)
	ids := unionKeys(beforeIndex, afterIndex)
	target := strings.TrimSpace(contractValue.Operation.Domain) + "/" + strings.TrimSpace(contractValue.Operation.Application)
	var deltas []SemanticDelta
	for _, id := range ids {
		left, leftOK := beforeIndex[id]
		right, rightOK := afterIndex[id]
		if id != target {
			if !leftOK || !rightOK || jsonValue(left) != jsonValue(right) {
				deltas = append(deltas, SemanticDelta{Category: "scope", Subject: "application:" + id, Field: "application", Before: optionalJSON(left, leftOK), After: optionalJSON(right, rightOK), Allowed: false})
			}
			continue
		}
		if !leftOK || !rightOK {
			deltas = append(deltas, SemanticDelta{Category: SemanticContract, Subject: "application:" + id, Field: "existence", Before: boolString(leftOK), After: boolString(rightOK), Allowed: semanticAllowed(contractValue, SemanticContract)})
			continue
		}
		deltas = appendIfChanged(deltas, contractValue, SemanticDependencies, "application:"+id, "requires", left.Requires, right.Requires)
		deltas = appendIfChanged(deltas, contractValue, SemanticCapabilities, "application:"+id, "capabilities", left.Capabilities, right.Capabilities)
	}
	return deltas
}

func operationPlanIndex(set operationplan.Set) map[string]operationplan.Plan {
	result := make(map[string]operationplan.Plan, len(set.Operations))
	for _, item := range set.Operations {
		result[item.OperationID] = item
	}
	return result
}

func applicationSemanticIndex(manifest contract.Manifest) map[string]applicationSemantic {
	manifest.Normalize()
	result := map[string]applicationSemantic{}
	for _, service := range manifest.Services {
		if service.Application == nil {
			continue
		}
		id := strings.TrimSpace(service.Domain) + "/" + strings.TrimSpace(service.Application.Name)
		if id == "/" {
			continue
		}
		result[id] = applicationSemantic{
			ID:           id,
			Requires:     append([]string(nil), service.Application.Requires...),
			Capabilities: append([]contract.CapabilityRequirement(nil), service.Application.Capabilities...),
		}
	}
	return result
}

func appendIfChanged(values []SemanticDelta, contractValue ChangeContract, category, subject, field string, before, after interface{}) []SemanticDelta {
	left := jsonValue(before)
	right := jsonValue(after)
	if left == right {
		return values
	}
	return append(values, SemanticDelta{Category: category, Subject: subject, Field: field, Before: left, After: right, Allowed: semanticAllowed(contractValue, category)})
}

func semanticAllowed(contractValue ChangeContract, category string) bool {
	return contains(contractValue.AllowedSemantic, category)
}

func unionKeys[T any](left, right map[string]T) []string {
	seen := map[string]struct{}{}
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func jsonValue(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func optionalJSON(value interface{}, ok bool) string {
	if !ok {
		return ""
	}
	return jsonValue(value)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sortSemanticDeltas(values []SemanticDelta) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Subject != values[j].Subject {
			return values[i].Subject < values[j].Subject
		}
		if values[i].Category != values[j].Category {
			return values[i].Category < values[j].Category
		}
		return values[i].Field < values[j].Field
	})
}

func currentCanonicalFilesExist(project projectflow.ProjectDescriptor) error {
	generated := projectflow.ResolveDescriptorPath(project, project.ContractGenerated)
	for _, name := range []string{contract.ManifestFilename, contract.OperationPlansFilename} {
		if _, err := os.Stat(filepath.Join(generated, name)); err != nil {
			return err
		}
	}
	return nil
}
