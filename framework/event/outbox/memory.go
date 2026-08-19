package outbox

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"yunka.io/framework/event"
)

// MemoryStore is deterministic and useful for tests/dev. It deliberately does
// not implement TransactionalStore and must not be described as durable or
// atomic with a business database transaction.
type MemoryStore struct {
	mu      sync.Mutex
	records map[string]Record
	now     func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]Record), now: func() time.Time { return time.Now().UTC() }}
}

func (store *MemoryStore) Enqueue(ctx context.Context, envelope event.Envelope) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	normalized, err := envelope.Normalize()
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.records[normalized.ID]; exists {
		return ErrDuplicate
	}
	now := store.now()
	store.records[normalized.ID] = Record{ID: normalized.ID, Envelope: normalized.Clone(), Status: StatusPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
	return nil
}

func (store *MemoryStore) Claim(ctx context.Context, options ClaimOptions) ([]Record, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	options = options.normalize()
	if options.Owner == "" {
		return nil, ErrInvalidTx
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	ready := make([]Record, 0)
	for _, record := range store.records {
		pending := record.Status == StatusPending && !record.NextAttemptAt.After(options.Now)
		expired := record.Status == StatusInFlight && !record.LeaseUntil.After(options.Now)
		if pending || expired {
			ready = append(ready, record)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].NextAttemptAt.Equal(ready[j].NextAttemptAt) {
			if ready[i].CreatedAt.Equal(ready[j].CreatedAt) {
				return ready[i].ID < ready[j].ID
			}
			return ready[i].CreatedAt.Before(ready[j].CreatedAt)
		}
		return ready[i].NextAttemptAt.Before(ready[j].NextAttemptAt)
	})
	if len(ready) > options.Limit {
		ready = ready[:options.Limit]
	}
	result := make([]Record, 0, len(ready))
	for _, record := range ready {
		record.Status = StatusInFlight
		record.Attempts++
		record.LeaseOwner = options.Owner
		record.LeaseUntil = options.Now.Add(options.Lease)
		record.UpdatedAt = options.Now
		store.records[record.ID] = record
		result = append(result, record.Clone())
	}
	return result, nil
}

func (store *MemoryStore) MarkPublished(ctx context.Context, id, owner string, at time.Time) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if at.IsZero() {
		at = store.now()
	}
	at = at.UTC()
	return store.changeOwned(id, owner, func(record *Record) {
		record.Status = StatusPublished
		record.PublishedAt = at
		record.LeaseOwner = ""
		record.LeaseUntil = time.Time{}
		record.LastError = ""
		record.UpdatedAt = at
	})
}
func (store *MemoryStore) Retry(ctx context.Context, id, owner string, next time.Time, cause error) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if next.IsZero() {
		next = store.now()
	}
	next = next.UTC()
	return store.changeOwned(id, owner, func(record *Record) {
		record.Status = StatusPending
		record.NextAttemptAt = next
		record.LeaseOwner = ""
		record.LeaseUntil = time.Time{}
		record.LastError = errorText(cause)
		record.UpdatedAt = store.now()
	})
}
func (store *MemoryStore) DeadLetter(ctx context.Context, id, owner string, cause error) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	return store.changeOwned(id, owner, func(record *Record) {
		record.Status = StatusDeadLetter
		record.LeaseOwner = ""
		record.LeaseUntil = time.Time{}
		record.LastError = errorText(cause)
		record.UpdatedAt = store.now()
	})
}
func (store *MemoryStore) changeOwned(id, owner string, mutate func(*Record)) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	record, ok := store.records[id]
	if !ok {
		return ErrNotFound
	}
	if record.Status != StatusInFlight || strings.TrimSpace(owner) == "" || record.LeaseOwner != owner {
		return ErrLeaseLost
	}
	mutate(&record)
	store.records[id] = record
	return nil
}
func (store *MemoryStore) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := contextErr(ctx); err != nil {
		return Snapshot{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	var s Snapshot
	for _, r := range store.records {
		switch r.Status {
		case StatusPending:
			s.Pending++
			if s.OldestPendingAt.IsZero() || r.CreatedAt.Before(s.OldestPendingAt) {
				s.OldestPendingAt = r.CreatedAt
			}
		case StatusInFlight:
			s.InFlight++
		case StatusPublished:
			s.Published++
		case StatusDeadLetter:
			s.DeadLetter++
		}
	}
	return s, nil
}
func (store *MemoryStore) Record(id string) (Record, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	r, ok := store.records[id]
	return r.Clone(), ok
}
func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len(value) > 2048 {
		value = value[:2048]
	}
	return value
}

func (store *MemoryStore) PurgePublished(ctx context.Context, before time.Time, limit int) (int64, error) {
	if err := contextErr(ctx); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	type candidate struct {
		id string
		at time.Time
	}
	values := make([]candidate, 0)
	for id, record := range store.records {
		if record.Status == StatusPublished && !record.PublishedAt.IsZero() && record.PublishedAt.Before(before) {
			values = append(values, candidate{id: id, at: record.PublishedAt})
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].at.Equal(values[j].at) {
			return values[i].id < values[j].id
		}
		return values[i].at.Before(values[j].at)
	})
	if len(values) > limit {
		values = values[:limit]
	}
	for _, value := range values {
		delete(store.records, value.id)
	}
	return int64(len(values)), nil
}
