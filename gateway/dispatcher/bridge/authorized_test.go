package bridge

import (
	"context"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/authz"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type authzExecutorFunc func(string, string, *request.Context, *meta.RuntimeApi) ([]byte, error)

func (fn authzExecutorFunc) Do(m, s string, rt *request.Context, api *meta.RuntimeApi) ([]byte, error) {
	return fn(m, s, rt, api)
}

type authzCheckerFunc func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error)

func (fn authzCheckerFunc) HasPermissions(ctx context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	return fn(ctx, tenant, roles, permissions, mode)
}

func TestAuthorizedExecutorEnforcesTypedOperation(t *testing.T) {
	called := 0
	inner := authzExecutorFunc(func(string, string, *request.Context, *meta.RuntimeApi) ([]byte, error) {
		called++
		return []byte(`ok`), nil
	})
	authorizer, err := authz.NewRBACAuthorizer(authzCheckerFunc(func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) {
		return true, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewAuthorizedExecutor(inner, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	raw := &fasthttp.RequestCtx{}
	rt := request.NewHTTPRequestContext(raw)
	rt.SetPrincipal(identity.Principal{Authenticated: true, TenantID: "t1", Roles: []string{"r1"}, AuthMethod: identity.AuthMethodJWT})
	api := &meta.RuntimeApi{Authorization: &meta.RuntimeAuthorization{OperationId: "device.machine.get", Permissions: []string{"device.machine.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodJWT}}}
	if _, err := executor.Do("device", "service", rt, api); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("called=%d want=1", called)
	}
}

func TestAuthorizedExecutorDeniesBeforeTarget(t *testing.T) {
	called := 0
	inner := authzExecutorFunc(func(string, string, *request.Context, *meta.RuntimeApi) ([]byte, error) { called++; return nil, nil })
	authorizer, _ := authz.NewRBACAuthorizer(authzCheckerFunc(func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) {
		return false, nil
	}))
	executor, _ := NewAuthorizedExecutor(inner, authorizer)
	raw := &fasthttp.RequestCtx{}
	rt := request.NewHTTPRequestContext(raw)
	rt.SetPrincipal(identity.Principal{Authenticated: true, TenantID: "t1", Roles: []string{"r1"}, AuthMethod: identity.AuthMethodJWT})
	api := &meta.RuntimeApi{Authorization: &meta.RuntimeAuthorization{OperationId: "device.machine.delete", Permissions: []string{"device.machine.delete"}, TenantRequired: true}}
	_, err := executor.Do("device", "service", rt, api)
	if !authz.IsDenied(err) {
		t.Fatalf("err=%v", err)
	}
	if called != 0 {
		t.Fatalf("target called=%d", called)
	}
}
