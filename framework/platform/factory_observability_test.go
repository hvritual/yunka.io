package platform

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCFactoryAlwaysPropagatesW3CTraceContext(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	received := make(chan grpcmetadata.MD, 1)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context,
		request interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		metadata, _ := grpcmetadata.FromIncomingContext(ctx)
		select {
		case received <- metadata.Copy():
		default:
		}
		return handler(ctx, request)
	}))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	factory := GRPCFactory{Configurations: map[string]GRPCConfig{
		"downstream": {
			Target:      "passthrough:///bufnet",
			Credentials: insecure.NewCredentials(),
			DialOptions: []grpc.DialOption{
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
			},
		},
	}}
	resource, err := factory.Open(context.Background(), "downstream")
	if err != nil {
		t.Fatal(err)
	}
	if resource.ShutdownFunc != nil {
		t.Cleanup(func() { _ = resource.ShutdownFunc(context.Background()) })
	}

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	callContext := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	callContext, cancel := context.WithTimeout(callContext, 3*time.Second)
	defer cancel()
	client := grpc_health_v1.NewHealthClient(resource.Connection)
	if _, err := client.Check(callContext, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("health call failed: %v", err)
	}

	select {
	case metadata := <-received:
		values := metadata.Get("traceparent")
		if len(values) != 1 {
			t.Fatalf("traceparent values=%v", values)
		}
		if !strings.Contains(values[0], traceID.String()) || !strings.Contains(values[0], spanID.String()) {
			t.Fatalf("traceparent=%q does not contain trace=%s span=%s", values[0], traceID, spanID)
		}
	case <-time.After(time.Second):
		t.Fatal("downstream server did not observe propagated metadata")
	}
}
