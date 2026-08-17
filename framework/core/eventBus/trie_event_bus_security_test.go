package eventBus

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestUnsubscribeTailKeepsEarlierSubscriber(t *testing.T) {
	bus := NewTrieEventBus()
	defer bus.Close()
	var first, second atomic.Int32
	_, _ = bus.Subscribe("topic", func(interface{}) { first.Add(1) })
	secondID, _ := bus.Subscribe("topic", func(interface{}) { second.Add(1) })
	if err := bus.UnSubscribe("topic", secondID); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish("topic", nil); err != nil {
		t.Fatal(err)
	}
	if first.Load() != 1 || second.Load() != 0 {
		t.Fatalf("first=%d second=%d", first.Load(), second.Load())
	}
}

func TestDelayPublishAndClose(t *testing.T) {
	bus := NewTrieEventBus()
	done := make(chan struct{}, 1)
	_, _ = bus.Subscribe("topic", func(interface{}) { done <- struct{}{} })
	if err := bus.DelayPublish("topic", time.Millisecond, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delayed publication timed out")
	}
	if err := bus.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish("topic", nil); err == nil {
		t.Fatal("closed bus accepted publication")
	}
}
