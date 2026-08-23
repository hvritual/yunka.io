//go:build integration

package outbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)

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
	assertConcurrentClaimPartition(t, 2, 6)
}

func TestGORMStoreConcurrentClaimStressIntegration(t *testing.T) {
	assertConcurrentClaimPartition(t, 10, 10)
}

func assertConcurrentClaimPartition(t *testing.T, workers, recordsPerWorker int) {
	t.Helper()
	store := integrationStore(t, WithSkipLocked(true))
	ctx := context.Background()
	total := workers * recordsPerWorker
	for index := 0; index < total; index++ {
		id := fmt.Sprintf("event-%04d", index)
		if err := store.Enqueue(ctx, integrationEnvelope(t, id)); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC().Add(time.Second)
	claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	type claimResult struct {
		owner   string
		records []Record
		err     error
	}
	// A successful claim contributes at least one record, so total+workers
	// bounds all positive batches plus one terminal error per worker.
	results := make(chan claimResult, total+workers)
	var wg sync.WaitGroup
	var claimed atomic.Int64
	for index := 0; index < workers; index++ {
		owner := fmt.Sprintf("worker-%02d", index)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				if claimed.Load() >= int64(total) {
					return
				}
				records, err := store.Claim(claimCtx, ClaimOptions{
					Owner: owner,
					Limit: recordsPerWorker,
					Lease: time.Minute,
					Now:   now,
				})
				if err != nil {
					results <- claimResult{owner: owner, err: err}
					cancel()
					return
				}
				if len(records) == 0 {
					// SKIP LOCKED may transiently return an empty or short batch
					// while another claim transaction owns the eligible index range.
					if claimed.Load() >= int64(total) {
						return
					}
					select {
					case <-claimCtx.Done():
						results <- claimResult{owner: owner, err: fmt.Errorf("claim convergence: %w", claimCtx.Err())}
						return
					case <-time.After(time.Millisecond):
					}
					continue
				}
				claimed.Add(int64(len(records)))
				results <- claimResult{owner: owner, records: records}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	seen := make(map[string]string, total)
	for current := range results {
		if current.err != nil {
			t.Fatalf("%s claim: %v", current.owner, current.err)
		}
		if len(current.records) > recordsPerWorker {
			t.Fatalf("%s claimed=%d exceeds limit %d", current.owner, len(current.records), recordsPerWorker)
		}
		for _, record := range current.records {
			if owner, exists := seen[record.ID]; exists {
				t.Fatalf("event %s claimed twice; first owner=%s second owner=%s", record.ID, owner, current.owner)
			}
			if record.LeaseOwner != current.owner || record.Attempts != 1 || record.Status != StatusInFlight {
				t.Fatalf("record=%+v worker=%s", record, current.owner)
			}
			seen[record.ID] = current.owner
		}
	}
	if got := claimed.Load(); got != int64(total) {
		t.Fatalf("claim counter=%d want %d", got, total)
	}
	if len(seen) != total {
		t.Fatalf("claimed=%d want %d", len(seen), total)
	}

	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 0 || snapshot.InFlight != int64(total) {
		t.Fatalf("snapshot=%+v want inFlight=%d", snapshot, total)
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
