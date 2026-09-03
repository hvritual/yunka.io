# Project Memory

> Document class: **DECISION**  
> Authority: durable repository governance and architecture invariants  
> Current delivery/status authority: [`docs/STATUS.md`](docs/STATUS.md)  
> Documentation governance: [`docs/DOCUMENTATION_GOVERNANCE.md`](docs/DOCUMENTATION_GOVERNANCE.md)

This file records decisions and invariants that must remain active across future work. It is intentionally **not** a current-state ledger. Current HEADs, repository visibility, active PR/task state, wave progress, release tables, and other independently changing facts belong in `docs/STATUS.md`, Git/GitHub state, or exact qualification records.

`AGENTS.md` is the repository-governance authority. If an older historical wave/issue/PR statement conflicts with an active invariant here or with `AGENTS.md`, follow the current authority and treat the older statement as historical evidence.

## Mandatory task bootstrap

Every repository task must:

1. read `AGENTS.md` completely;
2. read this file completely;
3. read `docs/STATUS.md` completely;
4. inspect the current repository/base/head before mutation;
5. inspect local branch/status/remotes when a local checkout exists;
6. preserve user changes and inspect relevant current, historical, and evidence documentation before editing.

## Repository and Git policy

- Canonical repository: `https://github.com/hvritual/yunka.io`.
- Default branch: `main`.
- Canonical local remote name: `origin` -> `https://github.com/hvritual/yunka.io.git`.
- Local Git is preferred when a usable checkout and authorization are available.
- The authorized GitHub Connector is a valid write path when local Git is unavailable.
- Normal implementation work uses an isolated task branch and repository review/merge flow.
- Do not move/overwrite `main`, force-update a branch, discard user changes, or destroy history without explicit authorization.
- Environment-local credentials and tokens are secrets and are never durable project state.

## Documentation governance baseline

- `docs/DOCUMENTATION_GOVERNANCE.md` owns documentation classes and truth-ownership rules.
- `docs/STATUS.md` is the single documentation authority for current framework/wave/release status and explicitly deferred pressure.
- `README.md` is current developer/product documentation.
- `PROJECT_MEMORY.md` owns durable active governance and architecture invariants only.
- `docs/waves/**` contains historical roadmaps, migration records, and exact qualification evidence; original `Planned`/`In progress` prose is not current truth once a roadmap is classified `HISTORICAL`.
- Executable code/tests/generated evidence outrank narrative claims about behavior. Documentation must be reconciled to executable truth, not the reverse.
- Qualification evidence proves the exact candidate/tree/consumer pair it names; later state must establish its own inheritance or qualification relationship.

## Toolchain and verification baseline

- Go toolchain baseline: `1.25.13`.
- Protobuf compiler baseline: protobuf release `21.12` / `libprotoc 3.21.12`.
- Exact tool versions/checksums are locked by `tools/toolchain.env`.
- `protoc-gen-go` and `protoc-gen-go-grpc` are the only RPC generators.
- Normal CI is read-only and must never auto-commit or auto-repair dependency/generated drift.
- Third-party GitHub Actions are pinned to immutable commit SHAs.
- `make verify` is the canonical standard repository gate.
- `make verify-production` extends the standard gate with real MySQL integration.
- Dependency/generated/worktree drift is a blocking signal, not something verification silently repairs.
- TestKit/memory fakes are deterministic test infrastructure only and never production fallback evidence.

## Workspace and dependency baseline

- The repository is a Go workspace rooted at `go.work`.
- Product modules are `pkg`, `framework`, `gateway`, and `app`; the repository also owns the narrow `compat/go-kit-kit-log` compatibility module required by the pinned SLS SDK.
- Historical monolithic dependency/runtime surfaces may remain only in explicitly reviewed compatibility islands.
- `tools/dependency-policy.json` is the durable dependency-graph guard.
- Legacy protobuf/runtime dependencies must not spread into new framework/application code.

## Explicit application composition baseline

- `core.NewApp(AppOptions)` / `framework/kernel.New` are the explicit application construction path.
- `core.App` is the sole application/process lifecycle owner for typed modules and application-scoped capabilities.
- There is no package-global default App, generic service locator, reflection-based DI container, runtime field injection, or fallback request Runtime in the active architecture.
- Lifecycle-bearing `sync.Pool` is forbidden for services, repositories, clients, connections, transactions, request contexts/scopes, or other owned resources.
- Runtime reflection may be used for ordinary data binding/serialization, not dependency discovery or business dispatch.
- Static module catalog registration is descriptor-only. Autoload registration may not read runtime configuration, perform I/O, create infrastructure/services, or start goroutines.
- Module startup order comes from explicit dependency DAG facts and is deterministic.

