package authz

import (
	"context"
	"errors"
	"testing"
)

type testGuard struct {
	name  string
	fail  bool
	calls *[]string
	key   ScopeKey[string]
}

func (guard testGuard) Prepare(ctx context.Context, _ AuthorizedOperation, _ any) (context.Context, error) {
	*guard.calls = append(*guard.calls, guard.name)
	if guard.fail {
		return nil, errors.New("stop")
	}
	return guard.key.With(ctx, guard.name), nil
}

func TestOperationGuardChainRunsInOrderAndStopsOnFailure(t *testing.T) {
	key := MustScopeKey[string]("device")
	calls := []string{}
	chain := NewOperationGuardChain(testGuard{name: "a", calls: &calls, key: key}, testGuard{name: "b", calls: &calls, key: key})
	ctx, err := chain.Prepare(context.Background(), AuthorizedOperation{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := key.Require(ctx); err != nil || value != "b" {
		t.Fatalf("scope=%q err=%v", value, err)
	}
	if len(calls) != 2 || calls[0] != "a" || calls[1] != "b" {
		t.Fatalf("calls=%v", calls)
	}
	calls = nil
	chain = NewOperationGuardChain(testGuard{name: "a", calls: &calls, key: key, fail: true}, testGuard{name: "b", calls: &calls, key: key})
	if _, err := chain.Prepare(context.Background(), AuthorizedOperation{}, nil); err == nil {
		t.Fatal("expected guard error")
	}
	if len(calls) != 1 || calls[0] != "a" {
		t.Fatalf("calls after failure=%v", calls)
	}
}

func TestTypedScopeKeysKeepDomainsIsolated(t *testing.T) {
	type deviceScope struct{ Site string }
	type customerScope struct{ Customer string }
	device := MustScopeKey[deviceScope]("device")
	customer := MustScopeKey[customerScope]("customer")
	ctx := device.With(context.Background(), deviceScope{Site: "s1"})
	ctx = customer.With(ctx, customerScope{Customer: "c1"})
	if got, _ := device.Require(ctx); got.Site != "s1" {
		t.Fatalf("device=%v", got)
	}
	if got, _ := customer.Require(ctx); got.Customer != "c1" {
		t.Fatalf("customer=%v", got)
	}
}
