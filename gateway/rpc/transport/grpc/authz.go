package grpc

import (
	"context"
	"errors"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

var (
	ErrGRPCAuthorizerUnavailable     = errors.New("grpc transport authz: authorizer is required")
	ErrGRPCPolicyResolverUnavailable = errors.New("grpc transport authz: policy resolver is required")
)

func AuthorizedUnaryServerInterceptor(authorizer authz.Authorizer, resolver authz.PolicyResolver) (grpcgo.UnaryServerInterceptor, error) {
	if authorizer == nil {
		return nil, ErrGRPCAuthorizerUnavailable
	}
	if resolver == nil {
		return nil, ErrGRPCPolicyResolverUnavailable
	}
	return func(ctx context.Context, req interface{}, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (interface{}, error) {
		if info == nil {
			return nil, status.Error(codes.Internal, "missing gRPC method metadata")
		}
		policy, ok := resolver.ResolvePolicy(ctx, info.FullMethod)
		if !ok || !policy.Protected() {
			return handler(ctx, req)
		}
		principal, _ := identity.FromContext(ctx)
		decision, err := authorizer.Authorize(ctx, principal, policy)
		if err != nil {
			return nil, status.Error(codes.Internal, "authorization unavailable")
		}
		if !decision.Allowed {
			if decision.Reason == authz.ReasonUnauthenticated || decision.Reason == authz.ReasonAuthenticationMethod {
				return nil, status.Error(codes.Unauthenticated, "authentication required")
			}
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}
		return handler(ctx, req)
	}, nil
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
