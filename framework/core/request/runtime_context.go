package request

import (
	"context"
	"strings"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/define"
)

// ContextRuntime is the optional richer runtime contract introduced in W2.
// Runtime itself remains unchanged so existing custom Runtime implementations
// retain source compatibility.
type ContextRuntime interface {
	Runtime
	SetContext(context.Context)
	SetPrincipal(identity.Principal)
	Principal() (identity.Principal, bool)
	SetMetadata(runtimecontext.Metadata)
	Metadata() (runtimecontext.Metadata, bool)
	SetTraceID(string)
	TraceID() string
}

func (wrt *WorkRuntime) SetPrincipal(principal identity.Principal) {
	wrt.base = identity.WithPrincipal(wrt.baseContext(), principal)
	if wrt.ctx == nil || wrt.ctx.RequestCtx == nil {
		return
	}
	// User values are server-owned and keep legacy RequestCtx accessors usable
	// without making query parameters an authorization source.
	wrt.ctx.SetUserValue(define.OrgUUID, principal.TenantID)
	wrt.ctx.SetUserValue(define.UserUUID, principal.UserID)
	wrt.ctx.SetUserValue(define.RoleUUID, append([]string(nil), principal.Roles...))
}

func (wrt *WorkRuntime) Principal() (identity.Principal, bool) {
	return identity.FromContext(wrt.baseContext())
}

func SetPrincipal(rt Runtime, principal identity.Principal) {
	if contextual, ok := rt.(interface{ SetPrincipal(identity.Principal) }); ok {
		contextual.SetPrincipal(principal)
	}
}

func PrincipalFromRuntime(rt Runtime) (identity.Principal, bool) {
	if rt == nil {
		return identity.Principal{}, false
	}
	if contextual, ok := rt.(interface {
		Principal() (identity.Principal, bool)
	}); ok {
		return contextual.Principal()
	}
	return identity.FromContext(rt)
}

func (wrt *WorkRuntime) SetMetadata(metadata runtimecontext.Metadata) {
	wrt.base = runtimecontext.WithMetadata(wrt.baseContext(), metadata)
}

func (wrt *WorkRuntime) Metadata() (runtimecontext.Metadata, bool) {
	return runtimecontext.MetadataFrom(wrt.baseContext())
}

func SetMetadata(rt Runtime, metadata runtimecontext.Metadata) {
	if contextual, ok := rt.(interface{ SetMetadata(runtimecontext.Metadata) }); ok {
		contextual.SetMetadata(metadata)
	}
}

func MetadataFromRuntime(rt Runtime) (runtimecontext.Metadata, bool) {
	if rt == nil {
		return runtimecontext.Metadata{}, false
	}
	if contextual, ok := rt.(interface {
		Metadata() (runtimecontext.Metadata, bool)
	}); ok {
		return contextual.Metadata()
	}
	return runtimecontext.MetadataFrom(rt)
}

func (wrt *WorkRuntime) SetTraceID(traceID string) {
	traceID = strings.TrimSpace(traceID)
	wrt.base = runtimecontext.WithTraceID(wrt.baseContext(), traceID)
	if wrt.ctx != nil && wrt.ctx.RequestCtx != nil {
		wrt.ctx.SetUserValue(define.TraceId, traceID)
	}
}

func (wrt *WorkRuntime) TraceID() string {
	return runtimecontext.TraceIDFrom(wrt.baseContext())
}

func SetTraceID(rt Runtime, traceID string) {
	if contextual, ok := rt.(interface{ SetTraceID(string) }); ok {
		contextual.SetTraceID(traceID)
	}
}

func TraceIDFromRuntime(rt Runtime) string {
	if rt == nil {
		return ""
	}
	if contextual, ok := rt.(interface{ TraceID() string }); ok {
		return contextual.TraceID()
	}
	return runtimecontext.TraceIDFrom(rt)
}
