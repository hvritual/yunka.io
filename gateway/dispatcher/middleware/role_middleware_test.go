package middleware

import (
	"reflect"
	"testing"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/define"
)

type roleCapture struct{ got bool }

func (*roleCapture) Name() string                                                { return "role-capture" }
func (*roleCapture) Use(proxy.MiddleWare) proxy.MiddleWare                       { return nil }
func (capture *roleCapture) Do(auth bool, _ request.Runtime, _ *meta.RuntimeApi) { capture.got = auth }

type allowRoleIntercept struct {
	meta.GatewayServiceServer
	called bool
	org    string
	roles  []string
}

func (intercept *allowRoleIntercept) VerifyRoleApiRight(_ string, org string, roles []string) ([]byte, bool) {
	intercept.called = true
	intercept.org = org
	intercept.roles = append([]string(nil), roles...)
	return nil, true
}

func TestRoleMiddlewareRejectsQueryOnlyIdentity(t *testing.T) {
	intercept := &allowRoleIntercept{}
	middleware := NewEnterpriseRoleMiddleware(intercept)
	capture := &roleCapture{}
	middleware.Use(capture)

	runtime := request.NewWorkRuntime()
	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Set(define.OrgUUID, "attacker-tenant")
	ctx.QueryArgs().Set(define.RoleUUID, "admin")
	runtime.SetRequestCtx(ctx)

	middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthRole})
	if intercept.called || capture.got {
		t.Fatalf("query identity was trusted: intercept=%v auth=%v", intercept.called, capture.got)
	}
}

func TestRoleMiddlewareUsesTrustedPrincipal(t *testing.T) {
	intercept := &allowRoleIntercept{}
	middleware := NewEnterpriseRoleMiddleware(intercept)
	capture := &roleCapture{}
	middleware.Use(capture)

	runtime := request.NewWorkRuntime()
	runtime.SetRequestCtx(&fasthttp.RequestCtx{})
	request.SetPrincipal(runtime, identity.Principal{
		Subject:       "user-1",
		TenantID:      "tenant-1",
		UserID:        "user-1",
		Roles:         []string{"admin", "operator"},
		AuthMethod:    identity.AuthMethodJWT,
		Authenticated: true,
	})

	middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthRole, Uuid: "api-1"})
	if !intercept.called || !capture.got {
		t.Fatalf("trusted principal not authorized: intercept=%v auth=%v", intercept.called, capture.got)
	}
	if intercept.org != "tenant-1" || !reflect.DeepEqual(intercept.roles, []string{"admin", "operator"}) {
		t.Fatalf("org=%q roles=%v", intercept.org, intercept.roles)
	}
}
