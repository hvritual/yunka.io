package observability

import (
	"context"
	"log/slog"
	"sort"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Event.Name should come from a bounded operational vocabulary. It is used as
// a metric attribute as well as a log/span-event discriminator.
type Event struct {
	Name       string
	Severity   slog.Level
	Attributes map[string]any
	Err        error
}

func (provider *Provider) Emit(ctx context.Context, event Event) {
	if provider == nil || event.Name == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := traceAttributes(ctx, provider.config.IncludeIdentity)
	keys := make([]string, 0, len(event.Attributes))
	for key := range event.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := []any{"signal", "event", "event.name", event.Name}
	for _, key := range keys {
		value := event.Attributes[key]
		attrs = append(attrs, attributeFromAny("event."+key, value))
		fields = append(fields, "event."+key, value)
	}
	if event.Err != nil {
		fields = append(fields, "error", event.Err.Error())
	}

	provider.runtimeEventCount.Add(ctx, 1, metric.WithAttributes(append(metricAttributes(ctx), attributeFromAny("event.name", event.Name))...))
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(event.Name, trace.WithAttributes(attrs...))
		if event.Err != nil {
			span.RecordError(event.Err)
		}
	}
	provider.log(ctx, event.Severity, event.Name, fields...)
}
