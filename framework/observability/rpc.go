package observability

import (
	"context"

	grpcgo "google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"
	"github.com/hvritual/yunka.io/framework/core/middleware"
	"github.com/hvritual/yunka.io/framework/core/resilience"
)

// UnaryClientPropagationInterceptor injects the canonical W3C Trace Context
// and Baggage into outgoing gRPC metadata. It deliberately does not create a
// span; callers may compose it with Provider middleware or use it as the
// transport-only correct-by-default propagation layer.
func UnaryClientPropagationInterceptor() grpcgo.UnaryClientInterceptor {
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
		carrier := outgoingGRPCMetadataCarrier(ctx)
		Inject(ctx, carrier)
		return invoker(carrier.Context(), method, request, reply, connection, options...)
	}
}

// StreamClientPropagationInterceptor provides the same canonical propagation
// contract for grpc-go streaming clients.
func StreamClientPropagationInterceptor() grpcgo.StreamClientInterceptor {
	return func(
		ctx context.Context,
		description *grpcgo.StreamDesc,
		connection *grpcgo.ClientConn,
		method string,
		streamer grpcgo.Streamer,
		options ...grpcgo.CallOption,
	) (grpcgo.ClientStream, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		carrier := outgoingGRPCMetadataCarrier(ctx)
		Inject(ctx, carrier)
		return streamer(carrier.Context(), description, connection, method, options...)
	}
}

// UnaryClientInterceptor instruments the single grpc-go unary client runtime
// and injects the resulting client span context before transport invocation.
func (provider *Provider) UnaryClientInterceptor() grpcgo.UnaryClientInterceptor {
	middlewares := make([]middleware.Middleware, 0, 1)
	if provider != nil {
		middlewares = append(middlewares, provider.Middleware())
	}
	return withUnaryPropagation(middleware.UnaryClientInterceptor(middlewares...))
}

// GovernedUnaryClientInterceptor creates one logical RPC observability span
// around W3 policy execution, records policy snapshots after the call, and
// injects the resulting client span context at the transport boundary.
func (provider *Provider) GovernedUnaryClientInterceptor(policy *resilience.RPCPolicy) grpcgo.UnaryClientInterceptor {
	middlewares := make([]middleware.Middleware, 0, 7)
	if provider != nil {
		middlewares = append(middlewares, provider.Middleware())
		if policy != nil {
			middlewares = append(middlewares, provider.ResilienceMiddleware(policy))
		}
	}
	if policy != nil {
		middlewares = append(middlewares, policy.Middlewares()...)
	}
	return withUnaryPropagation(middleware.UnaryClientInterceptor(middlewares...))
}

func withUnaryPropagation(runtime grpcgo.UnaryClientInterceptor) grpcgo.UnaryClientInterceptor {
	propagation := UnaryClientPropagationInterceptor()
	return func(
		ctx context.Context,
		method string,
		request interface{},
		reply interface{},
		connection *grpcgo.ClientConn,
		invoker grpcgo.UnaryInvoker,
		options ...grpcgo.CallOption,
	) error {
		return runtime(ctx, method, request, reply, connection, func(
			child context.Context,
			childMethod string,
			childRequest interface{},
			childReply interface{},
			childConnection *grpcgo.ClientConn,
			childOptions ...grpcgo.CallOption,
		) error {
			return propagation(child, childMethod, childRequest, childReply, childConnection, invoker, childOptions...)
		}, options...)
	}
}

type grpcMetadataCarrier struct {
	ctx context.Context
	md  grpcmetadata.MD
}

func outgoingGRPCMetadataCarrier(ctx context.Context) *grpcMetadataCarrier {
	metadata, _ := grpcmetadata.FromOutgoingContext(ctx)
	if metadata == nil {
		metadata = grpcmetadata.MD{}
	} else {
		metadata = metadata.Copy()
	}
	return &grpcMetadataCarrier{ctx: ctx, md: metadata}
}

func (carrier *grpcMetadataCarrier) Get(key string) string {
	if carrier == nil {
		return ""
	}
	values := carrier.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (carrier *grpcMetadataCarrier) Set(key, value string) {
	if carrier != nil {
		carrier.md.Set(key, value)
	}
}

func (carrier *grpcMetadataCarrier) Keys() []string {
	if carrier == nil {
		return nil
	}
	keys := make([]string, 0, len(carrier.md))
	for key := range carrier.md {
		keys = append(keys, key)
	}
	return keys
}

func (carrier *grpcMetadataCarrier) Context() context.Context {
	if carrier == nil {
		return context.Background()
	}
	return grpcmetadata.NewOutgoingContext(carrier.ctx, carrier.md)
}
