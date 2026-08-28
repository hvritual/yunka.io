package execution

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
