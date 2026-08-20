//go:build integration

package outbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/event"
)

func integrationStore(t *testing.T, options ...GORMOption) *GORMStore {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("YUNKA_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("YUNKA_TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)

	table := fmt.Sprintf("yunka_outbox_it_%d", time.Now().UnixNano())
	options = append([]GORMOption{WithTable(table)}, options...)
	store, err := NewGORMStore(db, options...)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate integration table: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(table)
		_ = sqlDB.Close()
	})
	return store
}

func integrationEnvelope(t *testing.T, id string) event.Envelope {
	t.Helper()
	envelope, err := event.NewJSON(
		"integration.event",
		"integration.event.v1",
		"test",
		map[string]string{"id": id},
	)
	if err != nil {
		t.Fatal(err)
	}
	envelope.ID = id
	envelope, err = envelope.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func TestGORMStoreConcurrentClaimIntegration(t *testing.T) {
	store := integrationStore(t, WithSkipLocked(true))
	ctx := context.Background()
	for index := 0; index < 12; index++ {
		id := fmt.Sprintf("event-%02d", index)
		if err := store.Enqueue(ctx, integrationEnvelope(t, id)); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC().Add(time.Second)
	start := make(chan struct{})
	type claimResult struct {
		records []Record
		err     error
	}
	results := make(chan claimResult, 2)
	var wg sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		owner := owner
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			records, err := store.Claim(ctx, ClaimOptions{
				Owner: owner,
				Limit: 6,
				Lease: time.Minute,
				Now:   now,
			})
			results <- claimResult{records: records, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	seen := make(map[string]string)
	for current := range results {
		if current.err != nil {
			t.Fatal(current.err)
		}
		for _, record := range current.records {
			if owner, exists := seen[record.ID]; exists {
				t.Fatalf("event %s claimed twice; first owner=%s", record.ID, owner)
			}
			seen[record.ID] = record.LeaseOwner
		}
	}
	if len(seen) != 12 {
		t.Fatalf("claimed=%d want 12", len(seen))
	}
}

func TestGORMStoreLeaseReclaimIntegration(t *testing.T) {
	store := integrationStore(t, WithSkipLocked(true))
	ctx := context.Background()
	envelope := integrationEnvelope(t, "lease-reclaim")
	if err := store.Enqueue(ctx, envelope); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Add(time.Second)
	first, err := store.Claim(ctx, ClaimOptions{
		Owner: "worker-a",
		Limit: 1,
		Lease: time.Second,
		Now:   now,
	})
	if err != nil || len(first) != 1 {
		t.Fatalf("first claim records=%d err=%v", len(first), err)
	}

	second, err := store.Claim(ctx, ClaimOptions{
		Owner: "worker-b",
		Limit: 1,
		Lease: time.Second,
		Now:   now.Add(2 * time.Second),
	})
	if err != nil || len(second) != 1 {
		t.Fatalf("reclaim records=%d err=%v", len(second), err)
	}
	if second[0].ID != envelope.ID ||
		second[0].LeaseOwner != "worker-b" ||
		second[0].Attempts != 2 {
		t.Fatalf("reclaimed=%+v", second[0])
	}
}
