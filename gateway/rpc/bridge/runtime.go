package bridge

import (
	"context"
	"errors"
	"strings"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/request"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/logExt"
)

var ErrRuntimeFactoryUnavailable = errors.New("rpc bridge: runtime factory is unavailable")

// RuntimeFactory creates the request.Runtime used by an unchanged framework
// Service implementation for one typed gRPC call.
type RuntimeFactory interface {
	New(context.Context, string) (request.Runtime, error)
}

// RuntimeFactoryFunc adapts a function to RuntimeFactory.
type RuntimeFactoryFunc func(context.Context, string) (request.Runtime, error)

func (factory RuntimeFactoryFunc) New(ctx context.Context, serviceName string) (request.Runtime, error) {
	if factory == nil {
		return nil, ErrRuntimeFactoryUnavailable
	}
	return factory(ctx, serviceName)
}

// WorkRuntimeFactory preserves the existing framework Service runtime ABI while
// deriving all trusted caller state from the authenticated gRPC context.
type WorkRuntimeFactory struct {
	Logger logExt.Logger
}

func (factory WorkRuntimeFactory) New(ctx context.Context, serviceName string) (request.Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime := request.NewWorkRuntime()
	runtime.SetContext(ctx)
	runtime.SetSrvName(strings.TrimSpace(serviceName))
	if factory.Logger != nil {
		runtime.SetLogger(factory.Logger)
	}
	if principal, ok := identity.FromContext(ctx); ok {
		runtime.SetPrincipal(principal)
	}
	if metadata, ok := runtimecontext.MetadataFrom(ctx); ok {
		runtime.SetMetadata(metadata)
	}
	if traceID := runtimecontext.TraceIDFrom(ctx); traceID != "" {
		runtime.SetTraceID(traceID)
	}
	return runtime, nil
}
