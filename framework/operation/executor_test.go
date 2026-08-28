package operation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"yunka.io/framework/core/runtimecontext"
	"yunka.io/pkg/operationplan"
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
		Security: operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.read"}, PermissionMode: "all"},
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
		{OperationID: "device.get", Phase: PhasePlan, Outcome: OutcomeStarted},
		{OperationID: "device.get", Phase: PhaseMetadata, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Phase: PhaseSecurity, Outcome: OutcomeStarted},
		{OperationID: "device.get", Phase: PhaseSecurity, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Phase: PhaseApplication, Outcome: OutcomeStarted},
		{OperationID: "device.get", Phase: PhaseApplication, Outcome: OutcomeSuccess},
		{OperationID: "device.get", Phase: PhaseOutcome, Outcome: OutcomeSuccess},
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
