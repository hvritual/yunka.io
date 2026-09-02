//go:build integration

package idempotencygorm

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestMySQLIdempotencyStoreAtomicCompletionAndLeaseFencing(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("YUNKA_TEST_MYSQL_DSN is not configured")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	now := base
	store, err := NewStore(database, Options{LeaseDuration: time.Minute, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("DELETE FROM yunka_operation_idempotency").Error; err != nil {
		t.Fatal(err)
	}
	coordinatorValue, err := execution.NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	coordinator := coordinatorValue.(execution.AtomicIdempotencyCoordinator)
	plan := operationplan.Plan{OperationID: "device.create"}
	baseContext := identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"})
	baseContext = execution.WithIdempotencyKey(baseContext, "request-atomic")
	claimed, err := coordinator.Begin(baseContext, plan)
	if err != nil {
		t.Fatal(err)
	}

	transaction := database.Begin()
	if transaction.Error != nil {
		t.Fatal(transaction.Error)
	}
	if err := coordinator.CompleteInTransaction(claimed, plan, transaction); err != nil {
		_ = transaction.Rollback().Error
		t.Fatal(err)
	}
	if err := transaction.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Begin(baseContext, plan); !errors.Is(err, execution.ErrIdempotencyCompleted) {
		t.Fatalf("completed claim err=%v", err)
	}

	leaseContext := execution.WithIdempotencyKey(identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"}), "request-lease")
	first, err := coordinator.Begin(leaseContext, plan)
	if err != nil {
		t.Fatal(err)
	}
	now = base.Add(2 * time.Minute)
	second, err := coordinator.Begin(leaseContext, plan)
	if err != nil {
		t.Fatalf("expired lease should be reclaimable: %v", err)
	}
	if err := coordinator.Complete(first, plan); !errors.Is(err, execution.ErrIdempotencyLeaseLost) {
		t.Fatalf("stale worker completion err=%v", err)
	}
	if err := coordinator.Fail(second, plan, errors.New("cleanup")); err != nil {
		t.Fatal(err)
	}
}
