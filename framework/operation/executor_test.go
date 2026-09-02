package operation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/runtimecontext"
	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

type securityStub struct {
	calls int
	seen  string
}

func (stub *securityStub) Prepare(ctx context.Context, plan operationplan.Plan, _ any) (context.Context, error) {
	stub.calls++
	stub.seen = plan.OperationID
	return context.WithValue(ctx, securityContextKey{}, "secured"), nil
}

type securityContextKey struct{}

type traceObserver struct{ events []Event }

func (observer *traceObserver) Observe(_ context.Context, event Event) {
	observer.events = append(observer.events, event)
}

func TestExecutorRunsOneSecurityDecisionAndApplication(t *testing.T) {
	security := &securityStub{}
	observer := &traceObserver{}
	runtime := NewExecutor(security, observer)
	plan := operationplan.Plan{
		OperationID: "device.get",
		Security:    operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.read"}, PermissionMode: "all"},
	}
	called := 0
	value, err := runtime.Execute(context.Background(), plan, "request", func(ctx context.Context) (any, error) {
		called++
		if got := ctx.Value(securityContextKey{}); got != "secured" {
			t.Fatalf("security context=%v", got)
		}
		metadata, ok := runtimecontext.MetadataFrom(ctx)
		if !ok || metadata.Operation != "device.get" {
			t.Fatalf("metadata=%+v ok=%v", metadata, ok)
		}
		return "response", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "response" || called != 1 || security.calls != 1 || security.seen != "device.get" {
		t.Fatalf("value=%v called=%d securityCalls=%d seen=%q", value, called, security.calls, security.seen)
	}
	want := []Event{
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhasePlan, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseMetadata, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseSecurity, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseSecurity, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyBegin, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyBegin, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseExecutionScope, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseExecutionScope, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseApplication, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseApplication, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseTransactionFinalize, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseTransactionFinalize, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyFinalize, Outcome: OutcomeStarted},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseIdempotencyFinalize, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Kind: InvocationRoot, Phase: PhaseOutcome, Outcome: OutcomeSuccess},
	}
	if !reflect.DeepEqual(observer.events, want) {
		t.Fatalf("events=%#v want=%#v", observer.events, want)
	}
}

func TestExecutorFailsClosedWithoutSecurity(t *testing.T) {
	called := false
	plan := operationplan.Plan{OperationID: "device.update", Security: operationplan.Security{Permissions: []string{"device.write"}, PermissionMode: "all"}}
	_, err := NewExecutor(nil).Execute(context.Background(), plan, nil, func(context.Context) (any, error) {
		called = true
		return nil, nil
	})
	if !errors.Is(err, ErrSecurityUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("protected application was invoked without security")
	}
}

func TestExecutorAllowsPublicPlanWithoutSecurity(t *testing.T) {
	plan := operationplan.Plan{OperationID: "health.get", Security: operationplan.Security{Public: true, PermissionMode: "all"}}
	value, err := NewExecutor(nil).Execute(context.Background(), plan, nil, func(context.Context) (any, error) { return "ok", nil })
	if err != nil || value != "ok" {
		t.Fatalf("value=%v err=%v", value, err)
	}
}

type transactionFactoryStub struct {
	unit   *transactionUnitStub
	begins int
}
type transactionUnitStub struct{ commits, rollbacks, closes int }

func (factory *transactionFactoryStub) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	factory.begins++
	factory.unit = &transactionUnitStub{}
	return factory.unit, nil
}
func (unit *transactionUnitStub) Commit(context.Context) error   { unit.commits++; return nil }
func (unit *transactionUnitStub) Rollback(context.Context) error { unit.rollbacks++; return nil }
func (unit *transactionUnitStub) Close() error                   { unit.closes++; return nil }

func TestExecutorOwnsTransactionAndChildJoinsWithoutSecondSecurityDecision(t *testing.T) {
	security := &securityStub{}
	transactions := &transactionFactoryStub{}
	runtime := NewExecutorWithOptions(security, ExecutorOptions{Transactions: transactions})
	parent := operationplan.Plan{
		OperationID: "device.transfer",
		Security:    operationplan.Security{Permissions: []string{"device.update", "site.read"}, PermissionMode: "all"},
		Execution:   operationplan.Execution{Transaction: "local", Idempotency: "none"},
		Composition: operationplan.Composition{RequiresOperations: []string{"site.validate"}},
	}
	child := operationplan.Plan{OperationID: "site.validate", Execution: operationplan.Execution{Transaction: "read_only", Idempotency: "none"}}
	_, err := runtime.Execute(context.Background(), parent, nil, func(ctx context.Context) (any, error) {
		return ExecuteChild(ctx, runtime, child, nil, func(childCtx context.Context) (any, error) {
			frame, ok := execution.Current(childCtx)
			if !ok || frame.Depth != 1 || frame.RootOperationID != "device.transfer" || frame.OperationID != "site.validate" {
				t.Fatalf("child frame=%#v ok=%v", frame, ok)
			}
			return "ok", nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if security.calls != 1 || transactions.begins != 1 || transactions.unit.commits != 1 || transactions.unit.closes != 1 {
		t.Fatalf("security=%d begins=%d unit=%+v", security.calls, transactions.begins, transactions.unit)
	}
}

func TestExecutorRequiredIdempotencySuppressesDuplicateExecution(t *testing.T) {
	store := execution.NewMemoryIdempotencyStore()
	coordinator, err := execution.NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewExecutorWithOptions(nil, ExecutorOptions{Idempotency: coordinator})
	plan := operationplan.Plan{OperationID: "public.create", Security: operationplan.Security{Public: true, PermissionMode: "all"}, Execution: operationplan.Execution{Transaction: "none", Idempotency: "required"}}
	ctx := execution.WithIdempotencyKey(context.Background(), "request-1")
	called := 0
	if _, err := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { called++; return "ok", nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { called++; return "duplicate", nil }); !errors.Is(err, execution.ErrIdempotencyCompleted) {
		t.Fatalf("duplicate err=%v", err)
	}
	if called != 1 {
		t.Fatalf("application calls=%d", called)
	}
}
