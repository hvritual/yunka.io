package outbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/yunka.io/framework/event"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInFlight   Status = "in-flight"
	StatusPublished  Status = "published"
	StatusDeadLetter Status = "dead-letter"
)

var (
	ErrDuplicate    = errors.New("outbox: duplicate event id")
	ErrNotFound     = errors.New("outbox: record not found")
	ErrLeaseLost    = errors.New("outbox: lease lost")
	ErrInvalidTx    = errors.New("outbox: invalid transaction handle")
	ErrInvalidOwner = errors.New("outbox: claim owner is required")
)

type Record struct {
	ID            string         `json:"id"`
	Envelope      event.Envelope `json:"envelope"`
	Status        Status         `json:"status"`
	Attempts      int            `json:"attempts"`
	NextAttemptAt time.Time      `json:"nextAttemptAt"`
	LeaseOwner    string         `json:"leaseOwner,omitempty"`
	LeaseUntil    time.Time      `json:"leaseUntil,omitempty"`
	LastError     string         `json:"lastError,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	PublishedAt   time.Time      `json:"publishedAt,omitempty"`
}

func (record Record) Clone() Record {
	clone := record
	clone.Envelope = record.Envelope.Clone()
	return clone
}

type ClaimOptions struct {
	Owner string
	Limit int
	Lease time.Duration
	Now   time.Time
}

func (options ClaimOptions) normalize() ClaimOptions {
	options.Owner = strings.TrimSpace(options.Owner)
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Lease <= 0 {
		options.Lease = 30 * time.Second
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	} else {
		options.Now = options.Now.UTC()
	}
	return options
}

type Snapshot struct {
	Pending         int64     `json:"pending"`
	InFlight        int64     `json:"inFlight"`
	Published       int64     `json:"published"`
	DeadLetter      int64     `json:"deadLetter"`
	OldestPendingAt time.Time `json:"oldestPendingAt,omitempty"`
}

// Store is the durable dispatcher contract. Enqueue alone is durable but is not
// automatically atomic with an application's business transaction. Production
// transactional-outbox adapters must additionally implement TransactionalStore.
type Store interface {
	Enqueue(context.Context, event.Envelope) error
	Claim(context.Context, ClaimOptions) ([]Record, error)
	MarkPublished(context.Context, string, string, time.Time) error
	Retry(context.Context, string, string, time.Time, error) error
	DeadLetter(context.Context, string, string, error) error
	Snapshot(context.Context) (Snapshot, error)
}

// TransactionalStore stages an event using the caller's already-open database
// transaction. The tx value is adapter-specific; passing an unrelated or base
// database handle forfeits atomicity and should be rejected by the adapter.
type TransactionalStore interface {
	EnqueueTx(context.Context, any, event.Envelope) error
}

// RetentionStore is an optional maintenance seam. Purging is intentionally not
// automatic so operators control retention independently from delivery.
type RetentionStore interface {
	PurgePublished(context.Context, time.Time, int) (int64, error)
}

func EnqueueTx(ctx context.Context, store TransactionalStore, tx any, envelope event.Envelope) error {
	if store == nil || tx == nil {
		return ErrInvalidTx
	}
	return store.EnqueueTx(ctx, tx, envelope)
}
