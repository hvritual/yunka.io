package operation

import (
	"context"
	"testing"

	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestExecutorEmitsToRequestScopedObserver(t *testing.T) {
	observer := &traceObserver{}
	ctx := WithObserver(context.Background(), observer)
	plan := operationplan.Plan{
		OperationID: "health.get",
		Security:    operationplan.Security{Public: true, PermissionMode: "all"},
	}
	value, err := NewExecutor(nil).Execute(ctx, plan, nil, func(context.Context) (any, error) { return "ok", nil })
	if err != nil {
		t.Fatal(err)
	}
	if value != "ok" {
		t.Fatalf("value=%v", value)
	}
	if len(observer.events) == 0 {
		t.Fatal("request-scoped observer received no Operation events")
	}
	last := observer.events[len(observer.events)-1]
	if last.OperationID != "health.get" || last.Phase != PhaseOutcome || last.Outcome != OutcomeSuccess {
		t.Fatalf("last event=%+v", last)
	}
}
