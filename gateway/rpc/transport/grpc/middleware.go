package grpc

import (
	"context"

	"google.golang.org/grpc"
	coremiddleware "yunka.io/framework/core/middleware"
	"yunka.io/framework/core/runtimecontext"
)

// UnaryServerInterceptor adapts the transport-neutral middleware chain to gRPC
// without modifying generated service registration code. It intentionally does
// not trust caller identity metadata; a service-side verifier must establish a
// Principal before authorization middleware consumes it.
func UnaryServerInterceptor(chain coremiddleware.Chain) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		metadata, _ := runtimecontext.MetadataFrom(ctx)
		metadata.Transport = "rpc"
		metadata.Protocol = "grpc"
		metadata.Operation = info.FullMethod
		metadata.Method = info.FullMethod
		ctx = runtimecontext.WithMetadata(ctx, metadata)

		var response interface{}
		err := chain.Handle(ctx, func(child context.Context) error {
			var callErr error
			response, callErr = handler(child, req)
			return callErr
		})
		return response, err
	}
}

func UnaryClientInterceptor(chain coremiddleware.Chain) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		metadata, _ := runtimecontext.MetadataFrom(ctx)
		metadata.Transport = "rpc"
		metadata.Protocol = "grpc"
		metadata.Operation = method
		metadata.Method = method
		ctx = runtimecontext.WithMetadata(ctx, metadata)

		return chain.Handle(ctx, func(child context.Context) error {
			return invoker(child, method, req, reply, cc, opts...)
		})
	}
}
