package requestscope

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

type fakeGORMUnit struct{ database *gorm.DB }

func (unit *fakeGORMUnit) Commit(context.Context) error   { return nil }
func (unit *fakeGORMUnit) Rollback(context.Context) error { return nil }
func (unit *fakeGORMUnit) Close() error                   { return nil }
func (unit *fakeGORMUnit) GORM() *gorm.DB                 { return unit.database }

func TestGORMRepositoriesUseRequestTransaction(t *testing.T) {
	database := &gorm.DB{}
	builderCalled := false
	factory := GORMRepositories(func(_ context.Context, current *gorm.DB) (testRepositories, error) {
		builderCalled = true
		if current != database {
			t.Fatalf("database=%p want=%p", current, database)
		}
		return testRepositories{ID: 9}, nil
	})
	repositories, err := factory(context.Background(), &fakeGORMUnit{database: database})
	if err != nil {
		t.Fatal(err)
	}
	if !builderCalled || repositories.ID != 9 {
		t.Fatalf("called=%v repositories=%+v", builderCalled, repositories)
	}
}

func TestGORMFromRejectsUnrelatedUnitOfWork(t *testing.T) {
	if _, err := GORMFrom(&fakeUnitOfWork{}); err == nil {
		t.Fatal("non-GORM unit of work accepted")
	}
}
