package resilience

import (
	"context"
	"errors"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

func TestRetryRequiresExplicitIdempotency(t *testing.T) {
	transient := errors.New("transient")
	attempts := 0
	config := RetryConfig{MaxAttempts: 3, Retryable: func(err error) bool { return errors.Is(err, transient) }}
	err := middleware.New(Retry(config)).Handle(context.Background(), func(context.Context) error {
		attempts++
		return transient
	})
	if !errors.Is(err, transient) || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestRetryUsesSharedContextAndAttemptNumber(t *testing.T) {
	transient := errors.New("transient")
	metadata := runtimecontext.Metadata{Operation: "/svc/Get", Attributes: map[string]string{"resilience.idempotent": "true"}}
	ctx := runtimecontext.WithMetadata(context.Background(), metadata)
	attempts := 0
	err := middleware.New(Retry(RetryConfig{
		MaxAttempts: 3,
		Retryable:   func(err error) bool { return errors.Is(err, transient) },
	})).Handle(ctx, func(ctx context.Context) error {
		attempts++
		if AttemptFrom(ctx) != attempts {
			t.Fatalf("attempt context=%d want=%d", AttemptFrom(ctx), attempts)
		}
		if attempts < 3 {
			return transient
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}
