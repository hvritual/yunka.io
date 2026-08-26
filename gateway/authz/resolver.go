package authz

import (
	"context"
	"strings"
)

type PolicyResolver interface {
	ResolvePolicy(context.Context, string) (Policy, bool)
}

type StaticResolver map[string]Policy

func NewStaticResolver(values map[string]Policy) StaticResolver {
	result := make(StaticResolver, len(values))
	for key, policy := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = Normalize(policy)
	}
	return result
}

func (resolver StaticResolver) ResolvePolicy(_ context.Context, key string) (Policy, bool) {
	policy, ok := resolver[strings.TrimSpace(key)]
	if !ok {
		return Policy{}, false
	}
	return Normalize(policy), true
}
