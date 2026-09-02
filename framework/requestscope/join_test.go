package requestscope

import (
	"context"
	"testing"

	"github.com/hvritual/yunka.io/framework/execution"
)

type joinUnit struct{ commits, rollbacks, closes int }

func (unit *joinUnit) Commit(context.Context) error   { unit.commits++; return nil }
func (unit *joinUnit) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *joinUnit) Close() error                   { unit.closes++; return nil }

type joinFactory struct {
	unit   *joinUnit
	begins int
}

func (factory *joinFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	factory.begins++
	factory.unit = &joinUnit{}
	return factory.unit, nil
}

func TestJoinBuildsRepositoryViewWithoutOwningTransaction(t *testing.T) {
	transactions := &joinFactory{}
	ctx, root, err := execution.BeginRoot(context.Background(), "device.update", execution.TransactionLocal, nil, transactions)
	if err != nil {
		t.Fatal(err)
	}
	repositories := RepositoryFactory[string](func(_ context.Context, unit UnitOfWork) (string, error) {
		if unit != transactions.unit {
			t.Fatalf("repository received different unit")
		}
		return "joined", nil
	})
	value, err := JoinValue(ctx, repositories, func(view *View[string]) (string, error) { return view.Repositories(), nil })
	if err != nil || value != "joined" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	if transactions.unit.commits != 0 || transactions.unit.rollbacks != 0 || transactions.unit.closes != 0 {
		t.Fatalf("join finalized root transaction: %+v", transactions.unit)
	}
	if err := root.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if transactions.unit.commits != 1 || transactions.unit.closes != 1 {
		t.Fatalf("root did not finalize: %+v", transactions.unit)
	}
}
