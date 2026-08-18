package grpc

import (
	"context"
	"testing"

	stdgrpc "google.golang.org/grpc"
	coremiddleware "yunka.io/framework/core/middleware"
	"yunka.io/framework/core/runtimecontext"
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
