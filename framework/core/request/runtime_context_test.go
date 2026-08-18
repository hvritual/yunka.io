package request

import (
	"context"
	"testing"

	"github.com/valyala/fasthttp"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
)

func TestWorkRuntimeCarriesPrincipalMetadataAndTrace(t *testing.T) {
	runtime := NewWorkRuntime()
	runtime.SetRequestCtx(&fasthttp.RequestCtx{})
	runtime.SetPrincipal(identity.Principal{
		Subject:       "user-1",
		TenantID:      "tenant-1",
		UserID:        "user-1",
		Roles:         []string{"admin"},
		AuthMethod:    identity.AuthMethodJWT,
		Authenticated: true,
	})
	runtime.SetMetadata(runtimecontext.Metadata{Transport: "http", Operation: "GET /devices"})
	runtime.SetTraceID("trace-1")

	principal, ok := identity.FromContext(runtime)
	if !ok || principal.TenantID != "tenant-1" || !principal.HasRole("admin") {
		t.Fatalf("principal=%#v ok=%v", principal, ok)
	}
	metadata, ok := runtimecontext.MetadataFrom(runtime)
	if !ok || metadata.Operation != "GET /devices" {
		t.Fatalf("metadata=%#v ok=%v", metadata, ok)
	}
	if got := runtimecontext.TraceIDFrom(runtime); got != "trace-1" {
		t.Fatalf("trace=%q", got)
	}
}

func TestSetRequestCtxDropsPriorRuntimeContext(t *testing.T) {
	runtime := NewWorkRuntime()
	runtime.SetContext(identity.WithPrincipal(context.Background(), identity.Principal{Authenticated: true}))
	runtime.SetMetadata(runtimecontext.Metadata{Transport: "http"})
	runtime.SetTraceID("trace-1")

	runtime.SetRequestCtx(&fasthttp.RequestCtx{})
	if _, ok := identity.FromContext(runtime); ok {
		t.Fatal("principal leaked into reused runtime")
	}
	if _, ok := runtimecontext.MetadataFrom(runtime); ok {
		t.Fatal("metadata leaked into reused runtime")
	}
	if got := runtimecontext.TraceIDFrom(runtime); got != "" {
		t.Fatalf("trace leaked: %q", got)
	}
}
