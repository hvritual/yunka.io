package platform

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"go.opentelemetry.io/otel/trace"
)

func TestGRPCFactoryAlwaysPropagatesW3CTraceContext(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	received := make(chan grpcmetadata.MD, 1)
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ interface{}, stream grpc.ServerStream) error {
		metadata, _ := grpcmetadata.FromIncomingContext(stream.Context())
		received <- metadata.Copy()
		request := &emptypb.Empty{}
		_ = stream.RecvMsg(request)
		return status.Error(codes.Unimplemented, "test completed")
	}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	factory := GRPCFactory{Configurations: map[string]GRPCConfig{
		"downstream": {
			Target:      "bufnet",
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
	_ = resource.Connection.Invoke(callContext, "/test.Service/Call", &emptypb.Empty{}, &emptypb.Empty{})

	select {
	case metadata := <-received:
		values := metadata.Get("traceparent")
		if len(values) != 1 {
			t.Fatalf("traceparent values=%v", values)
		}
		if !strings.Contains(values[0], traceID.String()) || !strings.Contains(values[0], spanID.String()) {
			t.Fatalf("traceparent=%q does not contain trace=%s span=%s", values[0], traceID, spanID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("downstream server did not receive the propagated call")
	}
}
