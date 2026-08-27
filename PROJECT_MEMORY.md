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

### 2026-08-18 — Dependency sync and verification baseline

- Pull requests must run dependency synchronization before the full verification gate. `make tidy` plus `go work sync` must leave all `go.mod`, `go.sum`, `go.work`, and `go.work.sum` files clean.
- Dependency drift is a blocking condition. Normal CI is read-only and must fail on drift; dependency metadata is repaired and committed through the ordinary local developer workflow, never by CI.
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

### 2026-08-19 — Application graph baseline

- `pkg/applicationgraph` is the canonical W09 graph schema and query layer. Node/edge IDs are deterministic and every relationship carries explicit evidence instead of relying on hidden naming assumptions.
- Graph evidence is classified as `declared`, `observed`, or `inferred`, with an explicit confidence level. Contract/configuration facts are declared, runtime/selector/resilience snapshots are observed, and any future inference must remain visibly labeled.
- W09 never invents module-to-service or cross-service call edges from grep, package names, or naming conventions. Absence of evidence is represented by absence of an edge.
- W06 protobuf contract contributes service/operation/message/explicit-HTTP edges; W07 core diagnostics contributes application/module/runtime-route inventory; W05 selector and W03 resilience contribute optional read-only runtime sources through `framework/applicationgraph`.
- `yunka graph build|inspect|find|impact` is the developer-facing graph surface. Dynamic graph artifacts are local/runtime products and are not committed as source-of-truth files by default.
- Impact analysis is directional: outbound traversal answers dependencies and inbound traversal answers dependents. It operates only on graph evidence already present and does not create new inferred edges during a query.

### 2026-08-19 — Developer runtime and TestKit baseline

- `yunka doctor` is read-only. It may inspect tool versions, workspace files, contract/graph consistency, Git status, and optional dev configuration, but it must never run dependency sync, generators, migrations, cleanup, or other mutating repair commands automatically.
- `yunka dev` runs only explicitly declared process manifests. Commands are argv arrays and are never executed through a shell; working directories must remain under the repository root and process dependency cycles/missing dependencies fail before startup.
- W10 process manifests may reference W09 graph node IDs for validation, but the graph never invents process startup commands. Local orchestration configuration remains an explicit developer decision.
- Child-process environment inheritance is explicit/allow-listable. Secrets are never copied into graph artifacts or diagnostic output merely because `yunka dev` inherits them for a process.
- `pkg/testkit` is the leaf-safe deterministic Clock/Registry layer; `framework/testkit` re-exports those helpers and adds the W08 Broker fake. Registry implements the complete current `registry.Registry` contract including watch events, so leaf packages no longer need partial interface fakes.
- TestKit is test-only infrastructure, not a runtime fallback. Production verification still requires real integration tests for databases, brokers, registries, and transports.
- Process readiness is not inferred from fixed sleeps. C1 manifest schema v2 uses bounded HTTP probes and may require the W01/W07 `core.health.ready` contract before dependents are started; schema v1 remains a compatibility input without readiness semantics.

### 2026-08-20 — C1 production-hardening baseline

- RPC caller identity is established only by a server-side `CredentialVerifier`. Production gRPC servers must apply the fail-closed authenticated interceptor to both unary and streaming calls before authorization middleware runs; verification errors are returned generically and verifier panics are contained.
- Static service tokens are a bootstrap workload-identity adapter, not end-user credentials. They use the dedicated `x-yunka-service-authorization` metadata key, require at least 32 visible ASCII bytes, support overlapping bindings for rotation, use digest-based constant-time comparison, and require transport privacy and integrity by default. Caller-supplied tenant, user, and role metadata remains untrusted.
- `yunka dev` schema v2 supports explicit readiness barriers. Dependents wait for bounded, no-redirect HTTP probes and may require `core.health.ready=true`; plain HTTP is restricted to literal loopback IP addresses and remote probes require HTTPS. Optional probe tokens are loaded only from a named environment variable, and a child exit before readiness fails the run and cancels the process group.
- The C0 MySQL Claim algorithm remains `READ COMMITTED` plus ID-only `FOR UPDATE SKIP LOCKED` queue reads and covering indexes. C1 real-MySQL regression covers 10 workers partitioning 100 records without overlap, in addition to lease reclaim.
- C1 does not include CI permission changes, pinned `protoc`, contract-source migration, or legacy RPC-generator removal; those remain C2/C3 scope.

### 2026-08-20 — C2 toolchain-determinism baseline

