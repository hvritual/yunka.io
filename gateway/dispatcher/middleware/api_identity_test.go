package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

func TestAPIMiddlewareEstablishesMachinePrincipal(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	middleware := NewAPIMiddleware(key)
	capture := &authCapture{}
	middleware.Use(capture)

	ctx := &fasthttp.RequestCtx{}
	runtime := request.NewHTTPRequestContext(ctx)
	ctx.Request.Header.Set(xCode, key)
	middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthApi})

	principal, ok := runtime.Principal()
	if !ok || !principal.Authenticated || principal.AuthMethod != identity.AuthMethodAPIKey || principal.Subject != "api-key" {
		t.Fatalf("principal=%#v ok=%v", principal, ok)
	}
}
