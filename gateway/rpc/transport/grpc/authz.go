package grpc

import (
	"context"
	"errors"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var (
	ErrGRPCAuthorizerUnavailable       = errors.New("grpc transport authz: authorizer is required")
	ErrGRPCPolicyResolverUnavailable   = errors.New("grpc transport authz: policy resolver is required")
	ErrGRPCOperationRuntimeUnavailable = errors.New("grpc transport authz: operation runtime is required")
)

func securityStatus(err error) error {
	if !authz.IsDenied(err) {
		return status.Error(codes.Internal, "authorization unavailable")
	}
	var denied *authz.DeniedError
	if errors.As(err, &denied) && (denied.Decision.Reason == authz.ReasonUnauthenticated || denied.Decision.Reason == authz.ReasonAuthenticationMethod) {
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	return status.Error(codes.PermissionDenied, "permission denied")
}

// SecuredUnaryServerInterceptor is the C8.5 pre-Application security boundary.
func SecuredUnaryServerInterceptor(runtime authz.OperationRuntime) (grpcgo.UnaryServerInterceptor, error) {
	if runtime == nil {
		return nil, ErrGRPCOperationRuntimeUnavailable
	}
	return func(ctx context.Context, req interface{}, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (interface{}, error) {
		if info == nil {
			return nil, status.Error(codes.Internal, "missing gRPC method metadata")
		}
		secured, err := runtime.Prepare(ctx, info.FullMethod, req)
		if err != nil {
			return nil, securityStatus(err)
		}
		return handler(secured, req)
	}, nil
}

// AuthorizedUnaryServerInterceptor is retained for C8.4 compatibility and is
// implemented through the C8.5 OperationRuntime without domain guards.
func AuthorizedUnaryServerInterceptor(authorizer authz.Authorizer, resolver authz.PolicyResolver) (grpcgo.UnaryServerInterceptor, error) {
	if authorizer == nil {
		return nil, ErrGRPCAuthorizerUnavailable
	}
	if resolver == nil {
		return nil, ErrGRPCPolicyResolverUnavailable
	}
	runtime, err := authz.NewOperationRuntime(resolver, authorizer, nil)
	if err != nil {
		return nil, err
	}
	return SecuredUnaryServerInterceptor(runtime)
}

func NewSecuredServer(runtime authz.OperationRuntime, options ...grpcgo.ServerOption) (*GrpcServer, error) {
	interceptor, err := SecuredUnaryServerInterceptor(runtime)
	if err != nil {
		return nil, err
	}
	opts := make([]grpcgo.ServerOption, 0, len(options)+1)
	opts = append(opts, grpcgo.ChainUnaryInterceptor(interceptor))
	opts = append(opts, options...)
	return NewTypedGrpcServer(grpcgo.NewServer(opts...))
}

func NewAuthorizedServer(authorizer authz.Authorizer, resolver authz.PolicyResolver, options ...grpcgo.ServerOption) (*GrpcServer, error) {
	interceptor, err := AuthorizedUnaryServerInterceptor(authorizer, resolver)
	if err != nil {
		return nil, err
	}
	opts := make([]grpcgo.ServerOption, 0, len(options)+1)
	opts = append(opts, grpcgo.ChainUnaryInterceptor(interceptor))
	opts = append(opts, options...)
	return NewTypedGrpcServer(grpcgo.NewServer(opts...))
}