- `tools/toolchain.env` is the canonical repository tool lock. C2 pins Go `1.25.13`, protobuf release `21.12` / `libprotoc 3.21.12`, the Linux x86_64 protoc archive SHA-256, and `govulncheck v1.7.0`.
- Normal GitHub Actions CI is read-only (`contents: read`). It must never run `git commit`, `git push`, or auto-repair dependency/generated state; drift is a blocking signal for the ordinary local developer workflow.
- Third-party GitHub Actions used by CI are pinned to immutable commit SHAs. CI installs protoc from the locked release archive and verifies its SHA-256 before use; package-manager-selected protoc versions are not accepted for contract verification.
- `make toolchain-check`, contract generation/checking, `make verify`, and `yunka doctor` enforce exact Go/protoc agreement with the lock. Newer compilers are intentionally rejected until the lock is deliberately updated and reviewed.
- CI proves determinism twice: `make tidy` must leave workspace dependency metadata unchanged, contract regeneration must leave `contracts/generated` unchanged, and the final worktree must be clean. C2 changes tooling/governance only and does not start C3 contract-source convergence.

### 2026-08-20 — C3 contract-convergence baseline

- `contracts/sources.json` is the canonical service-contract inventory. Contract source sets are explicit and complete; C3 compiles the legacy API root and gateway runtime root independently before deterministic manifest merge so same-basename imports cannot cross roots.
- Contract ownership fails closed: source roots/import paths must remain inside the repository, every `.proto` under an inventoried root must be listed, and duplicate canonical files or protobuf message/enum/service full names across source sets are rejected.
- The C2 manifest remains schema v1 and C3 is additive: existing `ApiService`/`UnitService` stay intact while `io.yunka.gateway.rpc.GatewayService` and its namespaced types enter the canonical artifacts. C2 -> C3 compatibility must contain no breaking changes.
- HTTP routes are never inferred from dynamic gateway metadata. Only committed explicit protobuf bindings may enter OpenAPI paths; otherwise RPC methods remain unbound under `x-yunka-rpc-methods`.
- Committed legacy gateway generated RPC files remain immutable. `make rpc-contract-check` verifies their registered protobuf descriptors against `gateway/rpc/pb/`; `gateway/rpc/gender.sh` is not a deterministic production generation path and is not used by C3 verification.

### 2026-08-20 — C4 dependency-convergence baseline

- Dependency convergence removes the historical external `github.com/go-kit/kit v0.10.0` graph and all `replace google.golang.org/genproto => ...` directives. The pinned Aliyun SLS SDK still imports the old logging package path, so `compat/go-kit-kit-log` is the sole repository-owned compatibility workspace module for that path and delegates to `github.com/go-kit/log v0.2.1`.
- The compatibility module is an explicit isolation boundary, not a general-purpose dependency: it remains a workspace main module at the exact repository path, while identical version-scoped local replacements in the affected product modules make single-module `go mod tidy` deterministic. First-party production code uses the split `github.com/go-kit/log` import directly.
- The converged build list must not select monolithic `google.golang.org/genproto`, legacy unsplit `go.etcd.io/etcd`, or legacy `github.com/grpc-ecosystem/grpc-gateway` v1. Split genproto api/rpc modules are the supported contract/runtime dependencies.
- `tools/dependency-policy.json` plus `make dependency-check` is the durable graph guard. It validates the local compatibility module and prevents legacy protobuf imports from expanding beyond explicit compatibility islands.
- Legacy `github.com/golang/protobuf` remains temporarily permitted only where C3/legacy RPC compatibility requires it: the isolated RPC generator, committed gateway/SMS generated protobuf, and bounded `pkg` RPC compatibility code. Protobuf packages must not be used merely for pointer helpers.
- The workspace now contains five product modules plus one compatibility module. `app/cmd/rpc` remains separate because its generator dependencies are an isolation boundary; module count is not optimized cosmetically.
- C4 is dependency-graph convergence, not a broad upgrade sweep and not C5 runtime closure.


### 2026-08-21 — C5 runtime-closure baseline

