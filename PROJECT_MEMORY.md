# Project Memory

This file is the canonical **current-state** durable memory for `hvritual/yunka.io`.

It records decisions and invariants that remain active for new work. Superseded migration details remain available in Git history and `docs/waves/**`; they should not stay in active memory when they no longer describe the repository.

`AGENTS.md` is the current repository-governance authority. If a historical decision conflicts with `AGENTS.md`, follow `AGENTS.md`.

## Repository baseline

- Canonical repository: `hvritual/yunka.io`.
- Canonical URL: `https://github.com/hvritual/yunka.io`.
- Default branch: `main`.
- Repository visibility: private.
- Canonical local remote name: `origin` -> `https://github.com/hvritual/yunka.io.git`.
- GitHub account/repository access has admin, push, and pull capability.
- Local Git is preferred when a usable checkout and authorization are available.
- GitHub Connector is an explicitly authorized repository write path under `AGENTS.md`; it may create task branches, files, commits, refs, and pull requests when local Git is unavailable.
- Normal implementation work uses an isolated task branch and repository review/merge flow. Do not move or overwrite `main` directly without explicit user authorization.
- Never force-update an existing branch or discard user changes without explicit authorization.
- Environment-local credentials and tokens are secrets and are never durable project state. Do not print, inspect, copy, commit, or reconstruct them from memory.

## Mandatory task bootstrap

Every new repository task must:

1. read `AGENTS.md` completely;
2. read this file completely;
3. inspect the current repository/base/head before mutation;
4. inspect local branch/status/remotes when a local checkout exists;
5. preserve user changes and inspect relevant wave/design documentation before editing.

## Toolchain and verification baseline

- Go toolchain baseline: `1.25.13`.
- Protobuf compiler baseline: protobuf release `21.12` / `libprotoc 3.21.12`.
- Exact tool versions and checksums are locked by `tools/toolchain.env`.
- `protoc-gen-go` and `protoc-gen-go-grpc` are the only RPC generators.
- Normal GitHub Actions CI is read-only (`contents: read`) and must never auto-commit or auto-repair dependency/generated drift.
- Third-party GitHub Actions are pinned to immutable commit SHAs.
- `make toolchain-check` rejects toolchain drift rather than accepting arbitrary newer compilers.
- `make dependency-check`, architecture/security/operation/domain/DSL/RPC/contract gates, tests, race, vet, vulnerability scanning, and build are part of the supported verification path.
- `make verify` is the canonical standard repository gate.
- `make verify-production` is the canonical production gate and extends `make verify` with real MySQL integration.
- `make tidy`, contract generation, and verification must leave dependency/generated/worktree state clean; drift is a blocking signal.
- TestKit/memory fakes are deterministic test infrastructure only and never production fallback evidence.

## Workspace and dependency baseline

- The repository is a Go workspace rooted at `go.work`.
- Product modules are `pkg`, `framework`, `gateway`, and `app`; the repository also owns the narrow `compat/go-kit-kit-log` compatibility module required by the pinned SLS SDK.
- `app` uses module path `yunka.io/app`; `framework` uses `yunka.io/framework`.
- Historical monolithic `github.com/go-kit/kit v0.10.0` and workspace-wide genproto replacement are removed from the active dependency graph.
- Split genproto api/rpc modules are the supported protobuf dependencies.
- `tools/dependency-policy.json` is the durable dependency-graph guard.
- Legacy protobuf/runtime dependencies may exist only in explicitly reviewed compatibility islands; they must not spread into new framework/application code.

## Explicit application composition baseline

- `core.NewApp(AppOptions)` / `framework/kernel.New` are the explicit application construction path.
- `core.App` is the lifecycle owner for typed modules and application-scoped capabilities.
- There is no package-global default App, generic service locator, reflection-based DI container, runtime field injection, or fallback request Runtime in the active architecture.
- Lifecycle-bearing `sync.Pool` is forbidden for services, repositories, clients, connections, transactions, request contexts/scopes, or other owned resources.
- Runtime reflection may be used for ordinary data binding/serialization, not dependency discovery or business dispatch.
- The static module catalog stores immutable descriptors only; it is not a runtime registry or service locator.
- Blank-import module enablement is preserved through bounded autoload packages that may only register an immutable descriptor.
- Autoload registration may not read runtime configuration, perform I/O, create infrastructure/services, or start goroutines.
- Module startup order is derived from explicit `DependsOn` DAG facts and is deterministic.
- Modules receive only explicitly declared typed requirements/capabilities.

