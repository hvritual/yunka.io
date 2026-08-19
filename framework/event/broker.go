package event

import (
	"context"
	"errors"
	"sync"

	"yunka.io/framework/core/middleware"
)

var (
	ErrClosed     = errors.New("event: broker closed")
	ErrNilHandler = errors.New("event: handler is nil")
	ErrNilBroker  = errors.New("event: broker is nil")
)

type Handler func(context.Context, Envelope) error

type Subscription interface {
	Close() error
}

type Broker interface {
	Name() string
	Publish(context.Context, Envelope) error
	Subscribe(context.Context, string, Handler) (Subscription, error)
	Close() error
}

type BrokerOptions struct {
	Middleware []middleware.Middleware
	Propagator Propagator
}

type Option func(*BrokerOptions)

func WithMiddleware(values ...middleware.Middleware) Option {
	return func(options *BrokerOptions) {
		options.Middleware = append(options.Middleware, values...)
	}
}

func WithPropagator(propagator Propagator) Option {
	return func(options *BrokerOptions) { options.Propagator = propagator }
}

type subscription struct {
	once    sync.Once
	closeFn func() error
	err     error
	done    chan struct{}
}

func (current *subscription) Close() error {
	if current == nil {
		return nil
	}
	current.once.Do(func() {
		if current.closeFn != nil {
			current.err = current.closeFn()
		}
		if current.done != nil {
			close(current.done)
		}
	})
	return current.err
}
