package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/identity"
)

type grantCheckerFunc func(context.Context, string, []string, []PermissionKey) ([]Grant, error)

func (f grantCheckerFunc) ResolveGrants(ctx context.Context, tenant string, roles []string, permissions []PermissionKey) ([]Grant, error) {
	return f(ctx, tenant, roles, permissions)
}

type guardFunc func(context.Context, AuthorizedOperation, any) (context.Context, error)

func (f guardFunc) Prepare(ctx context.Context, authorized AuthorizedOperation, input any) (context.Context, error) {
	return f(ctx, authorized, input)
}

type testGuardKey struct{}

func TestGrantAuthorizerBindsScopeToPermissionGrant(t *testing.T) {
	checker := grantCheckerFunc(func(_ context.Context, tenant string, roles []string, permissions []PermissionKey) ([]Grant, error) {
		if tenant != "tenant-a" || len(roles) != 2 || len(permissions) != 1 || permissions[0] != "device.read" {
			t.Fatalf("unexpected grant query tenant=%q roles=%v permissions=%v", tenant, roles, permissions)
		}
		return []Grant{{Permission: "device.read", RoleID: "reader", Scope: "self"}}, nil
	})
	authorizer, err := NewGrantAuthorizer(checker)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader", "other"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
	decision, err := authorizer.Authorize(context.Background(), principal, Policy{Operation: "device.list", Permissions: []PermissionKey{"device.read"}, Mode: PermissionAll, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || len(decision.Grants) != 1 || decision.Grants[0].RoleID != "reader" || decision.Grants[0].Scope != "self" {
		t.Fatalf("decision=%#v", decision)
	}
}

func TestOperationRuntimeRunsGuardBeforeApplicationBoundary(t *testing.T) {
	authorizer, err := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) {
		return []Grant{{Permission: "device.read", RoleID: "reader", Scope: "sites"}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewStaticResolver(map[string]Policy{"/svc/List": {Operation: "device.list", Permissions: []PermissionKey{"device.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}}})
	guardCalled := false
	guards := NewStaticGuardResolver(map[OperationID]OperationGuard{"device.list": guardFunc(func(ctx context.Context, authorized AuthorizedOperation, input any) (context.Context, error) {
		guardCalled = true
		if authorized.Decision.Allowed != true || len(authorized.Decision.Grants) != 1 || input != "request" {
			t.Fatalf("authorized=%#v input=%v", authorized, input)
		}
		if _, ok := AuthorizedOperationFromContext(ctx); !ok {
			t.Fatal("authorized context missing inside guard")
		}
		return context.WithValue(ctx, testGuardKey{}, "scoped"), nil
	})})
	runtime, err := NewOperationRuntime(resolver, authorizer, guards)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
	ctx := identity.WithPrincipal(context.Background(), principal)
	secured, err := runtime.Prepare(ctx, "/svc/List", "request")
	if err != nil {
		t.Fatal(err)
	}
	if !guardCalled {
		t.Fatal("guard not called")
	}
	if secured.Value(testGuardKey{}) != "scoped" {
		t.Fatal("guard context not propagated")
	}
	if _, err := RequireAuthorizedOperation(secured, "device.list"); err != nil {
		t.Fatal(err)
	}
}

func TestOperationRuntimeDenialNeverRunsGuard(t *testing.T) {
	authorizer, err := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) { return nil, nil }))
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewStaticResolver(map[string]Policy{"/svc/List": {Operation: "device.list", Permissions: []PermissionKey{"device.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}}})
	guardCalled := false
	runtime, err := NewOperationRuntime(resolver, authorizer, NewStaticGuardResolver(map[OperationID]OperationGuard{"device.list": guardFunc(func(ctx context.Context, _ AuthorizedOperation, _ any) (context.Context, error) {
		guardCalled = true
		return ctx, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
	_, err = runtime.Prepare(identity.WithPrincipal(context.Background(), principal), "/svc/List", nil)
	if !IsDenied(err) {
		t.Fatalf("err=%v", err)
	}
	if guardCalled {
		t.Fatal("denied request reached guard")
	}
}

func TestOperationRuntimeGuardErrorStopsBoundary(t *testing.T) {
	sentinel := errors.New("scope denied")
	authorizer, _ := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) {
		return []Grant{{Permission: "p"}}, nil
	}))
	resolver := NewStaticResolver(map[string]Policy{"/svc/Get": {Operation: "get", Permissions: []PermissionKey{"p"}, TenantRequired: true}})
	runtime, _ := NewOperationRuntime(resolver, authorizer, NewStaticGuardResolver(map[OperationID]OperationGuard{"get": guardFunc(func(context.Context, AuthorizedOperation, any) (context.Context, error) { return nil, sentinel })}))
	principal := identity.Principal{TenantID: "tenant", UserID: "u", Roles: []string{"r"}, Authenticated: true}
	_, err := runtime.Prepare(identity.WithPrincipal(context.Background(), principal), "/svc/Get", nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v", err)
	}
}
