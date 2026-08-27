package authz

import (
	"context"
	"errors"
)

// OperationGuardChain composes domain guards in deterministic order. Authorization
// is still evaluated exactly once by OperationRuntime before this chain runs.
type OperationGuardChain []OperationGuard

func NewOperationGuardChain(guards ...OperationGuard) OperationGuard {
	chain := make(OperationGuardChain, 0, len(guards))
	for _, guard := range guards {
		if guard != nil {
			chain = append(chain, guard)
		}
	}
	if len(chain) == 0 {
		return nil
	}
	return chain
}

func (chain OperationGuardChain) Prepare(ctx context.Context, authorized AuthorizedOperation, input any) (context.Context, error) {
	secured := ctx
	for _, guard := range chain {
		var err error
		secured, err = guard.Prepare(secured, authorized, input)
		if err != nil {
			return nil, err
		}
		if secured == nil {
			return nil, errors.New("gateway authz: operation guard chain returned nil context")
		}
	}
	return secured, nil
}

// NewStaticGuardChainResolver adapts N guards per operation to the existing
// GuardResolver contract, preserving C8.5/C8.6 API compatibility.
func NewStaticGuardChainResolver(values map[OperationID][]OperationGuard) StaticGuardResolver {
	result := make(StaticGuardResolver, len(values))
	for operation, guards := range values {
		if chain := NewOperationGuardChain(guards...); chain != nil {
			result[operation] = chain
		}
	}
	return result
}
