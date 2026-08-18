package middleware

import "context"

// Handler is transport-neutral. HTTP, RPC, events and jobs can all execute the
// same chain as long as they can provide a context.Context.
type Handler func(context.Context) error

// Middleware wraps a Handler. Middleware should derive child contexts instead
// of mutating transport-specific request objects whenever possible.
type Middleware func(Handler) Handler

type Chain struct {
	middlewares []Middleware
}

func New(middlewares ...Middleware) Chain {
	return Chain{}.Use(middlewares...)
}

// Use returns a new chain so configured chains are safe to reuse concurrently.
func (chain Chain) Use(middlewares ...Middleware) Chain {
	result := Chain{middlewares: append([]Middleware(nil), chain.middlewares...)}
	for _, current := range middlewares {
		if current != nil {
			result.middlewares = append(result.middlewares, current)
		}
	}
	return result
}

func (chain Chain) Then(final Handler) Handler {
	if final == nil {
		final = func(context.Context) error { return nil }
	}
	wrapped := final
	for index := len(chain.middlewares) - 1; index >= 0; index-- {
		wrapped = chain.middlewares[index](wrapped)
	}
	return wrapped
}

func (chain Chain) Handle(ctx context.Context, final Handler) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return chain.Then(final)(ctx)
}
