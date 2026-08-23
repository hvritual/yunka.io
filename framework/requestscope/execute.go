package requestscope

import (
	"context"
	"errors"
)

// Execute runs one request callback and applies deterministic transaction
// semantics: commit on success, rollback on error, rollback+close on panic, and
// close exactly once on every acquired scope.
func Execute[R any](ctx context.Context, factory ScopeFactory[R], call func(*Scope[R]) error) (err error) {
	if factory == nil {
		return ErrFactoryUnavailable
	}
	if call == nil {
		return errors.New("requestscope: callback is required")
	}
	scope, err := factory.New(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = scope.Rollback()
			_ = scope.Close()
			panic(recovered)
		}
	}()
	if err := call(scope); err != nil {
		return errors.Join(err, scope.Rollback(), scope.Close())
	}
	if commitErr := scope.Commit(); commitErr != nil {
		return errors.Join(commitErr, scope.Rollback(), scope.Close())
	}
	return scope.Close()
}

// ExecuteValue is Execute with a typed result.
func ExecuteValue[R any, T any](ctx context.Context, factory ScopeFactory[R], call func(*Scope[R]) (T, error)) (result T, err error) {
	if factory == nil {
		return result, ErrFactoryUnavailable
	}
	if call == nil {
		return result, errors.New("requestscope: callback is required")
	}
	scope, err := factory.New(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = scope.Rollback()
			_ = scope.Close()
			panic(recovered)
		}
	}()
	result, err = call(scope)
	if err != nil {
		return result, errors.Join(err, scope.Rollback(), scope.Close())
	}
	if commitErr := scope.Commit(); commitErr != nil {
		return result, errors.Join(commitErr, scope.Rollback(), scope.Close())
	}
	return result, scope.Close()
}
