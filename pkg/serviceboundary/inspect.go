// Package serviceboundary derives read-only architectural evidence from the
// canonical compiler model. An inspection is not a ServiceBoundaryDecision,
// mutation proof, service taxonomy, or runtime authorization mechanism.
package serviceboundary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

const SchemaVersion = 1
const ProjectionVersion = "canonical-service-boundary/v1"

// OperationEvidence retains per-Operation relationships. A union of permissions
// or contexts alone would lose which Operation owns each architectural fact.
type OperationEvidence struct {
	Plan              operationplan.Plan                `json:"plan"`
	ApplicationMethod string                            `json:"applicationMethod"`
	Boundary          *contract.BoundaryIntent          `json:"boundary,omitempty"`
	Sources           contract.OperationContractContext `json:"sources"`
	ClientStreaming   bool                              `json:"clientStreaming,omitempty"`
	ServerStreaming   bool                              `json:"serverStreaming,omitempty"`
}

type Fingerprint struct {
	ProjectionVersion    string                           `json:"projectionVersion"`
	Application          string                           `json:"application"`
	Service              string                           `json:"service"`
	ServiceSource        string                           `json:"serviceSource"`
	ApplicationRequires  []string                         `json:"applicationRequires"`
	Capabilities         []contract.CapabilityRequirement `json:"capabilities"`
	Operations           []OperationEvidence              `json:"operations"`
	DependencyOperations []OperationEvidence              `json:"dependencyOperations"`
	Files                []contract.File                  `json:"files"`
	Messages             []contract.Message               `json:"messages"`
	Enums                []contract.Enum                  `json:"enums"`
}

// IntentCoverage reports declarations, not cohesion or correctness. Even a
// complete set of identical context strings is not proof of a valid boundary.
type IntentCoverage struct {
	State                  string   `json:"state"` // empty, unknown, partial, declared
	DeclaredOperations     []string `json:"declaredOperations"`
	UnknownOperations      []string `json:"unknownOperations"`
	Contexts               []string `json:"contexts"`
	Aggregates             []string `json:"aggregates"`
	AggregateNotApplicable []string `json:"aggregateNotApplicable"`
}

type Inspection struct {
	SchemaVersion     int         `json:"schemaVersion"`
	Authority         string      `json:"authority"`
	Fingerprint       Fingerprint `json:"fingerprint"`
	FingerprintDigest string      `json:"fingerprintDigest"`
	// Digest of the complete canonical execution-plan set, distinct from the
	// target-Service fingerprint. Neither digest is a signed boundary proof.
	OperationPlansDigest string         `json:"operationPlansDigest"`
	IntentCoverage       IntentCoverage `json:"intentCoverage"`
	NotEvaluated         []string       `json:"notEvaluated"`
}

