package change

import (
	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/boundarycore"
)

const SemanticBoundary = "boundary"

func operationGrowthSemanticDeltas(value ChangeSet, before, after contract.Manifest) []SemanticDelta {
	result := []SemanticDelta{}
	for _, event := range boundarycore.EvaluateGrowth(value.BaseSHA, before, after) {
		result = append(result, SemanticDelta{
			Category: SemanticBoundary,
			Subject:  "operation:" + event.OperationID,
			Field:    event.Kind,
			Before:   jsonValue(struct{ Application, Service string }{event.BeforeApplication, event.BeforeService}),
			After:    jsonValue(struct{ Application, Service, Outcome, Reason string }{event.AfterApplication, event.AfterService, event.Outcome, event.Reason}),
			Allowed:  event.Outcome == boundarycore.ReuseExistingService,
		})
	}
	// Bind the stored create-plan BoundaryIntent even when another current fact
	// would still happen to produce a reusable policy outcome.
	current := operationBoundaryIndex(after)
	for _, subject := range value.Subjects {
		if subject.Create == nil {
			continue
		}
		expected := subject.Create.Expected.Semantics.Boundary
		actual := current[subject.Create.Operation.OperationID]
		if jsonValue(expected) != jsonValue(actual) {
			result = append(result, SemanticDelta{Category: SemanticBoundary, Subject: "operation:" + subject.Create.Operation.OperationID, Field: "boundary_intent", Before: jsonValue(expected), After: jsonValue(actual), Allowed: false})
		}
	}
	return result
}

func operationBoundaryIndex(manifest contract.Manifest) map[string]*OperationBoundarySemantics {
	result := map[string]*OperationBoundarySemantics{}
	convert := func(value *contract.BoundaryIntent) *OperationBoundarySemantics {
		if value == nil {
			return nil
		}
		return &OperationBoundarySemantics{Context: value.Context, Aggregate: value.Aggregate, AggregateNotApplicableReason: value.AggregateNotApplicableReason}
	}
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation != nil {
				result[method.Operation.ID] = convert(method.Operation.Boundary)
			}
		}
		if service.Application != nil {
			for _, operation := range service.Application.Operations {
				result[operation.ID] = convert(operation.Boundary)
			}
		}
	}
	return result
}

type OperationBoundarySemantics struct {
	Context                      string `json:"context"`
	Aggregate                    string `json:"aggregate,omitempty"`
	AggregateNotApplicableReason string `json:"aggregateNotApplicableReason,omitempty"`
}
