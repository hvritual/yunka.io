package execution

import (
	"context"
	"errors"
	"testing"
)

type fakeUnit struct{ commits, rollbacks, closes int }

func (unit *fakeUnit) Commit(context.Context) error   { unit.commits++; return nil }
func (unit *fakeUnit) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *fakeUnit) Close() error                   { unit.closes++; return nil }
func (unit *fakeUnit) TransactionHandle() any         { return unit }

type fakeFactory struct {
	unit   *fakeUnit
	begins int
	mode   TransactionMode
}

func (factory *fakeFactory) Begin(_ context.Context, mode TransactionMode) (UnitOfWork, error) {
	factory.begins++
	factory.mode = mode
	if factory.unit == nil {
		factory.unit = &fakeUnit{}
	}
	return factory.unit, nil
}

func TestRootOwnsOneTransactionAndChildrenJoinIt(t *testing.T) {
	factory := &fakeFactory{}
	ctx, root, err := BeginRoot(context.Background(), "device.transfer", TransactionLocal, []string{"site.validate"}, factory)
	if err != nil {
		t.Fatal(err)
	}
	child, err := JoinChild(ctx, "site.validate", TransactionReadOnly, nil)
	if err != nil {
		t.Fatal(err)
	}
	rootFrame, _ := Current(ctx)
	childFrame, _ := Current(child)
	if rootFrame.Depth != 0 || childFrame.Depth != 1 || childFrame.RootOperationID != "device.transfer" || childFrame.OperationID != "site.validate" {
		t.Fatalf("root=%#v child=%#v", rootFrame, childFrame)
	}
	rootUnit, _ := UnitOfWorkFrom(ctx)
	childUnit, _ := UnitOfWorkFrom(child)
	if rootUnit != childUnit || factory.begins != 1 {
		t.Fatalf("rootUnit=%p childUnit=%p begins=%d", rootUnit, childUnit, factory.begins)
	}
	if err := root.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if factory.unit.commits != 1 || factory.unit.rollbacks != 0 || factory.unit.closes != 1 {
		t.Fatalf("unit=%+v", factory.unit)
	}
}

func TestChildMustBeDeclaredAndCannotEscalateReadOnlyRoot(t *testing.T) {
	factory := &fakeFactory{}
	ctx, root, err := BeginRoot(context.Background(), "root", TransactionReadOnly, []string{"child"}, factory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Rollback(ctx)
	if _, err := JoinChild(ctx, "missing", TransactionNone, nil); !errors.Is(err, ErrChildUndeclared) {
		t.Fatalf("undeclared err=%v", err)
	}
	if _, err := JoinChild(ctx, "child", TransactionLocal, nil); !errors.Is(err, ErrTransactionConflict) {
		t.Fatalf("transaction err=%v", err)
	}
}
