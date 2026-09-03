package event

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

// LocalBroker adapts the legacy trie EventBus to the W8 Broker contract. It is
// process-local and non-durable; transactional durability is provided by the
// outbox, not by this adapter.
type LocalBroker struct {
	bus        eventBus.EventBus
	ownsBus    bool
	chain      middleware.Chain
	propagator Propagator

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	closed bool
}

func NewLocalBroker(bus eventBus.EventBus, options ...Option) *LocalBroker {
	ownsBus := false
	if bus == nil {
		bus = eventBus.NewTrieEventBus()
		ownsBus = true
	}
	config := BrokerOptions{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &LocalBroker{
		bus: bus, ownsBus: ownsBus, chain: middleware.New(config.Middleware...),
		propagator: config.Propagator, ctx: ctx, cancel: cancel,
	}
}

func (*LocalBroker) Name() string { return "local-eventbus" }

func (broker *LocalBroker) Publish(ctx context.Context, envelope Envelope) error {
	if broker == nil || broker.bus == nil {
		return ErrNilBroker
	}
	broker.mu.RLock()
	closed := broker.closed
	broker.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	prepared, err := PrepareForPublish(ctx, envelope, broker.propagator)
	if err != nil {
		return err
	}
	delivery := &localDelivery{ctx: ctx, envelope: prepared.Clone()}
	if err := broker.bus.Publish(prepared.Topic, delivery); err != nil {
		return err
	}
	return delivery.err()
}

func (broker *LocalBroker) Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error) {
	if broker == nil || broker.bus == nil {
		return nil, ErrNilBroker
	}
	if handler == nil {
		return nil, ErrNilHandler
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, ErrInvalidEnvelope
	}
	broker.mu.RLock()
	closed := broker.closed
	broker.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	index, err := broker.bus.Subscribe(topic, func(value interface{}) {
		delivery, ok := value.(*localDelivery)
		if !ok || delivery == nil {
			return
		}
		envelope := delivery.envelope.Clone()
		// Event handling starts a new trust context. Preserve only cancellation,
		// explicit event causality, and supported propagation data (for example
		// W3C trace context). Caller identity is never inherited implicitly.
		handlerCtx := context.Context(signalOnlyContext{Context: delivery.ctx})
		handlerCtx = WithEnvelopeContext(handlerCtx, envelope)
		if broker.propagator != nil {
			handlerCtx = broker.propagator.Extract(handlerCtx, envelope)
		}
		metadata, _ := runtimecontext.MetadataFrom(handlerCtx)
		metadata.Transport = "event"
		metadata.Protocol = broker.Name()
		metadata.Operation = envelope.Topic
		metadata.Method = envelope.Type
		if metadata.Attributes == nil {
			metadata.Attributes = make(map[string]string)
		}
		metadata.Attributes["event.type"] = envelope.Type
		handlerCtx = runtimecontext.WithMetadata(handlerCtx, metadata)
		if err := safeHandle(broker.chain, handlerCtx, handler, envelope); err != nil {
			delivery.add(err)
		}
	})
	if err != nil {
		return nil, err
	}
	sub := &subscription{done: make(chan struct{}), closeFn: func() error { return broker.bus.UnSubscribe(topic, index) }}
	go func() {
		select {
		case <-ctx.Done():
		case <-broker.ctx.Done():
		case <-sub.done:
			return
		}
		_ = sub.Close()
	}()
	return sub, nil
}

func (broker *LocalBroker) Close() error {
	if broker == nil {
		return nil
	}
	broker.mu.Lock()
	if broker.closed {
		broker.mu.Unlock()
		return nil
	}
	broker.closed = true
	broker.cancel()
	ownsBus := broker.ownsBus
	bus := broker.bus
	broker.mu.Unlock()
	if ownsBus && bus != nil {
		return bus.Close()
	}
	return nil
}

type signalOnlyContext struct{ context.Context }

func (signalOnlyContext) Value(any) any { return nil }

type localDelivery struct {
	ctx      context.Context
	envelope Envelope
	mu       sync.Mutex
	errors   []error
}

func (delivery *localDelivery) add(err error) {
	if err == nil {
		return
	}
	delivery.mu.Lock()
	delivery.errors = append(delivery.errors, err)
	delivery.mu.Unlock()
}

func (delivery *localDelivery) err() error {
	delivery.mu.Lock()
	defer delivery.mu.Unlock()
	return errors.Join(delivery.errors...)
}

func safeHandle(chain middleware.Chain, ctx context.Context, handler Handler, envelope Envelope) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Join(err, fmt.Errorf("event: consumer panic: %v", recovered))
		}
	}()
	return chain.Handle(ctx, func(current context.Context) error { return handler(current, envelope) })
}
