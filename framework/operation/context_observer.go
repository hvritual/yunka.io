package operation

import "context"

type observerContextKey struct{}

func WithObserver(ctx context.Context, observer Observer) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if observer == nil {
		return ctx
	}
	existing, _ := ctx.Value(observerContextKey{}).([]Observer)
	observers := make([]Observer, 0, len(existing)+1)
	observers = append(observers, existing...)
	observers = append(observers, observer)
	return context.WithValue(ctx, observerContextKey{}, observers)
}

func observersFromContext(ctx context.Context) []Observer {
	if ctx == nil {
		return nil
	}
	observers, _ := ctx.Value(observerContextKey{}).([]Observer)
	return append([]Observer(nil), observers...)
}
