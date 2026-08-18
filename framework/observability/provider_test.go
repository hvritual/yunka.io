package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/resilience"
	"yunka.io/framework/core/runtimecontext"
)

func newTestProvider(t *testing.T, includeIdentity bool) (*Provider, *bytes.Buffer) {
	t.Helper()
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	var output bytes.Buffer
	provider, err := New(context.Background(), Config{
		ServiceName:     "test-service",
		ServiceVersion:  "1.0.0",
		Environment:     "test",
		LogOutput:       &output,
		IncludeIdentity: includeIdentity,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider, &output
}

func TestNewRequiresServiceName(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	if _, err := New(context.Background(), Config{}); err == nil {
		t.Fatal("expected service name validation error")
	}
}

func TestMiddlewareEmitsStructuredCompletionAndCanonicalTraceID(t *testing.T) {
	provider, output := newTestProvider(t, false)
	ctx := runtimecontext.WithMetadata(context.Background(), runtimecontext.Metadata{
		Transport: "rpc",
		Protocol:  "grpc",
		Operation: "/device.Device/Get",
		Attributes: map[string]string{
			"rpc.direction": "client",
		},
	})
	var seenTraceID string
	err := provider.Middleware()(func(child context.Context) error {
		seenTraceID = runtimecontext.TraceIDFrom(child)
		return nil
	})(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(seenTraceID) != 32 {
		t.Fatalf("trace id=%q", seenTraceID)
	}
	line := lastJSONLine(t, output.String())
	if line["msg"] != "operation.completed" || line["operation"] != "/device.Device/Get" {
		t.Fatalf("unexpected log: %#v", line)
	}
	if line["trace_id"] != seenTraceID {
		t.Fatalf("log trace=%v want=%s", line["trace_id"], seenTraceID)
	}
}

func TestIdentityLoggingIsExplicitOptIn(t *testing.T) {
	provider, output := newTestProvider(t, false)
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{
		TenantID:      "tenant-secret",
		UserID:        "user-secret",
		Authenticated: true,
	})
	provider.Info(ctx, "privacy.test")
	if strings.Contains(output.String(), "tenant-secret") || strings.Contains(output.String(), "user-secret") {
		t.Fatalf("identity leaked without opt-in: %s", output.String())
	}
}

func TestResilienceRejectionEmitsRuntimeEvent(t *testing.T) {
	provider, output := newTestProvider(t, false)
	ctx := runtimecontext.WithMetadata(context.Background(), runtimecontext.Metadata{Operation: "/device.Device/Get"})
	rejected := &resilience.Rejection{Policy: "rate-limit", Key: "/device.Device/Get", Err: resilience.ErrRateLimited}
	err := provider.Middleware()(func(context.Context) error { return rejected })(ctx)
	if !errors.Is(err, resilience.ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(output.String(), `"event.name":"resilience.rejected"`) ||
		!strings.Contains(output.String(), `"signal":"event"`) {
		t.Fatalf("runtime event missing: %s", output.String())
	}
}

func lastJSONLine(t *testing.T, raw string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatal("no log lines")
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &value); err != nil {
		t.Fatalf("decode log: %v: %s", err, lines[len(lines)-1])
	}
	return value
}

func TestMetricAttributesNeverContainIdentityOrRequestIDs(t *testing.T) {
	ctx := runtimecontext.WithMetadata(context.Background(), runtimecontext.Metadata{
		Operation: "/device.Device/Get",
		RequestID: "request-high-cardinality",
		Attributes: map[string]string{
			"rpc.direction": "client",
			"tenant_id":     "must-not-be-a-metric-label",
		},
	})
	ctx = identity.WithPrincipal(ctx, identity.Principal{
		TenantID:      "tenant-secret",
		UserID:        "user-secret",
		Authenticated: true,
	})

	attrs := metricAttributes(ctx)
	for _, attr := range attrs {
		key := string(attr.Key)
		if key == "tenant_id" || key == "yunka.tenant.id" || key == "yunka.user.id" || key == "request_id" {
			t.Fatalf("high-cardinality attribute leaked into metrics: %s", key)
		}
	}
}

func TestLegacyLoggerUsesRuntimeContext(t *testing.T) {
	provider, output := newTestProvider(t, false)
	ctx := runtimecontext.WithMetadata(context.Background(), runtimecontext.Metadata{
		Operation: "/legacy.Service/Get",
	})
	logger := provider.LegacyLogger(ctx)
	logger.Info("legacy", " log")

	line := lastJSONLine(t, output.String())
	if line["operation"] != "/legacy.Service/Get" || line["msg"] != "legacy log" {
		t.Fatalf("unexpected legacy log: %#v", line)
	}
}
