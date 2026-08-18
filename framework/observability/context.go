package observability

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/resilience"
	"yunka.io/framework/core/runtimecontext"
)

func operationName(ctx context.Context) string {
	metadata, ok := runtimecontext.MetadataFrom(ctx)
	if ok && metadata.Operation != "" {
		return metadata.Operation
	}
	return "yunka.operation"
}

// traceAttributes may contain request-scoped dimensions because spans are not
// aggregated by attribute cardinality in the same way metrics are.
func traceAttributes(ctx context.Context, includeIdentity bool) []attribute.KeyValue {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	attrs := baseAttributes(metadata)

	keys := make([]string, 0, len(metadata.Attributes))
	for key := range metadata.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = appendStringAttr(attrs, key, metadata.Attributes[key])
	}

	if includeIdentity {
		if principal, ok := identity.FromContext(ctx); ok && principal.Authenticated {
			attrs = appendStringAttr(attrs, "yunka.identity.subject", principal.Subject)
			attrs = appendStringAttr(attrs, "yunka.tenant.id", principal.TenantID)
			attrs = appendStringAttr(attrs, "yunka.user.id", principal.UserID)
			attrs = appendStringAttr(attrs, "yunka.auth.method", principal.AuthMethod)
			if len(principal.Roles) > 0 {
				attrs = append(attrs, attribute.StringSlice("yunka.roles", append([]string(nil), principal.Roles...)))
			}
		}
	}
	if attempt := resilience.AttemptFrom(ctx); attempt > 0 {
		attrs = append(attrs, attribute.Int("yunka.retry.attempt", attempt))
	}
	if remaining, ok := resilience.RemainingBudget(ctx); ok {
		attrs = append(attrs, attribute.Float64("yunka.timeout.remaining_ms", float64(remaining)/float64(time.Millisecond)))
	}
	return attrs
}

// metricAttributes intentionally excludes identity, request IDs, raw metadata
// attributes and remaining budgets. SLS/Prometheus metric labels must stay low
// cardinality even when trace/log enrichment is enabled.
func metricAttributes(ctx context.Context) []attribute.KeyValue {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	attrs := baseAttributes(metadata)
	if metadata.Attributes != nil {
		attrs = appendStringAttr(attrs, "rpc.direction", metadata.Attributes["rpc.direction"])
	}
	return attrs
}

func baseAttributes(metadata runtimecontext.Metadata) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 8)
	attrs = appendStringAttr(attrs, "yunka.transport", metadata.Transport)
	attrs = appendStringAttr(attrs, "network.protocol.name", metadata.Protocol)
	attrs = appendStringAttr(attrs, "yunka.operation", metadata.Operation)
	attrs = appendStringAttr(attrs, "http.route", metadata.Route)
	attrs = appendStringAttr(attrs, "service.target.name", metadata.Service)
	attrs = appendStringAttr(attrs, "yunka.module", metadata.Module)
	attrs = appendStringAttr(attrs, "yunka.method", metadata.Method)
	return attrs
}

func appendStringAttr(attrs []attribute.KeyValue, key, value string) []attribute.KeyValue {
	if value == "" {
		return attrs
	}
	return append(attrs, attribute.String(key, value))
}

func slogContextFields(ctx context.Context, includeIdentity bool) []any {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	fields := make([]any, 0, 28)
	fields = appendField(fields, "transport", metadata.Transport)
	fields = appendField(fields, "protocol", metadata.Protocol)
	fields = appendField(fields, "operation", metadata.Operation)
	fields = appendField(fields, "route", metadata.Route)
	fields = appendField(fields, "target_service", metadata.Service)
	fields = appendField(fields, "module", metadata.Module)
	fields = appendField(fields, "method", metadata.Method)
	fields = appendField(fields, "request_id", metadata.RequestID)

	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		fields = append(fields, "trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String())
	} else if traceID := runtimecontext.TraceIDFrom(ctx); traceID != "" {
		fields = append(fields, "trace_id", traceID)
	}

	if includeIdentity {
		if principal, ok := identity.FromContext(ctx); ok && principal.Authenticated {
			fields = appendField(fields, "tenant_id", principal.TenantID)
			fields = appendField(fields, "user_id", principal.UserID)
			fields = appendField(fields, "auth_method", principal.AuthMethod)
		}
	}
	if attempt := resilience.AttemptFrom(ctx); attempt > 0 {
		fields = append(fields, "retry_attempt", attempt)
	}
	if remaining, ok := resilience.RemainingBudget(ctx); ok {
		fields = append(fields, "remaining_budget_ms", float64(remaining)/float64(time.Millisecond))
	}
	keys := make([]string, 0, len(metadata.Attributes))
	for key := range metadata.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("attr.%s", key), metadata.Attributes[key])
	}
	return fields
}

func appendField(fields []any, key, value string) []any {
	if value == "" {
		return fields
	}
	return append(fields, key, value)
}

func attributeFromAny(key string, value any) attribute.KeyValue {
	switch typed := value.(type) {
	case string:
		return attribute.String(key, typed)
	case bool:
		return attribute.Bool(key, typed)
	case int:
		return attribute.Int(key, typed)
	case int64:
		return attribute.Int64(key, typed)
	case float64:
		return attribute.Float64(key, typed)
	case []string:
		return attribute.StringSlice(key, append([]string(nil), typed...))
	case fmt.Stringer:
		return attribute.String(key, typed.String())
	default:
		return attribute.String(key, fmt.Sprint(value))
	}
}
