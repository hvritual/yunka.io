package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/rpc/meta"
)

func TestJwtMiddlewarePreservesPriorAPIAuthenticationWithoutToken(t *testing.T) {
	middleware := &JwtMiddleware{}
	capture := &authCapture{}
	middleware.Use(capture)

	runtime := request.NewWorkRuntime()
	runtime.SetRequestCtx(&fasthttp.RequestCtx{})
	middleware.Do(true, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthApi})
	if !capture.got {
		t.Fatal("JWT middleware cleared an already authenticated API-key request")
	}
}
