package requestscope

import (
	"context"
	"errors"
)

type Pair[A, B any] struct {
	First  A
	Second B
}
type Triple[A, B, C any] struct {
	First  A
	Second B
	Third  C
}

func Compose2[A, B any](first RepositoryFactory[A], second RepositoryFactory[B]) RepositoryFactory[Pair[A, B]] {
	return func(ctx context.Context, unit UnitOfWork) (Pair[A, B], error) {
		var zero Pair[A, B]
		if first == nil || second == nil {
			return zero, errors.New("requestscope: composed repository factory is required")
		}
		a, err := first(ctx, unit)
		if err != nil {
			return zero, err
		}
		b, err := second(ctx, unit)
		if err != nil {
			return zero, err
		}
		return Pair[A, B]{First: a, Second: b}, nil
	}
}

func Compose3[A, B, C any](first RepositoryFactory[A], second RepositoryFactory[B], third RepositoryFactory[C]) RepositoryFactory[Triple[A, B, C]] {
	return func(ctx context.Context, unit UnitOfWork) (Triple[A, B, C], error) {
		var zero Triple[A, B, C]
		if first == nil || second == nil || third == nil {
			return zero, errors.New("requestscope: composed repository factory is required")
		}
		a, err := first(ctx, unit)
		if err != nil {
			return zero, err
		}
		b, err := second(ctx, unit)
		if err != nil {
			return zero, err
		}
		c, err := third(ctx, unit)
		if err != nil {
			return zero, err
		}
		return Triple[A, B, C]{First: a, Second: b, Third: c}, nil
	}
}
