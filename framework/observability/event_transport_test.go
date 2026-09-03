package observability

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	frameworkevent "github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

func TestOutboxObserverRestoresPersistedTraceAndCausality(t *testing.T) {
	provider, output := newTestProvider(t, false)
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	envelope := frameworkevent.Envelope{
		Topic:         "device.command",
		Type:          "device.command.v1",
		Source:        "device-service",
		CorrelationID: "business-flow-1",
		CausationID:   "event-parent-1",
	}
	prepared, err := frameworkevent.PrepareForPublish(ctx, envelope, EventPropagator())
	if err != nil {
		t.Fatal(err)
	}
	record := outbox.Record{ID: prepared.ID, Envelope: prepared, Attempts: 2}
	OutboxObserver(provider).Published(context.Background(), record)

	raw := output.String()
	if !strings.Contains(raw, `"event.name":"outbox.published"`) ||
		!strings.Contains(raw, `"trace_id":"`+traceID.String()+`"`) ||
		!strings.Contains(raw, `"event.correlation_id":"business-flow-1"`) ||
		!strings.Contains(raw, `"event.causation_id":"event-parent-1"`) {
		t.Fatalf("outbox trace evidence missing: %s", raw)
	}
}
