package testkit

import (
	"context"
	"errors"
	"sort"
	"sync"

	"yunka.io/framework/event"
)

type Broker struct {
	mu        sync.RWMutex
	closed    bool
	nextID    uint64
	handlers  map[string]map[uint64]event.Handler
	published []event.Envelope
	done      chan struct{}
}

func NewBroker() *Broker {
	return &Broker{handlers: make(map[string]map[uint64]event.Handler), done: make(chan struct{})}
}
func (broker *Broker) Name() string { return "testkit" }

func (broker *Broker) Publish(ctx context.Context, envelope event.Envelope) error {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := envelope.Normalize()
	if err != nil {
		return err
	}
	broker.mu.Lock()
	if broker.closed {
		broker.mu.Unlock()
		return event.ErrClosed
	}
	broker.published = append(broker.published, normalized.Clone())
	ids := make([]uint64, 0, len(broker.handlers[normalized.Topic]))
	for id := range broker.handlers[normalized.Topic] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	handlers := make([]event.Handler, 0, len(ids))
	for _, id := range ids {
		handlers = append(handlers, broker.handlers[normalized.Topic][id])
	}
	broker.mu.Unlock()
	var joined error
	for _, handler := range handlers {
		if err := safeHandler(ctx, handler, normalized.Clone()); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (broker *Broker) Subscribe(ctx context.Context, topic string, handler event.Handler) (event.Subscription, error) {
	if handler == nil {
		return nil, event.ErrNilHandler
	}
	if ctx == nil {
		ctx = context.Background()
	}
	broker.mu.Lock()
	if broker.closed {
		broker.mu.Unlock()
		return nil, event.ErrClosed
	}
	broker.nextID++
	id := broker.nextID
	if broker.handlers[topic] == nil {
		broker.handlers[topic] = make(map[uint64]event.Handler)
	}
	broker.handlers[topic][id] = handler
	broker.mu.Unlock()
	subscription := &brokerSubscription{done: make(chan struct{}), closeFn: func() {
		broker.mu.Lock()
		defer broker.mu.Unlock()
		if handlers := broker.handlers[topic]; handlers != nil {
			delete(handlers, id)
			if len(handlers) == 0 {
				delete(broker.handlers, topic)
			}
		}
	}}
	go func() {
		select {
		case <-ctx.Done():
			_ = subscription.Close()
		case <-broker.done:
			_ = subscription.Close()
		case <-subscription.done:
		}
	}()
	return subscription, nil
}

func (broker *Broker) Published() []event.Envelope {
	broker.mu.RLock()
	defer broker.mu.RUnlock()
	result := make([]event.Envelope, 0, len(broker.published))
	for _, envelope := range broker.published {
		result = append(result, envelope.Clone())
	}
	return result
}

func (broker *Broker) Close() error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed {
		return nil
	}
	broker.closed = true
	broker.handlers = nil
	close(broker.done)
	return nil
}

func safeHandler(ctx context.Context, handler event.Handler, envelope event.Envelope) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("testkit broker: handler panic")
		}
	}()
	return handler(ctx, envelope)
}

type brokerSubscription struct {
	once    sync.Once
	closeFn func()
	done    chan struct{}
}

func (subscription *brokerSubscription) Close() error {
	if subscription == nil {
		return nil
	}
	subscription.once.Do(func() {
		if subscription.closeFn != nil {
			subscription.closeFn()
		}
		if subscription.done != nil {
			close(subscription.done)
		}
	})
	return nil
}
