package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/framework/event"
)

func TestMemoryStoreLeaseAndRetryLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	envelope, _ := event.NewJSON("orders.created", "orders.created.v1", "orders", map[string]string{"id": "1"})
	if err := store.Enqueue(context.Background(), envelope); err != nil {
		t.Fatal(err)
	}
	if err := store.Enqueue(context.Background(), envelope); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate err=%v", err)
	}
	records, err := store.Claim(context.Background(), ClaimOptions{Owner: "w1", Limit: 1, Lease: time.Minute, Now: now})
	if err != nil || len(records) != 1 || records[0].Attempts != 1 {
		t.Fatalf("records=%#v err=%v", records, err)
	}
	if err := store.MarkPublished(context.Background(), envelope.ID, "w2", now); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("lease err=%v", err)
	}
	next := now.Add(time.Second)
	if err := store.Retry(context.Background(), envelope.ID, "w1", next, errors.New("temporary")); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), ClaimOptions{Owner: "w2", Now: next, Lease: time.Minute})
	if err != nil || len(claimed) != 1 || claimed[0].Attempts != 2 {
		t.Fatalf("claimed=%#v err=%v", claimed, err)
	}
	if err := store.MarkPublished(context.Background(), envelope.ID, "w2", next); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := store.Snapshot(context.Background())
	if snapshot.Published != 1 || snapshot.Pending != 0 || snapshot.InFlight != 0 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
}

func TestExpiredLeaseCanBeReclaimed(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	env, _ := event.NewJSON("t", "t.v1", "", map[string]int{"x": 1})
	_ = store.Enqueue(context.Background(), env)
	_, _ = store.Claim(context.Background(), ClaimOptions{Owner: "old", Now: now, Lease: time.Second})
	claimed, err := store.Claim(context.Background(), ClaimOptions{Owner: "new", Now: now.Add(2 * time.Second), Lease: time.Second})
	if err != nil || len(claimed) != 1 || claimed[0].Attempts != 2 || claimed[0].LeaseOwner != "new" {
		t.Fatalf("claimed=%#v err=%v", claimed, err)
	}
}

func TestMemoryStorePurgesOnlyOldPublishedRecords(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	first, _ := event.NewJSON("t", "t.v1", "", map[string]int{"n": 1})
	second, _ := event.NewJSON("t", "t.v1", "", map[string]int{"n": 2})
	_ = store.Enqueue(context.Background(), first)
	_ = store.Enqueue(context.Background(), second)
	rows, _ := store.Claim(context.Background(), ClaimOptions{Owner: "w", Limit: 2, Now: now})
	for _, row := range rows {
		_ = store.MarkPublished(context.Background(), row.ID, "w", now)
	}
	purged, err := store.PurgePublished(context.Background(), now.Add(time.Second), 1)
	if err != nil || purged != 1 {
		t.Fatalf("purged=%d err=%v", purged, err)
	}
	snapshot, _ := store.Snapshot(context.Background())
	if snapshot.Published != 1 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
}
