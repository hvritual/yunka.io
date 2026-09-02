package grpc

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	stdgrpc "google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"
	coremiddleware "github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

func TestUnaryServerInterceptorDerivesRPCMetadata(t *testing.T) {
	var seen runtimecontext.Metadata
	chain := coremiddleware.New(func(next coremiddleware.Handler) coremiddleware.Handler {
		return func(ctx context.Context) error {
			seen, _ = runtimecontext.MetadataFrom(ctx)
			return next(ctx)
		}
	})
	interceptor := UnaryServerInterceptor(chain)
	response, err := interceptor(context.Background(), "request", &stdgrpc.UnaryServerInfo{FullMethod: "/device.Device/Get"},
		func(context.Context, interface{}) (interface{}, error) { return "response", nil })
	if err != nil {
		t.Fatal(err)
	}
	if response != "response" || seen.Transport != "rpc" || seen.Protocol != "grpc" || seen.Operation != "/device.Device/Get" {
		t.Fatalf("response=%v metadata=%#v", response, seen)
	}
}

func TestUnaryClientInterceptorInjectsTraceParent(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	}))

	interceptor := UnaryClientInterceptor(coremiddleware.New())
	err = interceptor(ctx, "/device.Device/Get", "request", new(string), &stdgrpc.ClientConn{},
		func(child context.Context, _ string, _, _ interface{}, _ *stdgrpc.ClientConn, _ ...stdgrpc.CallOption) error {
			md, ok := grpcmetadata.FromOutgoingContext(child)
			if !ok || len(md.Get("traceparent")) == 0 {
				t.Fatalf("traceparent not injected: %#v", md)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnaryServerInterceptorExtractsRemoteTraceParent(t *testing.T) {
	md := grpcmetadata.MD{}
	md.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	ctx := grpcmetadata.NewIncomingContext(context.Background(), md)

	interceptor := UnaryServerInterceptor(coremiddleware.New())
	_, err := interceptor(ctx, "request", &stdgrpc.UnaryServerInfo{FullMethod: "/device.Device/Get"},
		func(child context.Context, _ interface{}) (interface{}, error) {
			spanContext := trace.SpanContextFromContext(child)
			if !spanContext.IsRemote() || spanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
				t.Fatalf("remote trace context not extracted: %#v", spanContext)
			}
			return "response", nil
		})
	if err != nil {
		t.Fatal(err)
	}
}
