# Aliyun SLS observability deployment

Yunka keeps application instrumentation vendor-neutral while using Aliyun SLS as the default
production backend.

## Default production path

- Logs and runtime events: JSON stdout -> LoongCollector -> SLS Logstore/EventStore routing.
- Traces: OTLP -> OpenTelemetry Collector -> SLS Trace instance.
- Metrics: OTLP -> OpenTelemetry Collector -> SLS, or Prometheus scrape -> Remote Write -> SLS MetricStore.

Applications should normally send OTLP to a local/cluster Collector and should not keep SLS
AccessKeys in application configuration. The Collector deployment owns SLS credentials.

### Application environment

```bash
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
```

Set `OTEL_TRACES_EXPORTER=none` or `OTEL_METRICS_EXPORTER=none` when a signal is intentionally
disabled. The framework does not silently enable an SLS destination.

## Structured log contract

Every W4 framework log uses JSON and may include:

- `service.name`, `service.version`, `service.instance.id`
- `deployment.environment.name`, `cloud.provider`, `cloud.region`
- `trace_id`, `span_id`, `request_id`
- `transport`, `protocol`, `operation`, `route`, `target_service`, `module`, `method`
- `remaining_budget_ms`, `retry_attempt`

Runtime events additionally contain `signal=event` and `event.name`. LoongCollector routing can
send these records to a dedicated runtime-event Logstore/EventStore without changing application
code.

Identity fields are disabled by default. Enable `observability.Config.IncludeIdentity` only after
the deployment's privacy and retention rules have been reviewed.

## Collector to SLS

`otel-collector-sls.yaml.example` follows the Aliyun SLS Collector exporter layout. Replace the
placeholders through your deployment system; do not commit real AccessKeys.

## Prometheus Remote Write alternative

If a team wants SLS MetricStore/PromQL semantics, configure the OpenTelemetry metric exporter as
Prometheus and use the example Remote Write configuration. SLS requires HTTPS and BasicAuth where
the username/password are a least-privilege RAM AccessKey ID/Secret.

## Direct OTLP to SLS

Direct trace export is supported by SLS, but Collector forwarding is preferred. Direct mode needs
SLS-specific headers (`x-sls-otel-project`, `x-sls-otel-instance-id`, `x-sls-otel-ak-id`, and
`x-sls-otel-ak-secret`) or equivalent resource attributes. Keep this mode for constrained
environments where a Collector cannot be deployed.

## Cardinality and event naming

Metric attributes are intentionally limited to transport/protocol/operation/route/service/module/method,
`rpc.direction`, and bounded resilience/event dimensions. Tenant IDs, user IDs, request IDs, raw
policy keys, remaining budgets, and arbitrary runtime attributes are not metric labels.

Runtime event names are metrics dimensions and must come from a bounded operational vocabulary such
as `resilience.rejected`, `config.changed`, or `service.unhealthy`; do not put device IDs, request IDs,
or user-provided text in `Event.Name`.

Set `observability.Config.InstallGlobal=true` when third-party OpenTelemetry instrumentation should
use the same TracerProvider/MeterProvider. Yunka's own middleware works with the Provider directly and
does not require global provider installation.

The existing `pkg/logExt/ali_log.go` direct SLS writer is retained only as a compatibility path. New
runtime and application logging should prefer the structured stdout path so SLS connectivity cannot
block request execution.
