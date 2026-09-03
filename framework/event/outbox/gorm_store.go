package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"github.com/hvritual/yunka.io/framework/event"
)

const DefaultTable = "yunka_outbox"

type GORMStore struct {
	db         *gorm.DB
	table      string
	skipLocked bool
	propagator event.Propagator
}

type GORMOption func(*GORMStore) error

func WithTable(table string) GORMOption {
	return func(store *GORMStore) error {
		table = strings.TrimSpace(table)
		if !safeIdentifier(table) {
			return fmt.Errorf("outbox: invalid table name %q", table)
		}
		store.table = table
		return nil
	}
}

// WithSkipLocked enables SELECT ... FOR UPDATE SKIP LOCKED during claims.
// Enable it only for database versions that support the clause, such as
// MySQL 8+. The compatibility default remains plain FOR UPDATE.
func WithSkipLocked(enabled bool) GORMOption {
	return func(store *GORMStore) error {
		store.skipLocked = enabled
		return nil
	}
}

// WithPropagator injects canonical distributed propagation metadata before an
// event is serialized into the durable Outbox. This happens at the original
// caller/transaction boundary rather than later in the dispatcher worker.
func WithPropagator(propagator event.Propagator) GORMOption {
	return func(store *GORMStore) error {
		store.propagator = propagator
		return nil
	}
}

func NewGORMStore(db *gorm.DB, options ...GORMOption) (*GORMStore, error) {
	if db == nil {
		return nil, errors.New("outbox: gorm database is required")
	}
	store := &GORMStore{db: db, table: DefaultTable}
	for _, option := range options {
		if option != nil {
			if err := option(store); err != nil {
				return nil, err
			}
		}
	}
	return store, nil
}

func safeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, current := range value {
		if (current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z') || (current >= '0' && current <= '9') || current == '_' {
			continue
		}
		return false
	}
	return true
}

func (store *GORMStore) AutoMigrate(ctx context.Context) error {
	if store == nil || store.db == nil {
		return errors.New("outbox: gorm store is nil")
	}
	return store.db.WithContext(nonNilContext(ctx)).Table(store.table).AutoMigrate(&gormRecord{})
}

func (store *GORMStore) Enqueue(ctx context.Context, envelope event.Envelope) error {
	if store == nil || store.db == nil {
		return errors.New("outbox: gorm store is nil")
	}
	ctx = nonNilContext(ctx)
	return store.insert(ctx, store.db.WithContext(ctx), envelope)
}

// EnqueueTx writes through an already-open *gorm.DB transaction. For atomic
// business-change + outbox semantics, callers must pass the exact transaction
// handle used by their repositories, not the base database handle.
func (store *GORMStore) EnqueueTx(ctx context.Context, tx any, envelope event.Envelope) error {
	db, ok := tx.(*gorm.DB)
	if !ok || db == nil {
		return ErrInvalidTx
	}
	ctx = nonNilContext(ctx)
	return store.insert(ctx, db.WithContext(ctx), envelope)
}

func (store *GORMStore) prepare(ctx context.Context, envelope event.Envelope) (event.Envelope, error) {
	if store == nil {
		return event.Envelope{}, errors.New("outbox: gorm store is nil")
	}
	return event.PrepareForPublish(nonNilContext(ctx), envelope, store.propagator)
}

func (store *GORMStore) insert(ctx context.Context, db *gorm.DB, envelope event.Envelope) error {
	normalized, err := store.prepare(ctx, envelope)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row := gormRecord{ID: normalized.ID, Topic: normalized.Topic, EventType: normalized.Type, EventJSON: encoded, Status: StatusPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now}
	result := db.Table(store.table).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDuplicate
	}
	return nil
}

