package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/rpc/meta"
)

func TestAPIMiddlewareEstablishesMachinePrincipal(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	middleware := NewAPIMiddleware(key)
	capture := &authCapture{}
	middleware.Use(capture)

	runtime := request.NewWorkRuntime()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(xCode, key)
	runtime.SetRequestCtx(ctx)
	middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthApi})

	principal, ok := request.PrincipalFromRuntime(runtime)
	if !ok || !principal.Authenticated || principal.AuthMethod != identity.AuthMethodAPIKey || principal.Subject != "api-key" {
		t.Fatalf("principal=%#v ok=%v", principal, ok)
	}
}