## Platform ownership and request scope

- `framework/platform.Provider` is the App-owned capability owner for named shared infrastructure such as DB and RPC resources.
- Shared resources are prepared once, exposed only through a module's declared capability view, and participate in deterministic Start/Health/Shutdown ownership.
- Process-level DB/RPC connection configuration, DSNs, TLS material, and factories are not exposed to business modules.
- `framework/requestscope` provides a fresh typed Scope/repository view per operation and snapshots trusted Principal/runtime metadata/trace state.
- Under the active C9.7+ execution model, the root `framework/execution.ExecutionScope` owns the Operation UnitOfWork and commit/rollback lifecycle.
- Requestscope join views bind typed repositories to the active root UoW; they do not independently begin/finalize a second root transaction.
- Local child Operations join that same root ExecutionScope/UoW and may not silently open nested transactions or escalate transaction mode.
- The App-owned DB connection pool is never owned or closed by a request scope or ExecutionScope.

## Lifecycle and health

- Application/process resources use explicit `Startable`, `Shutdowner`, and `HealthChecker` contracts.
- Startup is deterministic and shutdown is reverse ordered.
- `core.App.Health(ctx)` is the transport-neutral source of liveness/readiness state.
- Gateway health endpoints, diagnostics, and local dev readiness adapt the same health contract rather than inventing independent lifecycle semantics.

## Runtime context and identity

- `context.Context` is the canonical execution boundary.
- `core/identity.Principal` is the canonical trusted caller identity.
- `Authenticated=true` may only result from server-side credential verification.
- `core/runtimecontext.Metadata` carries low-cardinality transport/application/operation metadata.
- Gateway HTTP creates a fresh request context per request; request transport state and mutable request storage are never pooled or retained by singleton services.
- Composite local HTTP execution creates a fresh request context and inherits only trusted/cancellation context values, not mutable response/request-local state.
- Query-supplied tenant/user/role IDs are compatibility data and are never authorization proof.

## RPC baseline

- `contracts/proto` is the only canonical RPC protobuf source tree.
- Standard generated grpc-go clients/servers, typed registration, standard full method names, and `bufconn` tests are the only RPC runtime/test transport.
- The historical XR generator, custom invoke transport, generated memory dispatcher, string handler registries, message pools, and `app/cmd/rpc` generator are retired.
- W3 resilience and observability compose through standard gRPC interceptors; generated RPC files are not hand-edited.
- Remote RPC identity is never trusted merely because metadata contains caller fields.
- Production server identity is established by a server-side `CredentialVerifier` boundary before authorization.
- Static service tokens are a bootstrap workload-identity adapter, not an end-user credential model; they require private/integrity-protected transport by default.

## Resilience baseline

- Resilience is transport-neutral middleware and does not belong in business services.
- Stateful policy is isolated by stable low-cardinality operation/service keys.
- Timeout budget is the outer call budget; retries share the same parent budget.
- Retry requires explicit idempotency and explicit retryable-error classification.
- Outbound RPC governance order is timeout budget -> retry -> rate limit -> load shed -> circuit breaker -> transport.
- Adaptive selector retries perform a fresh pick where appropriate; policy rejections must not poison node passive-health scoring.
- Rate-limit/load-shed/circuit/timeout results map to explicit transport semantics rather than generic failures.

## Observability baseline

- `framework/observability` is the canonical telemetry integration layer.
- Traces/metrics use OpenTelemetry standards; structured application logs use `log/slog` JSON.
- Preferred SLS architecture is OTLP -> Collector for traces/metrics and JSON stdout -> LoongCollector for logs/events.
- Application code does not depend directly on Aliyun SLS SDK APIs for business execution.
- W3C Trace Context is the canonical distributed propagation format.
- Identity attributes are excluded from telemetry by default and require explicit opt-in/privacy review.
- SLS/collector credentials are deployment secrets and never repository/application constants.

## Event, Outbox, and Saga baseline