- Dev manifest schema v3 closes the local runtime loop while schema v1/v2 remain compatible. Commands, working directories, dependencies, inherited environment-variable names, readiness endpoints, and graph ownership remain explicit configuration and are never inferred from code or naming.
- Closure mode requires every selected process to own one existing unique Application Graph node before startup. The graph records declared application/process containment, process dependencies, and exact process-to-node `runs` ownership; observed runtime facts never alter the plan.
- `yunka dev run` writes atomic mode-0600 local state and runtime-graph artifacts under repository-contained paths. Reports exclude commands, environment values, credentials, request identity, diagnostics component payloads, and child output; error text is redacted and bounded.
- Readiness may retain only the safe W07 core summary: application/health state, liveness/readiness, route count, RPC inventory, and event-bus presence. The same bounded authenticated no-redirect probe remains the trust boundary.
- Direct children shut down in reverse plan order under one bounded timeout. The runner sends a graceful platform signal first and kills only children that remain after the deadline; unexpected exit shuts down the remaining children and no restart policy is implied.
- Runtime state and graph files are local evidence, not committed source of truth. C5 does not provide process-tree/container management, a public control server, remote multi-host orchestration, or production deployment management.

### 2026-08-22 — C6 RPC-unification implementation decision

- C6 replaces the historical RPC generator through one canonical `contracts/proto` inventory, pinned standard Go protobuf/gRPC plugins, deterministic `rpc-generate`/`rpc-check`, and one grpc-go runtime.
- Existing framework Service business code is a protected ABI: Service method bodies, `core.BaseService` embedding, request/response type names, and the existing gateway `meta` import path are not changed by C6.
- Compatibility code may preserve source-level constructors and client methods, but it may not own protobuf messages/descriptors, network transports, dynamic service location, reflection injection, hidden `init` registration, or lifecycle-bearing object pools.
- The old XR wrappers may exist only during predeclared C6 subwaves. Final C6 deletes `app/cmd/rpc`, `gender.sh`, `*.xr_*.go`, generated memory transport, string registries, message pools, and legacy protobuf RPC output; there is no fallback generator.
- C7 remains responsible for removing the reflective module container, package globals, and lifecycle-bearing `sync.Pool`; C6 exposes typed seams so that C7 does not require business Service rewrites.

### 2026-08-22 — C6.2 typed compatibility bridge decision

- Existing framework Service business methods remain the protected ABI. C6.2 adapts them through a typed per-call provider and `request.Runtime` factory; it does not require Service struct, method-body, request/response, `core.BaseService`, or `GetName()` changes.
- The standard generated `GatewayServiceServer` is the only typed server contract. The handwritten bridge acquires one Service per call, derives Runtime identity/metadata/trace state from the authenticated gRPC context, executes finish hooks, clears Runtime, and releases the current module-container object exactly once.
- `gateway/rpc/client.GatewayServiceClient` is now a handwritten source-compatible facade over the standard generated typed client. `*Special(..., nodeIP)` resolves through an explicit target client factory; new composition must not create one connection per call.
- The pre-C6 `invoke.RpcClient` constructor path is isolated behind one unary-only compatibility connection adapter until C6.3. It is not a second protobuf owner or a permitted new integration path.
- Standard typed registration replaces the generated gRPC handler map. The deprecated string registration method delegates immediately to typed registration and must be deleted with the remaining XR bootstrap in C6.3.
- `bufconn` is the transport-level test seam. The generated memory dispatcher is no longer used to prove C6.2 behavior and remains only for C6.3 deletion.

### 2026-08-22 — C6.3/C6.4 single-runtime closure

- `contracts/proto` is the only RPC source root. `protoc-gen-go` and `protoc-gen-go-grpc` are the only generators; `app/cmd/rpc`, `gateway/rpc/pb`, `gender.sh`, all `*.xr_*.go`, generated memory dispatch, string handler registries, and `pkg/invoke` are deleted.
- grpc-go typed clients, typed service registration, standard full method names, and `bufconn` are the only RPC runtime and test transport.
- Historical Gateway client method names and `*Special(..., target)` remain as handwritten facades over an explicit typed Factory. They own no descriptors, routing registry, connection pool, or fallback transport.
- The real `RoleIntercept` business RPC method bodies are hash-frozen and compile directly as the standard generated server interface. C6.4 also runs an external-package consumer fixture through typed gRPC without changing the pre-C6 service shape.
- W3 resilience and observability now compose through `grpc.UnaryClientInterceptor`; W5 selects targets but does not implement an RPC transport.
- C7 remains responsible for replacing global composition holders, reflection DI, and lifecycle-bearing object pools. C6 does not reintroduce those concerns into RPC.

### 2026-08-22 — C7.1 static-module-catalog baseline