## Platform ownership and request scope

- `framework/platform.Provider` is the App-owned capability owner for named shared infrastructure such as DB and RPC resources.
- Shared resources are prepared once, exposed only through declared typed capability views, and participate in deterministic Start/Health/Shutdown ownership.
- Process-level DB/RPC connection configuration, DSNs, TLS material, and factories are not exposed to business modules.
- `framework/requestscope` provides a fresh typed Scope/repository view per operation and snapshots trusted Principal/runtime metadata/trace state.
- The root `framework/execution.ExecutionScope` owns the Operation UnitOfWork and commit/rollback lifecycle.
- Requestscope join views bind typed repositories to the active root UoW; they do not independently create/finalize a second root transaction.
- Local child Operations join the same root ExecutionScope/UoW and may not silently open nested transactions or escalate transaction mode.

## Lifecycle, runtime context, and identity

- Application/process resources use explicit `Startable`, `Shutdowner`, and `HealthChecker` contracts.
- Startup is deterministic and shutdown is reverse ordered.
- `core.App.Health(ctx)` is the transport-neutral source of liveness/readiness truth; Gateway health, diagnostics, and local dev readiness adapt that same model.
- `context.Context` is the canonical execution boundary.
- `core/identity.Principal` is the canonical trusted caller identity.
- `Authenticated=true` may only result from server-side credential verification.
- Query/request-supplied tenant/user/role IDs are compatibility/business data and are never authorization proof.
- Request contexts/scopes are fresh per request and are never retained by singleton services.

## RPC baseline

- `contracts/proto` is the only canonical RPC protobuf source tree.
- Standard generated grpc-go clients/servers, typed registration, standard full method names, and `bufconn` tests are the only RPC runtime/test transport.
- Historical XR generation, custom invoke transport, generated memory dispatcher, string handler registries, message pools, and the old `app/cmd/rpc` generator are retired.
- App-owned gRPC connections created by the canonical Platform RPC factory install W3C Trace Context/Baggage propagation for unary and streaming calls by default; a consumer may compose observability/resilience interceptors without owning trace continuity itself.
- Remote RPC identity is never trusted merely because metadata contains caller fields.
- Production server identity is established through a server-side `CredentialVerifier` before authorization.
- Static service tokens are a bootstrap workload-identity adapter, not an end-user credential model, and require privacy/integrity-protected transport by default.

## Resilience baseline

- Resilience is transport-neutral middleware and does not belong in business services.
- Stateful policy is isolated by stable low-cardinality operation/service keys.
- Timeout budget is the outer call budget; retries share the same parent budget.
- Retry requires explicit idempotency and explicit retryable-error classification.
- Outbound RPC governance order is timeout budget -> retry -> rate limit -> load shed -> circuit breaker -> transport.
- Adaptive selector retries perform a fresh pick where appropriate; policy rejections must not poison node passive-health scoring.

## Observability baseline

- `framework/observability` is the canonical telemetry integration layer.
- Traces/metrics use OpenTelemetry standards; structured logs use `log/slog` JSON.
- Preferred SLS architecture is OTLP -> Collector for traces/metrics and JSON stdout -> LoongCollector for logs/events.
- Application code does not depend directly on Aliyun SLS APIs for business execution.
- W3C Trace Context is the canonical distributed propagation format.
- Request-scoped Operation observers may emit phase/outcome evidence into the active trace/log stream, but observers are observational only and may not participate in authorization, transaction, idempotency, or application execution decisions.
- `framework/diagnostics.TraceAnalyzer` is the vendor-neutral read-only aggregation contract for backend-provided `span`, `log`, `operation`, and `event` evidence; SLS/other telemetry query implementations remain adapters outside the framework execution path.
- Trace evidence establishes technical execution and causality, not authoritative confirmation of an external side effect; remote-effect truth remains a separate consumer/domain concern unless a future proven framework pressure establishes a generic contract.
- Identity attributes are excluded from telemetry by default and require explicit opt-in/privacy review.

