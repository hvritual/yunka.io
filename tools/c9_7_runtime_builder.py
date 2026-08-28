from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content)


def edit(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"expected fragment not found in {path}: {old[:160]!r}")
    target.write_text(text.replace(old, new, 1))


def append(path: str, content: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if content.strip() not in text:
        target.write_text(text.rstrip() + "\n\n" + content.lstrip())


write("framework/execution/scope.go", r'''package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type TransactionMode string

const (
	TransactionNone     TransactionMode = "none"
	TransactionReadOnly TransactionMode = "read_only"
	TransactionLocal    TransactionMode = "local"
)

var (
	ErrScopeAlreadyActive      = errors.New("execution: root scope already active")
	ErrScopeUnavailable        = errors.New("execution: scope unavailable")
	ErrTransactionUnavailable  = errors.New("execution: transaction factory unavailable")
	ErrTransactionHandleAbsent = errors.New("execution: transaction handle unavailable")
	ErrChildUndeclared         = errors.New("execution: child operation is not declared by parent")
	ErrTransactionConflict     = errors.New("execution: child transaction policy conflicts with root scope")
)

type UnitOfWork interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	Close() error
}

type TransactionFactory interface {
	Begin(context.Context, TransactionMode) (UnitOfWork, error)
}

type TransactionHandleProvider interface {
	TransactionHandle() any
}

type frameKey struct{}

type rootState struct {
	mu        sync.Mutex
	rootID    string
	mode      TransactionMode
	unit      UnitOfWork
	finalized bool
}

type frame struct {
	root      *rootState
	operation string
	allowed   map[string]struct{}
	depth     int
}

type Root struct{ state *rootState }

type Frame struct {
	RootOperationID string          `json:"rootOperationId"`
	OperationID     string          `json:"operationId"`
	Transaction     TransactionMode `json:"transaction"`
	Depth           int             `json:"depth"`
}

func normalizeMode(mode TransactionMode) (TransactionMode, error) {
	mode = TransactionMode(strings.TrimSpace(string(mode)))
	if mode == "" {
		mode = TransactionNone
	}
	switch mode {
	case TransactionNone, TransactionReadOnly, TransactionLocal:
		return mode, nil
	default:
		return "", fmt.Errorf("execution: invalid transaction mode %q", mode)
	}
}

func allowedSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func BeginRoot(ctx context.Context, operationID string, mode TransactionMode, allowedChildren []string, factory TransactionFactory) (context.Context, *Root, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := currentFrame(ctx); ok {
		return nil, nil, ErrScopeAlreadyActive
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, nil, errors.New("execution: root operation id is required")
	}
	normalized, err := normalizeMode(mode)
	if err != nil {
		return nil, nil, err
	}
	var unit UnitOfWork
	if normalized != TransactionNone {
		if factory == nil {
			return nil, nil, ErrTransactionUnavailable
		}
		unit, err = factory.Begin(ctx, normalized)
		if err != nil {
			return nil, nil, fmt.Errorf("execution: begin %s transaction: %w", normalized, err)
		}
		if unit == nil {
			return nil, nil, errors.New("execution: transaction factory returned nil unit of work")
		}
	}
	state := &rootState{rootID: operationID, mode: normalized, unit: unit}
	value := &frame{root: state, operation: operationID, allowed: allowedSet(allowedChildren), depth: 0}
	return context.WithValue(ctx, frameKey{}, value), &Root{state: state}, nil
}

func currentFrame(ctx context.Context) (*frame, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(frameKey{}).(*frame)
	return value, ok && value != nil && value.root != nil
}

func Current(ctx context.Context) (Frame, bool) {
	value, ok := currentFrame(ctx)
	if !ok {
		return Frame{}, false
	}
	return Frame{RootOperationID: value.root.rootID, OperationID: value.operation, Transaction: value.root.mode, Depth: value.depth}, true
}

func UnitOfWorkFrom(ctx context.Context) (UnitOfWork, bool) {
	value, ok := currentFrame(ctx)
	if !ok || value.root.unit == nil {
		return nil, false
	}
	return value.root.unit, true
}

func TransactionHandleFrom(ctx context.Context) (any, error) {
	unit, ok := UnitOfWorkFrom(ctx)
	if !ok {
		return nil, ErrScopeUnavailable
	}
	provider, ok := unit.(TransactionHandleProvider)
	if !ok || provider == nil || provider.TransactionHandle() == nil {
		return nil, ErrTransactionHandleAbsent
	}
	return provider.TransactionHandle(), nil
}

func JoinChild(ctx context.Context, operationID string, mode TransactionMode, allowedChildren []string) (context.Context, error) {
	parent, ok := currentFrame(ctx)
	if !ok {
		return nil, ErrScopeUnavailable
	}
	operationID = strings.TrimSpace(operationID)
	if _, ok := parent.allowed[operationID]; !ok {
		return nil, fmt.Errorf("%w: parent=%s child=%s", ErrChildUndeclared, parent.operation, operationID)
	}
	childMode, err := normalizeMode(mode)
	if err != nil {
		return nil, err
	}
	switch parent.root.mode {
	case TransactionNone:
		if childMode != TransactionNone {
			return nil, fmt.Errorf("%w: root=%s child=%s", ErrTransactionConflict, parent.root.mode, childMode)
		}
	case TransactionReadOnly:
		if childMode == TransactionLocal {
			return nil, fmt.Errorf("%w: root=%s child=%s", ErrTransactionConflict, parent.root.mode, childMode)
		}
	case TransactionLocal:
		// A local root can host none/read-only/local child declarations in the
		// already-open local transaction. No nested transaction is opened.
	}
	child := &frame{root: parent.root, operation: operationID, allowed: allowedSet(allowedChildren), depth: parent.depth + 1}
	return context.WithValue(ctx, frameKey{}, child), nil
}

func (root *Root) Commit(ctx context.Context) error {
	if root == nil || root.state == nil {
		return ErrScopeUnavailable
	}
	root.state.mu.Lock()
	defer root.state.mu.Unlock()
	if root.state.finalized {
		return nil
	}
	root.state.finalized = true
	if root.state.unit == nil {
		return nil
	}
	commitErr := root.state.unit.Commit(nonNil(ctx))
	if commitErr != nil {
		rollbackErr := root.state.unit.Rollback(nonNil(ctx))
		return errors.Join(commitErr, rollbackErr, root.state.unit.Close())
	}
	return root.state.unit.Close()
}

func (root *Root) Rollback(ctx context.Context) error {
	if root == nil || root.state == nil {
		return ErrScopeUnavailable
	}
	root.state.mu.Lock()
	defer root.state.mu.Unlock()
	if root.state.finalized {
		return nil
	}
	root.state.finalized = true
	if root.state.unit == nil {
		return nil
	}
	return errors.Join(root.state.unit.Rollback(nonNil(ctx)), root.state.unit.Close())
}

func nonNil(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
''')

write("framework/execution/idempotency.go", r'''package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"yunka.io/framework/core/identity"
	"yunka.io/pkg/operationplan"
)

var (
	ErrIdempotencyUnavailable = errors.New("execution: idempotency coordinator unavailable")
	ErrIdempotencyKeyRequired = errors.New("execution: idempotency key required")
	ErrIdempotencyInProgress  = errors.New("execution: idempotent operation already in progress")
	ErrIdempotencyCompleted   = errors.New("execution: idempotent operation already completed")
)

type idempotencyKeyContext struct{}
type idempotencyIdentityContext struct{}

func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, idempotencyKeyContext{}, strings.TrimSpace(key))
}

func IdempotencyKeyFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(idempotencyKeyContext{}).(string)
	return strings.TrimSpace(value)
}

type IdempotencyState string

const (
	IdempotencyRunning   IdempotencyState = "running"
	IdempotencySucceeded IdempotencyState = "succeeded"
	IdempotencyFailed    IdempotencyState = "failed"
)

type IdempotencyIdentity struct {
	TenantID    string
	OperationID string
	Key         string
}

type IdempotencyStore interface {
	Claim(context.Context, IdempotencyIdentity) error
	Mark(context.Context, IdempotencyIdentity, IdempotencyState) error
	Lookup(context.Context, IdempotencyIdentity) (IdempotencyState, bool, error)
}

type IdempotencyCoordinator interface {
	Begin(context.Context, operationplan.Plan) (context.Context, error)
	Complete(context.Context, operationplan.Plan) error
	Fail(context.Context, operationplan.Plan, error) error
}

type coordinator struct{ store IdempotencyStore }

func NewIdempotencyCoordinator(store IdempotencyStore) (IdempotencyCoordinator, error) {
	if store == nil {
		return nil, errors.New("execution: idempotency store is required")
	}
	return &coordinator{store: store}, nil
}

func (coordinator *coordinator) Begin(ctx context.Context, plan operationplan.Plan) (context.Context, error) {
	if coordinator == nil || coordinator.store == nil {
		return nil, ErrIdempotencyUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := IdempotencyKeyFrom(ctx)
	if key == "" {
		return nil, fmt.Errorf("%w: %s", ErrIdempotencyKeyRequired, plan.OperationID)
	}
	principal, _ := identity.FromContext(ctx)
	identity := IdempotencyIdentity{TenantID: strings.TrimSpace(principal.TenantID), OperationID: strings.TrimSpace(plan.OperationID), Key: key}
	if err := coordinator.store.Claim(ctx, identity); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, idempotencyIdentityContext{}, identity), nil
}

func (coordinator *coordinator) Complete(ctx context.Context, _ operationplan.Plan) error {
	identity, ok := idempotencyIdentityFrom(ctx)
	if !ok {
		return ErrIdempotencyKeyRequired
	}
	return coordinator.store.Mark(ctx, identity, IdempotencySucceeded)
}

func (coordinator *coordinator) Fail(ctx context.Context, _ operationplan.Plan, _ error) error {
	identity, ok := idempotencyIdentityFrom(ctx)
	if !ok {
		return nil
	}
	return coordinator.store.Mark(ctx, identity, IdempotencyFailed)
}

func idempotencyIdentityFrom(ctx context.Context) (IdempotencyIdentity, bool) {
	if ctx == nil {
		return IdempotencyIdentity{}, false
	}
	value, ok := ctx.Value(idempotencyIdentityContext{}).(IdempotencyIdentity)
	return value, ok && value.OperationID != "" && value.Key != ""
}

type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[IdempotencyIdentity]IdempotencyState
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{records: map[IdempotencyIdentity]IdempotencyState{}}
}

func (store *MemoryIdempotencyStore) Claim(_ context.Context, identity IdempotencyIdentity) error {
	if store == nil {
		return ErrIdempotencyUnavailable
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.records == nil {
		store.records = map[IdempotencyIdentity]IdempotencyState{}
	}
	switch store.records[identity] {
	case IdempotencyRunning:
		return ErrIdempotencyInProgress
	case IdempotencySucceeded:
		return ErrIdempotencyCompleted
	case IdempotencyFailed, "":
		store.records[identity] = IdempotencyRunning
		return nil
	default:
		return ErrIdempotencyUnavailable
	}
}

func (store *MemoryIdempotencyStore) Mark(_ context.Context, identity IdempotencyIdentity, state IdempotencyState) error {
	if store == nil {
		return ErrIdempotencyUnavailable
	}
	if state != IdempotencySucceeded && state != IdempotencyFailed {
		return fmt.Errorf("execution: invalid terminal idempotency state %q", state)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.records == nil {
		store.records = map[IdempotencyIdentity]IdempotencyState{}
	}
	if store.records[identity] != IdempotencyRunning {
		return ErrIdempotencyUnavailable
	}
	store.records[identity] = state
	return nil
}

func (store *MemoryIdempotencyStore) Lookup(_ context.Context, identity IdempotencyIdentity) (IdempotencyState, bool, error) {
	if store == nil {
		return "", false, ErrIdempotencyUnavailable
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	state, ok := store.records[identity]
	return state, ok, nil
}
''')

write("framework/requestscope/join.go", r'''package requestscope

import (
	"context"
	"errors"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/execution"
)

var ErrExecutionScopeUnavailable = errors.New("requestscope: execution scope has no unit of work")

// View is a repository view joined to the root Operation's ExecutionScope.
// It deliberately has no Commit/Rollback/Close methods: transaction lifecycle
// remains owned by the root Executor.
type View[R any] struct {
	ctx          context.Context
	repositories R
}

func Join[R any](ctx context.Context, repositories RepositoryFactory[R]) (*View[R], error) {
	if repositories == nil {
		return nil, errors.New("requestscope: repository factory is required")
	}
	unit, ok := execution.UnitOfWorkFrom(ctx)
	if !ok {
		return nil, ErrExecutionScopeUnavailable
	}
	values, err := repositories(ctx, unit)
	if err != nil {
		return nil, err
	}
	return &View[R]{ctx: ctx, repositories: values}, nil
}

func JoinDo[R any](ctx context.Context, repositories RepositoryFactory[R], call func(*View[R]) error) error {
	if call == nil {
		return errors.New("requestscope: callback is required")
	}
	view, err := Join(ctx, repositories)
	if err != nil {
		return err
	}
	return call(view)
}

func JoinValue[R any, T any](ctx context.Context, repositories RepositoryFactory[R], call func(*View[R]) (T, error)) (result T, err error) {
	if call == nil {
		return result, errors.New("requestscope: callback is required")
	}
	view, err := Join(ctx, repositories)
	if err != nil {
		return result, err
	}
	return call(view)
}

func (view *View[R]) Context() context.Context {
	if view == nil || view.ctx == nil {
		return context.Background()
	}
	return view.ctx
}

func (view *View[R]) Repositories() R {
	if view == nil {
		var zero R
		return zero
	}
	return view.repositories
}

func (view *View[R]) Principal() (identity.Principal, bool) {
	if view == nil {
		return identity.Principal{}, false
	}
	principal, ok := identity.FromContext(view.ctx)
	return principal.Clone(), ok
}

func (view *View[R]) Metadata() (runtimecontext.Metadata, bool) {
	if view == nil {
		return runtimecontext.Metadata{}, false
	}
	metadata, ok := runtimecontext.MetadataFrom(view.ctx)
	return metadata.Clone(), ok
}

func (view *View[R]) TraceID() string {
	if view == nil {
		return ""
	}
	return runtimecontext.TraceIDFrom(view.ctx)
}
''')

edit(
    "framework/requestscope/scope.go",
    '"yunka.io/framework/core/runtimecontext"\n\t"yunka.io/pkg/logExt"',
    '"yunka.io/framework/core/runtimecontext"\n\t"yunka.io/framework/execution"\n\t"yunka.io/pkg/logExt"',
)
edit(
    "framework/requestscope/scope.go",
    '''// UnitOfWork is the request-owned transaction boundary. Implementations may
// wrap GORM, database/sql, or another transactional store, but they must not be
// shared between requests or retained by singleton services.
type UnitOfWork interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	Close() error
}
''',
    '''// UnitOfWork is retained as a source-compatible alias. C9.7 moves ownership
// of the root transaction lifecycle to framework/execution while requestscope
// continues to build typed repository views over that unit.
type UnitOfWork = execution.UnitOfWork
''',
)

edit(
    "framework/requestscope/gorm.go",
    '"gorm.io/gorm"\n)',
    '"gorm.io/gorm"\n\t"yunka.io/framework/execution"\n)',
)
append("framework/requestscope/gorm.go", r'''
// GORMExecutionFactory is the C9.7 root transaction factory used by the
// Operation Executor. Legacy GORMFactory remains available for pre-C9.7 callers.
type GORMExecutionFactory struct{ database *gorm.DB }

func NewGORMExecutionFactory(database *gorm.DB) (*GORMExecutionFactory, error) {
	if database == nil {
		return nil, errors.New("requestscope: GORM database is required")
	}
	return &GORMExecutionFactory{database: database}, nil
}

func (factory *GORMExecutionFactory) Begin(ctx context.Context, mode execution.TransactionMode) (execution.UnitOfWork, error) {
	if factory == nil || factory.database == nil {
		return nil, ErrFactoryUnavailable
	}
	if mode != execution.TransactionReadOnly && mode != execution.TransactionLocal {
		return nil, fmt.Errorf("requestscope: unsupported execution transaction mode %q", mode)
	}
	options := &sql.TxOptions{ReadOnly: mode == execution.TransactionReadOnly}
	transaction := factory.database.WithContext(normalizeContext(ctx)).Begin(options)
	if transaction.Error != nil {
		return nil, transaction.Error
	}
	return &gormUnitOfWork{transaction: transaction}, nil
}

// TransactionHandle lets framework mechanisms such as Saga/Outbox join the
// exact root transaction without exposing *gorm.DB through Application APIs.
func (unit *gormUnitOfWork) TransactionHandle() any {
	return unit.GORM()
}
''')

write("framework/operation/executor.go", r'''package operation

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
	if idempotent {
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
''')

write("framework/operation/diagnostics.go", r'''package operation

type RuntimeSnapshot struct {
	Phases             []Phase `json:"phases"`
	SecurityBound      bool    `json:"securityBound"`
	TransactionBound   bool    `json:"transactionBound"`
	IdempotencyBound   bool    `json:"idempotencyBound"`
	ChildExecution     bool    `json:"childExecution"`
	ObserverCount      int     `json:"observerCount"`
}

type snapshotter interface {
	Snapshot() RuntimeSnapshot
}

func (runtime *executor) Snapshot() RuntimeSnapshot {
	if runtime == nil {
		return RuntimeSnapshot{Phases: canonicalPhases()}
	}
	return RuntimeSnapshot{
		Phases:           canonicalPhases(),
		SecurityBound:    runtime.security != nil,
		TransactionBound: runtime.transactions != nil,
		IdempotencyBound: runtime.idempotency != nil,
		ChildExecution:   true,
		ObserverCount:    len(runtime.observers),
	}
}

func Snapshot(runtime Executor) (RuntimeSnapshot, bool) {
	if runtime == nil {
		return RuntimeSnapshot{}, false
	}
	value, ok := runtime.(snapshotter)
	if !ok {
		return RuntimeSnapshot{}, false
	}
	return value.Snapshot(), true
}

func canonicalPhases() []Phase {
	return []Phase{
		PhasePlan,
		PhaseMetadata,
		PhaseSecurity,
		PhaseIdempotencyBegin,
		PhaseExecutionScope,
		PhaseApplication,
		PhaseTransactionFinalize,
		PhaseIdempotencyFinalize,
		PhaseOutcome,
	}
}
''')

write("framework/execution/scope_test.go", r'''package execution

import (
	"context"
	"errors"
	"testing"
)

type fakeUnit struct{ commits, rollbacks, closes int }
func (unit *fakeUnit) Commit(context.Context) error { unit.commits++; return nil }
func (unit *fakeUnit) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *fakeUnit) Close() error { unit.closes++; return nil }
func (unit *fakeUnit) TransactionHandle() any { return unit }

type fakeFactory struct{ unit *fakeUnit; begins int; mode TransactionMode }
func (factory *fakeFactory) Begin(_ context.Context, mode TransactionMode) (UnitOfWork, error) {
	factory.begins++; factory.mode = mode
	if factory.unit == nil { factory.unit = &fakeUnit{} }
	return factory.unit, nil
}

func TestRootOwnsOneTransactionAndChildrenJoinIt(t *testing.T) {
	factory := &fakeFactory{}
	ctx, root, err := BeginRoot(context.Background(), "device.transfer", TransactionLocal, []string{"site.validate"}, factory)
	if err != nil { t.Fatal(err) }
	child, err := JoinChild(ctx, "site.validate", TransactionReadOnly, nil)
	if err != nil { t.Fatal(err) }
	rootFrame, _ := Current(ctx)
	childFrame, _ := Current(child)
	if rootFrame.Depth != 0 || childFrame.Depth != 1 || childFrame.RootOperationID != "device.transfer" || childFrame.OperationID != "site.validate" {
		t.Fatalf("root=%#v child=%#v", rootFrame, childFrame)
	}
	rootUnit, _ := UnitOfWorkFrom(ctx)
	childUnit, _ := UnitOfWorkFrom(child)
	if rootUnit != childUnit || factory.begins != 1 { t.Fatalf("rootUnit=%p childUnit=%p begins=%d", rootUnit, childUnit, factory.begins) }
	if err := root.Commit(ctx); err != nil { t.Fatal(err) }
	if factory.unit.commits != 1 || factory.unit.rollbacks != 0 || factory.unit.closes != 1 {
		t.Fatalf("unit=%+v", factory.unit)
	}
}

func TestChildMustBeDeclaredAndCannotEscalateReadOnlyRoot(t *testing.T) {
	factory := &fakeFactory{}
	ctx, root, err := BeginRoot(context.Background(), "root", TransactionReadOnly, []string{"child"}, factory)
	if err != nil { t.Fatal(err) }
	defer root.Rollback(ctx)
	if _, err := JoinChild(ctx, "missing", TransactionNone, nil); !errors.Is(err, ErrChildUndeclared) { t.Fatalf("undeclared err=%v", err) }
	if _, err := JoinChild(ctx, "child", TransactionLocal, nil); !errors.Is(err, ErrTransactionConflict) { t.Fatalf("transaction err=%v", err) }
}
''')

write("framework/execution/idempotency_test.go", r'''package execution

import (
	"context"
	"errors"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/pkg/operationplan"
)

func TestIdempotencyCoordinatorClaimsCompletesAndAllowsFailedRetry(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	coordinator, err := NewIdempotencyCoordinator(store)
	if err != nil { t.Fatal(err) }
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"})
	ctx = WithIdempotencyKey(ctx, "request-1")
	plan := operationplan.Plan{OperationID: "device.create"}
	claimed, err := coordinator.Begin(ctx, plan)
	if err != nil { t.Fatal(err) }
	if _, err := coordinator.Begin(ctx, plan); !errors.Is(err, ErrIdempotencyInProgress) { t.Fatalf("in-progress err=%v", err) }
	if err := coordinator.Complete(claimed, plan); err != nil { t.Fatal(err) }
	if _, err := coordinator.Begin(ctx, plan); !errors.Is(err, ErrIdempotencyCompleted) { t.Fatalf("completed err=%v", err) }

	ctx2 := WithIdempotencyKey(ctx, "request-2")
	claimed2, err := coordinator.Begin(ctx2, plan)
	if err != nil { t.Fatal(err) }
	if err := coordinator.Fail(claimed2, plan, errors.New("boom")); err != nil { t.Fatal(err) }
	if _, err := coordinator.Begin(ctx2, plan); err != nil { t.Fatalf("failed request should be retryable: %v", err) }
}
''')

write("framework/requestscope/join_test.go", r'''package requestscope

import (
	"context"
	"testing"

	"yunka.io/framework/execution"
)

type joinUnit struct{ commits, rollbacks, closes int }
func (unit *joinUnit) Commit(context.Context) error { unit.commits++; return nil }
func (unit *joinUnit) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *joinUnit) Close() error { unit.closes++; return nil }

type joinFactory struct{ unit *joinUnit; begins int }
func (factory *joinFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	factory.begins++
	factory.unit = &joinUnit{}
	return factory.unit, nil
}

func TestJoinBuildsRepositoryViewWithoutOwningTransaction(t *testing.T) {
	transactions := &joinFactory{}
	ctx, root, err := execution.BeginRoot(context.Background(), "device.update", execution.TransactionLocal, nil, transactions)
	if err != nil { t.Fatal(err) }
	repositories := RepositoryFactory[string](func(_ context.Context, unit UnitOfWork) (string, error) {
		if unit != transactions.unit { t.Fatalf("repository received different unit") }
		return "joined", nil
	})
	value, err := JoinValue(ctx, repositories, func(view *View[string]) (string, error) { return view.Repositories(), nil })
	if err != nil || value != "joined" { t.Fatalf("value=%q err=%v", value, err) }
	if transactions.unit.commits != 0 || transactions.unit.rollbacks != 0 || transactions.unit.closes != 0 {
		t.Fatalf("join finalized root transaction: %+v", transactions.unit)
	}
	if err := root.Commit(ctx); err != nil { t.Fatal(err) }
	if transactions.unit.commits != 1 || transactions.unit.closes != 1 { t.Fatalf("root did not finalize: %+v", transactions.unit) }
}
''')

# Update legacy executor test to the new deterministic root phase machine.
edit(
    "framework/operation/executor_test.go",
    '''\twant := []Event{\n\t\t{OperationID: "device.get", Phase: PhasePlan, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Phase: PhaseMetadata, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Phase: PhaseSecurity, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Phase: PhaseSecurity, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Phase: PhaseApplication, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Phase: PhaseApplication, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Phase: PhaseOutcome, Outcome: OutcomeSuccess},\n\t}\n'''.replace('\\n','\n').replace('\\t','\t'),
    '''\twant := []Event{\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhasePlan, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseMetadata, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseSecurity, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseSecurity, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyBegin, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyBegin, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseExecutionScope, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseExecutionScope, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseApplication, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseApplication, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseTransactionFinalize, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseTransactionFinalize, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyFinalize, Outcome: OutcomeStarted},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyFinalize, Outcome: OutcomeSuccess},\n\t\t{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseOutcome, Outcome: OutcomeSuccess},\n\t}\n'''.replace('\\n','\n').replace('\\t','\t'),
)
append("framework/operation/executor_test.go", r'''
type transactionFactoryStub struct{ unit *transactionUnitStub; begins int }
type transactionUnitStub struct{ commits, rollbacks, closes int }
func (factory *transactionFactoryStub) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	factory.begins++
	factory.unit = &transactionUnitStub{}
	return factory.unit, nil
}
func (unit *transactionUnitStub) Commit(context.Context) error { unit.commits++; return nil }
func (unit *transactionUnitStub) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *transactionUnitStub) Close() error { unit.closes++; return nil }

func TestExecutorOwnsTransactionAndChildJoinsWithoutSecondSecurityDecision(t *testing.T) {
	security := &securityStub{}
	transactions := &transactionFactoryStub{}
	runtime := NewExecutorWithOptions(security, ExecutorOptions{Transactions: transactions})
	parent := operationplan.Plan{
		OperationID: "device.transfer",
		Security: operationplan.Security{Permissions: []string{"device.update", "site.read"}, PermissionMode: "all"},
		Execution: operationplan.Execution{Transaction: "local", Idempotency: "none"},
		Composition: operationplan.Composition{RequiresOperations: []string{"site.validate"}},
	}
	child := operationplan.Plan{OperationID: "site.validate", Execution: operationplan.Execution{Transaction: "read_only", Idempotency: "none"}}
	_, err := runtime.Execute(context.Background(), parent, nil, func(ctx context.Context) (any, error) {
		return ExecuteChild(ctx, runtime, child, nil, func(childCtx context.Context) (any, error) {
			frame, ok := execution.Current(childCtx)
			if !ok || frame.Depth != 1 || frame.RootOperationID != "device.transfer" || frame.OperationID != "site.validate" {
				t.Fatalf("child frame=%#v ok=%v", frame, ok)
			}
			return "ok", nil
		})
	})
	if err != nil { t.Fatal(err) }
	if security.calls != 1 || transactions.begins != 1 || transactions.unit.commits != 1 || transactions.unit.closes != 1 {
		t.Fatalf("security=%d begins=%d unit=%+v", security.calls, transactions.begins, transactions.unit)
	}
}

func TestExecutorRequiredIdempotencySuppressesDuplicateExecution(t *testing.T) {
	store := execution.NewMemoryIdempotencyStore()
	coordinator, err := execution.NewIdempotencyCoordinator(store)
	if err != nil { t.Fatal(err) }
	runtime := NewExecutorWithOptions(nil, ExecutorOptions{Idempotency: coordinator})
	plan := operationplan.Plan{OperationID: "public.create", Security: operationplan.Security{Public: true, PermissionMode: "all"}, Execution: operationplan.Execution{Transaction: "none", Idempotency: "required"}}
	ctx := execution.WithIdempotencyKey(context.Background(), "request-1")
	called := 0
	if _, err := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { called++; return "ok", nil }); err != nil { t.Fatal(err) }
	if _, err := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { called++; return "duplicate", nil }); !errors.Is(err, execution.ErrIdempotencyCompleted) {
		t.Fatalf("duplicate err=%v", err)
	}
	if called != 1 { t.Fatalf("application calls=%d", called) }
}
''')
edit(
    "framework/operation/executor_test.go",
    '"yunka.io/framework/core/runtimecontext"\n\t"yunka.io/pkg/operationplan"',
    '"yunka.io/framework/core/runtimecontext"\n\t"yunka.io/framework/execution"\n\t"yunka.io/pkg/operationplan"',
)

print("C9.7 execution runtime source staged")
