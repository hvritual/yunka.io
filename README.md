# yunka.io

`yunka.io` is a Go framework repository containing shared packages, the core framework,
the gateway, command-line tooling, and the RPC generator.

## Requirements

- Go 1.25.13
- `protoc` 3.21.12 (protobuf release 21.12) for contract generation and verification
- GCC when running race tests or the SQLite-backed gateway tests

Exact tool versions are locked in `tools/toolchain.env`; local and CI verification must use that lock rather than accepting arbitrary newer compilers.

## Repository layout

- `pkg/`: shared leaf packages
- `framework/`: application, module, request, ingress, and infrastructure abstractions
- `gateway/`: HTTP gateway, authorization middleware, routing, and RPC adapters
- `app/`: the `yunka` command-line tool
- `contracts/proto/`: the single canonical RPC protobuf source tree
- `compat/go-kit-kit-log/`: repository-owned SLS logging compatibility module

The modules are joined by the root `go.work`. `app` intentionally uses the distinct module
path `yunka.io/app`; `framework` remains `github.com/hvritual/yunka.io/framework`.

## Documentation authority

This README is maintained as **current developer/product documentation**. For repository-wide status and documentation ownership:

- [`docs/STATUS.md`](docs/STATUS.md) is the current framework/wave/release status authority.
- [`docs/DOCUMENTATION_GOVERNANCE.md`](docs/DOCUMENTATION_GOVERNANCE.md) defines document classes and truth ownership.
- `PROJECT_MEMORY.md` records durable current governance and architecture invariants.
- `docs/waves/**` contains historical roadmaps, implementation records, and exact qualification evidence; an old roadmap status field is not current status unless its classification explicitly says so.

## Verification

```bash
make toolchain-check
make dependency-check
make architecture-check
make rpc-contract-check
make test
make race
make vet
make vuln
make build
```

`make toolchain-check` requires the exact Go/protoc versions in `tools/toolchain.env`. Normal CI is read-only: it runs `make tidy` and contract regeneration only to prove zero drift, and never commits or pushes repairs.

Run `make tidy` only when dependency metadata is intentionally being updated. The C1
production gate additionally requires a real MySQL 8 instance:

```bash
export YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/yunka_test?parseTime=true&charset=utf8mb4'
make verify-production
```

## Toolchain determinism

C2 makes `tools/toolchain.env` the canonical tool lock for Go, protoc, and govulncheck. CI downloads `protoc-21.12-linux-x86_64.zip`, verifies its locked SHA-256 before extraction, and pins third-party GitHub Actions to immutable commit SHAs. Dependency or generated-contract drift is a hard failure; CI has only `contents: read` and cannot auto-commit changes. See `docs/waves/C2-toolchain-determinism.md`.

## Dependency convergence

C4 removes the historical external `github.com/go-kit/kit v0.10.0` dependency graph and the workspace-wide genproto version replace. The pinned Aliyun SLS SDK still imports `github.com/go-kit/kit/log` and `github.com/go-kit/kit/log/level`, so the repository owns a narrow compatibility workspace module at `compat/go-kit-kit-log`. That module exposes only the logging surface required by the SDK and delegates to `github.com/go-kit/log v0.2.1`; it does not carry the monolithic kit module's historical etcd, gRPC, or genproto graph.

Because `go mod tidy` operates on one module at a time, each product module that reaches the SLS SDK carries the same version-scoped local replacement for `github.com/go-kit/kit v0.10.0`, while root `go.work` keeps the compatibility module as a workspace main module. The target is always the reviewed repository directory; arbitrary external replacements remain forbidden.

`tools/dependency-policy.json` is enforced by `yunka dependency check` / `make dependency-check`. The gate validates the repository-local compatibility module, rejects unsplit etcd, grpc-gateway v1, monolithic genproto, any external replacement, a reintroduced genproto replace, and new legacy protobuf imports outside approved compatibility islands. Existing generated gateway/SMS protobuf files remain reviewed compatibility artifacts where still required; C4 did not rewrite them.

The workspace contains four product modules plus the single logging compatibility module. C6 removed the isolated legacy RPC generator module.

