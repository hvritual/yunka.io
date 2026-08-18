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
- Singleton infrastructures start in binding order and shut down in reverse binding order.
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
