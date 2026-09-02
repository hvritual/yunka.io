package observability

import (
	"context"
	"log/slog"
	"time"

	frameworkevent "github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

type eventPropagator struct{}

func EventPropagator() frameworkevent.Propagator { return eventPropagator{} }

func (eventPropagator) Inject(ctx context.Context, envelope *frameworkevent.Envelope) {
	if envelope == nil {
		return
	}
	Inject(ctx, envelope)
}

func (eventPropagator) Extract(ctx context.Context, envelope frameworkevent.Envelope) context.Context {
	clone := envelope.Clone()
	return Extract(ctx, &clone)
}

type outboxObserver struct{ provider *Provider }

func OutboxObserver(provider *Provider) outbox.Observer {
	if provider == nil {
		return outbox.NopObserver{}
	}
	return &outboxObserver{provider: provider}
}

func (observer *outboxObserver) Published(ctx context.Context, record outbox.Record) {
	observer.provider.Emit(ctx, Event{Name: "outbox.published", Severity: slog.LevelInfo, Attributes: outboxEventAttributes(record)})
}

func (observer *outboxObserver) Retried(ctx context.Context, record outbox.Record, cause error, next time.Time) {
	attributes := outboxEventAttributes(record)
	attributes["next_attempt_at"] = next.UTC().Format(time.RFC3339Nano)
	observer.provider.Emit(ctx, Event{Name: "outbox.retry", Severity: slog.LevelWarn, Attributes: attributes, Err: cause})
}

func (observer *outboxObserver) DeadLettered(ctx context.Context, record outbox.Record, cause error) {
	observer.provider.Emit(ctx, Event{Name: "outbox.deadletter", Severity: slog.LevelError, Attributes: outboxEventAttributes(record), Err: cause})
}

func outboxEventAttributes(record outbox.Record) map[string]any {
	return map[string]any{"event_id": record.ID, "topic": record.Envelope.Topic, "type": record.Envelope.Type, "attempt": record.Attempts}
}
