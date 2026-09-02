package middleware

import (
	"context"

	grpcgo "google.golang.org/grpc"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

// UnaryClientInterceptor adapts the transport-neutral middleware chain to the
// single grpc-go unary client runtime. It does not own connections, selectors,
// retries, or generated clients.
func UnaryClientInterceptor(middlewares ...Middleware) grpcgo.UnaryClientInterceptor {
	chain := New(middlewares...)
	return func(
		ctx context.Context,
		method string,
		request interface{},
		reply interface{},
		connection *grpcgo.ClientConn,
		invoker grpcgo.UnaryInvoker,
		options ...grpcgo.CallOption,
	) error {
		if ctx == nil {
			ctx = context.Background()
		}
		metadata, _ := runtimecontext.MetadataFrom(ctx)
		metadata.Transport = "rpc"
		metadata.Protocol = "grpc"
		metadata.Operation = method
		metadata.Method = method
		if metadata.Attributes == nil {
			metadata.Attributes = make(map[string]string)
		}
		metadata.Attributes["rpc.direction"] = "client"
		ctx = runtimecontext.WithMetadata(ctx, metadata)
		return chain.Handle(ctx, func(child context.Context) error {
			return invoker(child, method, request, reply, connection, options...)
		})
	}
}
