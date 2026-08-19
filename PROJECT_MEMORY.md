# Project Memory

This file records durable decisions that every new task must read and follow.

## Repository baseline

- Canonical repository: `hvritual/yunka.io`
- Repository URL: `https://github.com/hvritual/yunka.io`
- Git remote name: `origin`
- Git remote URL: `https://github.com/hvritual/yunka.io.git`
- Default branch: `main`
- Repository visibility: private
- GitHub access verified on 2026-08-17 with admin, push, and pull permissions.
- Local GitHub account: `hvritual`
- Local Git transport: HTTPS

## Working decisions

### 2026-08-17 — Canonical task repository

All subsequent project tasks must use this repository as their source of truth and working scope. Project code, documentation, plans, and durable decisions belong in this repository unless the user explicitly changes that scope.

### 2026-08-17 — Local Git commits only

All work that requires a Git commit must be committed with the local `git` command-line tool. GitHub connector or API operations must not be used to construct commits or modify Git objects and refs.

The GitHub connector remains appropriate for reading repository metadata and collaborating through issues, pull requests, reviews, and permissions.

### 2026-08-17 — Mandatory memory read

Every new task must read `AGENTS.md` and this file before planning, analysis, edits, or task-specific commands. The task must also verify the current branch, working-tree status, and configured remotes before making changes.

### 2026-08-17 — Local GitHub authorization and reuse

Local GitHub authorization is established for account `hvritual` with the minimum `repo` OAuth scope required to read and write the private repository. Local Git fetch and push must reuse the checkout's configured HTTPS credential helper.

Current workspace tooling:

- GitHub CLI version: `2.45.0`
- GitHub CLI binary: `/workspace/scratch/7cbedf9749a1/tools/gh/usr/bin/gh`
- GitHub CLI wrapper: `/workspace/scratch/7cbedf9749a1/tools/bin/gh`
- GitHub CLI config: `/workspace/scratch/7cbedf9749a1/tools/gh-config`
- Local Git credential file: `/workspace/scratch/7cbedf9749a1/tools/git-credentials`
- Credential file permission: `0600`
- Required network endpoints: `github.com:443` and `api.github.com:443`

The credential and OAuth token are environment-local secrets. Never print, inspect, copy into project files, include in logs, commit, or expose them through tool output. Repository memory records only non-secret metadata. If the environment-local paths no longer exist, reauthenticate instead of reconstructing or requesting the token in chat.

Use the configured local `git` commands for fetch, merge, commit, and push. The local Git credential helper is the source of truth for repository authentication. The `gh` wrapper path is available for CLI execution, but its authentication must be checked separately and its permissions must not be broadened automatically beyond the repository operation requested by the user.

## Connection boundary

GitHub connector authorization and local Git authorization are separate. The connector remains available for repository collaboration operations, while local Git uses the environment-local credential described above.

### 2026-08-17 — MVP stabilization baseline

- The repository is a Go 1.25.13 workspace rooted at `go.work`; `app` uses module path `yunka.io/app` while `framework` remains `yunka.io/framework`.
- The supported verification gate is `make verify`: unit tests, race tests, `go vet`, `govulncheck`, and builds across all five modules.
- Gateway authentication is fail-closed. JWT configuration requires a secret of at least 32 bytes plus issuer and audience; tokens are accepted only through `Authorization: Bearer`.
- HTTP and etcd clients must verify TLS certificates. Disabling certificate verification is not an accepted runtime option.
- Runtime authorization identities are derived from validated server-side credentials; query-supplied identity fields must not be trusted.

### 2026-08-18 — Runtime lifecycle baseline

- Application and process-scoped resources use explicit lifecycle contracts: `Startable`, `Shutdowner`, and `HealthChecker`.
- Modules start in registration order and shut down in reverse registration order.
- Only singleton infrastructures are process-lifecycle managed; request-scoped `sync.Pool` infrastructure and repositories are not enumerated or closed by application shutdown.
- Singleton infrastructures start in binding order and shut down in reverse order.
- Application health is transport-neutral and exposed as a structured `HealthReport`; gateway health endpoints and diagnostics should adapt this report rather than define separate health semantics.

### 2026-08-18 — Runtime context and identity baseline

