package operation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/execution"
	"yunka.io/pkg/operationplan"
)

var (
	ErrExecutorUnavailable    = errors.New("operation: executor unavailable")
	ErrInvokerRequired        = errors.New("operation: invoker is required")
	ErrSecurityUnavailable    = errors.New("operation: security phase unavailable")
	ErrSecurityNilContext     = errors.New("operation: security phase returned nil context")
	ErrIdempotencyUnavailable = errors.New("operation: idempotency coordinator unavailable")
	ErrChildExecutionRequired = errors.New("operation: child execution requires an active root scope")
)

type Phase string

const (
	PhasePlan                Phase = "plan"
	PhaseMetadata            Phase = "metadata"
	PhaseSecurity            Phase = "security"
	PhaseIdempotencyBegin    Phase = "idempotency_begin"
	PhaseExecutionScope      Phase = "execution_scope"
	PhaseApplication         Phase = "application"
	PhaseTransactionFinalize Phase = "transaction_finalize"
	PhaseIdempotencyFinalize Phase = "idempotency_finalize"
	PhaseOutcome             Phase = "outcome"
)

type Outcome string

const (
	OutcomeStarted Outcome = "started"
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomePanic   Outcome = "panic"
)

type InvocationKind string

const (
	InvocationRoot  InvocationKind = "root"
	InvocationChild InvocationKind = "child"
)

type Event struct {
	OperationID string
	Kind        InvocationKind
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

type ExecutorOptions struct {
	Transactions execution.TransactionFactory
	Idempotency  execution.IdempotencyCoordinator
}

type executor struct {
	security     SecurityPhase
	transactions execution.TransactionFactory
	idempotency  execution.IdempotencyCoordinator
	observers    []Observer
}

func NewExecutor(security SecurityPhase, observers ...Observer) Executor {
	return NewExecutorWithOptions(security, ExecutorOptions{}, observers...)
}

func NewExecutorWithOptions(security SecurityPhase, options ExecutorOptions, observers ...Observer) Executor {
	filtered := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	return &executor{security: security, transactions: options.Transactions, idempotency: options.Idempotency, observers: filtered}
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
	if _, active := execution.Current(ctx); active {
		return nil, execution.ErrScopeAlreadyActive
	}
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhasePlan, OutcomeStarted)

	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Operation = plan.OperationID
	ctx = runtimecontext.WithMetadata(ctx, metadata)
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseMetadata, OutcomeSuccess)

	if runtime.security != nil {
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeStarted)
		secured, securityErr := runtime.security.Prepare(ctx, plan, input)
		if securityErr != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
			return nil, securityErr
		}
		if secured == nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
			return nil, ErrSecurityNilContext
		}
		ctx = secured
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeSuccess)
	} else if Protected(plan) {
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeFailure)
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
		return nil, fmt.Errorf("%w: %s", ErrSecurityUnavailable, plan.OperationID)
	} else {
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseSecurity, OutcomeSuccess)
	}

	idempotent := plan.Execution.Idempotency == "required"
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyBegin, OutcomeStarted)
	if idempotent {
		if runtime.idempotency == nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyBegin, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
			return nil, fmt.Errorf("%w: %s", ErrIdempotencyUnavailable, plan.OperationID)
		}
		claimed, claimErr := runtime.idempotency.Begin(ctx, plan)
		if claimErr != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyBegin, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
			return nil, claimErr
		}
		ctx = claimed
	}
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyBegin, OutcomeSuccess)

	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseExecutionScope, OutcomeStarted)
	scoped, root, scopeErr := execution.BeginRoot(ctx, plan.OperationID, execution.TransactionMode(plan.Execution.Transaction), plan.Composition.RequiresOperations, runtime.transactions)
	if scopeErr != nil {
		if idempotent {
			_ = runtime.idempotency.Fail(ctx, plan, scopeErr)
		}
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseExecutionScope, OutcomeFailure)
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
		return nil, scopeErr
	}
	ctx = scoped
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseExecutionScope, OutcomeSuccess)

	defer func() {
		if recovered := recover(); recovered != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseApplication, OutcomePanic)
			_ = root.Rollback(ctx)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomePanic)
			if idempotent {
				_ = runtime.idempotency.Fail(ctx, plan, fmt.Errorf("panic: %v", recovered))
			}
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomePanic)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomePanic)
			panic(recovered)
		}
	}()

	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseApplication, OutcomeStarted)
	result, err = invoke(ctx)
	if err != nil {
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseApplication, OutcomeFailure)
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeStarted)
		rollbackErr := root.Rollback(ctx)
		if rollbackErr != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeFailure)
		} else {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeSuccess)
		}
		var idempotencyErr error
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeStarted)
		if idempotent {
			idempotencyErr = runtime.idempotency.Fail(ctx, plan, err)
		}
		if idempotencyErr != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeFailure)
		} else {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeSuccess)
		}
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
		return result, errors.Join(err, rollbackErr, idempotencyErr)
	}
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseApplication, OutcomeSuccess)

	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeStarted)
	atomicIdempotency, atomicErr := runtime.stageAtomicIdempotency(ctx, plan, idempotent)
	if atomicErr != nil {
		rollbackErr := root.Rollback(ctx)
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeFailure)
		var idempotencyErr error
		if idempotent {
			idempotencyErr = runtime.idempotency.Fail(ctx, plan, atomicErr)
		}
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, outcomeFor(idempotencyErr))
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
		return result, errors.Join(atomicErr, rollbackErr, idempotencyErr)
	}
	if commitErr := root.Commit(ctx); commitErr != nil {
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeFailure)
		var idempotencyErr error
		if idempotent {
			idempotencyErr = runtime.idempotency.Fail(ctx, plan, commitErr)
		}
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, outcomeFor(idempotencyErr))
		runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
		return result, errors.Join(commitErr, idempotencyErr)
	}
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseTransactionFinalize, OutcomeSuccess)

	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeStarted)
	if idempotent && !atomicIdempotency {
		if completeErr := runtime.idempotency.Complete(ctx, plan); completeErr != nil {
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeFailure)
			runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeFailure)
			return result, completeErr
		}
	}
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseIdempotencyFinalize, OutcomeSuccess)
	runtime.observe(ctx, plan.OperationID, InvocationRoot, PhaseOutcome, OutcomeSuccess)
	return result, nil
}

