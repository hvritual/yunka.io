# W07 — Diagnostics and `yunka inspect`

## Goal

Expose one safe, read-only diagnostic surface that can explain what a yunka process is running
without making configuration, credentials, request identity, or mutable runtime internals part of
a debug API.

## Architecture

```text
core.App.Diagnostics()
  ├─ application state
  ├─ HealthReport
  ├─ module inventory
  ├─ route inventory
  └─ RPC/event-bus inventory
            ↓
framework/diagnostics.Collector
  ├─ Contract source (W06 manifest)
  ├─ Resilience source (W03 PeekSnapshot)
  ├─ Selector source (W05 Snapshot)
  └─ future read-only sources
            ↓
  JSON diagnostics report
      ├─ local admin HTTP handler
      └─ yunka inspect runtime
```

## Scope

### Included

- `App.Diagnostics(ctx)` transport-neutral core snapshot.
- Stable, sorted route inventory under the radix-tree read lock.
- Source collector with deterministic source ordering and panic/error isolation.
- Contract summary from the W06 manifest.
- Resilience snapshots for explicitly supplied low-cardinality operation keys.
- Selector snapshots for explicitly supplied service names.
- Side-effect-free `RPCPolicy.PeekSnapshot` for diagnostics.
- Optional `net/http` diagnostics handler.
- `yunka inspect runtime` and `yunka inspect contract`.

### Not included

- Mutating config, routes, registry state, breaker state, or selector state.
- Exposing environment variables, config bodies, credentials, tokens, request bodies, tenant IDs,
  user IDs, or roles.
- Automatically starting a public diagnostics listener.
- Inferring distributed call edges from incomplete runtime observations.
- Replacing SLS/OpenTelemetry dashboards.

Full distributed dependency edges belong to the later Application Graph wave. W07 provides the
read-only inventories that graph construction can consume.

## HTTP security boundary

The handler is not mounted automatically. Applications explicitly mount it on a dedicated admin
listener. Default behavior is loopback-only. Remote access requires an explicit token at handler
construction and uses `Authorization: Bearer`.

```text
127.0.0.1 admin listener
      ↓
/_yunka/diagnostics
      ↓
Collector.Snapshot
```

Responses set `Cache-Control: no-store`. The endpoint accepts GET only.

## CLI

```bash
yunka inspect runtime \
  --url http://127.0.0.1:16667/_yunka/diagnostics

yunka inspect runtime --format json

yunka inspect contract \
  --manifest contracts/generated/manifest.json
```

If the diagnostics handler uses a token, pass `--token` or `YUNKA_DIAGNOSTICS_TOKEN`.

## Read-only policy snapshots

W03 `RPCPolicy.Snapshot(key)` historically materializes policy state for a key. W07 must never
change runtime state merely because it is observed, so diagnostics uses `PeekSnapshot(key)`.
Unknown keys are reported inactive and remain absent from breaker/limiter/shedder maps.

Selector diagnostics already use the W05 `Snapshot(service)` seam and never call `Pick`.

## Acceptance

- Core routes are emitted in stable order while protected by the radix read lock.
- Source failures and panics do not fail the full report.
- Diagnostics never creates resilience state.
- Remote HTTP exposure without a token is rejected at construction.
- Loopback handler requires the configured Bearer token when one is set.
- Core/collector/resilience diagnostic code passes unit, race, and vet harnesses.
- `yunka inspect` compiles against the W06 contract model and performs read-only HTTP GET/file
  inspection only.
