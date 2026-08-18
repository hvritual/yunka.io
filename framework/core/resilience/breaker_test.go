package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndRecoversThroughHalfOpen(t *testing.T) {
	failure := errors.New("downstream")
	now := time.Unix(100, 0)
	breaker := NewCircuitBreaker(CircuitBreakerConfig{Enabled: true, FailureThreshold: 2, SuccessThreshold: 1, OpenTimeout: time.Second})
	breaker.now = func() time.Time { return now }

	for index := 0; index < 2; index++ {
		if err := breaker.Execute(context.Background(), func(context.Context) error { return failure }); !errors.Is(err, failure) {
			t.Fatalf("failure err=%v", err)
		}
	}
	if snapshot := breaker.Snapshot(); snapshot.State != CircuitOpen {
		t.Fatalf("state=%s", snapshot.State)
	}
	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("open err=%v", err)
	}

	now = now.Add(time.Second)
	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("half-open probe err=%v", err)
	}
	if snapshot := breaker.Snapshot(); snapshot.State != CircuitClosed {
		t.Fatalf("state=%s", snapshot.State)
	}
}

func TestCircuitBreakerGroupIsolatesOperations(t *testing.T) {
	group := NewCircuitBreakerGroup(CircuitBreakerConfig{Enabled: true, FailureThreshold: 1})
	group.Breaker("a").complete(errors.New("boom"))
	if group.Breaker("a").Snapshot().State != CircuitOpen {
		t.Fatal("breaker a should be open")
	}
	if group.Breaker("b").Snapshot().State != CircuitClosed {
		t.Fatal("breaker b should remain closed")
	}
}

func TestCircuitBreakerReleasesHalfOpenProbeOnPanic(t *testing.T) {
	now := time.Unix(100, 0)
	breaker := NewCircuitBreaker(CircuitBreakerConfig{Enabled: true, FailureThreshold: 1, OpenTimeout: time.Second, HalfOpenMaxRequests: 1})
	breaker.now = func() time.Time { return now }
	_ = breaker.Execute(context.Background(), func(context.Context) error { return errors.New("boom") })
	now = now.Add(time.Second)
	func() {
		defer func() { _ = recover() }()
		_ = breaker.Execute(context.Background(), func(context.Context) error { panic("boom") })
	}()
	if snapshot := breaker.Snapshot(); snapshot.State != CircuitOpen || snapshot.HalfOpenInFlight != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