- `framework/event.Envelope` is the canonical business-event transport contract.
- Event IDs remain stable across retries and are the consumer idempotency key.
- Event delivery is at-least-once; non-idempotent consumers must deduplicate by Envelope ID.
- Event handling starts a new trust boundary; authenticated Principal identity is not inherited implicitly across remote event delivery.
- Transactional Outbox atomicity exists only when the business write and outbox insert join the exact same local transaction.
- MySQL Claim semantics use `READ COMMITTED` plus bounded `FOR UPDATE SKIP LOCKED` acquisition and lease ownership.
- The concurrency invariant is bounded eventual partition with zero duplicate claims and exact total ownership, not one-shot batch fullness.
- Lease reclaim, bounded retry/backoff, dead-letter semantics, and published retention are explicit.
- `framework/workflow/saga` defines the explicit remote Saga/Outbox boundary; it does not imply distributed database transaction/2PC.

## Contract and DSL baseline

- Protobuf is the canonical writable DSL for RPC, explicit REST bindings, DTOs, Domain/Application declarations, stable Operation IDs, authentication/tenant requirements, Permission requirements, composition, and explicit execution-policy facts currently supported by C9.
- Runtime does not parse protobuf descriptors/comments per request to infer behavior; contract intent is compiled into deterministic derived artifacts.
- `contracts/generated/manifest.json`, `openapi.json`, `client.ts`, and `operation-plans.json` are derived evidence and are never hand-edited.
- HTTP paths are emitted only from explicit bindings. An RPC/Operation without an external binding must not receive an invented route.
- Contract compatibility blocks breaking service/method/message/enum/streaming/HTTP-binding changes unless explicitly managed.
- Stable Operation ID is the canonical business/execution identity; HTTP route and gRPC full method are transport bindings only.

## Domain compiler responsibility baseline

- Developer-owned PO structs are the persistence-schema source.
- PB DTO, Domain Entity, and PO are separate models; matching names/fields do not imply semantic equivalence.
- Domain compiler may generate persistence-facing Entity/basic Repository CRUD/GORM record-mapping/repository CRUD implementation from the persistence declaration and then stops.
- Business invariants, state machines, use-case orchestration, complex queries, DTO/domain semantic mapping, and cross-domain business decisions remain handwritten.
- PB-driven codegen may generate typed Application Ports, static policy metadata, typed transport adapters, OperationPlan bindings, and typed capability seams; it must not synthesize default business behavior.
- One Domain may contain multiple Applications; application identity and dependencies are explicit contract facts.

## Authorization and business scope boundary

- `gateway/authz` is the canonical authorization decision boundary.
- Roles own Permissions, never UI Buttons.
- Operation/API and UI metadata may reference Permission, but UI Button/Menu data is not a backend authorization grant.
- Authentication establishes trusted Principal; authorization evaluates explicit Operation/Permission policy.
- Authorization is fail-closed when required policy/mechanism is unavailable.
- Stable Permission/Operation identities are independent of API UUID, HTTP path, Button UUID, and database IDs.
- Permission grants may carry opaque scope values, but Yunka framework does not own a universal Customer/Site/Device resource taxonomy.
- Business domains interpret opaque scope grants and derive typed domain-specific authorized scope.
- Generic SQL/data-scope predicates are not encoded into the framework PB DSL.
- Application/business code must not repeat role/permission evaluation that belongs to the execution security boundary.

## Application composition baseline

- `ApplicationDeclaration.requires` declares typed Application capability dependencies.
- Application dependency cycles and missing capabilities fail at compile/lint time.
- `OperationDeclaration.requires_operations` declares explicit child Operation dependencies.
- Composite permission closure is statically validated and must include transitive required Operation permissions.
- Local cross-Application composition uses generated typed child capabilities; direct cross-Application repository/service-locator bypass is not the framework model.
- Local child Operations join the root execution scope/UoW and cannot silently create nested transactions or escalate transaction semantics.
- Remote composition uses explicit Saga/Outbox semantics rather than distributed local transaction assumptions.

## C9 Operation Contract and unified execution baseline

- C9.1-C9.6 establish `pkg/operationplan` as the leaf-safe immutable execution IR, `pkg/contract` as the compiler, and `framework/operation` as the transport-neutral Executor.
- Canonical REST and gRPC adapters invoke the same Executor and immutable OperationPlan.
- Runtime execution uses stable Operation ID, not route/full-method strings, for canonical operation identity.
- Protected Operations fail closed if the required security phase is unavailable.
- Authorization is evaluated once at the root Operation boundary; transport adapters do not own parallel permission/tenant/guard sequencing.
- Application code contains business use-case behavior, not duplicate authorization orchestration.
- OperationPlan/Application Graph/diagnostics are deterministic derived evidence and do not infer relationships from package/method naming.

## C9.7 execution semantics baseline

