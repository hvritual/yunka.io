package middleware

import (
	"context"
	"testing"

	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

func TestUnaryClientInterceptorPublishesStandardGRPCMetadata(t *testing.T) {
	called := false
	interceptor := UnaryClientInterceptor(func(next Handler) Handler {
		return func(ctx context.Context) error {
			metadata, ok := runtimecontext.MetadataFrom(ctx)
			if !ok {
				t.Fatal("runtime metadata missing")
			}
			if metadata.Protocol != "grpc" || metadata.Operation != "/svc/Get" || metadata.Attributes["rpc.direction"] != "client" {
				t.Fatalf("metadata=%+v", metadata)
			}
			return next(ctx)
		}
	})
	err := interceptor(context.Background(), "/svc/Get", nil, nil, nil,
		func(context.Context, string, interface{}, interface{}, *grpcgo.ClientConn, ...grpcgo.CallOption) error {
			called = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("grpc invoker was not called")
	}
}
