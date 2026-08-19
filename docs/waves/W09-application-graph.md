# W09 — Application Graph

## Goal

Compile yunka's declared and observed runtime facts into one deterministic graph that can answer:

- what is this module/service/operation/route/node;
- what does it depend on;
- what depends on it;
- where the relationship came from;
- whether the relationship was declared, observed, or inferred.

## Evidence model

Every node and edge carries evidence. Evidence is never implicit.

| Type | Meaning | Default confidence |
| --- | --- | --- |
| `declared` | protobuf contract, explicit event catalog, explicit configuration | high |
| `observed` | runtime diagnostics, selector state, resilience state | high |
| `inferred` | relationship derived from multiple facts | medium/low and must be labeled |

The absence of evidence is represented by the absence of an edge. W09 does not fabricate module-to-service or distributed call edges from naming conventions.

## Packages

- `pkg/applicationgraph`: canonical schema, deterministic builder, contract/runtime adapters, query and impact traversal.
- `framework/applicationgraph`: runtime source compiler for `core.App`, W03 resilience and W05 selector snapshots.
- `app/cmd/graph`: `yunka graph build|inspect|find|impact`.

## Graph IDs

IDs are stable strings of the form `<kind>:<canonical-name>`, for example:

```text
service:ApiService
operation:ApiService.FindAllRuntimeAPI
message:FindAllRuntimeAPIResponse
http_route:GET /v1/apis
module:device
selector_service:device-service
instance:device-service/v1/node-a
resilience_policy:ApiService.FindAllRuntimeAPI
```

## Contract edges

```text
service --contains--> operation
operation --accepts--> request message
operation --returns--> response message
http route --routes_to--> operation
```

Only explicit W06 HTTP bindings create HTTP graph nodes.

## Runtime edges

```text
application --contains--> module
application --exposes--> runtime route
selector service --selects--> runtime instance
operation --governed_by--> resilience policy
```

Selector and resilience state is consumed through the existing read-only snapshot APIs. Graph compilation must not create breaker/limiter/selector state.

## CLI

```bash
yunka graph build \
  --manifest contracts/generated/manifest.json \
  --diagnostics runtime-diagnostics.json \
  --out .yunka/application-graph.json

yunka graph inspect
yunka graph find --query ApiService
yunka graph impact --id operation:ApiService.FindAllRuntimeAPI --depth 3
```

A W07 diagnostics export is optional. Without it the graph is still a valid declared contract graph.

## Deferred

W09 intentionally does not infer cross-service call edges from names or static grep. Trace-derived distributed edges and confidence aggregation belong to the later context/graph enrichment work.