- `ExecutionPolicy` currently supports explicit transaction and idempotency declarations; semantics are never inferred from HTTP verb or method name.
- The root Operation owns the `ExecutionScope` and local UnitOfWork lifecycle.
- Executor phase order is fixed: plan -> metadata -> security -> idempotency begin -> execution scope -> application -> transaction finalize -> idempotency finalize -> outcome.
- Generated local child capabilities execute with `ExecuteChildTyped` and join the active root scope.
- Child execution fails closed without an active declared root scope and may not silently escalate transaction mode.
- Saga/Outbox staging joins the active execution transaction where required.
- Operation-level durable idempotency uses a MySQL-backed lease/fencing model and supports atomic success staging with the local transaction.
- Response/result replay remains outside the current idempotency contract.
- Distributed transaction/2PC, BPMN/generic workflow, generic SQL scope DSL, audit persistence, and cache policy remain outside C9.7 unless future repeated real business pressure proves a framework-level need.

## C9.8 canonical internal Operation baseline

- A canonical business Operation does not require an RPC or HTTP transport binding.
- `ApplicationDeclaration.operations` may declare application-level internal Operations with explicit Application method/request/response facts.
- Internal Operations compile into the same canonical OperationPlan model and Application Graph evidence as externally bound Operations.
- Generated typed child capabilities may invoke internal Operations without exposing fake RPC/REST endpoints.
- Internal-only DTOs remain in canonical contract evidence but are excluded from external OpenAPI/TypeScript projections unless reachable from a real externally bound service method.
- Real `hvritual/biz` cross-Application MySQL 8.4 pressure proved the shared root ExecutionScope/UoW child-Operation mechanism without framework bypass.
- C9.8 pressure disposition: FP-C9-001 CLOSED, FP-C9-002 CLOSED, FP-C9-003 CLOSED, FP-C9-004 CLOSED for durable duplicate suppression with response replay deferred, FP-C9-005 OPEN/DEFERRED, FP-C9-006 CLOSED.

## Application Graph and diagnostics baseline

- `pkg/applicationgraph` is the canonical deterministic graph/query model.
- Every node/edge carries evidence classified as declared, observed, or explicitly inferred; absence of evidence remains absence of an edge.
- Contract/OperationPlan facts contribute Domain/Application/Operation/Permission/message/binding/dependency evidence.
- Runtime health/routes/RPC/event inventory and resilience/selector snapshots may contribute observed evidence through explicit adapters.
- Graph/diagnostics never infer architecture from grep, package names, method naming, or raw URL patterns.
- `yunka graph build|inspect|find|impact` is the developer-facing static/runtime evidence surface.
- W07 diagnostics are read-only and must not expose credentials, payloads, caller identity, grant scopes, or secret configuration values.

## Developer runtime baseline

- `yunka doctor` is read-only and does not auto-repair dependencies/generation/migrations.
- `yunka dev` executes only explicit manifest commands as argv arrays; it does not infer startup commands from code/graph.
- Process dependency DAG, working directories, environment inheritance, readiness endpoints, and graph ownership are explicit configuration.
- Readiness uses bounded probes; plain HTTP is restricted to literal loopback and remote probes require HTTPS.
- Closure mode records declared process/Application Graph ownership and safe observed runtime evidence.
- Local runtime state is secret-free local evidence, not committed source of truth.
- Child processes shut down in reverse dependency order with bounded graceful shutdown and kill fallback; no implicit restart policy is assumed.

## C9.9 baseline and conformance closure

