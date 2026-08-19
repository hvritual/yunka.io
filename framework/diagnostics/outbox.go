package diagnostics

import (
	"context"
	"errors"
	"strings"
	"time"

	"yunka.io/framework/event/outbox"
)

type OutboxSnapshot struct {
	Pending            int64   `json:"pending"`
	InFlight           int64   `json:"inFlight"`
	Published          int64   `json:"published"`
	DeadLetter         int64   `json:"deadLetter"`
	OldestPendingAt    string  `json:"oldestPendingAt,omitempty"`
	OldestPendingAgeMS float64 `json:"oldestPendingAgeMs,omitempty"`
}

// Outbox exposes aggregate queue state only. Event IDs, payloads, metadata and
// last errors are intentionally excluded from the generic diagnostics surface.
func Outbox(name string, store outbox.Store) Source {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "outbox"
	}
	return SourceFunc{SourceName: name, Func: func(ctx context.Context) (any, error) {
		if store == nil {
			return nil, errors.New("diagnostics: outbox store is nil")
		}
		snapshot, err := store.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		result := OutboxSnapshot{Pending: snapshot.Pending, InFlight: snapshot.InFlight, Published: snapshot.Published, DeadLetter: snapshot.DeadLetter}
		if !snapshot.OldestPendingAt.IsZero() {
			result.OldestPendingAt = snapshot.OldestPendingAt.UTC().Format(time.RFC3339Nano)
			age := time.Since(snapshot.OldestPendingAt)
			if age > 0 {
				result.OldestPendingAgeMS = float64(age) / float64(time.Millisecond)
			}
		}
		return result, nil
	}}
}
