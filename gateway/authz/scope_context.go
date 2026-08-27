package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrAuthorizedScopeMissing = errors.New("gateway authz: authorized scope missing")

type ScopeKey[T any] struct{ name string }
type scopeContextKey[T any] struct{ name string }

func NewScopeKey[T any](name string) (ScopeKey[T], error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ScopeKey[T]{}, errors.New("gateway authz: scope key name is required")
	}
	return ScopeKey[T]{name: name}, nil
}

func MustScopeKey[T any](name string) ScopeKey[T] {
	key, err := NewScopeKey[T](name)
	if err != nil {
		panic(err)
	}
	return key
}

func (key ScopeKey[T]) Name() string { return key.name }

func (key ScopeKey[T]) With(ctx context.Context, value T) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, scopeContextKey[T]{name: key.name}, value)
}

func (key ScopeKey[T]) From(ctx context.Context) (T, bool) {
	var zero T
	if ctx == nil || key.name == "" {
		return zero, false
	}
	value, ok := ctx.Value(scopeContextKey[T]{name: key.name}).(T)
	return value, ok
}

func (key ScopeKey[T]) Require(ctx context.Context) (T, error) {
	if value, ok := key.From(ctx); ok {
		return value, nil
	}
	var zero T
	return zero, fmt.Errorf("%w: %s", ErrAuthorizedScopeMissing, key.name)
}