- C9.9 is **Complete and merged to `main`** and is a fact/release qualification wave, not a framework feature wave.
- Reconciled pre-merge product baseline was `main@407193d0b53f5fdbe2aad5c4ab152aba92d61097`, tree `d2454c393e9c19379a71980154d4e354bb4fce22`.
- C9.9 absorbed the useful R0/C9.8 release-closure work from issue #36 / PR #37; those trackers are closed/superseded and C9.9 is the single durable closure record.
- Permanent `.github/workflows/production.yml` is the read-only MySQL 8.4 release gate for PRs to `main`, pushes to `main`, and manual revalidation; normal `ci` remains the deterministic non-MySQL gate.
- Native Yunka Actions jobs on the private personal repository may currently terminate before any step with `steps=null`; this is a repository-hosted-runner infrastructure limitation, not passing evidence and not a code-test failure.
- C9.9 executable substitute evidence is run `33242472137`: exact merged C9.8 product-tree reconstruction, locked Go `1.25.13` / protoc `3.21.12`, canonical contract generation, full `make verify-production` on MySQL 8.4, real Biz regeneration, internal-Operation exposure assertions, `make verify`, and MySQL pressure all passed.
- The only stale Yunka derived artifacts found by executable closure were deterministic `contracts/generated/client.ts` and `contracts/generated/openapi.json`; the exact regenerated outputs are now committed in `main` through C9.9.
- Real consumer generated truth is the qualified 21-file state. Run `33244218676` proved second-generation zero drift, final internal Operation/composition invariants, `make verify`, MySQL pressure, and pushed the qualified generated commit.
- Permanent real-consumer conformance run `33244429584` passed on the final 7-Operation C9.8/C9.9 Pressure state.
- Current Pressure truth is: FP-C9-001 CLOSED, FP-C9-002 CLOSED, FP-C9-003 CLOSED, FP-C9-004 CLOSED for durable duplicate suppression with response replay deferred, FP-C9-005 OPEN/DEFERRED, FP-C9-006 CLOSED.
- C9.8 issue #34 and C9.9 issue #38 are closed. PR #39 is the durable C9.9 closure/evidence record; non-Draft PR #40 carried the exact same qualified head to `main` because the Connector Ready-for-review GraphQL wrapper could not transition #39 out of Draft.
- C9.9 main merge commit is `e091baff2730e04b402710606f655b3a9cd630b7`.
- No new runtime mechanism was added by C9.9; C10 or any new framework surface starts only as a separate pressure-driven wave after this closure.
- The current private personal repository tier does not provide usable GitHub rulesets/required-check enforcement; do not represent `main` as platform-protected. Enforce release policy through durable `ci`/`production`, review/merge discipline, and explicit release rules.

## C10 Runtime Assembly & Framework Productization roadmap

- C10 is the formally selected next framework wave after C9.9 and is tracked by GitHub issue #42 and `docs/waves/C10-runtime-assembly-framework-productization.md`.
- C10's objective is productization/assembly: allow developers to focus on PO, protobuf Contract, handwritten Application logic, and Operation intent while deterministic typed generation assembles framework plumbing.
- C10 is strictly ordered for **qualification and merge**: **C10.1 Assembly Contract -> C10.2 Compiler -> C10.3 Runtime Closure -> C10.4 Consumer Upgrade -> C10.5 Release Qualification**. A later subwave must not be marked Complete, merged into the qualified carrier, or used to qualify a predecessor.
- When the user explicitly directs work to continue, a later C10 subwave may begin as a **stacked implementation branch/PR** from the current predecessor head even if predecessor executable verification is temporarily blocked. Stacked work remains Draft/unqualified until every predecessor Gate is closed; defects in predecessor fact ownership discovered by stacked work must be fixed in the predecessor contract rather than hidden in the later implementation.
- C10.1 defines explicit assembly fact ownership and a deterministic AssemblyPlan/IR; it must not create a duplicate writable DSL for facts already owned by protobuf, project config, module descriptors, or dev runtime.
- C10.2 generates structural typed Go bootstrap/wiring only. It may generate composition, typed binding seams, and explicit REST/gRPC registration glue, but never business behavior, reflection DI, service lookup, or a parallel execution runtime.
- C10.3 closes generated assembly through the existing `core.App`, Platform Provider, C9 Executor/ExecutionScope, Health, Diagnostics, Application Graph, and `yunka dev` readiness/closure. Existing runtime owners remain authoritative.
- C10.4 uses real `hvritual/biz` as a consumer-pressure wave. Framework changes require a concrete generic assembly defect/seam; Biz-specific behavior remains in Biz and speculative framework pressure is deferred.
- C10.5 is a release qualification/fact-reconciliation wave, not a feature wave. It requires exact-tree deterministic generation, framework `make verify-production` on MySQL 8.4, real-consumer regenerate/zero-drift/verify/pressure, and code/generated/docs/memory/Pressure truth reconciliation before C10 is Complete.
- C10 does not introduce generic BPMN/workflow, distributed transaction/2PC, generic SQL/data-scope DSL, framework-owned business resource taxonomy, audit/cache policy, response replay, Saga topology expansion, or business-semantic inference unless an executable blocker proves the smallest separately reviewed mechanism.
- Normal C10 implementation follows isolated task branches and PR review/merge. Do not move `main` directly as part of implementation work.
