package requestscope

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

type fakeUnitOfWork struct {
	mu          sync.Mutex
	commits     int
	rollbacks   int
	closes      int
	commitErr   error
	rollbackErr error
	closeErr    error
}

func (unit *fakeUnitOfWork) Commit(context.Context) error {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.commits++
	return unit.commitErr
}

func (unit *fakeUnitOfWork) Rollback(context.Context) error {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.rollbacks++
	return unit.rollbackErr
}

func (unit *fakeUnitOfWork) Close() error {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.closes++
	return unit.closeErr
}

func (unit *fakeUnitOfWork) snapshot() (int, int, int) {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	return unit.commits, unit.rollbacks, unit.closes
}

type fakeUnitOfWorkFactory struct {
	mu      sync.Mutex
	units   []*fakeUnitOfWork
	newUnit func() *fakeUnitOfWork
}

func (factory *fakeUnitOfWorkFactory) Begin(context.Context) (UnitOfWork, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	unit := &fakeUnitOfWork{}
	if factory.newUnit != nil {
		unit = factory.newUnit()
	}
	factory.units = append(factory.units, unit)
	return unit, nil
}

func (factory *fakeUnitOfWorkFactory) latest() *fakeUnitOfWork {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.units[len(factory.units)-1]
}

type unitOfWorkFactoryFunc func(context.Context) (UnitOfWork, error)

func (factory unitOfWorkFactoryFunc) Begin(ctx context.Context) (UnitOfWork, error) {
	return factory(ctx)
}

type cleanupContextUnitOfWork struct {
	fakeUnitOfWork
	rollbackContextErr error
	rollbackValue      any
	valueKey           any
}

func (unit *cleanupContextUnitOfWork) Rollback(ctx context.Context) error {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.rollbacks++
	unit.rollbackContextErr = ctx.Err()
	unit.rollbackValue = ctx.Value(unit.valueKey)
	return unit.rollbackErr
}

func (unit *cleanupContextUnitOfWork) rollbackSnapshot() (error, any) {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	return unit.rollbackContextErr, unit.rollbackValue
}

type testRepositories struct {
	Unit UnitOfWork
	ID   int
}

