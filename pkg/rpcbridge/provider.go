package rpcbridge

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrProviderUnavailable = errors.New("rpc bridge: provider is unavailable")

// ReleaseFunc releases one acquired service or request scope. The call error is
// supplied so request-scoped transactions and finish hooks can make the same
// commit/rollback decision as the original runtime path. A non-nil return is
// treated as the final call error; nil preserves the original call result.
type ReleaseFunc func(callErr error) error

// Provider acquires one typed service for a single RPC call. Implementations
// own the service lifetime and must return a release function whenever an
// acquisition succeeds.
type Provider[T any] interface {
	Acquire(context.Context) (T, ReleaseFunc, error)
}

// ProviderFunc adapts a function to Provider.
type ProviderFunc[T any] func(context.Context) (T, ReleaseFunc, error)

func (provider ProviderFunc[T]) Acquire(ctx context.Context) (T, ReleaseFunc, error) {
	var zero T
	if provider == nil {
		return zero, nil, ErrProviderUnavailable
	}
	return provider(ctx)
}

// Static returns a provider for an already-constructed, application-owned
// service. It is useful for explicitly composed services and typed tests.
func Static[T any](service T) Provider[T] {
	return ProviderFunc[T](func(context.Context) (T, ReleaseFunc, error) {
		return service, NoopRelease, nil
	})
}

// NoopRelease is the release function for application-owned static services.
func NoopRelease(error) error { return nil }

// Once makes a release function idempotent. The first caller supplies the call
// error used by the underlying release; later calls return the first release
// result without executing cleanup again.
func Once(release ReleaseFunc) ReleaseFunc {
	if release == nil {
		release = NoopRelease
	}
	var (
		once sync.Once
		err  error
	)
	return func(callErr error) error {
		once.Do(func() {
			err = release(callErr)
		})
		return err
	}
}

// SafeRelease turns a cleanup panic into an error so a bad compatibility
// provider cannot bypass the RPC lifecycle cleanup boundary.
func SafeRelease(release ReleaseFunc, callErr error) (err error) {
	if release == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Join(callErr, fmt.Errorf("rpc bridge: release panicked: %v", recovered))
		}
	}()
	return release(callErr)
}