- `core/identity.Principal` is the canonical trusted caller identity. `Authenticated` may only be set after server-side credential validation.
- `core/runtimecontext.Metadata` is the transport-neutral operation metadata model; HTTP/RPC/event/job adapters derive child contexts rather than sharing mutable transport state.
- `core/middleware.Chain` uses `context.Context` as its common boundary so the same middleware can run across HTTP, RPC, events, and jobs.
- `request.Runtime` remains source-compatible. `request.ContextRuntime` is the optional richer contract implemented by `WorkRuntime` for Principal, metadata, trace ID, and base-context access.
- Gateway JWT/API-key authentication establishes a Principal. Role authorization reads only that trusted Principal; query-supplied `oid`/`uid`/`rid` values are compatibility data and are not authorization inputs.
- Generated RPC files remain immutable. RPC middleware must be attached through transport-neutral client wrappers or non-generated gRPC interceptors.
- Remote RPC identity metadata is not automatically trusted; a downstream service must validate its own service/caller credentials before marking a Principal authenticated.

### 2026-08-18 — Resilience baseline

- Resilience policies are transport-neutral middleware and must not be embedded directly in business services.
- Stateful policies are isolated by `runtimecontext.Metadata.Operation` (or an explicit key function); one failing RPC method must not open a global service-wide breaker by default.
- Timeout budget is the outer call budget and all retries share that same parent budget.
- Retries are fail-safe and require both explicit idempotency and explicit retryable-error classification; existing calls are never retried implicitly.
- Outbound RPC policy order is timeout budget → retry → rate limit → load shed → circuit breaker → transport, so every retry attempt remains governed.
- Load shedding uses concurrency admission plus minimum remaining deadline and adaptive latency feedback; policy snapshots are the stable seam for later SLS/OpenTelemetry observability.
- Resilience policy keys must be low-cardinality operation identifiers; request IDs, user IDs, raw URLs, and other unbounded values are forbidden as breaker/limiter keys.
- Gateway adapters map resilience rejections to transport semantics: rate limiting to HTTP 429, circuit/load shedding to 503, and timeout budget/deadline exhaustion to 504.

### 2026-08-18 — SLS observability baseline

- `framework/observability` is the canonical telemetry layer; application and domain code must not depend directly on Aliyun SLS SDKs.
- Traces and metrics use OpenTelemetry standards and standard `OTEL_*` exporter configuration. The preferred production path is OTLP to an OpenTelemetry Collector that owns SLS credentials.
- Framework/application logs use structured `log/slog` JSON output. LoongCollector is the preferred SLS log/event transport so application request latency is decoupled from SLS network writes.
- Runtime events are represented once and fanned out to structured logs, span events, and event metrics; `signal=event` is the routing discriminator for SLS ingestion pipelines.
- OpenTelemetry Go logs are not the default W4 log path because the Go log signal is still beta; the stable W4 default is `slog` JSON plus LoongCollector.
- W3C Trace Context is propagated across gateway HTTP and gRPC boundaries. Legacy `X-Trace-Id` remains a compatibility response/header surface but the OpenTelemetry trace ID is canonical once an OTel span exists.
- Telemetry identity attributes are disabled by default. Tenant/user/role identifiers require explicit `IncludeIdentity` opt-in and deployment-specific privacy/retention review.
- SLS AccessKeys must not be committed or embedded in application config; use least-privilege RAM credentials in Collector/Prometheus deployment secrets.

### 2026-08-18 — Dependency sync and verification baseline

- Pull requests must run dependency synchronization before the full verification gate. `make tidy` plus `go work sync` must leave all `go.mod`, `go.sum`, `go.work`, and `go.work.sum` files clean.
- Dependency drift is a blocking condition. Same-repository PRs may use the CI runner's local `git` to commit the exact generated dependency state; fork or non-writable runs must fail rather than silently continue.
- The full release gate remains `make verify` after dependency synchronization: unit tests, race tests, `go vet`, `govulncheck`, and builds across all five modules.
- The mDNS registry baseline uses maintained `github.com/hashicorp/mdns v1.0.6`; the old `github.com/micro/mdns v0.3.0` implementation is retired because its probe/shutdown path fails the race gate.
- mDNS watching uses bounded polling plus snapshot diff semantics instead of the legacy TTL/listen callback path; create, update, and delete results remain the registry watcher contract.

### 2026-08-19 — Selector 2.0 baseline

- The legacy `selector.Selector` contract remains source-compatible and `NewSelector()` keeps the historical Random default; adaptive routing is opt-in through `NewAdaptiveSelector()` or `EnableAdaptive`.
- `Picker.Pick` plus idempotent `Selection.Done` is the canonical W5 feedback lifecycle. It records in-flight requests, measured latency, success/failure outcome, and passive node health without changing generated RPC code.
- Adaptive selection supports P2C, EWMA, and LeastRequest. Selection state is scoped to service/version/node identity and reconciled against registry membership so removed nodes do not leak state.
- Passive outlier detection uses consecutive failures, bounded exponential ejection time, a maximum ejection percentage, single-node protection, and fail-open zero-value behavior; fail-closed operation must be explicit.
- W3 resilience must wrap W5 selection (`RPCPolicy.WrapSelected`) so every retry attempt performs a fresh node pick while rate-limit/load-shed/circuit rejections occur before selection and never poison a node's passive-health score.
- Selector snapshots are the stable diagnostics/observability seam and expose EWMA, score, in-flight requests, ejection state, and cumulative selection/outcome counters without coupling `pkg/selector` to the framework observability package.

