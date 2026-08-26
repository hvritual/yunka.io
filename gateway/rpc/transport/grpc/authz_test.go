package grpc

import (
	"context"
	"testing"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type grpcChecker func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error)

func (fn grpcChecker) HasPermissions(ctx context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	return fn(ctx, tenant, roles, permissions, mode)
}

func TestAuthorizedUnaryServerInterceptor(t *testing.T) {
	authorizer, _ := authz.NewRBACAuthorizer(grpcChecker(func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) {
		return true, nil
	}))
	resolver := authz.NewStaticResolver(map[string]authz.Policy{
		"/device.v1.Device/Get": {Operation: "device.machine.get", Permissions: []authz.PermissionKey{"device.machine.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodJWT}},
	})
	interceptor, err := AuthorizedUnaryServerInterceptor(authorizer, resolver)
	if err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{Authenticated: true, TenantID: "t1", Roles: []string{"r1"}, AuthMethod: identity.AuthMethodJWT})
	called := false
	_, err = interceptor(ctx, struct{}{}, &grpcgo.UnaryServerInfo{FullMethod: "/device.v1.Device/Get"}, func(context.Context, interface{}) (interface{}, error) { called = true; return struct{}{}, nil })
	if err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

func TestAuthorizedUnaryServerInterceptorDenies(t *testing.T) {
	authorizer, _ := authz.NewRBACAuthorizer(grpcChecker(func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) {
		return false, nil
	}))
	resolver := authz.NewStaticResolver(map[string]authz.Policy{"/device.v1.Device/Delete": {Operation: "device.machine.delete", Permissions: []authz.PermissionKey{"device.machine.delete"}, TenantRequired: true}})
	interceptor, _ := AuthorizedUnaryServerInterceptor(authorizer, resolver)
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{Authenticated: true, TenantID: "t1", Roles: []string{"r1"}, AuthMethod: identity.AuthMethodJWT})
	_, err := interceptor(ctx, struct{}{}, &grpcgo.UnaryServerInfo{FullMethod: "/device.v1.Device/Delete"}, func(context.Context, interface{}) (interface{}, error) { t.Fatal("handler invoked"); return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}
