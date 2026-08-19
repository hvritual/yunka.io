package testkit

import (
	"context"
	"testing"
	"time"

	"yunka.io/framework/event"
)

func TestLeafAliasesRemainAvailable(t *testing.T) {
	clock := NewClock(time.Unix(1, 0))
	if clock.Now().Unix() != 1 {
		t.Fatal("clock alias unavailable")
	}
	if NewRegistry() == nil {
		t.Fatal("registry alias unavailable")
	}
}

func TestBrokerCapturesClonedEnvelope(t *testing.T) {
	broker := NewBroker()
	received := make(chan event.Envelope, 1)
	subscription, err := broker.Subscribe(context.Background(), "topic", func(_ context.Context, envelope event.Envelope) error { received <- envelope; return nil })
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := event.NewJSON("topic", "created", "test", map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Publish(context.Background(), envelope); err != nil {
		t.Fatal(err)
	}
	got := <-received
	envelope.Payload[0] = 'X'
	published := broker.Published()
	if len(published) != 1 || published[0].ID != got.ID {
		t.Fatalf("published=%+v", published)
	}
	if published[0].Payload[0] == 'X' {
		t.Fatal("payload was not cloned")
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
}