- C7 preserves the old framework's one-line blank-import module enablement. An `autoload` package may use `init()` only to register one immutable `modulecatalog.Descriptor`; it may not read config/environment, perform I/O, start goroutines, create infrastructure or services, or mutate an App.
- The default module catalog stores descriptors only. Runtime configuration, DB/Redis handles, event bus, typed RPC connections/clients, services, repositories, request scopes, transactions, principals, and lifecycle state belong to an App instance or explicit owner and must never enter the catalog.
- Modules keep zero dependency-acquisition code but must declare typed requirements. `NewApp` aggregates requirements before any module build, resolves named shared DB/RPC capabilities once, and restricts each build context to the module's declared config/logger/database/event/RPC view. Generated `BuildFunc` wiring is ordinary Go checked by the compiler; the new path forbids runtime field injection, `reflect.Value.Call`, generic service lookup, and lifecycle-bearing `sync.Pool`.
- `core.NewApp(AppOptions)` and `framework/kernel.New` are the isolated typed construction path. They aggregate declared requirements before any module build, prepare named shared DB/RPC capabilities once, and give each module only its declared capability view. A supplied Catalog supports tests and multiple Apps; a nil kernel Catalog consumes the process-default descriptor catalog populated by blank imports.
- C7.1 is additive and independently mergeable. New App instances do not consume legacy prepare/initiator globals, cannot mutate `globalConf`, and own RPC one-time state independently; the package-level default App remains compatibility-only. The remaining global config/store, reflective `framework/core/module` container, `pkg/di`, and service/repository/infrastructure pools remain explicit migration debt. New modules and call sites must use the typed catalog path; C7.3 deletes the old path after C7.2 module/request-scope migration.
- Module startup order is determined by explicit `DependsOn` DAG and stable module names, never Go import/init order. App Start/Health/Shutdown remains the single lifecycle spine and typed modules shut down in reverse catalog order.

### 2026-08-23 — C7.2 platform-provider and request-scope baseline

- `framework/platform.Provider` is the App-owned capability owner for the typed catalog path. It opens only named DB/RPC requirements declared by the sealed plan, opens each required name once, exposes only a module's declared view, starts and checks resources in deterministic key order, and closes them in reverse order. The process-default catalog continues to store descriptors only.
- `kernel.Options.Platform` is the preferred typed bootstrap input and cannot be combined with direct capability fields or another `ContextFactory`. The existing App lifecycle remains the single Start/Health/Shutdown spine. The shared EventBus stays App-owned; named DB/RPC resources are provider-owned, and composition-build failure cleans both owners.
- Platform MySQL and gRPC configuration is process-level only. Modules never receive DSNs, TLS material, connection factories, or root configuration. gRPC construction requires explicit transport credentials; insecure transport is never an implicit default.
- `framework/requestscope` replaces request mutation of singleton Services with a fresh request-owned Scope. A Scope snapshots trusted Principal/runtime metadata/trace state, owns one Unit of Work and typed repository set, commits on success, rolls back on error or panic, and closes idempotently. Construction panic cleanup and rollback detach request cancellation while retaining trusted context values; a failed commit remains rollback-eligible. GORM request scopes begin exactly one transaction and own only that transaction; the App-owned pool is never closed by a request.
- Typed platform/request-scope paths may not import the legacy reflection module container, `pkg/di`, or legacy `request.Runtime`; may not use lifecycle-bearing `sync.Pool`; and may not call global service locators. C7.2.3 migrates the first real vertical slice before C7.3 deletes the compatibility path.

### 2026-08-24 — C7.3 single typed runtime baseline

- The static module catalog, `core.NewApp` / `kernel.New`, `platform.Provider`, explicit typed constructors, and `requestscope.Factory` are the only composition and ownership path. There is no package-level default App, global configuration/prepare/initiator/RPC holder, reflection container, generic service locator, or fallback request Runtime.
- `framework/core/module`, legacy `core.Module`/`Service`/repository contracts, `BaseService`, `BaseRepository`, `pkg/di`, `framework/ingress`, runtime-mutating ORM adapters, Gateway pooled local execution, and the RPC `ModuleGatewayProvider`/Runtime factory are deleted.
- Gateway HTTP execution creates one concrete `request.Context` per request. It owns transport state, cancellation/deadline, trusted Principal, runtime metadata, trace ID, logger, and request-local storage; it is never retained by a singleton service or stored in `sync.Pool`.
- Composite Gateway calls create a fresh HTTP Context, explicitly copy request transport data, and inherit the parent `context.Context`; mutable response and request-local storage are not shared. Database transaction/repository ownership remains exclusively in `framework/requestscope`.
- Gateway local execution through `App.GetModule → GetService → SetRuntime → PutService` is removed. `HandleMiddleware` requires an explicitly composed executor. Typed gRPC registers an explicitly owned `GatewayBusinessService` and no longer adapts a module service pool.
- JWT and in-memory SMS adapters receive configuration/logger dependencies explicitly. The old `yunka controller` generator is deleted; the supported generator is the typed module descriptor/autoload path.
- The C7 architecture gate permanently requires removed paths and APIs to remain absent and forbids request-context pooling in Gateway execution. Reflection used for ordinary binding/serialization and pools used for unrelated stateless transport buffers remain allowed.

