package middleware

import (
	"context"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/authz"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type middlewareChecker func(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error)

func (fn middlewareChecker) HasPermissions(ctx context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	return fn(ctx, tenant, roles, permissions, mode)
}

type middlewareExecutor struct{ calls int }

func (e *middlewareExecutor) Do(string, string, *request.Context, *meta.RuntimeApi) ([]byte, error) {
	e.calls++
	return []byte(`{"code":"0","msg":"","data":{}}`), nil
}

func TestTypedOperationFailsClosedWithoutAuthorizer(t *testing.T) {
	exec := &middlewareExecutor{}
	middleware := NewHandleMiddleware(exec)
	raw := &fasthttp.RequestCtx{}
	rt := request.NewHTTPRequestContext(raw)
	api := &meta.RuntimeApi{Authorization: &meta.RuntimeAuthorization{OperationId: "device.read", Permissions: []string{"device.read"}}}
	middleware.Do(false, rt, api)
	if exec.calls != 0 {
		t.Fatalf("calls=%d", exec.calls)
	}
}

func TestAuthorizedHandleAuthorizesCompositeChildrenIndependently(t *testing.T) {
	exec := &middlewareExecutor{}
	authorizer, _ := authz.NewRBACAuthorizer(middlewareChecker(func(_ context.Context, _ string, _ []string, permissions []authz.PermissionKey, _ authz.PermissionMode) (bool, error) {
		for _, p := range permissions {
			if p == "child.denied" {
				return false, nil
			}
		}
		return true, nil
	}))
	middleware := NewAuthorizedHandleMiddleware(exec, authorizer)
	raw := &fasthttp.RequestCtx{}
	rt := request.NewHTTPRequestContext(raw)
	rt.SetPrincipal(identity.Principal{Authenticated: true, TenantID: "t1", Roles: []string{"r1"}, AuthMethod: identity.AuthMethodJWT})
	parent := &meta.RuntimeApi{Uri: "/parent", Composes: []*meta.RuntimeApi{{Uri: "/child", Name: "child", Authorization: &meta.RuntimeAuthorization{OperationId: "child", Permissions: []string{"child.denied"}, TenantRequired: true}}}}
	middleware.Do(false, rt, parent)
	if exec.calls != 0 {
		t.Fatalf("denied child reached executor: %d", exec.calls)
	}
}
