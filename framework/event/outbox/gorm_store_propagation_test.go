package outbox

import (
	"context"
	"testing"

	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/event"
)

type storeTestPropagator struct{}

func (storeTestPropagator) Inject(_ context.Context, envelope *event.Envelope) {
	envelope.Set("traceparent", "persisted-trace")
}
func (storeTestPropagator) Extract(ctx context.Context, _ event.Envelope) context.Context { return ctx }

func TestGORMStorePreparesPropagationAndEventCausalityBeforePersistence(t *testing.T) {
	store, err := NewGORMStore(&gorm.DB{}, WithPropagator(storeTestPropagator{}))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := event.NewJSON("parent", "parent.v1", "tests", map[string]string{"id": "p"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := event.NewJSON("child", "child.v1", "tests", map[string]string{"id": "c"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := event.WithEnvelopeContext(context.Background(), parent)
	prepared, err := store.prepare(ctx, child)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Get("traceparent") != "persisted-trace" {
		t.Fatalf("traceparent=%q", prepared.Get("traceparent"))
	}
	if prepared.CorrelationID != parent.CorrelationID || prepared.CausationID != parent.ID {
		t.Fatalf("prepared causality=%+v parent=%+v", prepared, parent)
	}
}
