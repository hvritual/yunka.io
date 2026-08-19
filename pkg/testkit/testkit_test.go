package testkit

import (
	"context"
	"testing"
	"time"

	"yunka.io/pkg/registry"
)

func TestClockAdvance(t *testing.T) {
	start := time.Unix(100, 0).UTC()
	clock := NewClock(start)
	if got := clock.Advance(5 * time.Second); !got.Equal(start.Add(5 * time.Second)) {
		t.Fatalf("now=%s", got)
	}
}

func TestRegistryImplementsWatchLifecycle(t *testing.T) {
	current := NewRegistry()
	watcher, err := current.Watch(registry.WatchService("svc"))
	if err != nil {
		t.Fatal(err)
	}
	service := &registry.Service{Name: "svc", Version: "v1", Nodes: []*registry.Node{{Id: "a", Address: "1"}}}
	if err := current.Register(service); err != nil {
		t.Fatal(err)
	}
	result, err := watcher.Next()
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "create" || result.Service.Nodes[0].Id != "a" {
		t.Fatalf("result=%+v", result)
	}
	if err := current.Deregister(service); err != nil {
		t.Fatal(err)
	}
	result, err = watcher.Next()
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "delete" {
		t.Fatalf("result=%+v", result)
	}
	watcher.Stop()
	if _, err := watcher.Next(); err != registry.ErrWatcherStopped {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistryWatcherStopsWithContext(t *testing.T) {
	current := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	watcher, err := current.Watch(func(options *registry.WatchOptions) { options.Context = ctx })
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := watcher.Next(); err != registry.ErrWatcherStopped {
		t.Fatalf("err=%v", err)
	}
}
