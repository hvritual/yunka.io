package resilience

import (
	"context"
	"strings"

	"yunka.io/framework/core/runtimecontext"
)

const defaultPolicyKey = "default"

// KeyFunc scopes stateful policies such as breakers and limiters.
type KeyFunc func(context.Context) string

// OperationKey isolates policy state by the most specific runtime operation.
func OperationKey(ctx context.Context) string {
	metadata, ok := runtimecontext.MetadataFrom(ctx)
	if !ok {
		return defaultPolicyKey
	}
	for _, candidate := range []string{
		metadata.Operation,
		metadata.Method,
		metadata.Route,
		metadata.Service,
	} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return defaultPolicyKey
}

func resolveKey(ctx context.Context, keyFn KeyFunc) string {
	if keyFn == nil {
		keyFn = OperationKey
	}
	key := strings.TrimSpace(keyFn(ctx))
	if key == "" {
		return defaultPolicyKey
	}
	return key
}
