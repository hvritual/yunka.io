package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
)

type roleCapture struct{ got bool }

func (*roleCapture) Name() string                                                 { return "role-capture" }
func (*roleCapture) Use(proxy.MiddleWare) proxy.MiddleWare                        { return nil }
func (capture *roleCapture) Do(auth bool, _ *request.Context, _ *meta.RuntimeApi) { capture.got = auth }

type noDecisionRoleIntercept struct{ meta.GatewayServiceServer }

func TestRoleMiddlewareDoesNotMakeAuthorizationDecision(t *testing.T) {
	middleware := NewEnterpriseRoleMiddleware(&noDecisionRoleIntercept{})
	capture := &roleCapture{}
	middleware.Use(capture)

	runtime := request.NewHTTPRequestContext(&fasthttp.RequestCtx{})
	middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthRole, Uuid: "api-1"})
	if capture.got {
		t.Fatal("legacy role middleware promoted an unauthorized request")
	}

	middleware.Do(true, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthRole, Uuid: "api-1"})
	if !capture.got {
		t.Fatal("legacy role middleware did not preserve upstream authorization state")
	}
}
