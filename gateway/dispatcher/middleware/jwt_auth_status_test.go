package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

func TestJwtMiddlewarePreservesPriorAPIAuthenticationWithoutToken(t *testing.T) {
	middleware := &JwtMiddleware{}
	capture := &authCapture{}
	middleware.Use(capture)

	runtime := request.NewHTTPRequestContext(&fasthttp.RequestCtx{})
	middleware.Do(true, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthApi})
	if !capture.got {
		t.Fatal("JWT middleware cleared an already authenticated API-key request")
	}
}
