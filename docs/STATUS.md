# Yunka Current Status

> Document class: **STATUS**  
> Authority: current framework/wave/release/pressure status  
> Live Git HEAD authority: resolve the `main` ref from Git/GitHub; it is not duplicated as a permanent fact here  
> Behavioral reconciliation baseline: `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` (qualified candidate merged unchanged by PR #119)  
> Reconciled date: 2026-09-03  
> Governance: [`DOCUMENTATION_GOVERNANCE.md`](DOCUMENTATION_GOVERNANCE.md)

## Current framework state

| Area | Current state | Evidence / disposition |
| --- | --- | --- |
| C9 Operation Contract / execution semantics | **Complete / merged** | C9.9 closed the execution/conformance baseline; later C10/C11/B12 qualifications preserve the same canonical Executor/ExecutionScope ownership |
| C10 Runtime Assembly & Framework Productization | **Complete / qualified / merged** | issue #42 records ordered C10.1-C10.5 qualification and merge; roadmap is historical |
| C11 Developer Experience Productization | **Complete / production-qualified / merged** | issue #60 records C11.1-C11.7 complete with real Biz consumer qualification; roadmap is historical |
| Post-C11 five-gap DX convergence | **Complete / qualified / merged** | PR #104 merged the canonical four-command project closure without changing compiler/runtime/security/transaction semantics |
| B12 multi-tenant Access/IAM consumer pressure | **Complete / qualified** | real Biz pressure discovered two generic Yunka gaps; both are closed and reverse-qualified against the B12 behavioral baseline `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` |
| Distributed execution trace closure | **Complete / production-qualified / merged** | issue #118 / PR #119; exact candidate `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` passed CI #418 and production #178 and was merged unchanged into `main` |
| Separately versioned infrastructure extension module | **Active delivery / qualification pending** | issue #121 introduces root `infras` module, independent `infras/vX.Y.Z` release surface, typed module-plugin boundary, and a canonical Outbox facade without changing runtime semantics |
| Active numbered Yunka framework wave | **None selected** | new framework work remains pressure-driven rather than roadmap-driven |
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

Issue #121 establishes a new distribution boundary for optional framework infrastructure capabilities without widening the core runtime.

### Module and release boundary

**State: ACTIVE DELIVERY / QUALIFICATION PENDING.**

The candidate adds root Go module `github.com/hvritual/yunka.io/infras` to the workspace and canonical repository gates. Its public releases use the independent module tag namespace `infras/vX.Y.Z`, analogous to the separately versioned `gateway` module.

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

Qualification must pass the repository CI, production, dependency, architecture, race, vulnerability, and zero-drift gates before this status can be promoted to complete/merged.

## Current pressure frontier

The active real-consumer frontier is **B13 cross-tenant delegation and delegated device access** in `hvritual/biz` issue #11.

B13 is currently **pressure / hypothesis validation**, not a proven Yunka defect. It tests whether the existing public seams can safely express:

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