// Claim returns at most options.Limit records. With SKIP LOCKED, a short
// or empty batch can be transient while another claim transaction holds eligible
// rows; callers must poll again rather than treating batch size as a fairness guarantee.
func (store *GORMStore) Claim(ctx context.Context, options ClaimOptions) ([]Record, error) {
	if store == nil || store.db == nil {
		return nil, errors.New("outbox: gorm store is nil")
	}
	options = options.normalize()
	if options.Owner == "" {
		return nil, ErrInvalidOwner
	}

	var claimed []gormRecord
	txOptions := &sql.TxOptions{}
	if store.skipLocked {
		txOptions.Isolation = sql.LevelReadCommitted
	}
	err := store.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		locking := clause.Locking{Strength: "UPDATE"}
		if store.skipLocked {
			locking.Options = "SKIP LOCKED"
		}

		// Lock only queue IDs through covering indexes. Selecting full rows
		// can force a filesort and make one worker lock every eligible row,
		// preventing concurrent SKIP LOCKED workers from partitioning work.
		var candidateIDs []string
		var expiredIDs []string
		query := tx.Table(store.table).
			Clauses(locking).
			Where("status = ? AND lease_until IS NOT NULL AND lease_until <= ?", StatusInFlight, options.Now).
			Order("lease_until ASC").Order("created_at ASC").Order("id ASC").Limit(options.Limit).
			Pluck("id", &expiredIDs)
		if query.Error != nil {
			return query.Error
		}
		candidateIDs = append(candidateIDs, expiredIDs...)

		remaining := options.Limit - len(candidateIDs)
		if remaining > 0 {
			var pendingIDs []string
			query = tx.Table(store.table).
				Clauses(locking).
				Where("status = ? AND next_attempt_at <= ?", StatusPending, options.Now).
				Order("next_attempt_at ASC").Order("created_at ASC").Order("id ASC").Limit(remaining).
				Pluck("id", &pendingIDs)
			if query.Error != nil {
				return query.Error
			}
			candidateIDs = append(candidateIDs, pendingIDs...)
		}

		var candidates []gormRecord
		if len(candidateIDs) > 0 {
			var loaded []gormRecord
			if err := tx.Table(store.table).Where("id IN ?", candidateIDs).Find(&loaded).Error; err != nil {
				return err
			}
			byID := make(map[string]gormRecord, len(loaded))
			for _, row := range loaded {
				byID[row.ID] = row
			}
			candidates = make([]gormRecord, 0, len(candidateIDs))
			for _, id := range candidateIDs {
				candidate, ok := byID[id]
				if !ok {
					return ErrLeaseLost
				}
				candidates = append(candidates, candidate)
			}
		}

		for _, candidate := range candidates {
			// Decode before mutating lease state. Corrupt event JSON rolls
			// back the entire claim transaction instead of creating a poison lease.
			if _, err := decodeGORMRecord(candidate); err != nil {
				return err
			}
			leaseUntil := options.Now.Add(options.Lease)
			attempts := candidate.Attempts + 1
			result := tx.Table(store.table).Where("id = ?", candidate.ID).Updates(map[string]any{
				"status": StatusInFlight, "attempts": attempts, "lease_owner": options.Owner,
				"lease_until": leaseUntil, "updated_at": options.Now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrLeaseLost
			}
			candidate.Status = StatusInFlight
			candidate.Attempts = attempts
			candidate.LeaseOwner = options.Owner
			candidate.LeaseUntil = &leaseUntil
			candidate.UpdatedAt = options.Now
			claimed = append(claimed, candidate)
		}
		return nil
	}, txOptions)
	if err != nil {
		return nil, err
	}

	result := make([]Record, 0, len(claimed))
	for _, row := range claimed {
		record, err := decodeGORMRecord(row)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}
func (store *GORMStore) MarkPublished(ctx context.Context, id, owner string, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	return store.updateOwned(ctx, id, owner, map[string]any{"status": StatusPublished, "published_at": at, "lease_owner": "", "lease_until": nil, "last_error": "", "updated_at": at})
}
func (store *GORMStore) Retry(ctx context.Context, id, owner string, next time.Time, cause error) error {
	if next.IsZero() {
		next = time.Now().UTC()
	} else {
		next = next.UTC()
	}
	return store.updateOwned(ctx, id, owner, map[string]any{"status": StatusPending, "next_attempt_at": next, "lease_owner": "", "lease_until": nil, "last_error": errorText(cause), "updated_at": time.Now().UTC()})
}
func (store *GORMStore) DeadLetter(ctx context.Context, id, owner string, cause error) error {
	return store.updateOwned(ctx, id, owner, map[string]any{"status": StatusDeadLetter, "lease_owner": "", "lease_until": nil, "last_error": errorText(cause), "updated_at": time.Now().UTC()})
}
func (store *GORMStore) updateOwned(ctx context.Context, id, owner string, values map[string]any) error {
	if store == nil || store.db == nil {
		return errors.New("outbox: gorm store is nil")
	}
	result := store.db.WithContext(nonNilContext(ctx)).Table(store.table).Where("id = ? AND status = ? AND lease_owner = ?", id, StatusInFlight, owner).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLeaseLost
	}
	return nil
}