## Static module catalog and typed bootstrap

C7.1 preserves one-line module enablement through blank imports while replacing hidden runtime composition. An autoload package may only register an immutable `modulecatalog.Descriptor`; it cannot read configuration, perform I/O, create DB/RPC clients, start goroutines, construct services, or mutate an App. Modules declare typed config and platform requirements, and ordinary generated Go wiring receives shared capabilities from the framework.

`modulecatalog.Catalog` rejects duplicate modules, missing dependencies, cycles, and late registration, then resolves a deterministic topological order independent of import order. `core.NewApp(AppOptions)` and `framework/kernel.New` aggregate requirements before any module build, prepare named shared capabilities once, restrict each module to its declared config/logger/database/event/RPC view, and build isolated App instances from a supplied or process-default descriptor catalog. The default catalog stores descriptors only and is not a service locator.

C7.3 completes the migration: the reflective module container, package-global App/configuration/prepare/initiator holders, service/repository lifecycle pools, Runtime mutation, `pkg/di`, and local pooled ingress are deleted. The typed catalog is the only composition path. See `docs/waves/C7.1-static-module-catalog.md` and `docs/waves/C7.3-legacy-runtime-removal.md`.

## App-owned platform capabilities and request scope

C7.2 makes the C7.1 typed catalog operational without exposing infrastructure acquisition to modules. `framework/platform.Provider` opens only the sealed plan's named DB/RPC requirements, opens each name once, gives each module a restricted capability view and module-scoped logger, and participates in the existing App Start/Health/Shutdown lifecycle. `kernel.Options.Platform` is the single ergonomic entry; it cannot be mixed with a second context factory or direct capability set. MySQL pool configuration and gRPC transport credentials remain process-level platform inputs and are never visible to module code.

`framework/requestscope` still provides a fresh typed Scope/repository view per operation and snapshots trusted Principal/metadata/trace state. Under the active C9.7+ execution model, however, the root `framework/execution.ExecutionScope` owns the Operation UnitOfWork and commit/rollback lifecycle. Requestscope join views bind typed repositories to that active UoW; they do not independently open or finalize a second root transaction. The underlying DB connection pool remains App-owned, and request scopes, repositories, transactions, identities, and contexts are never managed by `sync.Pool`. See `docs/waves/C7.2-platform-request-scope.md` for the historical introduction and `docs/waves/C9.7-execution-semantics-closure.md` for the current ownership model.

## Single typed runtime

C7.3 removes the compatibility runtime. `core.App` owns typed catalog instances, the capability factory, logger, event bus, route inventory, and lifecycle state; there is no package-level default App, global configuration store, reflection container, generic Service lookup, or mutable Runtime attached to singleton services. Start/Health/Shutdown operate only on typed modules and App-owned capabilities.

Gateway HTTP execution creates one concrete `request.Context` per request. The Context carries cancellation, trusted Principal, runtime metadata, trace ID, logger, and transport state without entering a `sync.Pool`. Composed HTTP calls create a fresh Context, copy request transport data, and inherit only the parent `context.Context`; response state and mutable request storage are not shared. Root Operation transaction/UoW ownership belongs to the active `ExecutionScope`; `framework/requestscope` supplies typed repository views joined to that execution state.

The old local `GetModule → GetService → SetRuntime → PutService` path, `ModuleGatewayProvider`, runtime-mutating ORM hooks, `pkg/di`, and controller generator are deleted. `make architecture-check` permanently blocks their return while allowing reflection used for ordinary data binding and pools used for unrelated transport buffers. See `docs/waves/C7.3-legacy-runtime-removal.md`.

## Security defaults

- HTTP clients verify TLS certificates and enforce timeouts.
- API, JWT, and role authorization fail closed.
- JWTs are accepted from the `Authorization: Bearer` header, not query parameters.
- Generated RPC files are immutable compatibility artifacts. C3 validates committed gateway
  descriptors against canonical protobuf source; do not hand-edit generated files.

Gateway JWT configuration must provide a secret of at least 32 bytes plus explicit issuer
and audience values:

