package grpc

import (
	"context"

	"google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"
	coremiddleware "yunka.io/framework/core/middleware"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/observability"
)

// UnaryServerInterceptor adapts the transport-neutral middleware chain to gRPC
// without modifying generated service registration code. It intentionally does
// not trust caller identity metadata; a service-side verifier must establish a
// Principal before authorization middleware consumes it.
func UnaryServerInterceptor(chain coremiddleware.Chain) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = observability.Extract(ctx, incomingMetadataCarrier(ctx))
		metadata, _ := runtimecontext.MetadataFrom(ctx)
		metadata.Transport = "rpc"
		metadata.Protocol = "grpc"
		metadata.Operation = info.FullMethod
		metadata.Method = info.FullMethod
		if metadata.Attributes == nil {
			metadata.Attributes = make(map[string]string)
		}
		metadata.Attributes["rpc.direction"] = "server"
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
		if metadata.Attributes == nil {
			metadata.Attributes = make(map[string]string)
		}
		metadata.Attributes["rpc.direction"] = "client"
		ctx = runtimecontext.WithMetadata(ctx, metadata)

		return chain.Handle(ctx, func(child context.Context) error {
			carrier := outgoingMetadataCarrier(child)
			observability.Inject(child, carrier)
			return invoker(carrier.Context(), method, req, reply, cc, opts...)
		})
	}
}

// metadataCarrier adapts gRPC metadata to the transport-neutral OTel carrier.
type metadataCarrier struct {
	ctx context.Context
	md  grpcmetadata.MD
}

func incomingMetadataCarrier(ctx context.Context) metadataCarrier {
	md, _ := grpcmetadata.FromIncomingContext(ctx)
	return metadataCarrier{ctx: ctx, md: md.Copy()}
}

func outgoingMetadataCarrier(ctx context.Context) *metadataCarrier {
	md, _ := grpcmetadata.FromOutgoingContext(ctx)
	if md == nil {
		md = grpcmetadata.MD{}
	} else {
		md = md.Copy()
	}
	return &metadataCarrier{ctx: ctx, md: md}
}

func (carrier metadataCarrier) Get(key string) string {
	values := carrier.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (carrier metadataCarrier) Set(key, value string) {
	carrier.md.Set(key, value)
}

func (carrier metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(carrier.md))
	for key := range carrier.md {
		keys = append(keys, key)
	}
	return keys
}

func (carrier *metadataCarrier) Context() context.Context {
	return grpcmetadata.NewOutgoingContext(carrier.ctx, carrier.md)
}
