package request

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
)

func TestContextCarriesTrustedRuntimeContext(t *testing.T) {
	raw := &fasthttp.RequestCtx{}
	ctx := NewContext(context.Background(), raw)
	principal := identity.Principal{Subject: "caller", TenantID: "org", UserID: "user", Roles: []string{"admin"}, Authenticated: true}
	metadata := runtimecontext.Metadata{Transport: "http", Service: "gateway", Operation: "GET /orders"}
	ctx.SetPrincipal(principal)
	ctx.SetMetadata(metadata)
	ctx.SetTraceID("trace-1")

	gotPrincipal, ok := ctx.Principal()
	if !ok || gotPrincipal.Subject != principal.Subject || gotPrincipal.TenantID != principal.TenantID {
		t.Fatalf("principal=%+v ok=%v", gotPrincipal, ok)
	}
	gotMetadata, ok := ctx.Metadata()
	if !ok || !reflect.DeepEqual(gotMetadata, metadata) {
		t.Fatalf("metadata=%+v ok=%v", gotMetadata, ok)
	}
	if got := ctx.TraceID(); got != "trace-1" {
		t.Fatalf("trace=%q", got)
	}
	if got := ctx.GetServiceName(); got != "gateway" {
		t.Fatalf("service=%q", got)
	}
}

func TestCloneRequestIsolatesMutableStateAndInheritsCancellation(t *testing.T) {
	parentBase, cancel := context.WithCancel(context.Background())
	raw := &fasthttp.RequestCtx{}
	raw.Request.Header.Set("X-Test", "parent")
	parent := NewContext(parentBase, raw)
	parent.SetPrincipal(identity.Principal{Subject: "caller", Authenticated: true})
	parent.Store().Set("mutable", "parent")

	child := parent.CloneRequest()
	if child == parent || child.GetRequestCtx() == parent.GetRequestCtx() {
		t.Fatal("clone reused request context")
	}
	if got := child.Value("X-Test"); got != "parent" {
		t.Fatalf("header=%v", got)
	}
	if _, ok := child.Principal(); !ok {
		t.Fatal("trusted principal was not inherited")
	}
	if got := child.Store().Get("mutable"); got != nil {
		t.Fatalf("mutable store crossed request boundary: %v", got)
	}
	child.Set("X-Test", "child")
	if got := parent.Value("X-Test"); got != "parent" {
		t.Fatalf("parent header mutated: %v", got)
	}
	cancel()
	<-child.Done()
	if child.Err() == nil {
		t.Fatal("cancellation did not propagate")
	}
}

func TestConcurrentContextsDoNotShareIdentityMetadataOrStore(t *testing.T) {
	const requests = 100
	failures := make(chan string, requests)
	var group sync.WaitGroup
	for index := 0; index < requests; index++ {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			raw := &fasthttp.RequestCtx{}
			current := NewContext(context.Background(), raw)
			value := fmt.Sprintf("request-%d", index)
			current.SetPrincipal(identity.Principal{Subject: value, Authenticated: true})
			current.SetMetadata(runtimecontext.Metadata{Operation: value})
			current.SetTraceID(value)
			current.Store().Set("value", value)
			principal, _ := current.Principal()
			metadata, _ := current.Metadata()
			if principal.Subject != value || metadata.Operation != value || current.TraceID() != value || current.Store().Get("value") != value {
				failures <- value
			}
		}()
	}
	group.Wait()
	close(failures)
	for failure := range failures {
		t.Fatalf("request state crossed isolation boundary: %s", failure)
	}
}
