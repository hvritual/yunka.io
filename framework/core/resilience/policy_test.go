package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	grpcgo "google.golang.org/grpc"
	"yunka.io/framework/core/middleware"
	"yunka.io/framework/core/runtimecontext"
)

func TestRPCPolicyRetriesIdempotentUnaryCallAndKeepsKeyedState(t *testing.T) {
	transient := errors.New("transient")
	attempts := 0
	policy := NewRPCPolicy(RPCPolicyConfig{
		Retry:     RetryConfig{MaxAttempts: 2, Idempotent: func(context.Context) bool { return true }, Retryable: func(err error) bool { return errors.Is(err, transient) }},
		Circuit:   CircuitBreakerConfig{Enabled: true, FailureThreshold: 5},
		RateLimit: RateLimitConfig{Enabled: true, Rate: 1000, Burst: 10},
		LoadShed:  LoadShedConfig{Enabled: true, MinLimit: 1, MaxLimit: 10, InitialLimit: 10},
	})
	ctx := runtimecontext.WithMetadata(context.Background(), runtimecontext.Metadata{Operation: "/svc/Get"})
	err := policy.UnaryClientInterceptor()(ctx, "/svc/Get", nil, nil, nil,
		func(context.Context, string, interface{}, interface{}, *grpcgo.ClientConn, ...grpcgo.CallOption) error {
			attempts++
			if attempts < 2 {
				return transient
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
	snapshot := policy.Snapshot("/svc/Get")
	if snapshot.Circuit.State != CircuitClosed || snapshot.Load.Limit == 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestRPCPolicyRetriesShareOneTimeoutBudget(t *testing.T) {
	transient := errors.New("transient")
	attempts := 0
	policy := NewRPCPolicy(RPCPolicyConfig{
		Timeout: TimeoutBudgetConfig{Timeout: 45 * time.Millisecond},
		Retry: RetryConfig{
			MaxAttempts: 5,
			Idempotent:  func(context.Context) bool { return true },
			Retryable:   func(err error) bool { return errors.Is(err, transient) },
			BaseDelay:   20 * time.Millisecond,
		},
	})
	started := time.Now()
	err := middleware.New(policy.Middlewares()...).Handle(context.Background(), func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			return transient
		}
		select {
		case <-time.After(40 * time.Millisecond):
			return transient
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v attempts=%d elapsed=%v", err, attempts, elapsed)
	}
	if elapsed > 90*time.Millisecond {
		t.Fatalf("retry reset timeout budget: elapsed=%v", elapsed)
	}
}
