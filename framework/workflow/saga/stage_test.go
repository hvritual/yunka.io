package saga

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/execution"
)

type stageUnit struct{ handle any }

func (*stageUnit) Commit(context.Context) error   { return nil }
func (*stageUnit) Rollback(context.Context) error { return nil }
func (*stageUnit) Close() error                   { return nil }
func (unit *stageUnit) TransactionHandle() any    { return unit.handle }

type stageFactory struct{ unit *stageUnit }

func (factory stageFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	return factory.unit, nil
}

type stageStore struct {
	handles []any
	events  []event.Envelope
}

func (store *stageStore) EnqueueTx(_ context.Context, tx any, envelope event.Envelope) error {
	store.handles = append(store.handles, tx)
	store.events = append(store.events, envelope)
	return nil
}

func TestStagerUsesExactExecutionScopeTransaction(t *testing.T) {
	handle := &struct{ id string }{"tx-1"}
	ctx, root, err := execution.BeginRoot(context.Background(), "device.provision", execution.TransactionLocal, nil, stageFactory{unit: &stageUnit{handle: handle}})
	if err != nil {
		t.Fatal(err)
	}
	defer root.Rollback(ctx)
	store := &stageStore{}
	stager, err := NewStager(store)
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{ID: "saga-1", IdempotencyKey: "request-1", Steps: []Step{{ID: "reserve", Command: Command{Topic: "inventory", Type: "reserve", Payload: json.RawMessage(`{"id":"d1"}`)}}}}
	if err := stager.Stage(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if len(store.handles) != 1 || store.handles[0] != handle || len(store.events) != 1 {
		t.Fatalf("handles=%v events=%v", store.handles, store.events)
	}
}

func TestStagerFailsOutsideTransactionalExecutionScope(t *testing.T) {
	stager, err := NewStager(&stageStore{})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{ID: "saga-1", IdempotencyKey: "request-1", Steps: []Step{{ID: "x", Command: Command{Topic: "t", Type: "x"}}}}
	if err := stager.Stage(context.Background(), plan); err == nil {
		t.Fatal("expected missing execution scope failure")
	}
}
