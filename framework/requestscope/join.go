package requestscope

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
