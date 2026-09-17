package boundarycore

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
)

const GrowthPolicyVersion = "operation-growth/v1"

const (
	GrowthOperationAdded     = "operation_added"
	GrowthOperationMoved     = "operation_moved_service"
	GrowthContextChanged     = "operation_context_changed"
	GrowthAggregateChanged   = "operation_aggregate_changed"
	GrowthPermissionChanged  = "permission_domain_changed"
	GrowthDependencyExpanded = "dependency_expanded"
	GrowthCompositionChanged = "composition_changed"
	GrowthConsistencyChanged = "consistency_changed"
)

type GrowthEvent struct {
	PolicyVersion     string                   `json:"policyVersion"`
	Kind              string                   `json:"kind"`
	OperationID       string                   `json:"operationId"`
	BeforeApplication string                   `json:"beforeApplication,omitempty"`
	AfterApplication  string                   `json:"afterApplication,omitempty"`
	BeforeService     string                   `json:"beforeService,omitempty"`
	AfterService      string                   `json:"afterService,omitempty"`
	Outcome           string                   `json:"outcome"`
	Reason            string                   `json:"reason"`
	Decision          *ServiceBoundaryDecision `json:"decision,omitempty"`
}

type operationState struct {
	Application string
	Service     string
	Operation   contract.OperationDeclaration
}

func EvaluateGrowth(baseSHA string, before, after contract.Manifest) []GrowthEvent {
	before = detachedManifest(before)
	after = detachedManifest(after)
	left := operationStates(before)
	right := operationStates(after)
	ids := map[string]struct{}{}
	for id := range left {
		ids[id] = struct{}{}
	}
	for id := range right {
		ids[id] = struct{}{}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	events := []GrowthEvent{}
	for _, id := range ordered {
		b, bok := left[id]
		a, aok := right[id]
		if !aok { // removal is not growth
			continue
		}
		if !bok {
			event := GrowthEvent{PolicyVersion: GrowthPolicyVersion, Kind: GrowthOperationAdded, OperationID: id, AfterApplication: a.Application, AfterService: a.Service, Outcome: ArchitectureReviewRequired, Reason: "new Operation requires a boundary decision against the immutable base"}
			if a.Application != "" {
				decision, err := EvaluateAddition(AdditionRequest{BaseSHA: baseSHA, Application: a.Application, OperationID: id}, before, after)
				if err == nil {
					event.Outcome = decision.Outcome
					event.Reason = "single-addition boundary policy recomputed from canonical base/current facts"
					event.Decision = &decision
				} else {
					event.Reason = "boundary decision could not be established: " + err.Error()
				}
			}
			events = append(events, event)
			continue
		}
		appendEvent := func(kind, outcome, reason string) {
			events = append(events, GrowthEvent{PolicyVersion: GrowthPolicyVersion, Kind: kind, OperationID: id, BeforeApplication: b.Application, AfterApplication: a.Application, BeforeService: b.Service, AfterService: a.Service, Outcome: outcome, Reason: reason})
		}
		if b.Application != a.Application || b.Service != a.Service {
			appendEvent(GrowthOperationMoved, ArchitectureReviewRequired, "existing Operation moved across its canonical Application/Service projection")
		}
		if boundaryContext(b.Operation.Boundary) != boundaryContext(a.Operation.Boundary) {
			outcome := ArchitectureReviewRequired
			if b.Operation.Boundary != nil && a.Operation.Boundary != nil {
				outcome = CreateNewService
			}
			appendEvent(GrowthContextChanged, outcome, "explicit context boundary changed")
		}
		if boundaryAggregate(b.Operation.Boundary) != boundaryAggregate(a.Operation.Boundary) {
			outcome := ArchitectureReviewRequired
			if b.Operation.Boundary != nil && a.Operation.Boundary != nil {
				outcome = CreateNewService
			}
			appendEvent(GrowthAggregateChanged, outcome, "explicit aggregate boundary changed")
		}
		if securityBoundary(b.Operation) != securityBoundary(a.Operation) {
			appendEvent(GrowthPermissionChanged, ArchitectureReviewRequired, "canonical access/permission/authentication facts changed; permission names are not reinterpreted as business domains")
		}
		if expanded(b.Operation.RequiresOperations, a.Operation.RequiresOperations) {
			appendEvent(GrowthDependencyExpanded, ArchitectureReviewRequired, "Operation dependency set expanded")
		}
		if strings.TrimSpace(b.Operation.Composition) != strings.TrimSpace(a.Operation.Composition) {
			appendEvent(GrowthCompositionChanged, ArchitectureReviewRequired, "composition boundary changed")
		}
		if jsonValue(b.Operation.Execution) != jsonValue(a.Operation.Execution) {
			appendEvent(GrowthConsistencyChanged, ArchitectureReviewRequired, "execution consistency policy changed")
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].OperationID != events[j].OperationID {
			return events[i].OperationID < events[j].OperationID
		}
		return events[i].Kind < events[j].Kind
	})
	return events
}

func operationStates(manifest contract.Manifest) map[string]operationState {
	result := map[string]operationState{}
	for _, service := range manifest.Services {
		application := ""
		if service.Application != nil {
			application = strings.TrimSpace(service.Domain) + "/" + strings.TrimSpace(service.Application.Name)
		}
		for _, method := range service.Methods {
			if method.Operation == nil || strings.TrimSpace(method.Operation.ID) == "" {
				continue
			}
			result[method.Operation.ID] = operationState{Application: application, Service: service.FullName, Operation: *method.Operation}
		}
		if service.Application != nil {
			for _, operation := range service.Application.Operations {
				if strings.TrimSpace(operation.ID) == "" {
					continue
				}
				result[operation.ID] = operationState{Application: application, Service: service.FullName, Operation: operation}
			}
		}
	}
	return result
}

func boundaryContext(value *contract.BoundaryIntent) string {
	if value == nil {
		return "<unknown>"
	}
	return strings.TrimSpace(value.Context)
}
func boundaryAggregate(value *contract.BoundaryIntent) string {
	if value == nil {
		return "<unknown>"
	}
	if v := strings.TrimSpace(value.Aggregate); v != "" {
		return "aggregate:" + v
	}
	return "not-applicable:" + strings.TrimSpace(value.AggregateNotApplicableReason)
}
func securityBoundary(value contract.OperationDeclaration) string {
	return jsonValue(struct {
		Public         bool     `json:"public"`
		Permissions    []string `json:"permissions"`
		PermissionMode string   `json:"permissionMode"`
		TenantRequired bool     `json:"tenantRequired"`
		Authentication []string `json:"authentication"`
	}{value.Public, value.Permissions, value.PermissionMode, value.TenantRequired, value.Authentication})
}
func expanded(before, after []string) bool {
	known := map[string]struct{}{}
	for _, value := range before {
		known[strings.TrimSpace(value)] = struct{}{}
	}
	for _, value := range after {
		if _, ok := known[strings.TrimSpace(value)]; !ok {
			return true
		}
	}
	return false
}
func jsonValue(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