func (store *GORMStore) Snapshot(ctx context.Context) (Snapshot, error) {
	if store == nil || store.db == nil {
		return Snapshot{}, errors.New("outbox: gorm store is nil")
	}
	db := store.db.WithContext(nonNilContext(ctx))
	type countRow struct {
		Status Status
		Count  int64
	}
	var counts []countRow
	if err := db.Table(store.table).Select("status, COUNT(*) AS count").Group("status").Scan(&counts).Error; err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	for _, row := range counts {
		switch row.Status {
		case StatusPending:
			snapshot.Pending = row.Count
		case StatusInFlight:
			snapshot.InFlight = row.Count
		case StatusPublished:
			snapshot.Published = row.Count
		case StatusDeadLetter:
			snapshot.DeadLetter = row.Count
		}
	}
	var oldest []gormRecord
	result := db.Table(store.table).Select("created_at").Where("status = ?", StatusPending).Order("created_at ASC").Limit(1).Find(&oldest)
	if result.Error != nil {
		return Snapshot{}, result.Error
	}
	if len(oldest) > 0 {
		snapshot.OldestPendingAt = oldest[0].CreatedAt.UTC()
	}
	return snapshot, nil
}

func (store *GORMStore) PurgePublished(ctx context.Context, before time.Time, limit int) (int64, error) {
	if store == nil || store.db == nil {
		return 0, errors.New("outbox: gorm store is nil")
	}
	if before.IsZero() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	db := store.db.WithContext(nonNilContext(ctx))
	var ids []string
	if err := db.Table(store.table).Where("status = ? AND published_at < ?", StatusPublished, before.UTC()).Order("published_at ASC").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Table(store.table).Where("id IN ?", ids).Delete(&gormRecord{})
	return result.RowsAffected, result.Error
}

func decodeGORMRecord(row gormRecord) (Record, error) {
	var envelope event.Envelope
	if err := json.Unmarshal(row.EventJSON, &envelope); err != nil {
		return Record{}, fmt.Errorf("outbox: decode %s: %w", row.ID, err)
	}
	envelope = envelope.Clone()
	record := Record{ID: row.ID, Envelope: envelope, Status: row.Status, Attempts: row.Attempts, NextAttemptAt: row.NextAttemptAt.UTC(), LeaseOwner: row.LeaseOwner, LastError: row.LastError, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
	if row.LeaseUntil != nil {
		record.LeaseUntil = row.LeaseUntil.UTC()
	}
	if row.PublishedAt != nil {
		record.PublishedAt = row.PublishedAt.UTC()
	}
	return record, nil
}
func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type gormRecord struct {
	ID            string     `gorm:"column:id;primaryKey;size:64"`
	Topic         string     `gorm:"column:topic;size:255;not null;index:idx_yunka_outbox_topic"`
	EventType     string     `gorm:"column:event_type;size:255;not null"`
	EventJSON     []byte     `gorm:"column:event_json;type:longblob;not null"`
	Status        Status     `gorm:"column:status;size:32;not null;index:idx_yunka_outbox_ready,priority:1;index:idx_yunka_outbox_lease,priority:1"`
	Attempts      int        `gorm:"column:attempts;not null"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;not null;index:idx_yunka_outbox_ready,priority:2"`
	LeaseOwner    string     `gorm:"column:lease_owner;size:128"`
	LeaseUntil    *time.Time `gorm:"column:lease_until;index:idx_yunka_outbox_lease,priority:2"`
	LastError     string     `gorm:"column:last_error;type:text"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;index:idx_yunka_outbox_ready,priority:3;index:idx_yunka_outbox_lease,priority:3"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null"`
}
