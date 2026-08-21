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
- `app/cmd/rpc/`: the legacy RPC code generator
- `compat/go-kit-kit-log/`: repository-owned SLS logging compatibility module

The modules are joined by the root `go.work`. `app` intentionally uses the distinct module
path `yunka.io/app`; `framework` remains `yunka.io/framework`.

## Verification

```bash
make toolchain-check
make dependency-check
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

`tools/dependency-policy.json` is enforced by `yunka dependency check` / `make dependency-check`. The gate validates the repository-local compatibility module, rejects unsplit etcd, grpc-gateway v1, monolithic genproto, any external replacement, a reintroduced genproto replace, and new legacy protobuf imports outside approved compatibility islands. Existing generated gateway/SMS protobuf and the isolated legacy RPC generator remain compatibility artifacts; C4 does not rewrite them. See `docs/waves/C4-dependency-convergence.md`.

The workspace contains five product modules plus the single compatibility module. `app/cmd/rpc` remains separate so legacy generator dependencies do not become normal application CLI dependencies.

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

The framework owns the lifecycle of process-scoped singleton infrastructures. A singleton
infrastructure may implement any of the following optional interfaces from
`yunka.io/framework/core`:

- `Startable` for explicit startup work.
- `Shutdowner` for context-aware graceful shutdown.
- `HealthChecker` for dependency health checks.

Modules start in registration order and shut down in reverse registration order. Singleton
infrastructures follow their binding order and are shut down in reverse order. Request-scoped
objects held in `sync.Pool` are intentionally excluded from process lifecycle management.

`App.Health(ctx)` returns a structured `HealthReport` with application state, liveness,
readiness, and module health checks. The report is intended to be exposed later by gateway
health endpoints and diagnostics without coupling health semantics to a specific transport.

## Runtime context, identity, and middleware

W2 introduces a transport-neutral execution context without breaking the existing
`request.Runtime` interface:

- `core/identity.Principal` is the canonical trusted caller identity. `Authenticated` is set
  only after server-side credential verification.
- `core/runtimecontext.Metadata` carries transport, protocol, operation, route, service,
  module, method, request, and trace metadata through `context.Context`.
- `core/middleware.Chain` wraps `context.Context`, so the same middleware model can be used
  by HTTP, RPC, events, and jobs.
- `request.ContextRuntime` is an optional richer contract implemented by `WorkRuntime` for
  Principal, metadata, and trace access while preserving existing custom Runtime implementations.
- Gateway JWT and API-key authentication establish a trusted Principal. Role authorization
  reads only that Principal and never trusts query-supplied `oid`, `uid`, or `rid` values.
- Composite gateway calls inherit the parent identity, deadline, and trace context while
  deriving child operation metadata.
- RPC middleware is attached through `core/middleware.WrapRPCClient` or non-generated gRPC
  interceptors; generated RPC files remain untouched.

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

W06 makes protobuf the contract source of truth and adds deterministic contract artifacts and
compatibility checks without replacing the legacy RPC generator.

```bash
yunka contract lint
yunka contract generate
yunka contract inspect
yunka contract check
```

C3 makes `contracts/sources.json` the canonical service-contract inventory. The historical
`app/cmd/rpc/pb/` source remains as `legacy-api`, while the actual gateway runtime protobuf under
`gateway/rpc/pb/` is compiled independently as `gateway-runtime`. Separate protoc invocations
prevent same-basename imports from crossing source roots. The normalized results are merged
deterministically into the committed `contracts/generated/` artifacts.

`make rpc-contract-check` additionally proves that the descriptors registered by the committed
legacy `gateway/rpc/meta/*.pb.go` files still match `gateway/rpc/pb/`. Generated RPC files remain
immutable in C3; the historical destructive generator is not treated as an authoritative contract
source.

Only explicit HTTP bindings are emitted into OpenAPI `paths`. Standard `google.api.http` is
preferred; `@yunka.http` comments remain a migration bridge. C3 does not infer routes from dynamic
gateway metadata, so unbound RPC methods remain under `x-yunka-rpc-methods` rather than receiving
invented paths. Pull-request CI continues to reject breaking service, method, field, enum,
streaming, and HTTP-binding changes.

See `contracts/README.md`, `docs/waves/W06-contract-pipeline.md`, and
`docs/waves/C3-contract-convergence.md` for the contract model and convergence boundary.

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
