package grpc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	coremiddleware "yunka.io/framework/core/middleware"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/observability"
)

// UnaryServerInterceptor adapts the transport-neutral middleware chain to gRPC
// without modifying generated service registration code. It intentionally does
// not establish a trusted Principal; production services should use
// AuthenticatedUnaryServerInterceptor at the RPC trust boundary.
func UnaryServerInterceptor(chain coremiddleware.Chain) grpc.UnaryServerInterceptor {
	return unaryServerInterceptor(chain, nil, false)
}

// AuthenticatedUnaryServerInterceptor fails closed unless verifier establishes
// a complete authenticated Principal before authorization middleware runs.
func AuthenticatedUnaryServerInterceptor(chain coremiddleware.Chain, verifier CredentialVerifier) grpc.UnaryServerInterceptor {
	return unaryServerInterceptor(chain, verifier, true)
}

func unaryServerInterceptor(chain coremiddleware.Chain, verifier CredentialVerifier, requireAuthentication bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		fullMethod := ""
		if info != nil {
			fullMethod = info.FullMethod
		}
		ctx = deriveRPCServerContext(ctx, fullMethod)
		if requireAuthentication {
			var err error
			ctx, err = authenticatedRPCContext(ctx, verifier)
			if err != nil {
				return nil, err
			}
		}

		var response interface{}
		err := chain.Handle(ctx, func(child context.Context) error {
			var callErr error
			response, callErr = handler(child, req)
			return callErr
		})
		return response, err
	}
}

// StreamServerInterceptor provides the same transport-neutral metadata boundary
// for streaming RPCs but does not authenticate callers.
func StreamServerInterceptor(chain coremiddleware.Chain) grpc.StreamServerInterceptor {
	return streamServerInterceptor(chain, nil, false)
}

// AuthenticatedStreamServerInterceptor closes the streaming bypass: the same
// verifier and Principal rules used for unary RPCs are applied before a stream
// handler receives its context.
func AuthenticatedStreamServerInterceptor(chain coremiddleware.Chain, verifier CredentialVerifier) grpc.StreamServerInterceptor {
	return streamServerInterceptor(chain, verifier, true)
}

func streamServerInterceptor(chain coremiddleware.Chain, verifier CredentialVerifier, requireAuthentication bool) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		fullMethod := ""
		if info != nil {
			fullMethod = info.FullMethod
		}
		ctx := deriveRPCServerContext(stream.Context(), fullMethod)
		if requireAuthentication {
			var err error
			ctx, err = authenticatedRPCContext(ctx, verifier)
			if err != nil {
				return err
			}
		}
		wrapped := &serverStreamWithContext{ServerStream: stream, ctx: ctx}
		return chain.Handle(ctx, func(child context.Context) error {
			wrapped.ctx = child
			return handler(srv, wrapped)
		})
	}
}

func UnaryClientInterceptor(chain coremiddleware.Chain) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = deriveRPCClientContext(ctx, method)
		return chain.Handle(ctx, func(child context.Context) error {
			carrier := outgoingMetadataCarrier(child)
			observability.Inject(child, carrier)
			return invoker(carrier.Context(), method, req, reply, cc, opts...)
		})
	}
}

func deriveRPCServerContext(ctx context.Context, fullMethod string) context.Context {
	ctx = observability.Extract(ctx, incomingMetadataCarrier(ctx))
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Transport = "rpc"
	metadata.Protocol = "grpc"
	metadata.Operation = fullMethod
	metadata.Method = fullMethod
	if metadata.Attributes == nil {
		metadata.Attributes = make(map[string]string)
	}
	metadata.Attributes["rpc.direction"] = "server"
	return runtimecontext.WithMetadata(ctx, metadata)
}

func deriveRPCClientContext(ctx context.Context, method string) context.Context {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	metadata.Transport = "rpc"
	metadata.Protocol = "grpc"
	metadata.Operation = method
	metadata.Method = method
	if metadata.Attributes == nil {
		metadata.Attributes = make(map[string]string)
	}
	metadata.Attributes["rpc.direction"] = "client"
	return runtimecontext.WithMetadata(ctx, metadata)
}

func authenticatedRPCContext(ctx context.Context, verifier CredentialVerifier) (context.Context, error) {
	if verifier == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	principal, err := verifyCredential(ctx, verifier)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return nil, status.Error(codes.Canceled, context.Canceled.Error())
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
		default:
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
	}
	if !principal.Authenticated || strings.TrimSpace(principal.Subject) == "" || strings.TrimSpace(principal.AuthMethod) == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return identity.WithPrincipal(ctx, principal), nil
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (stream *serverStreamWithContext) Context() context.Context { return stream.ctx }

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
