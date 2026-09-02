package operation

import (
	"context"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

type atomicSequenceStore struct {
	sequence *[]string
	claim    execution.IdempotencyIdentity
}

func (store *atomicSequenceStore) Claim(_ context.Context, claim execution.IdempotencyIdentity) error {
	store.claim = claim
	return nil
}

func (store *atomicSequenceStore) Mark(_ context.Context, _ execution.IdempotencyIdentity, state execution.IdempotencyState) error {
	*store.sequence = append(*store.sequence, "mark:"+string(state))
	return nil
}

func (store *atomicSequenceStore) MarkTx(_ context.Context, transaction any, claim execution.IdempotencyIdentity, state execution.IdempotencyState) error {
	if transaction != "tx-handle" {
		panic("unexpected transaction handle")
	}
	if claim.Attempt == "" || claim.Attempt != store.claim.Attempt {
		panic("idempotency attempt was not preserved")
	}
	*store.sequence = append(*store.sequence, "mark_tx:"+string(state))
	return nil
}

func (*atomicSequenceStore) Lookup(context.Context, execution.IdempotencyIdentity) (execution.IdempotencyState, bool, error) {
	return "", false, nil
}

type atomicSequenceUnit struct{ sequence *[]string }

func (unit *atomicSequenceUnit) Commit(context.Context) error {
	*unit.sequence = append(*unit.sequence, "commit")
	return nil
}
func (unit *atomicSequenceUnit) Rollback(context.Context) error {
	*unit.sequence = append(*unit.sequence, "rollback")
	return nil
}
func (unit *atomicSequenceUnit) Close() error {
	*unit.sequence = append(*unit.sequence, "close")
	return nil
}
func (*atomicSequenceUnit) TransactionHandle() any { return "tx-handle" }

type atomicSequenceFactory struct{ sequence *[]string }

func (factory atomicSequenceFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	*factory.sequence = append(*factory.sequence, "begin")
	return &atomicSequenceUnit{sequence: factory.sequence}, nil
}

func TestExecutorStagesDurableIdempotencySuccessBeforeLocalCommit(t *testing.T) {
	sequence := []string{}
	store := &atomicSequenceStore{sequence: &sequence}
	coordinator, err := execution.NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewExecutorWithOptions(nil, ExecutorOptions{Transactions: atomicSequenceFactory{sequence: &sequence}, Idempotency: coordinator})
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{TenantID: "tenant-a"})
	ctx = execution.WithIdempotencyKey(ctx, "request-1")
	plan := operationplan.Plan{
		OperationID: "device.create",
		Execution:   operationplan.Execution{Transaction: "local", Idempotency: "required"},
		Security:    operationplan.Security{Public: true, PermissionMode: "all"},
	}
	value, err := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { return "ok", nil })
	if err != nil {
		t.Fatal(err)
	}
	if value != "ok" {
		t.Fatalf("value=%v", value)
	}
	want := []string{"begin", "mark_tx:succeeded", "commit", "close"}
	if len(sequence) != len(want) {
		t.Fatalf("sequence=%v want=%v", sequence, want)
	}
	for index := range want {
		if sequence[index] != want[index] {
			t.Fatalf("sequence=%v want=%v", sequence, want)
		}
	}
}
