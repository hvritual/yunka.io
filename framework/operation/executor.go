package operation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/operationplan"
)

var (
	ErrExecutorUnavailable = errors.New("operation: executor unavailable")
	ErrInvokerRequired     = errors.New("operation: invoker is required")
	ErrSecurityUnavailable = errors.New("operation: security phase unavailable")
	ErrSecurityNilContext  = errors.New("operation: security phase returned nil context")
)

type Phase string

const (
	PhasePlan        Phase = "plan"
	PhaseMetadata    Phase = "metadata"
	PhaseSecurity    Phase = "security"
	PhaseApplication Phase = "application"
	PhaseOutcome     Phase = "outcome"
)

type Outcome string

const (
	OutcomeStarted Outcome = "started"
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomePanic   Outcome = "panic"
)

type Event struct {
	OperationID string
	Phase       Phase
	Outcome     Outcome
}

type Observer interface {
	Observe(context.Context, Event)
}

type SecurityPhase interface {
	Prepare(context.Context, operationplan.Plan, any) (context.Context, error)
}

type Invoker func(context.Context) (any, error)

type Executor interface {
	Execute(context.Context, operationplan.Plan, any, Invoker) (any, error)
}

type executor struct {
	security  SecurityPhase
	observers []Observer
}

func NewExecutor(security SecurityPhase, observers ...Observer) Executor {
	filtered := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	return &executor{security: security, observers: filtered}
}

func (runtime *executor) Execute(ctx context.Context, plan operationplan.Plan, input any, invoke Invoker) (result any, err error) {
	if runtime == nil {
		return nil, ErrExecutorUnavailable
	}
	if invoke == nil {
		return nil, ErrInvokerRequired
	}
	plan = normalizeRuntimePlan(plan)
	if plan.OperationID == "" {
		return nil, errors.New("operation: operationId is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runtime.observe(ctx, plan.OperationID, PhasePlan, OutcomeStarted)

	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Operation = plan.OperationID
	ctx = runtimecontext.WithMetadata(ctx, metadata)
	runtime.observe(ctx, plan.OperationID, PhaseMetadata, OutcomeSuccess)

	if runtime.security != nil {
		runtime.observe(ctx, plan.OperationID, PhaseSecurity, OutcomeStarted)
		secured, securityErr := runtime.security.Prepare(ctx, plan, input)
		if securityErr != nil {
			runtime.observe(ctx, plan.OperationID, PhaseSecurity, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomeFailure)
			return nil, securityErr
		}
		if secured == nil {
			runtime.observe(ctx, plan.OperationID, PhaseSecurity, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomeFailure)
			return nil, ErrSecurityNilContext
		}
		ctx = secured
		runtime.observe(ctx, plan.OperationID, PhaseSecurity, OutcomeSuccess)
	} else if Protected(plan) {
		runtime.observe(ctx, plan.OperationID, PhaseSecurity, OutcomeFailure)
		runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomeFailure)
		return nil, fmt.Errorf("%w: %s", ErrSecurityUnavailable, plan.OperationID)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			runtime.observe(ctx, plan.OperationID, PhaseApplication, OutcomePanic)
			runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomePanic)
			panic(recovered)
		}
	}()
	runtime.observe(ctx, plan.OperationID, PhaseApplication, OutcomeStarted)
	result, err = invoke(ctx)
	if err != nil {
		runtime.observe(ctx, plan.OperationID, PhaseApplication, OutcomeFailure)
		runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomeFailure)
		return result, err
	}
	runtime.observe(ctx, plan.OperationID, PhaseApplication, OutcomeSuccess)
	runtime.observe(ctx, plan.OperationID, PhaseOutcome, OutcomeSuccess)
	return result, nil
}

func ExecuteTyped[Request, Response any](ctx context.Context, runtime Executor, plan operationplan.Plan, input *Request, invoke func(context.Context, *Request) (*Response, error)) (*Response, error) {
	if runtime == nil {
		return nil, ErrExecutorUnavailable
	}
	if invoke == nil {
		return nil, ErrInvokerRequired
	}
	value, err := runtime.Execute(ctx, plan, input, func(callContext context.Context) (any, error) {
		return invoke(callContext, input)
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	response, ok := value.(*Response)
	if !ok {
		return nil, fmt.Errorf("operation: %s returned unexpected response type %T", plan.OperationID, value)
	}
	return response, nil
}

func Protected(plan operationplan.Plan) bool {
	return plan.Security.TenantRequired || len(plan.Security.Authentication) > 0 || len(plan.Security.Permissions) > 0
}

func normalizeRuntimePlan(plan operationplan.Plan) operationplan.Plan {
	plan.OperationID = strings.TrimSpace(plan.OperationID)
	return plan
}

func (runtime *executor) observe(ctx context.Context, operationID string, phase Phase, outcome Outcome) {
	event := Event{OperationID: operationID, Phase: phase, Outcome: outcome}
	for _, observer := range runtime.observers {
		func() {
			defer func() { _ = recover() }()
			observer.Observe(ctx, event)
		}()
	}
}
