package middleware

import (
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/dispatcher/proxy"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type authCapture struct{ got bool }

func (*authCapture) Name() string                                           { return "capture" }
func (*authCapture) Use(proxy.MiddleWare) proxy.MiddleWare                  { return nil }
func (c *authCapture) Do(auth bool, _ *request.Context, _ *meta.RuntimeApi) { c.got = auth }

func TestAPIMiddlewareRequiresExactConfiguredKey(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name, configured, supplied string
		want                       bool
	}{
		{name: "exact", configured: key, supplied: key, want: true},
		{name: "wrong", configured: key, supplied: key + "x"},
		{name: "empty supplied", configured: key},
		{name: "short configured key", configured: "short", supplied: "short"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware := NewAPIMiddleware(test.configured)
			capture := &authCapture{}
			middleware.Use(capture)
			ctx := &fasthttp.RequestCtx{}
			runtime := request.NewHTTPRequestContext(ctx)
			ctx.Request.Header.Set(xCode, test.supplied)
			middleware.Do(false, runtime, &meta.RuntimeApi{Auth: meta.AuthBit_AuthApi})
			if capture.got != test.want {
				t.Fatalf("auth=%v want=%v", capture.got, test.want)
			}
		})
	}
}
