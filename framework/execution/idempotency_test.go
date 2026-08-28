package execution

import (
	"context"
	"errors"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/pkg/operationplan"
)

func TestIdempotencyCoordinatorClaimsCompletesAndAllowsFailedRetry(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	coordinator, err := NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"})
	ctx = WithIdempotencyKey(ctx, "request-1")
	plan := operationplan.Plan{OperationID: "device.create"}
	claimed, err := coordinator.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Begin(ctx, plan); !errors.Is(err, ErrIdempotencyInProgress) {
		t.Fatalf("in-progress err=%v", err)
	}
	if err := coordinator.Complete(claimed, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Begin(ctx, plan); !errors.Is(err, ErrIdempotencyCompleted) {
		t.Fatalf("completed err=%v", err)
	}

	ctx2 := WithIdempotencyKey(ctx, "request-2")
	claimed2, err := coordinator.Begin(ctx2, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Fail(claimed2, plan, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Begin(ctx2, plan); err != nil {
		t.Fatalf("failed request should be retryable: %v", err)
	}
}

func TestMemoryIdempotencyStoreFencesPreviousAttemptAfterRetry(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	coordinator, err := NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"})
	ctx = WithIdempotencyKey(ctx, "request-fenced")
	plan := operationplan.Plan{OperationID: "device.create"}

	first, err := coordinator.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	firstIdentity, ok := idempotencyIdentityFrom(first)
	if !ok {
		t.Fatal("first claim identity missing")
	}
	if err := coordinator.Fail(first, plan, errors.New("retry")); err != nil {
		t.Fatal(err)
	}
	second, err := coordinator.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	secondIdentity, ok := idempotencyIdentityFrom(second)
	if !ok {
		t.Fatal("second claim identity missing")
	}
	if firstIdentity.Attempt == secondIdentity.Attempt {
		t.Fatal("retry reused fencing attempt")
	}
	if err := store.Mark(context.Background(), firstIdentity, IdempotencySucceeded); !errors.Is(err, ErrIdempotencyLeaseLost) {
		t.Fatalf("stale worker mark err=%v", err)
	}
	if err := coordinator.Complete(second, plan); err != nil {
		t.Fatal(err)
	}
}
