package observability

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

// Carrier mirrors the OpenTelemetry TextMapCarrier contract without exposing
// OTel types to transport adapter packages.
type Carrier interface {
	Get(string) string
	Set(string, string)
	Keys() []string
}

var defaultPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

func Extract(ctx context.Context, carrier Carrier) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if carrier == nil {
		return ctx
	}
	return defaultPropagator.Extract(ctx, carrier)
}

func Inject(ctx context.Context, carrier Carrier) {
	if ctx == nil || carrier == nil {
		return
	}
	defaultPropagator.Inject(ctx, carrier)
}
