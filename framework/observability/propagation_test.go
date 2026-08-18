package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type mapCarrier map[string]string

func (carrier mapCarrier) Get(key string) string { return carrier[key] }
func (carrier mapCarrier) Set(key, value string) { carrier[key] = value }
func (carrier mapCarrier) Keys() []string {
	keys := make([]string, 0, len(carrier))
	for key := range carrier {
		keys = append(keys, key)
	}
	return keys
}

func TestW3CTraceContextRoundTrip(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	}))

	carrier := mapCarrier{}
	Inject(ctx, carrier)
	if carrier["traceparent"] == "" {
		t.Fatal("traceparent was not injected")
	}
	extracted := Extract(context.Background(), carrier)
	got := trace.SpanContextFromContext(extracted)
	if !got.IsRemote() || got.TraceID() != traceID || got.SpanID() != spanID {
		t.Fatalf("unexpected extracted span context: %#v", got)
	}
}
