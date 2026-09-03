package observability

import (
	"context"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestMiddlewareAutomaticallyEmitsOperationEvidenceWithTraceID(t *testing.T) {
	provider, output := newTestProvider(t, false)
	runtime := operation.NewExecutor(nil)
	plan := operationplan.Plan{
		OperationID: "health.get",
		Security:    operationplan.Security{Public: true, PermissionMode: "all"},
	}
	var traceID string
	err := provider.Middleware()(func(ctx context.Context) error {
		traceID = traceIDFromContext(ctx)
		_, executeErr := runtime.Execute(ctx, plan, nil, func(context.Context) (any, error) { return "ok", nil })
		return executeErr
	})(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if traceID == "" {
		t.Fatal("trace id was not established")
	}
	raw := output.String()
	if !strings.Contains(raw, `"event.name":"operation.phase"`) ||
		!strings.Contains(raw, `"event.operation_id":"health.get"`) ||
		!strings.Contains(raw, `"event.phase":"outcome"`) ||
		!strings.Contains(raw, `"event.outcome":"success"`) ||
		!strings.Contains(raw, `"trace_id":"`+traceID+`"`) {
		t.Fatalf("operation trace evidence missing: %s", raw)
	}
}

func traceIDFromContext(ctx context.Context) string {
	fields := slogContextFields(ctx, false)
	for index := 0; index+1 < len(fields); index += 2 {
		key, _ := fields[index].(string)
		if key == "trace_id" {
			value, _ := fields[index+1].(string)
			return value
		}
	}
	return ""
}
