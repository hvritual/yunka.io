# Yunka Current Status

> Document class: **STATUS**  
> Authority: current framework/wave/release/pressure status  
> Live Git HEAD authority: resolve the `main` ref from Git/GitHub; it is not duplicated as a permanent fact here  
> Behavioral reconciliation baseline: `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` (qualified candidate merged unchanged by PR #119)  
> Reconciled date: 2026-09-04  
> Governance: [`DOCUMENTATION_GOVERNANCE.md`](DOCUMENTATION_GOVERNANCE.md)

## Current framework state

| Area | Current state | Evidence / disposition |
| --- | --- | --- |
| C9 Operation Contract / execution semantics | **Complete / merged** | C9.9 closed the execution/conformance baseline; later C10/C11/B12 qualifications preserve the same canonical Executor/ExecutionScope ownership |
| C10 Runtime Assembly & Framework Productization | **Complete / qualified / merged** | issue #42 records ordered C10.1-C10.5 qualification and merge; roadmap is historical |
| C11 Developer Experience Productization | **Complete / production-qualified / merged** | issue #60 records C11.1-C11.7 complete with real Biz consumer qualification; roadmap is historical |
| Post-C11 five-gap DX convergence | **Complete / qualified / merged** | PR #104 merged the canonical four-command project closure without changing compiler/runtime/security/transaction semantics |
| Agent Experience control-plane convergence | **AX1-AX7 complete / production-qualified / pressure-qualified / merged** | AX1-AX6 merged through PRs #125, #126, #127, #129, #132, #133. AX7 strict pressure candidate `38036a66e0b264a87526f18aaa8494bea6355a28` proved a handwritten-placement escape in CI #489; minimal closure candidate `45526e6f79f5e9b430f30f641080738e45c5b72a` passed CI #491 and production #249; final PR head `d9038f37f467497dd36c7a45cfc6170b19922994` passed CI #493 and production #251 and merged through PR #138 as `67d8b99e641c4c3179dc31941927631f3d30b7fd` |
| Framework Terminalization T0-T2 Audit foundation | **Complete / production-qualified / real-consumer pressure-qualified / merged** | PR #141. Final behavioral candidate `3348c7aaf613e6443b295d746f8bc659477f9c22` passed CI #506 and production #264. Biz B13 reverse qualification run `33777462988` proved clean-consumer zero findings and exact detection of `AUDIT-APP-001`, `AUDIT-AUTH-001`, `AUDIT-INFRA-001` under temporary adversarial pressure. Final docs-only PR head `df87a9fd29fb99e7957106301879686195c7c786` passed CI #507 / run `33778300974` and production #265 / run `33778301062`, then merged as `d93fcc4b9c85840c95d9dcc310e34bfe46349450`. |
| B12 multi-tenant Access/IAM consumer pressure | **Complete / qualified** | real Biz pressure discovered two generic Yunka gaps; both are closed and reverse-qualified against the B12 behavioral baseline `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` |
| Distributed execution trace closure | **Complete / production-qualified / merged** | issue #118 / PR #119; exact candidate `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` passed CI #418 and production #178 and was merged unchanged into `main` |
| Separately versioned infrastructure extension module | **Complete / production-qualified / merged** | issue #121 / PR #122; exact candidate `fc09f296cccb14ae18891bc43642d5efe43bd484` passed CI #430 and production #190, then merged as `70611d6cee5dd4e37ae6a803bcb38b938acd59c9`; independent `infras/vX.Y.Z` tag surface exists, but no `infras/v0.1.0` release tag is claimed yet |
| Typed infrastructure capability export / binding | **Complete / production-qualified / merged** | issue #124 / PR #131; exact candidate `c3123fb4a3e7731f0edf5539c3d8003fc0e41bc7` passed CI #475 and production #233 after synchronization with AX6 main, merged as `d06a6330db9093e0bc586decb6bdc00122b4aa99`, and exact-main push CI #476 / production #234 also passed |
| Active numbered Yunka framework wave | **None selected** | new framework work remains pressure-driven rather than roadmap-driven; AX control-plane work does not by itself create a numbered framework wave |
| Proven open Yunka P0/P1 runtime/compiler/authz/persistence/trace-closure defects | **0 known at reconciliation** | do not promote hypotheses into framework defects without executable consumer evidence |

## Current developer workflow

The correct-by-default path is:

```text
yunka init
    ↓
define developer-owned PO + protobuf + Application logic
    ↓
yunka generate
    ↓
yunka check
    ↓
yunka dev
    ↓
Ready / Health / Diagnostics / Graph / Runtime Closure evidence
```

Current top-level generation/check owns the canonical managed project closure: Domain generation/check, standard protobuf Go/gRPC generation/check, typed Provider preflight, Contract, Module, Assembly, and read-only drift validation. `yunka dev` reuses the canonical DevRuntime/readiness/runtime-closure model.

## Agent Experience control-plane convergence

AX1-AX7 are closed, production-qualified, and merged. They productize an AI/automation-facing control plane around the existing canonical project/compiler/runtime contracts rather than introducing a second compiler, business DSL, ownership manifest, or runtime.

### AX1 — Context Contract

**State: COMPLETE / QUALIFIED / MERGED.**

`yunka context --json` exposes the canonical project descriptor, key source/generated locations, file evidence, and next-step commands through the same project resolution used by `generate/check`. It is read-only and does not persist another Source of Truth.

Evidence: PR #125.

### AX2 — Derived Ownership Guard

**State: COMPLETE / QUALIFIED / MERGED.**

`yunka ownership inspect|check` derives `editable`, `generated-only`, and `unclassified` mutation decisions from canonical project/profile, contract-source, protobuf-output, Domain-generator, and generated-marker facts. Declarative modules are writable only at `modules/<name>/module.yunka.json`; arbitrary module runtime files remain unclassified.

Evidence: PR #126.

### AX3 — Agent Diagnostic Contract

**State: COMPLETE / QUALIFIED / MERGED.**

`check`, `generate`, and `doctor` support opt-in `agent-json` projections with stable diagnostic identity plus `cause`, `target`, `remediation`, and `retry` fields. Existing human text/JSON contracts remain compatible.

Evidence: PR #127.

### AX4 — Evidence-backed Change Planning

**State: COMPLETE / QUALIFIED / MERGED.**

`yunka change plan` resolves only existing canonical Operations, rebuilds static impact from canonical Contract + OperationPlan facts in memory, integrates AX2 ownership for exact editable targets, emits unresolved targets when evidence cannot uniquely locate handwritten source, and reports generated effects, canonical risk facts, and verification gates without inferring business semantics.

Evidence: PR #129.

### AX5 — Explicit Structural Scaffolds

**State: COMPLETE / PRODUCTION-QUALIFIED / MERGED.**

`yunka add` now provides explicit structural authoring for Application, Operation, event DTO, and declarative module scaffolds. Operation access, tenant binding, permissions/authentication, transaction, idempotency, composition, dependencies, and optional HTTP binding come only from explicit caller input. The scaffold creates developer-owned contract structure and a handwritten implementation landing file with TODO guidance; it does not generate business logic, persistence behavior, Saga/Outbox policy, event publication, external-effect semantics, or a second runtime path.

AX5 exact candidate `c3fbfb425aa910b2bbbf0428d6bef543ca5fa7de` passed CI run `33739090019` including full Verify and determinism, production run `33739090079` on MySQL 8.4 with clean-worktree, and canonical scaffold compile qualification through Contract compile/lint/artifact rendering/application codegen. PR #132 merged it as `54093423d9d838c9f0f9f24ca589e54f66963746`.

### AX6 — Runtime Event Stream

**State: COMPLETE / PRODUCTION-QUALIFIED / MERGED.**

`yunka dev [run] --event-format jsonl` exposes a stable machine-readable projection of the existing dev plan and canonical RuntimeReport while reserving stdout for JSONL events and keeping child process output off the machine channel. `yunka dev status --format jsonl` provides the finite snapshot form. Runtime/application state, diagnostics, and closure-complete evidence remain projections of existing runtime truth; AX6 does not add a second supervisor, readiness model, Executor path, security model, or persistence format.

Evidence: PR #133; exact candidate `7538b8279ef16adf19cd258a837fa0a13f98042f`; CI run `33740395756` PASS; production run `33740395812` PASS on MySQL 8.4 with clean-worktree; merged as `ee0f1097fa6eaed900a92cd9563cca31dbfedab1`.

### AX7 — Bounded Change Contract / Conformance

**State: COMPLETE / PRODUCTION-QUALIFIED / PRESSURE-QUALIFIED / MERGED.**

PR #138 adds a transient, Git-baselined Change Contract around existing canonical Operations. `yunka change begin` records only target identity, allowed semantic categories, and derived editable/generated boundaries; it does not copy Manifest/OperationPlan/Application Graph facts into another Source of Truth. `yunka change check` reconciles tracked and untracked Git delta against those bounds plus AX2 ownership. Broad generated-impact scopes do not authorize handwritten mutations unless AX2 independently classifies the concrete path as `generated-only`.

`yunka change verify` composes Git scope/ownership reconciliation, canonical full `yunka check`, normalized base/current OperationPlan + target Application semantic reconciliation, and Go tests when applicable. It emits `.yunka/change-attestation.json`. Target permission, tenant binding, authentication/public access, transaction, idempotency, composition, dependency, capability, transport, and contract changes must be explicitly allowed; semantic changes to unrelated Operations/Applications are always rejected as out of scope.

AX7.1-AX7.4 exact candidate `1adb35bfc3840beb3480a11e71d0e2bfc0ec24af` passed CI #482 / run `33747031578` including full Verify and determinism, and production #240 / run `33747031547` on MySQL 8.4 including clean-worktree verification.

AX7.5 then deliberately pressure-tested scope, generated-file tampering, tenant/permission/transaction drift, capability drift, unrelated semantic changes, and same-Application code organization. Strict pressure candidate `38036a66e0b264a87526f18aaa8494bea6355a28` failed CI #489 exactly because `internal/tenant/application/global_helper.go` was accepted as `developer-code/editable` with no violation. This proved that a broad Application scope plus AX2 ownership did not constrain newly introduced handwritten architectural surface.

The promoted closure is deliberately smaller than an Architecture Delta analyzer: existing handwritten M/D changes remain eligible inside the declared Application scope, while A/R/C handwritten destinations require an exact Change Contract `EditablePaths` declaration before mutation and still pass AX2 ownership. Explicit exact new paths remain supported. Final behavioral candidate `45526e6f79f5e9b430f30f641080738e45c5b72a` passed CI #491 / run `33749854560` including full Verify and determinism, and production #249 / run `33749854762` on MySQL 8.4 including clean-worktree verification.

Final PR head `d9038f37f467497dd36c7a45cfc6170b19922994` changed only `docs/STATUS.md` and `docs/waves/AX7-change-contract.md` after the behavioral candidate, passed CI #493 / run `33750319832` plus production #251 / run `33750319803`, and merged through PR #138 as `67d8b99e641c4c3179dc31941927631f3d30b7fd`.

No AST/SSA/DDD analyzer or repository-wide Architecture Delta engine was introduced because pressure did not justify one.

The merged Agent change flow for an existing Operation is now:

```text
yunka context --json
    ↓
yunka change plan --operation <id> --format json
    ↓
yunka change begin --operation <id> [--intent ...] [--path ...] [--allow-semantic ...]
    ↓
Agent edits only declared developer-owned targets
    ↓
yunka change check
    ↓
yunka generate
    ↓
yunka change verify --format agent-json
    ↓
yunka dev --event-format jsonl   # when runtime qualification is required
```

New structure still begins with explicit `yunka add ...`; AX7 does not infer undeclared Operations or business semantics.

## B12 framework-pressure disposition

B12 is closed. It proved and then closed two generic Yunka defects rather than working around them in the consumer.

### B12-FP-001 — authorization incorrectly coupled to tenant binding

**State: CLOSED.**

Real failing behavior: a protected platform Operation with `tenant_required=false` and a tenantless Principal was denied as `tenant_required` even though the generated contract was correct.

Durable architecture after the fix:

```text
Permission Authorization != Tenant Binding
```

`Policy.TenantRequired` is the tenant-binding fact. Principal-aware `GrantResolver` supports non-tenant/platform/service authority models; the legacy tenant-only GrantChecker compatibility seam fails closed for non-tenant permission policies. No synthetic tenant, permission-prefix inference, PB taxonomy expansion, or second authorization runtime was introduced.

Evidence: Yunka issue #106; qualified integration PR #108; real Biz reverse qualification.

### B12-FP-006 — child-capability codegen ownership collision

**State: CLOSED.**

Real failing topology:

```text
Application A -> Application C
Application B -> Application C
```

The previous generator emitted colliding target-owned child-capability symbols in one Go package. The qualified A+ fix changed generated capability identity to the source edge and required Operation subset:

```text
(source Application -> target Application -> required Operations)
```

This removes symbol collisions and prevents capability widening by unioning unrelated target Operations. PB DSL, OperationPlan, AssemblyPlan, Executor, authorization, and root UoW semantics were not expanded.

Evidence: Yunka issue #110; qualified integration PR #112; behavioral framework baseline `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`; real Biz reverse qualification.

## Distributed execution trace closure

Issue #118 / PR #119 closed the previously observed gaps between available telemetry primitives and a correct-by-default cross-system trace chain.

### P1 — canonical RPC propagation

**State: CLOSED / MERGED.**

App-owned gRPC connections created through the canonical Platform RPC factory now install W3C Trace Context/Baggage propagation for unary and streaming calls. Observability client interceptors inject the client-span context at the transport boundary, so a Yunka consumer does not need to remember a separate propagation interceptor merely to preserve trace continuity.

### P1 — durable Outbox/Event trace and causality

**State: CLOSED / MERGED.**

Event publication has one canonical preparation boundary. A child event emitted from an event-consumer context inherits the parent correlation chain and defaults `CausationID` to the parent Event ID unless the caller supplied an explicit non-default value. Transactional Outbox staging injects propagation metadata before durable serialization, so delayed/restarted dispatcher workers do not depend on the original request context still existing.

Authenticated Principal identity is still not inherited across event trust boundaries.

### P2 — TraceID evidence analysis

**State: CLOSED / MERGED.**

Operation phase/outcome observations and Outbox lifecycle observations enter the same trace-correlated telemetry stream through request-scoped observers. `framework/diagnostics.TraceAnalyzer` is the vendor-neutral read-only aggregation contract for `span`, `log`, `operation`, and `event` evidence supplied by backend adapters.

This does **not** make a telemetry trace an authoritative external-effect receipt: a trace can establish technical execution and causality, while external side-effect confirmation still requires whatever authoritative readback/reconciliation contract the consuming domain needs.

Evidence: exact candidate `19bed965852d9dc2ef39e91dcadd7fb6bea4c871`; CI #418 success; production #178 success on MySQL 8.4; merge PR #119.

## Infrastructure extension module delivery

Issue #121 / PR #122 establishes the merged distribution boundary for optional framework infrastructure capabilities without widening the core runtime.

### Module and release boundary

**State: COMPLETE / PRODUCTION-QUALIFIED / MERGED.**

The merged delivery adds root Go module `github.com/hvritual/yunka.io/infras` to the workspace and canonical repository gates. Its public releases use the independent module tag namespace `infras/vX.Y.Z`, analogous to the separately versioned `gateway` module.

Dependency direction is intentionally one-way:

```text
application
   ↓
infras plugin
   ↓
framework stable contracts/runtime
```

`framework` is forbidden from importing `infras`. Existing `framework/infras/**` packages remain compatibility/internal surfaces in this delivery; no bulk migration is claimed.

### Plugin boundary

Infrastructure plugins reuse `framework/core/modulecatalog`. Programs may register a descriptor explicitly or enable a plugin with a descriptor-only blank import. The delivery does not introduce Go runtime `plugin`/`.so` loading or a second module/runtime registry.

The initial `infras/modules/outboxruntime` plugin is a facade over the canonical `framework/modules/outboxruntime` descriptor and implementation. This establishes the separately versioned distribution surface while preserving one Outbox module identity, one config namespace, and one transaction/event runtime.

Evidence: exact candidate `fc09f296cccb14ae18891bc43642d5efe43bd484`; CI #430 success including dependency drift, contract reproducibility/compatibility, full `make verify`, and determinism recheck; production #190 success on MySQL 8.4 with clean-worktree; merge PR #122 as `70611d6cee5dd4e37ae6a803bcb38b938acd59c9`.

The independent release/tag mechanism is now part of the repository contract, but this status does **not** claim that an `infras/v0.1.0` tag or release has already been published. Publication is a separate release action.

## Typed infrastructure capability binding

Issue #124 / PR #131 closes the typed provider-to-Application constructor path required by separately versioned infrastructure plugins.

**State: COMPLETE / PRODUCTION-QUALIFIED / MERGED.**

A module descriptor may declare `Descriptor.Provides` entries identified by a logical capability name plus Go package/type identity. A built module that provides values implements `modulecatalog.CapabilityExporter`; descriptor promises and runtime exports must match exactly. Duplicate providers, undeclared exports, missing exports, contract mismatches, and non-assignable values fail before App start. Capability values remain owned by the provider module/App lifecycle.

Application protobuf declares `ApplicationDeclaration.capabilities` separately from cross-Application `requires` and Operation `requires_operations`. The compiler carries capability identity into Manifest/AssemblyPlan and generated Application dependency structs. Generated Assembly resolves the typed value only during Application construction and passes a normal Go interface through generated `*Dependencies`; business runtime code does not retain `CapabilitySet` and there is no `app.Get(string)`, reflection DI, or public untyped service locator. Separately versioned plugin descriptors are explicit `AdditionalModules` composition inputs.

The current declarative module authoring surface (`module.yunka.json` / `yunka module add`) still models module input `Requirements` only and does not synthesize `Descriptor.Provides`. An exporting plugin therefore composes its capability-providing descriptor explicitly in handwritten Go (without editing generated module source) and passes that descriptor to generated Assembly. This is an authoring/DX limitation, not a runtime/compiler binding gap.

Evidence: issue #124; exact candidate `c3123fb4a3e7731f0edf5539c3d8003fc0e41bc7`; CI #475 and production #233 success; merge PR #131 as `d06a6330db9093e0bc586decb6bdc00122b4aa99`; exact-main push CI #476 and production #234 success.

## Framework Terminalization T0-T2

T0-T2 are **complete, production-qualified, real-consumer pressure-qualified, and merged through PR #141**. They add a deterministic architecture-evidence layer above the existing AX1-AX7 control plane without introducing a second compiler, runtime, security model, architecture Source of Truth, automatic remediation path, or LLM dependency.

### T0 — Agent protocol reconciliation

`yunka context --json` schema v2 makes the merged Agent protocol self-describing: explicit new-structure authoring, bounded existing-Operation changes, deterministic Audit, Agent JSON diagnostics, and JSONL runtime evidence all point at one control-plane flow.

### T1 — Deterministic Audit MVP

`yunka audit` is a read-only app/control-plane command. It derives evidence from canonical project/Manifest facts plus Go parser direct-import evidence and deliberately supports only narrow proven structural violations. It does not infer business correctness, DDD quality, architectural intent, or design hypotheses.

The first proven rules are `AUDIT-APP-001`, `AUDIT-INFRA-001`, and `AUDIT-AUTH-001`. Real Biz pressure proved and closed the exact consumer module-identity compatibility needed for `yunka.io/framework/platform` and `yunka.io/gateway/authz` without widening the matcher to arbitrary suffixes.

### T2 — Architecture Debt Delta

`yunka audit --base <git-ref>` compares stable proven Finding IDs against an immutable Git baseline as `existing / new / fixed`. Baseline source/Manifest facts are read from Git objects without checkout, historical observations are not turned into blocking debt, and module-identity migration fails closed.

Exact evidence is recorded in `docs/waves/TERMINALIZATION-t0-t2-audit.md`. Final behavioral candidate `3348c7aaf613e6443b295d746f8bc659477f9c22` passed CI #506 and production #264. Biz reverse qualification run `33777462988` returned zero clean findings and then detected all three expected new-debt rules under a temporary real-form adversarial fixture before restoring clean Git status. Final docs-only head `df87a9fd29fb99e7957106301879686195c7c786` passed CI #507 and production #265 and was integrated through PR #141 as `d93fcc4b9c85840c95d9dcc310e34bfe46349450`.

The next terminalization stage is **T3 — AI Architecture Advisor Evidence Contract**. T3 remains outside the Yunka runtime and consumes deterministic Audit evidence only. An advisor output is recommendation/hypothesis evidence, not executable truth and not authority to mutate code, contracts, runtime/security boundaries, or business/domain design without the existing bounded change protocol and human approval where authority widens.

## Current pressure frontier

The active real-consumer frontier is **B13 cross-tenant delegation and delegated device access** in `hvritual/biz` issue #11.

AX7.5 adversarial change-conformance pressure is **closed and merged through PR #138**. It proved one generic handwritten-placement escape and qualified the minimal exact-path closure described above; it is no longer an active pressure stream.

B13 remains real-consumer runtime/domain pressure and is not a proven Yunka defect. It tests whether the existing public seams can safely express:

```text
actor tenant != resource-owner tenant

local actor authority
  ∩ active owner->grantee delegation
  ∩ delegated resource/permission scope
  = effective access
```

No Yunka primitive should be added preemptively. If the current APIs cannot express this safely, the consumer must first preserve a minimal failing case, classify the generic gap, stop at the framework boundary, and only then open a Yunka change.

## Known deferred limitations

These are explicit limitations/non-goals, not current release blockers:

- `FP-C9-005` — Saga step/topology evidence is not represented as a complete Application Graph/Diagnostics topology: **OPEN / DEFERRED**. PR #119 adds Trace/Event/Operation evidence correlation but does not claim full Saga topology representation in Application Graph.
- Durable Operation idempotency provides duplicate-execution suppression; response/result replay remains outside the current contract unless future real pressure justifies it.
- Declarative module specs currently author module input `Requirements` but do not synthesize typed capability `Descriptor.Provides`; exporting infra plugins use explicit handwritten descriptor composition until real DX pressure justifies extending module-spec authoring.

A deferred limitation does not become an active framework wave merely because it is listed here.

## Status authority rules

- Use this file for answers to **what is currently complete, active, qualified, deferred, or under pressure**.
- Resolve the live `main` HEAD from Git/GitHub rather than copying it into durable memory or treating a historical SHA in this document as the live ref.
- Use `PROJECT_MEMORY.md` for durable architecture/governance invariants.
- Use `README.md` and current authoring guides for current developer-facing behavior.
- Use exact qualification/release records for exact SHA/tree/consumer evidence.
- Treat `HISTORICAL` roadmap status blocks in `docs/waves/**` as preserved planning snapshots, not current status truth.
- Do not copy current HEAD, repository visibility, active PR/task state, or wave status into `PROJECT_MEMORY.md`.
- Reconcile this file whenever framework/wave/pressure semantic status changes. A documentation-only Git commit does not require rewriting the behavioral reconciliation baseline merely because the live HEAD SHA changed.