```toml
jwt = "replace-with-a-random-secret-of-at-least-32-bytes"
jwtIssuer = "yunka"
jwtAudience = "yunka-gateway"
```

The API metadata CLI requires a separate 32-byte key through `--api-key` or
`YUNKA_API_KEY`. See `.env.example` for the supported environment variable name; never
commit a real key.

## Application lifecycle and health

The typed App owns process-scoped capabilities and module instances. A capability or module may
implement the following optional interfaces from `github.com/hvritual/yunka.io/framework/core`:

- `Startable` for explicit startup work.
- `Shutdowner` for context-aware graceful shutdown.
- `HealthChecker` for dependency health checks.

Modules start in deterministic catalog dependency order and shut down in reverse order. The
Platform Provider starts named resources in stable key order and closes them in reverse order.
Request-owned Context, Scope, transaction, and repository objects are not part of process
lifecycle management and are never retained by singleton services.

`App.Health(ctx)` returns a structured `HealthReport` with application state, liveness,
readiness, capability health, and typed module checks. Gateway health endpoints and diagnostics
adapt this report without defining separate lifecycle semantics.

## Runtime context, identity, and middleware

C7.3 makes `context.Context` the sole execution boundary:

- `core/identity.Principal` is the canonical trusted caller identity. `Authenticated` is set only after server-side credential verification.
- `core/runtimecontext.Metadata` carries transport, protocol, operation, route, service, module, method, request, and trace metadata.
- `core/middleware.Chain` wraps `context.Context`, so the same middleware model is used by HTTP, RPC, events, and jobs.
- Gateway creates a fresh concrete `request.Context` per request; there is no `request.Runtime`, `WorkRuntime`, `BaseRuntime`, or Runtime mutation on singleton services.
- Gateway JWT and API-key authentication establish a trusted Principal. Role authorization reads only that Principal and never trusts query-supplied `oid`, `uid`, or `rid` values.
- Composite calls inherit identity, deadline, cancellation, and trace values through the parent context while using an isolated request/response/store object.
- RPC middleware is attached through `core/middleware.WrapRPCClient` or non-generated gRPC interceptors; generated RPC files remain untouched.

Remote RPC identity is never trusted merely because it arrived in metadata. C1 adds the
non-generated `CredentialVerifier` boundary plus fail-closed unary and streaming server
interceptors. `NewStaticServiceTokenVerifier` is the bootstrap adapter for deployments that do
not yet have workload identity: it uses the dedicated
`x-yunka-service-authorization` metadata key, supports overlapping tokens during rotation, and
requires a transport with privacy and integrity by default. Outbound callers use
`StaticServiceTokenCredentials` as standard gRPC per-RPC credentials. Caller-supplied tenant,
user, and role metadata is never accepted as identity proof. See
`docs/waves/C1-production-hardening.md`.

## Resilience policies

W3 adds transport-neutral resilience middleware under `framework/core/resilience`:

- `TimeoutBudget` derives child deadlines from the caller budget and can reserve time for
  caller cleanup.
- `Retry` is fail-safe: it retries only when the operation is explicitly idempotent and the
  error is explicitly retryable.
- `CircuitBreakerGroup`, `RateLimiterGroup`, and `LoadShedderGroup` isolate state by runtime
  operation instead of sharing one global failure domain.
- Load shedding combines a concurrency ceiling, minimum remaining deadline budget, and an
  AIMD-style adaptive limit driven by observed latency/overload signals.
- `RPCPolicy` composes outbound RPC governance in the order timeout budget → retry → rate
  limit → load shed → circuit breaker → transport.

All stateful policies expose snapshots for later diagnostics and SLS/OpenTelemetry metrics.
No resilience policy is silently enabled for existing services; callers opt in through
middleware or `RPCPolicy.Wrap`.

Policy keys must remain low-cardinality (for example service/method or normalized route). Never
key breaker/limiter state by request ID, user ID, raw URL, or other unbounded values. Gateway
adapters translate policy rejections to HTTP semantics: 429 for rate limiting, 503 for
circuit/load shedding, and 504 for timeout-budget/deadline exhaustion.

