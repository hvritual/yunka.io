package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	EnvelopeSchemaVersion  = 1
	MaxIDLength            = 128
	MaxTopicLength         = 255
	MaxTypeLength          = 255
	MaxSourceLength        = 255
	MaxSubjectLength       = 512
	MaxMetadataEntries     = 64
	MaxMetadataKeyLength   = 128
	MaxMetadataValueLength = 4096
)

var ErrInvalidEnvelope = errors.New("event: invalid envelope")

// Envelope is the immutable business-event contract shared by brokers and the
// transactional outbox. Payload and Metadata are cloned at every boundary so
// publishers and consumers cannot share mutable state accidentally.
type Envelope struct {
	SchemaVersion   int               `json:"schemaVersion"`
	ID              string            `json:"id"`
	Topic           string            `json:"topic"`
	Type            string            `json:"type"`
	Source          string            `json:"source,omitempty"`
	Subject         string            `json:"subject,omitempty"`
	CorrelationID   string            `json:"correlationId,omitempty"`
	CausationID     string            `json:"causationId,omitempty"`
	OccurredAt      time.Time         `json:"occurredAt"`
	ContentType     string            `json:"contentType"`
	DeliveryAttempt int               `json:"deliveryAttempt,omitempty"`
	Payload         json.RawMessage   `json:"payload,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func NewJSON(topic, eventType, source string, value any) (Envelope, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Topic: topic, Type: eventType, Source: source, Payload: payload}.Normalize()
}

// Normalize validates required routing fields and fills stable envelope
// defaults. An existing ID and timestamp are always preserved so retries keep
// the same idempotency key.
func (envelope Envelope) Normalize() (Envelope, error) {
	envelope = envelope.Clone()
	envelope.Topic = strings.TrimSpace(envelope.Topic)
	envelope.Type = strings.TrimSpace(envelope.Type)
	envelope.Source = strings.TrimSpace(envelope.Source)
	envelope.Subject = strings.TrimSpace(envelope.Subject)
	envelope.ID = strings.TrimSpace(envelope.ID)
	if envelope.Topic == "" || envelope.Type == "" || len(envelope.Topic) > MaxTopicLength || len(envelope.Type) > MaxTypeLength || len(envelope.Source) > MaxSourceLength || len(envelope.Subject) > MaxSubjectLength || len(envelope.ID) > MaxIDLength {
		return Envelope{}, ErrInvalidEnvelope
	}
	if len(envelope.Metadata) > MaxMetadataEntries {
		return Envelope{}, ErrInvalidEnvelope
	}
	for key, value := range envelope.Metadata {
		if strings.TrimSpace(key) == "" || len(key) > MaxMetadataKeyLength || len(value) > MaxMetadataValueLength {
			return Envelope{}, ErrInvalidEnvelope
		}
	}
	if envelope.SchemaVersion == 0 {
		envelope.SchemaVersion = EnvelopeSchemaVersion
	}
	if envelope.SchemaVersion < 1 {
		return Envelope{}, ErrInvalidEnvelope
	}
	if envelope.ID == "" {
		id, err := newID()
		if err != nil {
			return Envelope{}, err
		}
		envelope.ID = id
	}
	if envelope.OccurredAt.IsZero() {
		envelope.OccurredAt = time.Now().UTC()
	} else {
		envelope.OccurredAt = envelope.OccurredAt.UTC()
	}
	if strings.TrimSpace(envelope.ContentType) == "" {
		envelope.ContentType = "application/json"
	}
	if envelope.CorrelationID == "" {
		envelope.CorrelationID = envelope.ID
	}
	if envelope.DeliveryAttempt < 0 {
		envelope.DeliveryAttempt = 0
	}
	return envelope, nil
}

func (envelope Envelope) Clone() Envelope {
	clone := envelope
	clone.Payload = append(json.RawMessage(nil), envelope.Payload...)
	if envelope.Metadata != nil {
		clone.Metadata = make(map[string]string, len(envelope.Metadata))
		for key, value := range envelope.Metadata {
			clone.Metadata[key] = value
		}
	}
	return clone
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// Get/Set/Keys intentionally mirror the W4 propagation carrier shape. The
// event package does not depend on OpenTelemetry; a Propagator adapter may use
// these methods to carry traceparent/tracestate/baggage.
func (envelope *Envelope) Get(key string) string {
	if envelope == nil || envelope.Metadata == nil {
		return ""
	}
	return envelope.Metadata[key]
}

func (envelope *Envelope) Set(key, value string) {
	if envelope == nil || strings.TrimSpace(key) == "" {
		return
	}
	if envelope.Metadata == nil {
		envelope.Metadata = make(map[string]string)
	}
	envelope.Metadata[key] = value
}

func (envelope *Envelope) Keys() []string {
	if envelope == nil || len(envelope.Metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(envelope.Metadata))
	for key := range envelope.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Propagator carries trace/correlation context across event transports. It
// deliberately has no identity API: authenticated Principal state must be
// re-established at remote trust boundaries.
type Propagator interface {
	Inject(context.Context, *Envelope)
	Extract(context.Context, Envelope) context.Context
}
