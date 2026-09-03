package event

import (
	"context"
	"testing"
)

func TestLocalBrokerChildEventInheritsConsumedEventCausality(t *testing.T) {
	broker := NewLocalBroker(nil, WithPropagator(testPropagator{}))
	t.Cleanup(func() { _ = broker.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	childSeen := make(chan Envelope, 1)
	if _, err := broker.Subscribe(ctx, "child", func(_ context.Context, envelope Envelope) error {
		childSeen <- envelope
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Subscribe(ctx, "parent", func(handlerCtx context.Context, parent Envelope) error {
		causality, ok := CausalityFromContext(handlerCtx)
		if !ok || causality.EventID != parent.ID || causality.CorrelationID != parent.CorrelationID {
			t.Fatalf("handler causality=%+v ok=%v parent=%+v", causality, ok, parent)
		}
		child, err := NewJSON("child", "child.v1", "tests", map[string]string{"id": "c"})
		if err != nil {
			return err
		}
		return broker.Publish(handlerCtx, child)
	}); err != nil {
		t.Fatal(err)
	}

	parent, err := NewJSON("parent", "parent.v1", "tests", map[string]string{"id": "p"})
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	child := <-childSeen
	if child.CorrelationID != parent.CorrelationID {
		t.Fatalf("child correlation=%q want=%q", child.CorrelationID, parent.CorrelationID)
	}
	if child.CausationID != parent.ID {
		t.Fatalf("child causation=%q want=%q", child.CausationID, parent.ID)
	}
	if child.Get("traceparent") != "test" {
		t.Fatalf("child traceparent=%q", child.Get("traceparent"))
	}
}
