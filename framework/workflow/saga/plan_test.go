package saga

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

type txStore struct{ values []event.Envelope }

func (store *txStore) EnqueueTx(_ context.Context, tx any, envelope event.Envelope) error {
	if tx == nil {
		return outbox.ErrInvalidTx
	}
	for _, current := range store.values {
		if current.ID == envelope.ID {
			return outbox.ErrDuplicate
		}
	}
	store.values = append(store.values, envelope)
	return nil
}

func TestPlanUsesStableIdempotencyAndReverseCompensation(t *testing.T) {
	plan := Plan{ID: "deploy-1", IdempotencyKey: "request-1", Source: "deployment", Steps: []Step{
		{ID: "reserve", Command: Command{Topic: "device.command", Type: "device.reserve", Payload: json.RawMessage(`{"id":"d1"}`)}, Compensation: &Command{Topic: "device.command", Type: "device.release"}},
		{ID: "bind", Command: Command{Topic: "site.command", Type: "site.bind"}, Compensation: &Command{Topic: "site.command", Type: "site.unbind"}},
	}}
	first, err := plan.Envelopes()
	if err != nil {
		t.Fatal(err)
	}
	second, err := plan.Envelopes()
	if err != nil {
		t.Fatal(err)
	}
	if first[0].ID != second[0].ID || first[1].ID != second[1].ID {
		t.Fatal("envelope ids are not idempotent")
	}
	comp, err := plan.CompensationEnvelopes(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(comp) != 2 || comp[0].Type != "site.unbind" || comp[1].Type != "device.release" {
		t.Fatalf("compensation=%v", comp)
	}
	store := &txStore{}
	if err := EnqueueTx(context.Background(), store, struct{}{}, plan); err != nil {
		t.Fatal(err)
	}
	if err := EnqueueTx(context.Background(), store, struct{}{}, plan); err != nil {
		t.Fatal(err)
	}
	if len(store.values) != 2 {
		t.Fatalf("idempotent enqueue count=%d", len(store.values))
	}
}
