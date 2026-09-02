package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/resilience"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

// Middleware creates one operation span, records common metrics, emits a
// structured completion log, and converts resilience rejections into runtime
// operational events. It is transport-neutral and can wrap HTTP, RPC, events,
// or jobs.
func (provider *Provider) Middleware() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context) (err error) {
			if provider == nil {
				return next(ctx)
			}
			if ctx == nil {
				ctx = context.Background()
			}

			ctx, span := provider.tracer.Start(ctx, operationName(ctx),
				trace.WithSpanKind(spanKind(ctx)),
				trace.WithAttributes(traceAttributes(ctx, provider.config.IncludeIdentity)...),
			)
			if spanContext := span.SpanContext(); spanContext.IsValid() {
				ctx = runtimecontext.WithTraceID(ctx, spanContext.TraceID().String())
			}
			start := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					panicErr := fmt.Errorf("panic: %v", recovered)
					provider.finishOperation(ctx, span, start, panicErr)
					span.End()
					panic(recovered)
				}
				provider.finishOperation(ctx, span, start, err)
				span.End()
			}()

			err = next(ctx)
			return err
		}
	}
}

func (provider *Provider) finishOperation(ctx context.Context, span trace.Span, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	metricAttrs := metric.WithAttributes(metricAttributes(ctx)...)
	provider.requestCount.Add(ctx, 1, metricAttrs)
	provider.requestDuration.Record(ctx, duration, metricAttrs)
	if err != nil {
		provider.requestErrors.Add(ctx, 1, metricAttrs)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	if rejection := resilienceRejection(err); rejection != nil {
		// Keep metric dimensions bounded. The rejection key may contain a raw
		// operation identifier and is retained in logs/events, not metric labels.
		rejectionAttrs := append(metricAttributes(ctx),
			attributeFromAny("resilience.policy", rejection.Policy),
		)
		provider.resilienceRejects.Add(ctx, 1, metric.WithAttributes(rejectionAttrs...))
		provider.Emit(ctx, Event{
			Name:     "resilience.rejected",
			Severity: slog.LevelWarn,
			Attributes: map[string]any{
				"policy": rejection.Policy,
				"key":    rejection.Key,
			},
			Err: err,
		})
	}

	provider.Info(ctx, "operation.completed",
		"duration_ms", duration*1000,
		"success", err == nil,
	)
}

func resilienceRejection(err error) *resilience.Rejection {
	if err == nil {
		return nil
	}
	var rejection *resilience.Rejection
	if errors.As(err, &rejection) {
		return rejection
	}
	return nil
}

func spanKind(ctx context.Context) trace.SpanKind {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	if metadata.Attributes != nil {
		switch metadata.Attributes["rpc.direction"] {
		case "client":
			return trace.SpanKindClient
		case "server":
			return trace.SpanKindServer
		}
	}
	switch metadata.Transport {
	case "http":
		return trace.SpanKindServer
	case "event":
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindInternal
	}
}
