package observability

import (
	"context"
	"strings"
	"testing"

	grpcgo "google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"
	"go.opentelemetry.io/otel/trace"
)

func TestUnaryClientPropagationInterceptorInjectsTraceContext(t *testing.T) {
	ctx, traceID, spanID := testPropagationContext(t)
	interceptor := UnaryClientPropagationInterceptor()
	called := false
	err := interceptor(ctx, "/test.Service/Call", nil, nil, nil, func(
		callContext context.Context,
		_ string,
		_ interface{},
		_ interface{},
		_ *grpcgo.ClientConn,
		_ ...grpcgo.CallOption,
	) error {
		called = true
		assertOutgoingTraceparent(t, callContext, traceID.String(), spanID.String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("transport invoker was not called")
	}
}

func TestStreamClientPropagationInterceptorInjectsTraceContext(t *testing.T) {
	ctx, traceID, spanID := testPropagationContext(t)
	interceptor := StreamClientPropagationInterceptor()
	called := false
	_, err := interceptor(ctx, &grpcgo.StreamDesc{}, nil, "/test.Service/Stream", func(
		callContext context.Context,
		_ *grpcgo.StreamDesc,
		_ *grpcgo.ClientConn,
		_ string,
		_ ...grpcgo.CallOption,
	) (grpcgo.ClientStream, error) {
		called = true
		assertOutgoingTraceparent(t, callContext, traceID.String(), spanID.String())
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("transport streamer was not called")
	}
}

func testPropagationContext(t *testing.T) (context.Context, trace.TraceID, trace.SpanID) {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), spanContext), traceID, spanID
}

func assertOutgoingTraceparent(t *testing.T, ctx context.Context, traceID, spanID string) {
	t.Helper()
	metadata, ok := grpcmetadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("outgoing gRPC metadata is missing")
	}
	values := metadata.Get("traceparent")
	if len(values) != 1 {
		t.Fatalf("traceparent values=%v", values)
	}
	if !strings.Contains(values[0], traceID) || !strings.Contains(values[0], spanID) {
		t.Fatalf("traceparent=%q does not contain trace=%s span=%s", values[0], traceID, spanID)
	}
}
