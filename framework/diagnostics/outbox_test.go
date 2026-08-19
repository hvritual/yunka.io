package diagnostics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	frameworkevent "yunka.io/framework/event"
	"yunka.io/framework/event/outbox"
)

func TestOutboxSourceExposesCountsWithoutPayload(t *testing.T) {
	store := outbox.NewMemoryStore()
	envelope, err := frameworkevent.NewJSON("orders.created", "orders.created.v1", "orders", map[string]string{"secret": "must-not-leak"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Enqueue(context.Background(), envelope); err != nil {
		t.Fatal(err)
	}
	source := Outbox("outbox", store)
	value, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "must-not-leak") || strings.Contains(string(encoded), envelope.ID) {
		t.Fatalf("diagnostics leaked event data: %s", encoded)
	}
	var snapshot OutboxSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 1 || snapshot.InFlight != 0 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
}
