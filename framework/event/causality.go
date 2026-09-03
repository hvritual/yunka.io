package event

import (
	"context"
	"strings"
)

// Causality is the minimal business-event identity retained in a consumer
// context so a subsequently published event can preserve the originating
// correlation chain without inheriting caller identity across the trust
// boundary.
type Causality struct {
	EventID       string
	CorrelationID string
}

type causalityContextKey struct{}

// WithEnvelopeContext records only event causality. It deliberately does not
// carry authenticated Principal state; transport adapters must re-establish
// identity at every remote event trust boundary.
func WithEnvelopeContext(ctx context.Context, envelope Envelope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	eventID := strings.TrimSpace(envelope.ID)
	correlationID := strings.TrimSpace(envelope.CorrelationID)
	if correlationID == "" {
		correlationID = eventID
	}
	if eventID == "" && correlationID == "" {
		return ctx
	}
	return context.WithValue(ctx, causalityContextKey{}, Causality{
		EventID:       eventID,
		CorrelationID: correlationID,
	})
}

func CausalityFromContext(ctx context.Context) (Causality, bool) {
	if ctx == nil {
		return Causality{}, false
	}
	causality, ok := ctx.Value(causalityContextKey{}).(Causality)
	if !ok {
		return Causality{}, false
	}
	if strings.TrimSpace(causality.EventID) == "" && strings.TrimSpace(causality.CorrelationID) == "" {
		return Causality{}, false
	}
	return causality, true
}

// PrepareForPublish closes the canonical event propagation boundary before an
// envelope enters a broker or durable Outbox. It preserves explicit causality,
// inherits parent-event causality when the child still carries only the
// Normalize-generated self-correlation default, injects transport propagation
// metadata, and then validates the fully prepared immutable envelope.
func PrepareForPublish(ctx context.Context, envelope Envelope, propagator Propagator) (Envelope, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	prepared := envelope.Clone()
	inheritEventCausality(ctx, &prepared)

	normalized, err := prepared.Normalize()
	if err != nil {
		return Envelope{}, err
	}
	if propagator != nil {
		propagator.Inject(ctx, &normalized)
		// Propagation may add metadata entries after the first Normalize. Run
		// validation again so trace/baggage injection cannot bypass envelope
		// metadata bounds. Stable ID/timestamp values are preserved.
		normalized, err = normalized.Normalize()
		if err != nil {
			return Envelope{}, err
		}
	}
	return normalized, nil
}

func inheritEventCausality(ctx context.Context, envelope *Envelope) {
	if envelope == nil {
		return
	}
	parent, ok := CausalityFromContext(ctx)
	if !ok {
		return
	}
	parentCorrelation := strings.TrimSpace(parent.CorrelationID)
	if parentCorrelation == "" {
		parentCorrelation = strings.TrimSpace(parent.EventID)
	}
	currentCorrelation := strings.TrimSpace(envelope.CorrelationID)
	currentID := strings.TrimSpace(envelope.ID)
	currentCausation := strings.TrimSpace(envelope.CausationID)

	// NewJSON/Normalize establishes CorrelationID=ID for a first event. When
	// that normalized envelope is later published from an event-consumer
	// context and has no explicit causation, treat the self-correlation as the
	// generated default rather than an explicit request to start a new chain.
	defaultSelfCorrelation := currentCorrelation == "" ||
		(currentID != "" && currentCorrelation == currentID && currentCausation == "")
	if defaultSelfCorrelation && parentCorrelation != "" {
		envelope.CorrelationID = parentCorrelation
	}
	if currentCausation == "" && strings.TrimSpace(parent.EventID) != "" {
		envelope.CausationID = strings.TrimSpace(parent.EventID)
	}
}
