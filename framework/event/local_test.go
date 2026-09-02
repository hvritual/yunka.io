package event

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

type testPropagator struct{}

func (testPropagator) Inject(_ context.Context, envelope *Envelope) {
	envelope.Set("traceparent", "test")
}
func (testPropagator) Extract(ctx context.Context, envelope Envelope) context.Context {
	if envelope.Get("traceparent") == "test" {
		return context.WithValue(ctx, "trace", "ok")
	}
	return ctx
}

func TestEnvelopeNormalizeIsStableAndCloneSafe(t *testing.T) {
	envelope, err := NewJSON("device.changed", "device.changed.v1", "device", map[string]string{"id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	copy := envelope.Clone()
	copy.Metadata = map[string]string{"a": "b"}
	copy.Payload[0] = 'x'
	if envelope.ID == "" || envelope.CorrelationID != envelope.ID || envelope.ContentType != "application/json" {
		t.Fatalf("bad envelope: %#v", envelope)
	}
	if len(envelope.Payload) == 0 || envelope.Payload[0] == 'x' {
		t.Fatal("payload was shared")
	}
}

func TestLocalBrokerRunsMiddlewareAndReturnsConsumerError(t *testing.T) {
	bus := eventBus.NewTrieEventBus()
	seen := false
	broker := NewLocalBroker(bus,
		WithPropagator(testPropagator{}),
		WithMiddleware(func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context) error {
				metadata, _ := runtimecontext.MetadataFrom(ctx)
				if metadata.Transport != "event" || metadata.Operation != "device.changed" || ctx.Value("trace") != "ok" {
					t.Fatalf("bad context: %#v", metadata)
				}
				seen = true
				return next(ctx)
			}
		}),
	)
	want := errors.New("consumer failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := broker.Subscribe(ctx, "device.changed", func(context.Context, Envelope) error { return want })
	if err != nil {
		t.Fatal(err)
	}
	envelope, _ := NewJSON("device.changed", "device.changed.v1", "device", map[string]string{"id": "1"})
	if err := broker.Publish(context.Background(), envelope); !errors.Is(err, want) {
		t.Fatalf("publish err=%v", err)
	}
	if !seen {
		t.Fatal("middleware not called")
	}
}

func TestLocalBrokerSubscriptionClosesWithContext(t *testing.T) {
	broker := NewLocalBroker(nil)
	ctx, cancel := context.WithCancel(context.Background())
	sub, err := broker.Subscribe(ctx, "topic", func(context.Context, Envelope) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(10 * time.Millisecond)
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := broker.Close(); err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(context.Background(), Envelope{Topic: "topic", Type: "t"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}

func TestLocalBrokerConvertsConsumerPanicToPublishError(t *testing.T) {
	broker := NewLocalBroker(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := broker.Subscribe(ctx, "panic", func(context.Context, Envelope) error { panic("boom") })
	if err != nil {
		t.Fatal(err)
	}
	err = broker.Publish(context.Background(), Envelope{Topic: "panic", Type: "panic.v1"})
	if err == nil {
		t.Fatal("consumer panic did not become a publish error")
	}
}

func TestEnvelopeRejectsUnboundedRoutingMetadata(t *testing.T) {
	metadata := make(map[string]string)
	for i := 0; i < MaxMetadataEntries+1; i++ {
		metadata[fmt.Sprintf("k%d", i)] = "v"
	}
	_, err := (Envelope{Topic: "topic", Type: "type", Metadata: metadata}).Normalize()
	if !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("err=%v", err)
	}
	_, err = (Envelope{Topic: strings.Repeat("x", MaxTopicLength+1), Type: "type"}).Normalize()
	if !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("err=%v", err)
	}
}
