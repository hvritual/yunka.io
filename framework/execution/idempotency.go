package execution

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