## Aliyun SLS observability

W4 adds a unified observability provider under `framework/observability` while keeping SLS behind
standard telemetry protocols:

- traces and metrics use OpenTelemetry Go and are configured through standard `OTEL_*` variables;
- logs use `log/slog` JSON output so LoongCollector can batch, route, retry, and send without
  coupling request latency to the SLS network;
- runtime events are emitted as structured `signal=event` logs, OpenTelemetry span events, and a
  runtime-event counter;
- gateway and gRPC adapters propagate W3C trace context without modifying generated RPC files;
- W3 resilience rejections are recorded as metrics and runtime events with low-cardinality policy
  and operation attributes.

The recommended production path is application -> OTLP Collector for traces/metrics and JSON
stdout -> LoongCollector for logs/events. SLS AccessKeys remain in Collector/Prometheus deployment
configuration rather than application configuration. See `deploy/observability/` for templates.

Identity fields are not added to telemetry by default. `observability.Config.IncludeIdentity` is
an explicit opt-in for deployments whose privacy and retention rules allow those identifiers.

## Contract pipeline

Protobuf is the canonical contract source of truth. The current repository uses deterministic standard protobuf/gRPC generation and read-only drift/compatibility checks; the historical legacy RPC generator has already been removed from the active workspace.

```bash
yunka contract lint
yunka contract generate
yunka contract inspect
yunka contract check
```

`contracts/sources.json` is the canonical service-contract inventory over sources beneath `contracts/proto`. Historical compatibility schemas may remain under explicitly reviewed compatibility roots such as `contracts/proto/legacy`, and committed compatibility descriptors such as `gateway/rpc/meta/*.pb.go` or `pkg/rpcmeta/legacy/**` remain generated artifacts rather than writable contract sources.

`make rpc-contract-check` validates committed contract/descriptor consistency against the canonical source inventory. Generated RPC files are immutable outputs; there is no current `app/cmd/rpc` destructive generator path to treat as contract truth.

Only explicit HTTP bindings are emitted into OpenAPI `paths`. Standard `google.api.http` is
preferred; `@yunka.http` comments remain a migration bridge. C3 does not infer routes from dynamic
gateway metadata, so unbound RPC methods remain under `x-yunka-rpc-methods` rather than receiving
invented paths. Pull-request CI continues to reject breaking service, method, field, enum,
streaming, and HTTP-binding changes.

See `contracts/README.md`, `docs/waves/W06-contract-pipeline.md`, and
`docs/waves/C3-contract-convergence.md` for the historical contract-pipeline introduction and convergence boundary; use this README and `PROJECT_MEMORY.md` for the current runtime ownership model.

## C9 Operation contract and execution semantics

C9 compiles protobuf Operation intent into deterministic `operation-plans.json` and executes canonical REST/gRPC adapters through one transport-neutral `framework/operation.Executor`. C9.7 adds explicit transaction/idempotency policy, root `ExecutionScope` ownership, typed local child execution, Saga/Outbox transaction joining and durable MySQL-backed idempotency. C9.8 allows canonical internal Operations without fake RPC/REST exposure and excludes internal-only DTOs from external OpenAPI/TypeScript projections unless transport-reachable.

See `docs/waves/C9-operation-contract-runtime.md`, `docs/waves/C9.7-execution-semantics-closure.md`, and `docs/waves/C9.8-canonical-internal-operations.md`.

## Diagnostics and `yunka inspect`

W07 adds a read-only diagnostic control plane. `core.App.Diagnostics(ctx)` reports application
state, health, modules, registered routes, and RPC/event-bus inventory without exposing config
values, credentials, or request identity. `framework/diagnostics.Collector` composes additional
contract, resilience, and selector snapshots through the stable W03/W05/W06 snapshot seams.

The HTTP handler is opt-in and should be mounted on a dedicated loopback admin listener. Remote
access is rejected unless a Bearer token is explicitly configured. No diagnostics listener is
started automatically.