## Event, Outbox, and Saga baseline

- `framework/event.Envelope` is the canonical business-event transport contract.
- Event IDs remain stable across retries and are consumer idempotency keys.
- Event delivery is at-least-once; non-idempotent consumers must deduplicate by Envelope ID.
- Canonical event publish preparation owns event causality propagation: when a child event carries only the Normalize-generated self-correlation default, it inherits the parent correlation chain and defaults `CausationID` to the consumed parent Event ID; explicit non-default causality remains caller-owned.
- Event handling starts a new trust boundary; authenticated Principal identity is not inherited implicitly across remote event delivery.
- Transactional Outbox atomicity exists only when the business write and outbox insert join the exact same local transaction.
- Transactional Outbox staging injects canonical propagation metadata before durable Envelope serialization, so delayed/restarted dispatcher workers do not depend on the originating request context remaining alive.
- MySQL claim semantics use `READ COMMITTED` plus bounded `FOR UPDATE SKIP LOCKED` acquisition and lease ownership.
- Lease reclaim, bounded retry/backoff, dead-letter semantics, and published retention are explicit.
- `framework/workflow/saga` is the explicit remote Saga/Outbox boundary and never implies distributed database transaction/2PC.

## Contract and DSL baseline

- Protobuf is the canonical writable DSL for RPC, explicit REST bindings, DTOs, Domain/Application declarations, stable Operation IDs, authentication/tenant requirements, Permission requirements, composition, and currently supported execution-policy facts.
- Runtime does not parse descriptors/comments per request to infer behavior; contract intent is compiled into deterministic derived artifacts.
- Generated contract, OpenAPI, TypeScript, OperationPlan, and AssemblyPlan artifacts are derived evidence and are never hand-edited.
- HTTP paths are emitted only from explicit bindings. An internal RPC/Operation without an external binding must not receive an invented route.
- Stable Operation ID is the canonical business/execution identity; HTTP route and gRPC full method are transport bindings only.
- Contract compatibility blocks breaking service/method/message/enum/streaming/HTTP-binding changes unless explicitly managed.

## Domain compiler responsibility baseline

- Developer-owned PO structs are the persistence-schema source.
- PB DTO, Domain Entity, and PO are separate models; matching names/fields do not imply semantic equivalence.
- Domain compiler may generate persistence-facing Entity/basic Repository CRUD/GORM record mapping and stops at the mechanical persistence boundary.
- Business invariants, state machines, use-case orchestration, complex queries, DTO/domain semantic mapping, and cross-domain business decisions remain handwritten.
- Generated Domain Entity files use the canonical entity-owned file boundary; generated framework-internal filenames must not leak as the domain model's ownership convention.
- PB-driven codegen may generate typed Application ports, static policy metadata, typed transport adapters, OperationPlan bindings, and typed capability seams; it must not synthesize default business behavior.

## Authorization and tenant-binding baseline

- `gateway/authz` is the canonical root authorization decision boundary.
- Authentication establishes trusted Principal; authorization evaluates explicit Operation/Permission policy.
- **Permission authorization is not tenant binding.** `Policy.TenantRequired` is the canonical contract fact deciding whether a trusted tenant context is mandatory.
- Principal-aware `GrantResolver` is the extension seam for IAM models that can authorize non-tenant/platform/service principals.
- Legacy tenant-only `GrantChecker` compatibility remains explicitly tenant-bound and must fail closed for non-tenant permission policies.
- Missing required authorization machinery fails closed; no permission-prefix inference, synthetic tenant, or second authorization runtime is allowed.
- Roles own Permissions, never UI Buttons. UI metadata may reference permissions but never grants backend authority.
- Stable Permission/Operation identities are independent of HTTP paths, UI IDs, and database IDs.
- Permission grants may carry opaque scope values, but Yunka does not own a universal Customer/Site/Device resource taxonomy or generic SQL data-scope DSL.
- Business domains interpret opaque scope grants and derive typed domain-specific authorized scope.
- Application/business code must not repeat role/permission evaluation owned by the execution security boundary.

## Application composition and least-authority capability baseline