func (runtime *executor) stageAtomicIdempotency(ctx context.Context, plan operationplan.Plan, idempotent bool) (bool, error) {
	if !idempotent || plan.Execution.Transaction != "local" || runtime.idempotency == nil {
		return false, nil
	}
	capabilities, ok := runtime.idempotency.(execution.IdempotencyCapabilityReporter)
	if !ok || !capabilities.SupportsAtomicCompletion() {
		return false, nil
	}
	atomic, ok := runtime.idempotency.(execution.AtomicIdempotencyCoordinator)
	if !ok {
		return false, execution.ErrIdempotencyAtomicUnavailable
	}
	transaction, err := execution.TransactionHandleFrom(ctx)
	if err != nil {
		return false, errors.Join(execution.ErrIdempotencyAtomicUnavailable, err)
	}
	if err := atomic.CompleteInTransaction(ctx, plan, transaction); err != nil {
		return false, err
	}
	return true, nil
}

func outcomeFor(err error) Outcome {
	if err != nil {
		return OutcomeFailure
	}
	return OutcomeSuccess
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

type childExecutor interface {
	ExecuteChild(context.Context, operationplan.Plan, any, Invoker) (any, error)
}

func ExecuteChild(ctx context.Context, runtime Executor, plan operationplan.Plan, input any, invoke Invoker) (any, error) {
	if runtime == nil {
		return nil, ErrExecutorUnavailable
	}
	child, ok := runtime.(childExecutor)
	if !ok {
		return nil, ErrChildExecutionRequired
	}
	return child.ExecuteChild(ctx, plan, input, invoke)
}

func ExecuteChildTyped[Request, Response any](ctx context.Context, runtime Executor, plan operationplan.Plan, input *Request, invoke func(context.Context, *Request) (*Response, error)) (*Response, error) {
	value, err := ExecuteChild(ctx, runtime, plan, input, func(callContext context.Context) (any, error) {
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
		return nil, fmt.Errorf("operation: child %s returned unexpected response type %T", plan.OperationID, value)
	}
	return response, nil
}

func (runtime *executor) ExecuteChild(ctx context.Context, plan operationplan.Plan, input any, invoke Invoker) (any, error) {
	if runtime == nil {
		return nil, ErrExecutorUnavailable
	}
	if invoke == nil {
		return nil, ErrInvokerRequired
	}
	plan = normalizeRuntimePlan(plan)
	if _, ok := execution.Current(ctx); !ok {
		return nil, ErrChildExecutionRequired
	}
	joined, err := execution.JoinChild(ctx, plan.OperationID, execution.TransactionMode(plan.Execution.Transaction), plan.Composition.RequiresOperations)
	if err != nil {
		return nil, err
	}
	runtime.observe(joined, plan.OperationID, InvocationChild, PhasePlan, OutcomeStarted)
	metadata, _ := runtimecontext.MetadataFrom(joined)
	metadata.Operation = plan.OperationID
	joined = runtimecontext.WithMetadata(joined, metadata)
	runtime.observe(joined, plan.OperationID, InvocationChild, PhaseMetadata, OutcomeSuccess)
	runtime.observe(joined, plan.OperationID, InvocationChild, PhaseApplication, OutcomeStarted)
	value, err := invoke(joined)
	if err != nil {
		runtime.observe(joined, plan.OperationID, InvocationChild, PhaseApplication, OutcomeFailure)
		runtime.observe(joined, plan.OperationID, InvocationChild, PhaseOutcome, OutcomeFailure)
		return value, err
	}
	runtime.observe(joined, plan.OperationID, InvocationChild, PhaseApplication, OutcomeSuccess)
	runtime.observe(joined, plan.OperationID, InvocationChild, PhaseOutcome, OutcomeSuccess)
	return value, nil
}

func Protected(plan operationplan.Plan) bool {
	return plan.Security.TenantRequired || len(plan.Security.Authentication) > 0 || len(plan.Security.Permissions) > 0
}

func normalizeRuntimePlan(plan operationplan.Plan) operationplan.Plan {
	plan.OperationID = strings.TrimSpace(plan.OperationID)
	plan.Execution.Transaction = strings.TrimSpace(plan.Execution.Transaction)
	if plan.Execution.Transaction == "" {
		plan.Execution.Transaction = "none"
	}
	plan.Execution.Idempotency = strings.TrimSpace(plan.Execution.Idempotency)
	if plan.Execution.Idempotency == "" {
		plan.Execution.Idempotency = "none"
	}
	return plan
}

func (runtime *executor) observe(ctx context.Context, operationID string, kind InvocationKind, phase Phase, outcome Outcome) {
	event := Event{OperationID: operationID, Kind: kind, Phase: phase, Outcome: outcome}
	for _, observer := range runtime.observers {
		func() {
			defer func() { _ = recover() }()
			observer.Observe(ctx, event)
		}()
	}
}
