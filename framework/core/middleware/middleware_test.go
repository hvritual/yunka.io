package middleware

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestChainOrderAndErrorPropagation(t *testing.T) {
	var calls []string
	wrap := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context) error {
				calls = append(calls, name+":before")
				err := next(ctx)
				calls = append(calls, name+":after")
				return err
			}
		}
	}
	wantErr := errors.New("terminal")
	chain := New(wrap("one"), nil, wrap("two"))
	err := chain.Handle(context.Background(), func(context.Context) error {
		calls = append(calls, "handler")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error=%v want=%v", err, wantErr)
	}
	want := []string{"one:before", "two:before", "handler", "two:after", "one:after"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestUseDoesNotMutateExistingChain(t *testing.T) {
	base := New(func(next Handler) Handler { return next })
	extended := base.Use(func(next Handler) Handler { return next })
	if len(base.middlewares) != 1 || len(extended.middlewares) != 2 {
		t.Fatalf("base=%d extended=%d", len(base.middlewares), len(extended.middlewares))
	}
}