```bash
yunka inspect runtime --url http://127.0.0.1:16667/_yunka/diagnostics
yunka inspect runtime --format json
yunka inspect contract --manifest contracts/generated/manifest.json
```

Resilience inspection uses `RPCPolicy.PeekSnapshot`, which never allocates policy state. Selector
inspection calls `Snapshot(service)` and never performs a pick. See
`docs/waves/W07-diagnostics.md` for the security boundary and source model.

## Application Graph

W09 compiles declared contract facts and optional observed runtime facts into an evidence-backed
application graph. Relationships are never silently inferred from naming conventions: every node
and edge records whether its evidence is declared, observed, or explicitly inferred.

```bash
yunka graph build --manifest contracts/generated/manifest.json \
  --diagnostics runtime-diagnostics.json \
  --out .yunka/application-graph.json
yunka graph inspect
yunka graph find --query ApiService
yunka graph impact --id operation:ApiService.FindAllRuntimeAPI --depth 3
```

Without a W07 diagnostics export, `graph build` still produces a deterministic W06 contract graph.
Runtime selector/resilience sources are available through `framework/applicationgraph`. See
`docs/waves/W09-application-graph.md` for the evidence and impact model.

## Developer Runtime and TestKit

W10 adds a read-only environment doctor, explicit local process planning, and shared deterministic
test infrastructure.

```bash
yunka doctor
yunka doctor --strict
yunka dev plan --target api
yunka dev run --target api
yunka dev status --state .yunka/dev-runtime.json
```

`yunka doctor` never repairs the workspace automatically. `yunka dev` consumes `.yunka/dev.json`;
commands are argv arrays, working directories must remain below the repository root, dependency
cycles fail before startup, and optional `graphNode` values are validated against the W09 graph.
C1 manifest schema v2 adds an optional HTTP readiness barrier. A dependent process starts only
after its dependency returns the configured 2xx status and, when `diagnosticsReady=true`, reports
`core.health.ready=true`. Probes are bounded, do not follow redirects, read tokens only from a
named environment variable, allow plain HTTP only to literal loopback IP addresses, and require
HTTPS for remote endpoints. Schema v1 remains supported but cannot silently enable readiness.

C5 schema v3 closes the local runtime lifecycle. The explicit manifest process DAG is added to the
Application Graph as declared `process` nodes and `depends_on` / `runs` edges; commands are never
inferred. `yunka dev run` writes secret-free atomic mode-0600 state and observed runtime-graph
artifacts, optionally retains only the safe W07 core health/runtime summary from the readiness
request, and supervises direct children with bounded reverse-order graceful shutdown plus kill
fallback. Schema v1/v2 behavior remains compatible, while closure mode requires every selected
process to own one existing unique graph node. See `deploy/dev/yunka-dev.example.json` and
`docs/waves/C5-runtime-closure.md`.

`pkg/testkit` provides the leaf-safe deterministic Clock and Registry. `framework/testkit`
re-exports those helpers and adds a W08 Broker fake. TestKit is for deterministic tests only and is
not a production fallback. See `docs/waves/W10-dev-runtime-testkit.md` for the full boundary.

### C6 RPC generation foundation

C6 preserves existing Service business method signatures while replacing the historical RPC generator. `contracts/proto` is the canonical source root, and `make rpc-generate` is the only mutating standard protobuf/gRPC generation command. `make rpc-check` is read-only and blocks generated drift; `make rpc-compat-check` compares the current semantic contract against the C5 baseline. The legacy XR generator/runtime path has already been removed; remaining bridge/client packages are typed compatibility adapters over the standard grpc-go runtime, not a second protobuf or execution source of truth.

### C6 single RPC runtime

RPC contracts now come only from `contracts/proto` and are generated only with pinned standard protobuf/gRPC plugins. Existing Service business method signatures and the `gateway/rpc/meta` import path remain stable, while standard grpc-go typed registration and clients replace the XR generator, custom memory dispatcher, string registries, message pools, and legacy invoke transport. `make rpc-legacy-check` blocks architectural regression and `make rpc-consumer-check` proves the real Gateway Service business methods remain unchanged.