package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"yunka.io/framework/core/identity"
	"yunka.io/pkg/operationplan"
)

var (
	ErrIdempotencyUnavailable       = errors.New("execution: idempotency coordinator unavailable")
	ErrIdempotencyKeyRequired       = errors.New("execution: idempotency key required")
	ErrIdempotencyInProgress        = errors.New("execution: idempotent operation already in progress")
	ErrIdempotencyCompleted         = errors.New("execution: idempotent operation already completed")
	ErrIdempotencyAtomicUnavailable = errors.New("execution: atomic idempotency finalization unavailable")
	ErrIdempotencyLeaseLost         = errors.New("execution: idempotency claim lease lost")
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

// IdempotencyIdentity is the stable operation identity plus one opaque claim
// attempt used for fencing. Stores must key records by TenantID + OperationID +
// Key and use Attempt only to reject stale workers after lease takeover.
type IdempotencyIdentity struct {
	TenantID    string
	OperationID string
	Key         string
	Attempt     string
}

type IdempotencyStore interface {
	Claim(context.Context, IdempotencyIdentity) error
	Mark(context.Context, IdempotencyIdentity, IdempotencyState) error
	Lookup(context.Context, IdempotencyIdentity) (IdempotencyState, bool, error)
}

// TransactionalIdempotencyStore can stage a terminal idempotency state in the
// same local transaction used by the business mutation and Outbox staging.
// transaction is deliberately opaque to the execution core; adapter packages
// own concrete database types.
type TransactionalIdempotencyStore interface {
	IdempotencyStore
	MarkTx(context.Context, any, IdempotencyIdentity, IdempotencyState) error
}

type IdempotencyCoordinator interface {
	Begin(context.Context, operationplan.Plan) (context.Context, error)
	Complete(context.Context, operationplan.Plan) error
	Fail(context.Context, operationplan.Plan, error) error
}

// AtomicIdempotencyCoordinator is an optional capability used by the Executor
// when a required-idempotency Operation owns a local transaction. A successful
// call only stages the success marker; transaction commit remains owned by the
// root ExecutionScope.
type AtomicIdempotencyCoordinator interface {
	IdempotencyCoordinator
	CompleteInTransaction(context.Context, operationplan.Plan, any) error
}

// IdempotencyCapabilityReporter lets the Executor distinguish a durable store
// that can join the business transaction from compatibility stores that only
// support post-commit completion.
type IdempotencyCapabilityReporter interface {
	SupportsAtomicCompletion() bool
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
	attempt, err := newIdempotencyAttempt()
	if err != nil {
		return nil, err
	}
	principal, _ := identity.FromContext(ctx)
	claim := IdempotencyIdentity{
		TenantID:    strings.TrimSpace(principal.TenantID),
		OperationID: strings.TrimSpace(plan.OperationID),
		Key:         key,
		Attempt:     attempt,
	}
	if err := coordinator.store.Claim(ctx, claim); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, idempotencyIdentityContext{}, claim), nil
}

func (coordinator *coordinator) Complete(ctx context.Context, _ operationplan.Plan) error {
	claim, ok := idempotencyIdentityFrom(ctx)
	if !ok {
		return ErrIdempotencyKeyRequired
	}
	return coordinator.store.Mark(ctx, claim, IdempotencySucceeded)
}

func (coordinator *coordinator) SupportsAtomicCompletion() bool {
	if coordinator == nil || coordinator.store == nil {
		return false
	}
	_, ok := coordinator.store.(TransactionalIdempotencyStore)
	return ok
}

func (coordinator *coordinator) CompleteInTransaction(ctx context.Context, _ operationplan.Plan, transaction any) error {
	claim, ok := idempotencyIdentityFrom(ctx)
	if !ok {
		return ErrIdempotencyKeyRequired
	}
	store, ok := coordinator.store.(TransactionalIdempotencyStore)
	if !ok {
		return ErrIdempotencyAtomicUnavailable
	}
	if transaction == nil {
		return ErrIdempotencyAtomicUnavailable
	}
	return store.MarkTx(ctx, transaction, claim, IdempotencySucceeded)
}

func (coordinator *coordinator) Fail(ctx context.Context, _ operationplan.Plan, _ error) error {
	claim, ok := idempotencyIdentityFrom(ctx)
	if !ok {
		return nil
	}
	return coordinator.store.Mark(ctx, claim, IdempotencyFailed)
}

func idempotencyIdentityFrom(ctx context.Context) (IdempotencyIdentity, bool) {
	if ctx == nil {
		return IdempotencyIdentity{}, false
	}
	value, ok := ctx.Value(idempotencyIdentityContext{}).(IdempotencyIdentity)
	return value, ok && value.OperationID != "" && value.Key != "" && value.Attempt != ""
}

func newIdempotencyAttempt() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("execution: create idempotency attempt: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

type memoryIdempotencyKey struct {
	TenantID    string
	OperationID string
	Key         string
}

type memoryIdempotencyRecord struct {
	State   IdempotencyState
	Attempt string
}

type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[memoryIdempotencyKey]memoryIdempotencyRecord
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{records: map[memoryIdempotencyKey]memoryIdempotencyRecord{}}
}

func memoryKey(identity IdempotencyIdentity) memoryIdempotencyKey {
	return memoryIdempotencyKey{TenantID: identity.TenantID, OperationID: identity.OperationID, Key: identity.Key}
}

func (store *MemoryIdempotencyStore) Claim(_ context.Context, identity IdempotencyIdentity) error {
	if store == nil {
		return ErrIdempotencyUnavailable
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.records == nil {
		store.records = map[memoryIdempotencyKey]memoryIdempotencyRecord{}
	}
	key := memoryKey(identity)
	record := store.records[key]
	switch record.State {
	case IdempotencyRunning:
		return ErrIdempotencyInProgress
	case IdempotencySucceeded:
		return ErrIdempotencyCompleted
	case IdempotencyFailed, "":
		store.records[key] = memoryIdempotencyRecord{State: IdempotencyRunning, Attempt: identity.Attempt}
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
		store.records = map[memoryIdempotencyKey]memoryIdempotencyRecord{}
	}
	key := memoryKey(identity)
	record := store.records[key]
	if record.State != IdempotencyRunning || record.Attempt != identity.Attempt {
		return ErrIdempotencyLeaseLost
	}
	store.records[key] = memoryIdempotencyRecord{State: state, Attempt: identity.Attempt}
	return nil
}

func (store *MemoryIdempotencyStore) Lookup(_ context.Context, identity IdempotencyIdentity) (IdempotencyState, bool, error) {
	if store == nil {
		return "", false, ErrIdempotencyUnavailable
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[memoryKey(identity)]
	return record.State, ok, nil
}
