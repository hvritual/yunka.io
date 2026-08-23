package requestscope

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/logExt"
)

var (
	ErrFactoryUnavailable = errors.New("requestscope: factory is unavailable")
	ErrScopeClosed        = errors.New("requestscope: scope is closed")
	ErrScopeCommitted     = errors.New("requestscope: scope is already committed")
	ErrScopeRolledBack    = errors.New("requestscope: scope is already rolled back")
)

// UnitOfWork is the request-owned transaction boundary. Implementations may
// wrap GORM, database/sql, or another transactional store, but they must not be
// shared between requests or retained by singleton services.
type UnitOfWork interface {
	Commit(context.Context) error
	Rollback(context.Context) error
	Close() error
}

// UnitOfWorkFactory begins one request-owned UnitOfWork.
type UnitOfWorkFactory interface {
	Begin(context.Context) (UnitOfWork, error)
}

// RepositoryFactory builds the typed repository set for one UnitOfWork.
type RepositoryFactory[R any] func(context.Context, UnitOfWork) (R, error)

// LoggerFactory may derive a request logger from trusted context state. It is
// optional; nil means the Scope has no logger.
type LoggerFactory func(context.Context, identity.Principal, bool, runtimecontext.Metadata, bool) logExt.Logger

// ScopeFactory is the typed seam consumed by handlers.
type ScopeFactory[R any] interface {
	New(context.Context) (*Scope[R], error)
}

// Factory owns no mutable request state. Every New call begins a fresh Unit of
// Work and creates a fresh typed repository set.
type Factory[R any] struct {
	unitOfWork   UnitOfWorkFactory
	repositories RepositoryFactory[R]
	logger       LoggerFactory
}

type FactoryOptions[R any] struct {
	UnitOfWork   UnitOfWorkFactory
	Repositories RepositoryFactory[R]
	Logger       LoggerFactory
}

func NewFactory[R any](options FactoryOptions[R]) (*Factory[R], error) {
	if options.UnitOfWork == nil {
		return nil, errors.New("requestscope: unit of work factory is required")
	}
	if options.Repositories == nil {
		return nil, errors.New("requestscope: repository factory is required")
	}
	return &Factory[R]{
		unitOfWork:   options.UnitOfWork,
		repositories: options.Repositories,
		logger:       options.Logger,
	}, nil
}

func (factory *Factory[R]) New(ctx context.Context) (*Scope[R], error) {
	if factory == nil || factory.unitOfWork == nil || factory.repositories == nil {
		return nil, ErrFactoryUnavailable
	}
	ctx = normalizeContext(ctx)
	unitOfWork, err := factory.unitOfWork.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("requestscope: begin unit of work: %w", err)
	}
	if unitOfWork == nil {
		return nil, errors.New("requestscope: unit of work factory returned nil")
	}

	// Repository and logger construction are application code and may panic.
	// Until the Scope has been returned successfully, this Factory owns the
	// Unit of Work and must clean it up without losing the original panic.
	constructed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			if !constructed {
				cleanupCtx := cleanupContext(ctx)
				_ = safeUnitOfWorkCall("rollback scope construction", func() error {
					return unitOfWork.Rollback(cleanupCtx)
				})
				_ = safeUnitOfWorkCall("close scope construction", unitOfWork.Close)
			}
			panic(recovered)
		}
	}()

	repositories, err := factory.repositories(ctx, unitOfWork)
	if err != nil {
		cleanupCtx := cleanupContext(ctx)
		cleanupErr := errors.Join(safeUnitOfWorkCall("rollback repository build", func() error {
			return unitOfWork.Rollback(cleanupCtx)
		}), safeUnitOfWorkCall("close repository build", unitOfWork.Close))
		return nil, errors.Join(fmt.Errorf("requestscope: build repositories: %w", err), cleanupErr)
	}
	principal, hasPrincipal := identity.FromContext(ctx)
	metadata, hasMetadata := runtimecontext.MetadataFrom(ctx)
	var logger logExt.Logger
	if factory.logger != nil {
		logger = factory.logger(ctx, principal.Clone(), hasPrincipal, metadata.Clone(), hasMetadata)
	}
	scope := &Scope[R]{
		ctx:          ctx,
		unitOfWork:   unitOfWork,
		repositories: repositories,
		logger:       logger,
		principal:    principal.Clone(),
		hasPrincipal: hasPrincipal,
		metadata:     metadata.Clone(),
		hasMetadata:  hasMetadata,
		traceID:      runtimecontext.TraceIDFrom(ctx),
	}
	constructed = true
	return scope, nil
}

type finalAction uint8