### 2026-08-25 — C7.4 typed module developer experience baseline

- `yunka module new` is the canonical module bootstrap. It generates atomically, resolves the owning Go module from `--root`, declares Config/Logger/named DB/EventBus/named RPC/DependsOn capabilities explicitly, and emits compiler-checked `Dependencies`, `GeneratedDescriptor`, and one-purpose autoload registration.
- `yunka module check` plus `make module-check` is the permanent structural/DX gate. Generated wiring is owned by the generator; business config, dependencies, and lifecycle remain ordinary reviewed Go source.
- Module packages may not acquire environment configuration, construct database/RPC connections, import `platform.Provider`, use package-global service lookup, reflection composition, hidden `init`, or lifecycle-bearing `sync.Pool`. Autoload remains the only bounded `init` surface and may register one immutable descriptor only.
- `framework/modules/outboxruntime` is the second real typed module and the reference App-lifecycle shape. It declares Config + Logger + named `primary` DB + EventBus, owns one GORM outbox store/local broker/dispatcher per App, supports deterministic Start/Health/Shutdown, and keeps request transactions/repositories outside the module in `requestscope`.
- C7.4 does not introduce a DI container, generic service locator, framework-owned business repository abstraction, contract changes, or implicit infrastructure defaults.

### 2026-08-26 — C8.3 gateway authorization convergence decision

- `gateway/authz` is the canonical authorization boundary. Authentication establishes `identity.Principal`; authorization evaluates explicit Operation policies and stable Permission keys.
- Roles own Permissions, never Buttons. API/Operation and UI Button/Menu may both reference a Permission; Button/Menu binding is UI metadata and is not a backend authorization grant.
- PB method declarations are the authorization source of truth. Stable operation/permission identities must not be derived from API UUID, HTTP path, Button UUID, or database identifiers.
- Gateway enforcement is fail-closed and must converge on one Authorizer at every execution boundary, including composite child operations. Resource/data-scope evaluation remains a separate seam and must not be encoded as SQL in PB.
- Existing `AuthBit`, `api_module_button`, and `role_module_button` are migration compatibility inputs only; they must not be expanded as the new authorization model.

### 2026-08-27 — C8.4 PB DSL and domain compiler responsibility decision

- Protobuf is the canonical writable DSL for RPC, explicit REST bindings, DTOs, Domain/Application declarations, stable Operation IDs, authentication/tenant requirements, and Permission requirements. Typed protobuf options are the target authoring form; existing `@yunka.*` comments are a bounded migration bridge only.
- Developer-owned PO structs are the persistence-schema source. The domain compiler may reverse PO into generated Entity, basic Repository CRUD interface, GORM record/mapping, and Repository CRUD implementation, then it must stop.
- `yunka domain` must not generate Application `DefaultService` behavior, REST/RPC adapters, protobuf sources or generated protobuf, gRPC bridges, runtime registration, or wiring. Business invariants, use-case orchestration, DTO/domain mapping, state machines, complex queries, and cross-domain workflows remain handwritten.
- PB DTO, Domain Entity, and PO are separate models. The framework must not infer their equivalence or generate business-semantic mappings from matching field names.
- REST and RPC adapters generated from PB must invoke the same Application Port and the same Gateway `Authorizer` policy. C8.3 Role-to-Permission storage and fail-closed authorization semantics remain unchanged; data-scope predicates remain outside PB.
- Migration follows expand → shadow → cutover → contract. Contract Manifest V2 and Domain Manifest V3 retain read compatibility during migration, while new generation writes only the new canonical forms. Destructive legacy full-stack generator removal is isolated after deterministic, architecture, authorization, race, compatibility, and MySQL integration gates pass.