- `ApplicationDeclaration.requires` declares typed Application dependencies.
- `OperationDeclaration.requires_operations` declares explicit child Operation dependencies.
- Application dependency cycles/missing capabilities fail at compile/lint time.
- Composite permission closure is statically validated and includes transitive required Operation permissions.
- Local cross-Application composition uses generated typed child capabilities; direct cross-Application repository/service-locator bypass is forbidden.
- Child capability identity is source-edge owned: `(source Application -> target Application -> required Operations)`.
- A generated source-edge capability exposes only target Operations explicitly required by that source; generators must not widen capability surfaces by unioning unrelated target Operations across sources.
- Multiple source Applications may depend on the same target Application without generated symbol ownership collisions.
- Local child Operations join the root ExecutionScope/UoW and cannot silently create nested transactions or escalate transaction semantics.
- Remote composition uses explicit Saga/Outbox semantics rather than distributed local transaction assumptions.

## Operation execution baseline

- `pkg/operationplan` is the leaf-safe immutable execution IR, `pkg/contract` is the compiler, and `framework/operation.Executor` is the sole canonical transport-neutral Operation runtime.
- Canonical REST and gRPC adapters invoke the same Executor and immutable OperationPlan.
- Runtime execution uses stable Operation ID, not route/full-method strings, for canonical identity.
- Protected Operations fail closed if the required security phase is unavailable.
- Authorization is evaluated once at the root Operation boundary.
- Root Executor owns `ExecutionScope`, transaction/UoW, and durable idempotency phase ordering.
- Generated local child capabilities use `ExecuteChildTyped` and join the active root scope.
- A canonical business Operation does not require an RPC/HTTP binding; internal Operations remain first-class canonical Operations without fake transport exposure.
- Operation-level durable idempotency uses a MySQL-backed lease/fencing model and supports atomic success staging with the local transaction.

## Application Graph and diagnostics baseline

- `pkg/applicationgraph` is the canonical deterministic graph/query model.
- Every node/edge carries declared, observed, or explicitly inferred evidence; absence of evidence remains absence of an edge.
- Contract/OperationPlan facts contribute Domain/Application/Operation/Permission/message/binding/dependency evidence.
- Runtime Health/routes/RPC/event inventory and resilience/selector snapshots may contribute observed evidence through explicit adapters.
- Trace evidence aggregation is read-only runtime evidence and does not by itself create Application Graph edges or imply complete Saga topology.
- Graph/diagnostics never infer architecture from grep, package names, method naming, or raw URL patterns.
- W07 diagnostics are read-only and must not expose credentials, payloads, caller identity, grant scopes, or secret configuration values.

## Developer workflow and runtime baseline

- The normal developer workflow is `yunka init -> yunka generate -> yunka check -> yunka dev`.
- Top-level generate/check reuse the canonical Domain/protobuf/Provider/Contract/Module/Assembly pipeline; they are not a second compiler.
- `yunka check` is read-only. Fast-feedback evidence is disposable optimization data and unsafe/missing evidence falls back to canonical validation.
- `yunka doctor` is read-only and does not auto-repair dependencies/generation/migrations.
- `yunka dev` reuses the single `pkg/devruntime` process supervisor/readiness/runtime-report model.
- Process commands, dependencies, working directories, environment inheritance, readiness endpoints, and graph ownership are explicit configuration; startup commands are never inferred from code/graph.
- One OS process may explicitly own multiple canonical Application graph nodes.
- Unix dev commands own isolated process groups so graceful shutdown/kill applies to the full owned process tree; Windows retains direct-process semantics.
- Local runtime state is secret-free local evidence, not committed source of truth.

## Framework evolution discipline

- Yunka evolves from real consumer pressure, not from abstract framework completeness goals.
- A framework change requires a concrete generic gap backed by a minimal executable failing consumer case whenever practical.
- Consumer pressure should classify blockers explicitly (for example consumer bug/model gap versus Yunka DX/compiler/runtime/authz/persistence gap) before framework mutation.
- When a framework gap is proven, preserve the failing evidence, stop consumer work at the framework boundary, implement the smallest generic fix, qualify the framework, reverse-qualify the real consumer, then resume consumer work.
- Do not introduce a second source of truth/runtime/security model as a workaround for a consumer blocker.
- Generic BPMN/workflow engines, distributed transaction/2PC, generic SQL/data-scope DSL, framework-owned business resource taxonomies, universal audit/cache policy, idempotent response replay, and expanded Saga topology semantics remain outside the default framework unless repeated real pressure proves a generic need.
- Current open/deferred/proven pressure state belongs in `docs/STATUS.md`, not in this file.
