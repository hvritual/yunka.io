package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/framework/event"
)

type fakeBroker struct {
	mu       sync.Mutex
	failures int
	seen     []event.Envelope
}

func (*fakeBroker) Name() string { return "fake" }
func (b *fakeBroker) Publish(_ context.Context, e event.Envelope) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seen = append(b.seen, e.Clone())
	if b.failures > 0 {
		b.failures--
		return errors.New("broker unavailable")
	}
	return nil
}
func (*fakeBroker) Subscribe(context.Context, string, event.Handler) (event.Subscription, error) {
	return nil, errors.New("unsupported")
}
func (*fakeBroker) Close() error { return nil }

type observer struct{ published, retried, dead int }

func (o *observer) Published(context.Context, Record)                 { o.published++ }
func (o *observer) Retried(context.Context, Record, error, time.Time) { o.retried++ }
func (o *observer) DeadLettered(context.Context, Record, error)       { o.dead++ }

func TestDispatcherRetriesWithStableEventID(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	env, _ := event.NewJSON("device.changed", "device.changed.v1", "device", map[string]string{"id": "1"})
	_ = store.Enqueue(context.Background(), env)
	broker := &fakeBroker{failures: 1}
	obs := &observer{}
	dispatcher, _ := NewDispatcher(store, broker, DispatcherConfig{WorkerID: "w", BatchSize: 1, Concurrency: 1, RetryBase: time.Second, RetryMax: time.Second, RetryJitter: 0, MaxAttempts: 3, Now: func() time.Time { return now }, Observer: obs})
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, _ := store.Record(env.ID)
	if record.Status != StatusPending || record.Attempts != 1 || obs.retried != 1 {
		t.Fatalf("record=%#v obs=%#v", record, obs)
	}
	now = record.NextAttemptAt
	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, _ = store.Record(env.ID)
	if record.Status != StatusPublished || record.Attempts != 2 || obs.published != 1 {
		t.Fatalf("record=%#v", record)
	}
	if len(broker.seen) != 2 || broker.seen[0].ID != env.ID || broker.seen[1].ID != env.ID || broker.seen[1].DeliveryAttempt != 2 {
		t.Fatalf("seen=%#v", broker.seen)
	}
}

func TestDispatcherDeadLettersAtMaxAttempts(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	env, _ := event.NewJSON("t", "t.v1", "", nil)
	_ = store.Enqueue(context.Background(), env)
	broker := &fakeBroker{failures: 10}
	obs := &observer{}
	d, _ := NewDispatcher(store, broker, DispatcherConfig{WorkerID: "w", BatchSize: 1, MaxAttempts: 2, RetryBase: time.Nanosecond, RetryMax: time.Nanosecond, Now: func() time.Time { return now }, Observer: obs})
	_ = d.RunOnce(context.Background())
	r, _ := store.Record(env.ID)
	now = r.NextAttemptAt
	_ = d.RunOnce(context.Background())
	r, _ = store.Record(env.ID)
	if r.Status != StatusDeadLetter || r.Attempts != 2 || obs.dead != 1 {
		t.Fatalf("record=%#v obs=%#v", r, obs)
	}
}

func TestDispatcherRejectsLeaseShorterThanBatchWindow(t *testing.T) {
	_, err := NewDispatcher(NewMemoryStore(), &fakeBroker{}, DispatcherConfig{BatchSize: 8, Concurrency: 2, PublishTimeout: 10 * time.Second, LeaseDuration: 30 * time.Second})
	if err == nil {
		t.Fatal("unsafe lease configuration accepted")
	}
}

type panicBroker struct{ fakeBroker }

func (*panicBroker) Publish(context.Context, event.Envelope) error { panic("boom") }

func TestDispatcherConvertsBrokerPanicIntoRetry(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	env, _ := event.NewJSON("t", "t.v1", "", nil)
	_ = store.Enqueue(context.Background(), env)
	d, _ := NewDispatcher(store, &panicBroker{}, DispatcherConfig{WorkerID: "w", BatchSize: 1, Concurrency: 1, PublishTimeout: time.Second, LeaseDuration: 2 * time.Second, RetryBase: time.Second, RetryMax: time.Second, Now: func() time.Time { return now }})
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, _ := store.Record(env.ID)
	if record.Status != StatusPending || record.Attempts != 1 {
		t.Fatalf("record=%#v", record)
	}
}

type panicObserver struct{}
func (panicObserver) Published(context.Context, Record) { panic("observer") }
func (panicObserver) Retried(context.Context, Record, error, time.Time) { panic("observer") }
func (panicObserver) DeadLettered(context.Context, Record, error) { panic("observer") }

func TestDispatcherIsolatesObserverPanic(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	env, _ := event.NewJSON("t", "t.v1", "", nil)
	_ = store.Enqueue(context.Background(), env)
	d, _ := NewDispatcher(store, &fakeBroker{}, DispatcherConfig{WorkerID: "w", BatchSize: 1, Concurrency: 1, PublishTimeout: time.Second, LeaseDuration: 2 * time.Second, Observer: panicObserver{}, Now: func() time.Time { return now }})
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, _ := store.Record(env.ID)
	if record.Status != StatusPublished {
		t.Fatalf("record=%#v", record)
	}
}
