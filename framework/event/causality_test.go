package event

import (
	"context"
	"testing"
)

func TestPrepareForPublishInheritsParentCausalityFromNormalizedChild(t *testing.T) {
	parent, err := NewJSON("parent", "parent.v1", "tests", map[string]string{"id": "p"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewJSON("child", "child.v1", "tests", map[string]string{"id": "c"})
	if err != nil {
		t.Fatal(err)
	}
	if child.CorrelationID != child.ID {
		t.Fatalf("precondition correlation=%q id=%q", child.CorrelationID, child.ID)
	}

	prepared, err := PrepareForPublish(WithEnvelopeContext(context.Background(), parent), child, nil)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.CorrelationID != parent.CorrelationID {
		t.Fatalf("correlation=%q want=%q", prepared.CorrelationID, parent.CorrelationID)
	}
	if prepared.CausationID != parent.ID {
		t.Fatalf("causation=%q want=%q", prepared.CausationID, parent.ID)
	}
	if prepared.ID != child.ID {
		t.Fatalf("child id changed: got=%q want=%q", prepared.ID, child.ID)
	}
}

func TestPrepareForPublishPreservesExplicitCausality(t *testing.T) {
	parent, err := (Envelope{Topic: "parent", Type: "parent.v1"}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	child := Envelope{
		Topic:         "child",
		Type:          "child.v1",
		CorrelationID: "explicit-correlation",
		CausationID:   "explicit-causation",
	}
	prepared, err := PrepareForPublish(WithEnvelopeContext(context.Background(), parent), child, nil)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.CorrelationID != "explicit-correlation" || prepared.CausationID != "explicit-causation" {
		t.Fatalf("explicit causality changed: %#v", prepared)
	}
}

func TestPrepareForPublishInjectsPropagationBeforeFinalValidation(t *testing.T) {
	prepared, err := PrepareForPublish(context.Background(), Envelope{Topic: "topic", Type: "topic.v1"}, testPropagator{})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Get("traceparent") != "test" {
		t.Fatalf("traceparent=%q", prepared.Get("traceparent"))
	}
}
