package observability

import (
	"context"
	"log/slog"

	"github.com/hvritual/yunka.io/framework/operation"
)

type operationObserver struct{ provider *Provider }

func OperationObserver(provider *Provider) operation.Observer {
	if provider == nil {
		return nil
	}
	return &operationObserver{provider: provider}
}

func (observer *operationObserver) Observe(ctx context.Context, event operation.Event) {
	if observer == nil || observer.provider == nil {
		return
	}
	observer.provider.Emit(ctx, Event{
		Name:     "operation.phase",
		Severity: operationEventSeverity(event),
		Attributes: map[string]any{
			"operation_id": event.OperationID,
			"invocation":   string(event.Kind),
			"phase":        string(event.Phase),
			"outcome":      string(event.Outcome),
		},
	})
}

func operationEventSeverity(event operation.Event) slog.Level {
	if event.Outcome == operation.OutcomePanic || event.Outcome == operation.OutcomeFailure {
		return slog.LevelError
	}
	if event.Phase == operation.PhaseOutcome {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
