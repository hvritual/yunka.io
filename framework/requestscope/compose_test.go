package requestscope

import (
	"context"
	"testing"
)

type composeUnit struct{}

func (*composeUnit) Commit(context.Context) error   { return nil }
func (*composeUnit) Rollback(context.Context) error { return nil }
func (*composeUnit) Close() error                   { return nil }

func TestComposeRepositoryFactoriesShareOneUnitOfWork(t *testing.T) {
	unit := &composeUnit{}
	seen := []UnitOfWork{}
	first := func(context.Context, UnitOfWork) (string, error) { seen = append(seen, unit); return "a", nil }
	second := func(context.Context, UnitOfWork) (int, error) { seen = append(seen, unit); return 2, nil }
	third := func(context.Context, UnitOfWork) (bool, error) { seen = append(seen, unit); return true, nil }
	value, err := Compose3(first, second, third)(context.Background(), unit)
	if err != nil {
		t.Fatal(err)
	}
	if value.First != "a" || value.Second != 2 || !value.Third {
		t.Fatalf("value=%+v", value)
	}
	for _, got := range seen {
		if got != unit {
			t.Fatal("factory did not share one unit of work")
		}
	}
}