func newTestFactory(t *testing.T, units *fakeUnitOfWorkFactory) *Factory[testRepositories] {
	t.Helper()
	factory, err := NewFactory(FactoryOptions[testRepositories]{
		UnitOfWork: units,
		Repositories: func(_ context.Context, unit UnitOfWork) (testRepositories, error) {
			return testRepositories{Unit: unit, ID: 7}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return factory
}

func TestExecuteCommitsOnSuccessAndCloses(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	if err := Execute(context.Background(), factory, func(scope *Scope[testRepositories]) error {
		if scope.Repositories().ID != 7 || scope.Repositories().Unit != scope.UnitOfWork() {
			t.Fatal("repositories were not bound to request unit of work")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 1 || rollbacks != 0 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestExecuteRollsBackWhenCommitFailsAndCloses(t *testing.T) {
	commitErr := errors.New("commit failed")
	units := &fakeUnitOfWorkFactory{newUnit: func() *fakeUnitOfWork {
		return &fakeUnitOfWork{commitErr: commitErr}
	}}
	factory := newTestFactory(t, units)
	err := Execute(context.Background(), factory, func(*Scope[testRepositories]) error { return nil })
	if !errors.Is(err, commitErr) {
		t.Fatalf("error=%v", err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 1 || rollbacks != 1 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestCommitFailureRemainsRollbackEligibleAndIdempotent(t *testing.T) {
	commitErr := errors.New("commit failed")
	unit := &fakeUnitOfWork{commitErr: commitErr}
	factory, err := NewFactory(FactoryOptions[testRepositories]{
		UnitOfWork: unitOfWorkFactoryFunc(func(context.Context) (UnitOfWork, error) { return unit, nil }),
		Repositories: func(_ context.Context, current UnitOfWork) (testRepositories, error) {
			return testRepositories{Unit: current}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := factory.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := scope.Commit(); !errors.Is(err, commitErr) {
		t.Fatalf("first commit error=%v", err)
	}
	if err := scope.Commit(); !errors.Is(err, commitErr) {
		t.Fatalf("second commit error=%v", err)
	}
	if err := scope.Rollback(); err != nil {
		t.Fatalf("rollback after failed commit=%v", err)
	}
	if err := scope.Rollback(); err != nil {
		t.Fatalf("second rollback=%v", err)
	}
	if err := scope.Commit(); !errors.Is(err, ErrScopeRolledBack) || !errors.Is(err, commitErr) {
		t.Fatalf("commit after rollback error=%v", err)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if commits, rollbacks, closes := unit.snapshot(); commits != 1 || rollbacks != 1 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestExecuteRollsBackOnErrorAndCloses(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	businessErr := errors.New("business failed")
	err := Execute(context.Background(), factory, func(*Scope[testRepositories]) error { return businessErr })
	if !errors.Is(err, businessErr) {
		t.Fatalf("error=%v", err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestExecuteRollsBackAndClosesOnPanic(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	defer func() {
		if recovered := recover(); recovered != "panic-value" {
			t.Fatalf("recovered=%v", recovered)
		}
		if commits, rollbacks, closes := units.latest().snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
			t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
		}
	}()
	_ = Execute(context.Background(), factory, func(*Scope[testRepositories]) error {
		panic("panic-value")
	})
}

func TestScopeFinalizationIsIdempotentAndMutuallyExclusive(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	scope, err := factory.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := scope.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Rollback(); !errors.Is(err, ErrScopeCommitted) {
		t.Fatalf("rollback after commit error=%v", err)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 1 || rollbacks != 0 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}

	scope, err = factory.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Commit(); !errors.Is(err, ErrScopeRolledBack) {
		t.Fatalf("commit after implicit rollback error=%v", err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
		t.Fatalf("implicit rollback commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestFactoryCleansUnitOfWorkWhenRepositoryConstructionFails(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	buildErr := errors.New("repository failed")
	factory, err := NewFactory(FactoryOptions[testRepositories]{
		UnitOfWork: units,
		Repositories: func(context.Context, UnitOfWork) (testRepositories, error) {
			return testRepositories{}, buildErr
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := factory.New(context.Background()); !errors.Is(err, buildErr) {
		t.Fatalf("error=%v", err)
	}
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}

func TestFactoryCleansUnitOfWorkWhenConstructionPanics(t *testing.T) {
	tests := []struct {
		name       string
		repository RepositoryFactory[testRepositories]
		logger     LoggerFactory
		wantPanic  string
	}{
		{
			name: "repository",
			repository: func(context.Context, UnitOfWork) (testRepositories, error) {
				panic("repository-panic")
			},
			wantPanic: "repository-panic",
		},
		{
			name: "logger",
			repository: func(_ context.Context, unit UnitOfWork) (testRepositories, error) {
				return testRepositories{Unit: unit}, nil
			},
			logger: func(context.Context, identity.Principal, bool, runtimecontext.Metadata, bool) logExt.Logger {
				panic("logger-panic")
			},
			wantPanic: "logger-panic",
		},
	}
	for _, current := range tests {
		t.Run(current.name, func(t *testing.T) {
			unit := &fakeUnitOfWork{}
			factory, err := NewFactory(FactoryOptions[testRepositories]{
				UnitOfWork: unitOfWorkFactoryFunc(func(context.Context) (UnitOfWork, error) {
					return unit, nil
				}),
				Repositories: current.repository,
				Logger:       current.logger,
			})
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if recovered := recover(); recovered != current.wantPanic {
						t.Fatalf("recovered=%v want=%v", recovered, current.wantPanic)
					}
				}()
				_, _ = factory.New(context.Background())
				t.Fatal("construction panic was not propagated")
			}()
			if commits, rollbacks, closes := unit.snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
				t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
			}
		})
	}
}

func TestRollbackCleanupDetachesCancellationAndPreservesValues(t *testing.T) {
	type contextKey string
	key := contextKey("request-key")
	unit := &cleanupContextUnitOfWork{valueKey: key}
	factory, err := NewFactory(FactoryOptions[testRepositories]{
		UnitOfWork: unitOfWorkFactoryFunc(func(context.Context) (UnitOfWork, error) {
			return unit, nil
		}),
		Repositories: func(_ context.Context, current UnitOfWork) (testRepositories, error) {
			return testRepositories{Unit: current}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "trusted-value"))
	scope, err := factory.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := scope.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	contextErr, value := unit.rollbackSnapshot()
	if contextErr != nil || value != "trusted-value" {
		t.Fatalf("rollback context err=%v value=%v", contextErr, value)
	}
}

func TestScopeSnapshotsTrustedIdentityAndMetadata(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	principal := identity.Principal{Subject: "caller", TenantID: "tenant-a", Roles: []string{"admin"}, Authenticated: true}
	metadata := runtimecontext.Metadata{Operation: "orders.create", RequestID: "request-a", Attributes: map[string]string{"source": "rpc"}}
	ctx := identity.WithPrincipal(context.Background(), principal)
	ctx = runtimecontext.WithMetadata(ctx, metadata)
	ctx = runtimecontext.WithTraceID(ctx, "trace-a")
	scope, err := factory.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	principal.Roles[0] = "mutated"
	metadata.Attributes["source"] = "mutated"
	gotPrincipal, ok := scope.Principal()
	if !ok || gotPrincipal.TenantID != "tenant-a" || !reflect.DeepEqual(gotPrincipal.Roles, []string{"admin"}) {
		t.Fatalf("principal=%+v ok=%v", gotPrincipal, ok)
	}
	gotMetadata, ok := scope.Metadata()
	if !ok || gotMetadata.RequestID != "request-a" || gotMetadata.Attributes["source"] != "rpc" {
		t.Fatalf("metadata=%+v ok=%v", gotMetadata, ok)
	}
	gotPrincipal.Roles[0] = "changed-again"
	gotMetadata.Attributes["source"] = "changed-again"
	againPrincipal, _ := scope.Principal()
	againMetadata, _ := scope.Metadata()
	if againPrincipal.Roles[0] != "admin" || againMetadata.Attributes["source"] != "rpc" || scope.TraceID() != "trace-a" {
		t.Fatalf("scope snapshots mutated: principal=%+v metadata=%+v trace=%q", againPrincipal, againMetadata, scope.TraceID())
	}
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentCloseExecutesCleanupOnce(t *testing.T) {
	units := &fakeUnitOfWorkFactory{}
	factory := newTestFactory(t, units)
	scope, err := factory.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for index := 0; index < 20; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_ = scope.Close()
		}()
	}
	wait.Wait()
	if commits, rollbacks, closes := units.latest().snapshot(); commits != 0 || rollbacks != 1 || closes != 1 {
		t.Fatalf("commit/rollback/close=%d/%d/%d", commits, rollbacks, closes)
	}
}