### 2026-08-19 — Contract pipeline baseline

- Protobuf is the W06 API contract source of truth. Existing legacy gateway/HTTP metadata remains compatibility state until explicitly migrated into protobuf; the contract generator must never invent HTTP routes for unbound RPC methods.
- `pkg/contract` compiles protobuf through `protoc` `FileDescriptorSet` and normalizes it into a vendor-neutral deterministic manifest without adding a new protobuf runtime dependency to the shared package.
- Committed contract artifacts are `contracts/generated/manifest.json`, `openapi.json`, and `client.ts`; generated files are never hand-edited and `make contract-check` blocks drift.
- Standard `google.api.http` is the preferred HTTP binding. `@yunka.http` source comments are a migration bridge only; other `@yunka.*` method directives are preserved as contract metadata for later auth/resilience/graph integration.
- Contract Guard treats service/method removal, request/response or streaming changes, protobuf field removal/renumber/type/cardinality/presence/JSON-name changes, enum removals/renumbering, and existing HTTP binding removal/change as breaking changes.
- W06 extends the release gate so `make verify` includes contract drift checking. Pull-request CI additionally compares the base commit manifest when one exists and blocks breaking changes.
- The legacy RPC generator remains operational and generated RPC files remain immutable. W06 establishes a parallel contract pipeline first; generator convergence is a later step after the new pipeline is stable.

### 2026-08-19 — Diagnostics baseline

- W07 diagnostics are read-only by design. `core.App.Diagnostics` exposes application state, health, module/route inventory, and RPC/event-bus presence but never configuration values, credentials, request payloads, or caller identity.
- `framework/diagnostics.Collector` composes optional Contract, Resilience, Selector, and future sources without making `framework/core` depend on W3/W5/W6 packages; source failure/panic is isolated to that component.
- Diagnostics consume stable public snapshot seams only. W3 adds `RPCPolicy.PeekSnapshot` because the legacy `Snapshot` path may materialize policy state; observation must not create breaker/limiter/load-shedder state.
- Selector and resilience diagnostic scopes are explicit service names and low-cardinality operation keys; diagnostics must not introduce request/user/raw-URL cardinality.
- The diagnostics HTTP handler is opt-in, GET-only, no-store, and loopback-only by default. Remote exposure requires an explicit Bearer token; yunka never starts a public diagnostics listener automatically.
- `yunka inspect runtime` performs read-only HTTP inspection and `yunka inspect contract` reads the committed W06 manifest. Distributed call-edge inference is intentionally deferred to the Application Graph wave.

### 2026-08-19 — Event broker and transactional outbox baseline

- `framework/event.Envelope` is the canonical business-event transport contract. Event IDs remain stable across retries and are the consumer idempotency key; payload/metadata are cloned at transport boundaries, while topic/type and metadata size are bounded to protect routing/telemetry cardinality.
- `event.Broker` is transport-neutral. The W08 `LocalBroker` adapts the existing trie EventBus for process-local delivery only and must never be described as durable messaging.
- Event handling starts a new trust context. Trace propagation may cross the event boundary, but authenticated `Principal` identity is never inherited implicitly; remote consumers must re-establish trusted identity when authorization is required.
- Transactional-outbox atomicity exists only when the business write and outbox row use the exact same database transaction. The GORM adapter uses `ORM.TransactionDB()`/`EnqueueTx`; `MemoryStore` intentionally does not implement the transactional-store contract.
- Outbox delivery is at-least-once. Broker success followed by a failed `MarkPublished` may cause redelivery after lease expiry, so consumers with non-idempotent effects must deduplicate by `Envelope.ID`.
- Dispatcher claims are lease-based, bounded-concurrency, and validated so the lease covers the worst-case claimed batch publish window. Retry uses bounded exponential backoff; exhausted records become dead-letter and are never auto-replayed.
- Published-event retention is explicit through `RetentionStore`; the dispatcher does not silently purge published history. Generic diagnostics expose aggregate counts/age only and never event payload, metadata, ID, or error body.
- W4 trace/metrics/log integration is adapter-based and SLS remains observability only. External Kafka/RabbitMQ/Pulsar adapters are deployment choices and are not dependencies of the W08 framework baseline.