const (
	finalActionNone finalAction = iota
	finalActionCommit
	finalActionCommitFailed
	finalActionRollback
)

// Scope contains one request's trusted identity snapshot, operation metadata,
// Unit of Work, and typed repositories. Scope is concurrency-safe for cleanup,
// but business use should remain request-confined.
type Scope[R any] struct {
	ctx          context.Context
	unitOfWork   UnitOfWork
	repositories R
	logger       logExt.Logger
	principal    identity.Principal
	hasPrincipal bool
	metadata     runtimecontext.Metadata
	hasMetadata  bool
	traceID      string

	mu          sync.Mutex
	action      finalAction
	commitErr   error
	rollbackErr error
	closed      bool
	closeErr    error
}

func (scope *Scope[R]) Context() context.Context {
	if scope == nil {
		return context.Background()
	}
	return scope.ctx
}

func (scope *Scope[R]) Repositories() R {
	if scope == nil {
		var zero R
		return zero
	}
	return scope.repositories
}

func (scope *Scope[R]) Logger() logExt.Logger {
	if scope == nil {
		return nil
	}
	return scope.logger
}

func (scope *Scope[R]) Principal() (identity.Principal, bool) {
	if scope == nil {
		return identity.Principal{}, false
	}
	return scope.principal.Clone(), scope.hasPrincipal
}

func (scope *Scope[R]) Metadata() (runtimecontext.Metadata, bool) {
	if scope == nil {
		return runtimecontext.Metadata{}, false
	}
	return scope.metadata.Clone(), scope.hasMetadata
}

func (scope *Scope[R]) TraceID() string {
	if scope == nil {
		return ""
	}
	return scope.traceID
}

func (scope *Scope[R]) UnitOfWork() UnitOfWork {
	if scope == nil {
		return nil
	}
	return scope.unitOfWork
}

func (scope *Scope[R]) Commit() error {
	if scope == nil || scope.unitOfWork == nil {
		return ErrScopeClosed
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	switch scope.action {
	case finalActionCommit, finalActionCommitFailed:
		return scope.commitErr
	case finalActionRollback:
		return errors.Join(ErrScopeRolledBack, scope.commitErr, scope.rollbackErr)
	}
	if scope.closed {
		return ErrScopeClosed
	}
	scope.commitErr = safeUnitOfWorkCall("commit", func() error {
		return scope.unitOfWork.Commit(scope.ctx)
	})
	if scope.commitErr != nil {
		scope.action = finalActionCommitFailed
	} else {
		scope.action = finalActionCommit
	}
	return scope.commitErr
}

func (scope *Scope[R]) Rollback() error {
	if scope == nil || scope.unitOfWork == nil {
		return ErrScopeClosed
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	switch scope.action {
	case finalActionRollback:
		return scope.rollbackErr
	case finalActionCommit:
		return errors.Join(ErrScopeCommitted, scope.commitErr)
	}
	if scope.closed {
		return ErrScopeClosed
	}
	// A failed commit remains rollback-eligible. The commit error is preserved
	// separately so callers receive both the failed commit and cleanup result.
	scope.action = finalActionRollback
	scope.rollbackErr = safeUnitOfWorkCall("rollback", func() error {
		return scope.unitOfWork.Rollback(cleanupContext(scope.ctx))
	})
	return scope.rollbackErr
}

// Close is idempotent. An unfinalized scope is rolled back before the Unit of
// Work is closed. A previously committed or rolled-back scope is only closed.
func (scope *Scope[R]) Close() error {
	if scope == nil || scope.unitOfWork == nil {
		return nil
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.closed {
		return scope.closeErr
	}
	var cleanupErr error
	if scope.action == finalActionNone || scope.action == finalActionCommitFailed {
		scope.action = finalActionRollback
		scope.rollbackErr = safeUnitOfWorkCall("rollback on close", func() error {
			return scope.unitOfWork.Rollback(cleanupContext(scope.ctx))
		})
		cleanupErr = scope.rollbackErr
	}
	cleanupErr = errors.Join(cleanupErr, safeUnitOfWorkCall("close", scope.unitOfWork.Close))
	scope.closed = true
	scope.closeErr = cleanupErr
	return cleanupErr
}

func safeUnitOfWorkCall(name string, call func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("requestscope: %s panicked: %v", name, recovered)
		}
	}()
	return call()
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// cleanupContext preserves trusted values while detaching rollback and close
// operations from request cancellation. Cleanup must still run after deadline
// exhaustion or client disconnect.
func cleanupContext(ctx context.Context) context.Context {
	return context.WithoutCancel(normalizeContext(ctx))
}
