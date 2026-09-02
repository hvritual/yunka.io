package idempotencygorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"github.com/hvritual/yunka.io/framework/execution"
)

const defaultLeaseDuration = 2 * time.Minute

type Options struct {
	LeaseDuration time.Duration
	Now           func() time.Time
}

type Store struct {
	database *gorm.DB
	lease    time.Duration
	now      func() time.Time
}

type Record struct {
	TenantID    string    `gorm:"column:tenant_id;size:128;primaryKey"`
	OperationID string    `gorm:"column:operation_id;size:191;primaryKey"`
	KeyHash     string    `gorm:"column:key_hash;size:64;primaryKey"`
	State       string    `gorm:"column:state;size:24;not null;index"`
	Attempt     string    `gorm:"column:attempt;size:64;not null"`
	LeaseUntil  time.Time `gorm:"column:lease_until;not null;index"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (Record) TableName() string { return "yunka_operation_idempotency" }

func NewStore(database *gorm.DB, options Options) (*Store, error) {
	if database == nil {
		return nil, errors.New("idempotencygorm: database is required")
	}
	lease := options.LeaseDuration
	if lease <= 0 {
		lease = defaultLeaseDuration
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{database: database, lease: lease, now: now}, nil
}

func (store *Store) EnsureSchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("idempotencygorm: store unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := store.database.WithContext(ctx).AutoMigrate(&Record{}); err != nil {
		return fmt.Errorf("idempotencygorm: migrate: %w", err)
	}
	return nil
}

func (store *Store) Claim(ctx context.Context, identity execution.IdempotencyIdentity) error {
	if store == nil || store.database == nil {
		return execution.ErrIdempotencyUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	identity = normalizeIdentity(identity)
	if identity.OperationID == "" || identity.Key == "" || identity.Attempt == "" {
		return execution.ErrIdempotencyUnavailable
	}

	for attempt := 0; attempt < 4; attempt++ {
		now := store.now().UTC()
		candidate := recordFor(identity, execution.IdempotencyRunning, now.Add(store.lease), now)
		result := store.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if result.Error != nil {
			return fmt.Errorf("idempotencygorm: claim insert: %w", result.Error)
		}
		if result.RowsAffected == 1 {
			return nil
		}

		existing, err := store.lookupRecord(ctx, store.database, identity)
		if err != nil {
			return err
		}
		switch execution.IdempotencyState(existing.State) {
		case execution.IdempotencySucceeded:
			return execution.ErrIdempotencyCompleted
		case execution.IdempotencyRunning:
			if existing.LeaseUntil.After(now) {
				return execution.ErrIdempotencyInProgress
			}
		case execution.IdempotencyFailed:
			// Failed attempts are immediately retryable.
		default:
			return execution.ErrIdempotencyUnavailable
		}

		updates := map[string]any{
			"state":       string(execution.IdempotencyRunning),
			"attempt":     identity.Attempt,
			"lease_until": now.Add(store.lease),
			"updated_at":  now,
		}
		updated := store.database.WithContext(ctx).Model(&Record{}).
			Where("tenant_id = ? AND operation_id = ? AND key_hash = ? AND state = ? AND attempt = ?", existing.TenantID, existing.OperationID, existing.KeyHash, existing.State, existing.Attempt).
			Updates(updates)
		if updated.Error != nil {
			return fmt.Errorf("idempotencygorm: claim takeover: %w", updated.Error)
		}
		if updated.RowsAffected == 1 {
			return nil
		}
	}
	return execution.ErrIdempotencyInProgress
}

func (store *Store) Mark(ctx context.Context, identity execution.IdempotencyIdentity, state execution.IdempotencyState) error {
	if store == nil || store.database == nil {
		return execution.ErrIdempotencyUnavailable
	}
	return store.markWithDatabase(ctx, store.database, identity, state)
}

func (store *Store) MarkTx(ctx context.Context, transaction any, identity execution.IdempotencyIdentity, state execution.IdempotencyState) error {
	database, ok := transaction.(*gorm.DB)
	if !ok || database == nil {
		return execution.ErrIdempotencyAtomicUnavailable
	}
	return store.markWithDatabase(ctx, database, identity, state)
}

func (store *Store) Lookup(ctx context.Context, identity execution.IdempotencyIdentity) (execution.IdempotencyState, bool, error) {
	if store == nil || store.database == nil {
		return "", false, execution.ErrIdempotencyUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	record, err := store.lookupRecord(ctx, store.database, normalizeIdentity(identity))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return execution.IdempotencyState(record.State), true, nil
}

func (store *Store) markWithDatabase(ctx context.Context, database *gorm.DB, identity execution.IdempotencyIdentity, state execution.IdempotencyState) error {
	if state != execution.IdempotencySucceeded && state != execution.IdempotencyFailed {
		return fmt.Errorf("idempotencygorm: invalid terminal state %q", state)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	identity = normalizeIdentity(identity)
	if identity.OperationID == "" || identity.Key == "" || identity.Attempt == "" {
		return execution.ErrIdempotencyUnavailable
	}
	now := store.now().UTC()
	key := recordKey(identity)
	result := database.WithContext(ctx).Model(&Record{}).
		Where("tenant_id = ? AND operation_id = ? AND key_hash = ? AND state = ? AND attempt = ?", key.TenantID, key.OperationID, key.KeyHash, string(execution.IdempotencyRunning), identity.Attempt).
		Updates(map[string]any{"state": string(state), "lease_until": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("idempotencygorm: mark %s: %w", state, result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}

	existing, err := store.lookupRecord(ctx, database, identity)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return execution.ErrIdempotencyLeaseLost
		}
		return err
	}
	if existing.Attempt == identity.Attempt && existing.State == string(state) {
		return nil
	}
	return execution.ErrIdempotencyLeaseLost
}

func (store *Store) lookupRecord(ctx context.Context, database *gorm.DB, identity execution.IdempotencyIdentity) (Record, error) {
	key := recordKey(identity)
	var record Record
	err := database.WithContext(ctx).Where("tenant_id = ? AND operation_id = ? AND key_hash = ?", key.TenantID, key.OperationID, key.KeyHash).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Record{}, gorm.ErrRecordNotFound
		}
		return Record{}, fmt.Errorf("idempotencygorm: lookup: %w", err)
	}
	return record, nil
}

func normalizeIdentity(identity execution.IdempotencyIdentity) execution.IdempotencyIdentity {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.OperationID = strings.TrimSpace(identity.OperationID)
	identity.Key = strings.TrimSpace(identity.Key)
	identity.Attempt = strings.TrimSpace(identity.Attempt)
	return identity
}

func recordKey(identity execution.IdempotencyIdentity) Record {
	digest := sha256.Sum256([]byte(identity.Key))
	return Record{TenantID: identity.TenantID, OperationID: identity.OperationID, KeyHash: hex.EncodeToString(digest[:])}
}

func recordFor(identity execution.IdempotencyIdentity, state execution.IdempotencyState, leaseUntil, now time.Time) Record {
	record := recordKey(identity)
	record.State = string(state)
	record.Attempt = identity.Attempt
	record.LeaseUntil = leaseUntil
	record.CreatedAt = now
	record.UpdatedAt = now
	return record
}