// Inspect compiles execution plans from the supplied canonical Manifest, rather
// than accepting a potentially stale second plan set. It neither performs I/O
// nor mutates its input. Source paths retain the Manifest's canonical namespace.
func Inspect(input contract.Manifest, application string) (Inspection, error) {
	application = strings.TrimSpace(application)
	parts := strings.Split(application, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Inspection{}, fmt.Errorf("boundary inspect: application must be <domain>/<application>")
	}
	if input.SchemaVersion < 0 || input.SchemaVersion > contract.ManifestVersion {
		return Inspection{}, fmt.Errorf("boundary inspect: unsupported manifest schemaVersion %d", input.SchemaVersion)
	}
	// Manifest.Normalize sorts nested slices in place; detach before invoking it.
	data, err := json.Marshal(input)
	if err != nil {
		return Inspection{}, err
	}
	var manifest contract.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Inspection{}, err
	}
	manifest.Normalize()
	diagnostics := contract.Lint(manifest)
	if contract.HasErrors(diagnostics) {
		for _, d := range diagnostics {
			if d.Severity == contract.SeverityError {
				return Inspection{}, fmt.Errorf("boundary inspect: invalid canonical facts at %s: %s", d.Path, d.Message)
			}
		}
	}
	plans, err := contract.CompileOperationPlans(manifest)
	if err != nil {
		return Inspection{}, err
	}
	plans = operationplan.Normalize(plans)
	plansDigest, err := operationplan.Digest(plans)
	if err != nil {
		return Inspection{}, err
	}
	var target *contract.Service
	for i := range manifest.Services {
		s := &manifest.Services[i]
		if s.Application == nil || s.Domain+"/"+s.Application.Name != application {
			continue
		}
		if target != nil {
			return Inspection{}, fmt.Errorf("boundary inspect: application %s has multiple Service projections", application)
		}
		target = s
	}
	if target == nil {
		return Inspection{}, fmt.Errorf("boundary inspect: application %q was not found", application)
	}
	contexts, err := contract.OperationContractContexts(manifest)
	if err != nil {
		return Inspection{}, err
	}
	sourceIndex := map[string]contract.OperationContractContext{}
	for _, item := range contexts {
		sourceIndex[item.OperationID] = item
	}
	evidenceIndex := map[string]OperationEvidence{}
	for _, s := range manifest.Services {
		for _, m := range s.Methods {
			if m.Operation == nil {
				continue
			}
			evidenceIndex[m.Operation.ID] = OperationEvidence{ApplicationMethod: m.Name, Boundary: m.Operation.Boundary, Sources: sourceIndex[m.Operation.ID], ClientStreaming: m.ClientStreaming, ServerStreaming: m.ServerStreaming}
		}
		if s.Application != nil {
			for _, op := range s.Application.Operations {
				evidenceIndex[op.ID] = OperationEvidence{ApplicationMethod: op.ApplicationMethod, Boundary: op.Boundary, Sources: sourceIndex[op.ID]}
			}
		}
	}
	for _, plan := range plans.Operations {
		evidence, exists := evidenceIndex[plan.OperationID]
		if !exists || evidence.Sources.OperationID == "" {
			return Inspection{}, fmt.Errorf("boundary inspect: missing provenance for %s", plan.OperationID)
		}
		evidence.Plan = plan
		evidenceIndex[plan.OperationID] = evidence
	}
	fingerprint := Fingerprint{
		ProjectionVersion: ProjectionVersion, Application: application, Service: target.FullName, ServiceSource: target.SourceFile,
		ApplicationRequires: append([]string{}, target.Application.Requires...), Capabilities: append([]contract.CapabilityRequirement{}, target.Application.Capabilities...),
		Operations: []OperationEvidence{}, DependencyOperations: []OperationEvidence{}, Files: []contract.File{}, Messages: []contract.Message{}, Enums: []contract.Enum{},
	}
	local := map[string]bool{}
	for _, plan := range plans.Operations {
		if plan.Domain+"/"+plan.Application == application {
			fingerprint.Operations = append(fingerprint.Operations, evidenceIndex[plan.OperationID])
			local[plan.OperationID] = true
		}
	}
	// Retain transitive dependency facts, not merely the dependency names.
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		op := evidenceIndex[id]
		if !local[id] {
			fingerprint.DependencyOperations = append(fingerprint.DependencyOperations, op)
		}
		for _, dep := range op.Plan.Composition.RequiresOperations {
			visit(dep)
		}
	}
	for _, op := range fingerprint.Operations {
		visit(op.Plan.OperationID)
	}
	sort.Slice(fingerprint.DependencyOperations, func(i, j int) bool {
		return fingerprint.DependencyOperations[i].Plan.OperationID < fingerprint.DependencyOperations[j].Plan.OperationID
	})
	fileSet := map[string]bool{target.SourceFile: true}
	messageSet, enumSet := map[string]bool{}, map[string]bool{}
	for _, group := range [][]OperationEvidence{fingerprint.Operations, fingerprint.DependencyOperations} {
		for _, op := range group {
			for _, p := range op.Sources.SourceFiles {
				fileSet[p] = true
			}
			for _, p := range op.Sources.MessageTypes {
				messageSet[p] = true
			}
			for _, p := range op.Sources.EnumTypes {
				enumSet[p] = true
			}
		}
	}
	for _, file := range manifest.Files {
		if fileSet[file.Name] {
			fingerprint.Files = append(fingerprint.Files, file)
			delete(fileSet, file.Name)
		}
	}
	if len(fileSet) != 0 {
		return Inspection{}, fmt.Errorf("boundary inspect: missing canonical Service source provenance")
	}
	for _, message := range manifest.Messages {
		if messageSet[message.FullName] {
			fingerprint.Messages = append(fingerprint.Messages, message)
		}
	}
	for _, enum := range manifest.Enums {
		if enumSet[enum.FullName] {
			fingerprint.Enums = append(fingerprint.Enums, enum)
		}
	}
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return Inspection{}, err
	}
	digest := sha256.Sum256(append([]byte(ProjectionVersion+"\n"), encoded...))
	return Inspection{
		SchemaVersion: SchemaVersion, Authority: "read_only", Fingerprint: fingerprint, FingerprintDigest: hex.EncodeToString(digest[:]), OperationPlansDigest: plansDigest,
		IntentCoverage: coverage(fingerprint.Operations),
		NotEvaluated:   []string{"business_ontology", "client_cohesion", "operation_growth", "provider_runtime_state", "release_lifecycle", "runtime_availability", "service_boundary_decision"},
	}, nil
}

func coverage(operations []OperationEvidence) IntentCoverage {
	result := IntentCoverage{State: "empty", DeclaredOperations: []string{}, UnknownOperations: []string{}, Contexts: []string{}, Aggregates: []string{}, AggregateNotApplicable: []string{}}
	contexts, aggregates := map[string]bool{}, map[string]bool{}
	for _, op := range operations {
		if op.Boundary == nil {
			result.UnknownOperations = append(result.UnknownOperations, op.Plan.OperationID)
			continue
		}
		result.DeclaredOperations = append(result.DeclaredOperations, op.Plan.OperationID)
		contexts[op.Boundary.Context] = true
		if op.Boundary.Aggregate != "" {
			aggregates[op.Boundary.Aggregate] = true
		} else {
			result.AggregateNotApplicable = append(result.AggregateNotApplicable, op.Plan.OperationID)
		}
	}
	for value := range contexts {
		result.Contexts = append(result.Contexts, value)
	}
	for value := range aggregates {
		result.Aggregates = append(result.Aggregates, value)
	}
	sort.Strings(result.Contexts)
	sort.Strings(result.Aggregates)
	switch {
	case len(operations) == 0:
	case len(result.DeclaredOperations) == 0:
		result.State = "unknown"
	case len(result.UnknownOperations) != 0:
		result.State = "partial"
	default:
		result.State = "declared"
	}
	return result
}
