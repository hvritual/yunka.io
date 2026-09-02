package diagnostics

import (
	"context"
	"fmt"

	frameworkoperation "github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

type OperationSummary struct {
	OperationID string `json:"operationId"`
	Domain      string `json:"domain"`
	Application string `json:"application"`
	Composition string `json:"composition,omitempty"`
	Transaction string `json:"transaction"`
	Idempotency string `json:"idempotency"`
	Protected   bool   `json:"protected"`
}

type OperationDiagnostics struct {
	SchemaVersion  int                                 `json:"schemaVersion"`
	Digest         string                              `json:"digest"`
	OperationCount int                                 `json:"operationCount"`
	Operations     []OperationSummary                  `json:"operations,omitempty"`
	ExecutorBound  bool                                `json:"executorBound"`
	Runtime        *frameworkoperation.RuntimeSnapshot `json:"runtime,omitempty"`
}

type OperationSource struct {
	plans    operationplan.Set
	executor frameworkoperation.Executor
}

func NewOperationSource(plans operationplan.Set, executor frameworkoperation.Executor) (*OperationSource, error) {
	plans = operationplan.Normalize(plans)
	if err := operationplan.Validate(plans); err != nil {
		return nil, fmt.Errorf("diagnostics operation source: %w", err)
	}
	return &OperationSource{plans: plans, executor: executor}, nil
}

func (*OperationSource) Name() string { return "operation-execution" }

func (source *OperationSource) Snapshot(ctx context.Context) (any, error) {
	if source == nil {
		return nil, fmt.Errorf("diagnostics operation source: source is nil")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	digest, err := operationplan.Digest(source.plans)
	if err != nil {
		return nil, err
	}
	result := OperationDiagnostics{
		SchemaVersion:  source.plans.SchemaVersion,
		Digest:         digest,
		OperationCount: len(source.plans.Operations),
		ExecutorBound:  source.executor != nil,
		Operations:     make([]OperationSummary, 0, len(source.plans.Operations)),
	}
	for _, plan := range source.plans.Operations {
		result.Operations = append(result.Operations, OperationSummary{
			OperationID: plan.OperationID,
			Domain:      plan.Domain,
			Application: plan.Application,
			Composition: plan.Composition.Boundary,
			Transaction: plan.Execution.Transaction,
			Idempotency: plan.Execution.Idempotency,
			Protected:   frameworkoperation.Protected(plan),
		})
	}
	if snapshot, ok := frameworkoperation.Snapshot(source.executor); ok {
		copy := snapshot
		copy.Phases = append([]frameworkoperation.Phase(nil), snapshot.Phases...)
		result.Runtime = &copy
	}
	return result, nil
}
