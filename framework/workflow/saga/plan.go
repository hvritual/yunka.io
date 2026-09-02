package saga

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

var ErrInvalidPlan = errors.New("saga: invalid plan")

type Command struct {
	Topic   string          `json:"topic"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Step struct {
	ID           string   `json:"id"`
	Command      Command  `json:"command"`
	Compensation *Command `json:"compensation,omitempty"`
}

type Plan struct {
	ID             string `json:"id"`
	IdempotencyKey string `json:"idempotencyKey"`
	Source         string `json:"source,omitempty"`
	Steps          []Step `json:"steps"`
}

func (plan Plan) Validate() error {
	if strings.TrimSpace(plan.ID) == "" || strings.TrimSpace(plan.IdempotencyKey) == "" || len(plan.Steps) == 0 {
		return ErrInvalidPlan
	}
	seen := map[string]struct{}{}
	for _, step := range plan.Steps {
		step.ID = strings.TrimSpace(step.ID)
		if step.ID == "" || strings.TrimSpace(step.Command.Topic) == "" || strings.TrimSpace(step.Command.Type) == "" {
			return ErrInvalidPlan
		}
		if _, ok := seen[step.ID]; ok {
			return fmt.Errorf("%w: duplicate step %s", ErrInvalidPlan, step.ID)
		}
		seen[step.ID] = struct{}{}
		if step.Compensation != nil && (strings.TrimSpace(step.Compensation.Topic) == "" || strings.TrimSpace(step.Compensation.Type) == "") {
			return ErrInvalidPlan
		}
	}
	return nil
}

func deterministicID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func (plan Plan) Envelopes() ([]event.Envelope, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	result := make([]event.Envelope, 0, len(plan.Steps))
	for index, step := range plan.Steps {
		metadata := map[string]string{
			"saga.id":         plan.ID,
			"saga.step":       step.ID,
			"saga.index":      fmt.Sprint(index),
			"idempotency.key": plan.IdempotencyKey,
		}
		if step.Compensation != nil {
			metadata["saga.compensation.topic"] = step.Compensation.Topic
			metadata["saga.compensation.type"] = step.Compensation.Type
		}
		envelope, err := (event.Envelope{ID: deterministicID("saga", plan.IdempotencyKey, step.ID), Topic: step.Command.Topic, Type: step.Command.Type, Source: plan.Source, Subject: step.ID, CorrelationID: plan.ID, Payload: append(json.RawMessage(nil), step.Command.Payload...), Metadata: metadata}).Normalize()
		if err != nil {
			return nil, err
		}
		result = append(result, envelope)
	}
	return result, nil
}

func (plan Plan) CompensationEnvelopes(completed int) ([]event.Envelope, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if completed < 0 || completed > len(plan.Steps) {
		return nil, ErrInvalidPlan
	}
	result := []event.Envelope{}
	for i := completed - 1; i >= 0; i-- {
		step := plan.Steps[i]
		if step.Compensation == nil {
			continue
		}
		envelope, err := (event.Envelope{ID: deterministicID("saga-compensation", plan.IdempotencyKey, step.ID), Topic: step.Compensation.Topic, Type: step.Compensation.Type, Source: plan.Source, Subject: step.ID, CorrelationID: plan.ID, Payload: append(json.RawMessage(nil), step.Compensation.Payload...), Metadata: map[string]string{"saga.id": plan.ID, "saga.step": step.ID, "idempotency.key": plan.IdempotencyKey, "saga.compensation": "true"}}).Normalize()
		if err != nil {
			return nil, err
		}
		result = append(result, envelope)
	}
	return result, nil
}

// EnqueueTx is the explicit remote composition boundary: commands are staged in
// the caller's already-open local transaction and published only after commit.
func EnqueueTx(ctx context.Context, store outbox.TransactionalStore, tx any, plan Plan) error {
	if store == nil || tx == nil {
		return outbox.ErrInvalidTx
	}
	envelopes, err := plan.Envelopes()
	if err != nil {
		return err
	}
	for _, envelope := range envelopes {
		if err := outbox.EnqueueTx(ctx, store, tx, envelope); err != nil && !errors.Is(err, outbox.ErrDuplicate) {
			return err
		}
	}
	return nil
}

func EnqueueCompensationsTx(ctx context.Context, store outbox.TransactionalStore, tx any, plan Plan, completed int) error {
	if store == nil || tx == nil {
		return outbox.ErrInvalidTx
	}
	envelopes, err := plan.CompensationEnvelopes(completed)
	if err != nil {
		return err
	}
	for _, envelope := range envelopes {
		if err := outbox.EnqueueTx(ctx, store, tx, envelope); err != nil && !errors.Is(err, outbox.ErrDuplicate) {
			return err
		}
	}
	return nil
}
