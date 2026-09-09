# Yunka Current Status

> Document class: **STATUS**  
> Authority: current framework/wave/release/pressure status  
> Live Git HEAD authority: resolve the `main` ref from Git/GitHub; it is not duplicated as a permanent fact here  
> Behavioral reconciliation baseline: `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` (qualified candidate merged unchanged by PR #119)  
> Reconciled date: 2026-09-09
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
| Framework Terminalization T3 Advisor evidence contract | **Complete / production-qualified / real-consumer pressure-qualified / merged** | PR #143. T3.1 candidate `01260e60b0043c726aecd5808bd352e41ef8fb94` passed CI #512 and production #270; T3.2 behavioral candidate `5738ac71ef5e838fae9231e908ae1a87ee7e2b31` passed CI #515 and production #273. Biz B13 adversarial qualification run `33815375770` proved schema-v3 discoverability, valid evidence-bound `advisory_only` attestation, and fail-closed rejection of unknown Finding, `safe_to_merge`, digest drift, `apply_patch`, and zero-evidence advice. Final PR head `b1ff3515be70769f6a9aa7f1c6e95f0d8348467a` passed CI #517 / run `33815799648` and production #275 / run `33815799615`, then merged as `63aa9881f84aa3f1a53fa1a1f46b999ddbbdde83`. |
| Framework Terminalization T4 ChangeSet + remediation convergence | **Complete / production-qualified / real-consumer-qualified / merged** | PR #145 merged T4.1/T4.2 candidate `07a1210d06020a01380b5d51f5812408e6e553c2` as `15f19ff0845e421039ffb675937bf23c3bb8a79d`. PR #146 post-review behavior candidate `a584da287d52271a4a05ad80167f89fdbe0d3ee0` passed CI #563 / run `33842121802` and production #321 / run `33842121900`; Biz replay run `33842351609` passed with artifact `9925335952`. Final PR head `8cc3967a1b2baaf5eecbbb22fa21d65eb2c80c2b` passed CI #565 / run `33844574960` and production #323 / run `33844574953`, merged as `ba896a8776987e91d2525f6bb15a188653857980`, and post-merge main CI `33844917226` plus production `33844917372` both passed. Exact evidence: `docs/waves/TERMINALIZATION-t4-changeset-remediation.md`. |
| Framework Terminalization T5.1 Proof-of-Change | **Complete / production-qualified / real-consumer pressure-qualified / merged** | PR #148. Rebased behavioral candidate `a102de3c53f1078c8c6cce70e334c91fbbb94bb2` passed CI #580 / run `33954262081` and production #338 / run `33954262031`. Biz B13 qualification run `33954431712` passed against pressure SHA `506e9c117822855db318f8b4b6689d318a62ded1`; artifact `9965889476`, digest `sha256:8070e9876fc5395cf5be4ca82539feceb31b1fb69482c8ea148d0169260ce520`, proves new/existing/fixed architecture-debt states plus deterministic attestation and clean final worktree. Final pre-integration head `0a742d0cff0b634b5d80660fee2a77441bfdc1e1` passed CI #582 / run `33955577654` and production #340 / run `33955577637`, then merged through PR #148 as `2f588ed2e5f40b06c6ce3b7f5803d1c51afceefc`; exact-main CI #586 / run `33955864123` and production #344 / run `33955864115` both passed. Exact evidence: `docs/waves/TERMINALIZATION-t5-proof-of-change.md`. |
| B12 multi-tenant Access/IAM consumer pressure | **Complete / qualified** | real Biz pressure discovered two generic Yunka gaps; both are closed and reverse-qualified against the B12 behavioral baseline `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` |
| Distributed execution trace closure | **Complete / production-qualified / merged** | issue #118 / PR #119; exact candidate `19bed965852d9dc2ef39e91dcadd7fb6bea4c871` passed CI #418 and production #178 and was merged unchanged into `main` |
| Separately versioned infrastructure extension module | **Complete / production-qualified / merged** | issue #121 / PR #122; exact candidate `fc09f296cccb14ae18891bc43642d5efe43bd484` passed CI #430 and production #190, then merged as `70611d6cee5dd4e37ae6a803bcb38b938acd59c9`; independent `infras/vX.Y.Z` tag surface exists, but no `infras/v0.1.0` release tag is claimed yet |
| Typed infrastructure capability export / binding | **Complete / production-qualified / merged** | issue #124 / PR #131; exact candidate `c3123fb4a3e7731f0edf5539c3d8003fc0e41bc7` passed CI #475 and production #233 after synchronization with AX6 main, merged as `d06a6330db9093e0bc586decb6bdc00122b4aa99`, and exact-main push CI #476 / production #234 also passed |
| Active numbered Yunka framework wave | **None selected** | new framework work remains pressure-driven rather than roadmap-driven; AX/Terminalization control-plane work does not by itself create a numbered framework wave |
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

`yunka change verify` composes Git scope/ownership reconciliation, canonical full `yunka check`, normalized base/current OperationPlan + target Application semantic reconciliation, and Go tests when applicable. It emits `.git/yunka/change-attestation.json`. Target permission, tenant binding, authentication/public access, transaction, idempotency, composition, dependency, capability, transport, and contract changes must be explicitly allowed; semantic changes to unrelated Operations/Applications are always rejected as out of scope.

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

AX7 remains the single-existing-Operation protocol. New-Operation and multi-subject control is extended by Terminalization T4 below; AX7 itself still does not infer undeclared Operations or business semantics.

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

## Framework Terminalization T3

T3 is **complete, production-qualified, real-consumer pressure-qualified, and merged through PR #143**. It adds an evidence-bound advisory protocol above deterministic Audit without model execution, automatic remediation, mutation authority, merge authority, or a second architecture Source of Truth.

### T3.1 — Advisor evidence contract

`yunka advisor request [--base <git-ref>] --format agent-json` reuses canonical Audit evidence and binds the normalized report with SHA-256 Audit/request digests. `yunka advisor validate --request <request.json> --response <response.json> --format agent-json` strictly validates an external response against that exact request.

The request authority is fixed to `advisory_only`; mutation is not authorized and any authority expansion requires a human decision. Allowed recommendations are limited to remediation recommendations, investigation hypotheses, and design questions. Every recommendation must bind to current Audit Finding IDs, and remediation may bind only to proven violations. Unknown fields, unknown findings, unsupported kinds, duplicate bindings, digest drift, or non-empty advice without deterministic findings fail closed.

T3.1 exact candidate `01260e60b0043c726aecd5808bd352e41ef8fb94` passed CI #512 / run `33780256579` and production #270 / run `33780256560`.

### T3.2 — Agent Context protocol v3

`yunka context --json` is explicitly upgraded to schema v3 and exposes the qualified `advisor request` and `advisor validate` machine protocol. T3.2 changes discoverability only; it does not widen Advisor authority.

T3.2 behavioral candidate `5738ac71ef5e838fae9231e908ae1a87ee7e2b31` passed CI #515 / run `33815046944` and production #273 / run `33815046909`.

### T3.3 — Real Biz adversarial qualification

Biz B13 pressure tree `506e9c117822855db318f8b4b6689d318a62ded1` was qualified against exact Yunka `5738ac71ef5e838fae9231e908ae1a87ee7e2b31` in run `33815375770` with evidence artifact `9916336880`. Clean B13 produced zero findings and accepted only an empty advisory response. A temporary real-form pressure fixture produced exactly `AUDIT-APP-001`, `AUDIT-AUTH-001`, and `AUDIT-INFRA-001`; a response bound to those exact findings validated only as `advisory_only`.

The same real-consumer run proved fail-closed rejection of an unknown Finding ID, injected `authority: safe_to_merge`, request-digest drift, unsupported `apply_patch`, and non-empty advice against a zero-finding request. The temporary fixture was removed and Biz Git status was reconciled to its pre-qualification state.

Final PR head `b1ff3515be70769f6a9aa7f1c6e95f0d8348467a` passed CI #517 / run `33815799648` and production #275 / run `33815799615`, then was integrated through PR #143 as `63aa9881f84aa3f1a53fa1a1f46b999ddbbdde83`. Exact evidence is recorded in `docs/waves/TERMINALIZATION-t3-advisor.md`. T3 is closed. T4 is complete and merged through PRs #145 and #146; T5.1 Proof-of-Change is complete, production-qualified, real-consumer pressure-qualified, and merged through PR #148.

## Framework Terminalization T4

T4 is **complete, production-qualified, real-consumer-qualified, and merged through PRs #145 and #146**.

T4 extends the AX/T0-T3 control plane; it does not introduce a second runtime, compiler, authorization path, persistence layer, architecture Source of Truth, or LLM execution path.

### T4.1 — plan before mutation

`yunka add operation ... --plan` is the read-only prospective form of new-Operation authoring. It uses the same preparation path as apply and records exact prospective writable/generated effects plus normalized explicit Operation semantics before source mutation.

The current new-Operation flow is therefore plan-first rather than "write first, inspect later".

### T4.2 — ChangeSet v2

`yunka change set begin|check` composes existing AX7 Change Contract v1 subjects and create-Operation plans on one immutable Git baseline. The logical default transient path remains `.git/yunka/change-set.json`; physical Git-private storage is resolved through Git's actual directory so linked worktrees and submodules do not require `.git` to be a directory.

Set-wide reconciliation validates actual Git delta, AX2 ownership, exact generated boundaries, and canonical semantic readback without treating another declared subject in the same ChangeSet as unrelated drift. The ChangeSet does not replace protobuf, Manifest, OperationPlan, or Application Graph as a Source of Truth.

T4.1/T4.2 exact candidate `07a1210d06020a01380b5d51f5812408e6e553c2` passed CI #549 / run `33835565704` and production #307 / run `33835565727`; Biz B13 reverse qualification run `33835971657` passed. PR #145 integrated the candidate as `15f19ff0845e421039ffb675937bf23c3bb8a79d`.

### T4.3 — Audit remediation proof binding

`yunka change set remediation bind --finding <id>` binds exact proven Audit findings to the active ChangeSet base and normalized ChangeSet digest. The logical binding path remains `.git/yunka/change-remediation.json`, while physical Git-private storage is resolved through Git's actual directory. The binding grants no additional mutation authority.

`yunka change set remediation check` composes normal ChangeSet reconciliation with Audit debt comparison. It passes only when the ChangeSet is conformant, every bound target is in `fixed`, no target remains, and no new proven Audit debt is introduced. Unknown findings, stale/tampered digests, base mismatch, and targets absent from the immutable base fail closed. Existing unrelated historical debt is not promoted into blocking debt merely because one finding is being remediated.

Agent Context schema v4 exposes the plan-first new-Operation, ChangeSet, apply, and remediation protocols without widening Advisor authority.

After ready-for-review pressure exposed the linked-worktree `.git` portability defect, exact post-review candidate `a584da287d52271a4a05ad80167f89fdbe0d3ee0` resolved both ChangeSet and remediation state through Git-native `rev-parse --git-path` semantics. It passed CI #563 / run `33842121802` including linked-worktree regression and determinism, and production #321 / run `33842121900` on MySQL 8.4 with clean-worktree verification.

Final Biz B13 replay used qualification commit `d2e26f9eeebce1f9989d88ebaaf07f0d3f6196a3` and run `33842351609`, which **PASS**ed the unchanged remediation pressure body against exact Yunka `a584da287d52271a4a05ad80167f89fdbe0d3ee0`. Evidence artifact `9925335952` has digest `sha256:891d92a07de5b555b15f22a33776060f35f8dc9339e4ba126643b366a3c0f1d4`.

The real consumer pressure proved all three required states:

```text
bound target unchanged
  -> remaining target -> FAIL

old target fixed but replacement introduces AUDIT-AUTH-001
  -> new proven debt -> FAIL

actual violating file removed inside ChangeSet scope
  -> fixed target + zero remaining + zero new debt -> PASS
```

Final PR head `8cc3967a1b2baaf5eecbbb22fa21d65eb2c80c2b` passed CI #565 / run `33844574960` and production #323 / run `33844574953`, then merged through PR #146 as `ba896a8776987e91d2525f6bb15a188653857980`. Exact-main post-merge CI run `33844917226` and production run `33844917372` both passed.

Exact evidence is recorded in `docs/waves/TERMINALIZATION-t4-changeset-remediation.md`.

Current machine change flows are:

```text
existing Operation:
context -> change plan -> change begin -> bounded edit -> change check -> generate -> change verify

new Operation:
context -> add operation --plan -> change set begin -> add operation -> generate -> change set check

Audit remediation:
audit -> change set remediation bind -> bounded edit -> change set remediation check
```

## Framework Terminalization T5

T5.1 is **complete, production-qualified, real-consumer pressure-qualified, and merged through PR #148**.

T5.1 upgrades `yunka change verify` from structural/semantic/behavior reconciliation to a complete generic Proof-of-Change aggregation for architecture debt by attaching the existing T2 `auditcore.DebtDelta` as `architectureDebt.existing/new/fixed` in Change Attestation schema v2.

Conformance is fail-closed on newly introduced proven architecture debt:

```text
new proven violation > 0
  -> architecture-debt FAIL
  -> attestation conformant=false

existing historical proven debt only
  -> reported / nonblocking

fixed proven debt
  -> reported / nonblocking
```

Audit/New-Debt proof runs only after canonical `yunka check` succeeds; `go test ./...` remains the independent generic Behavior Proof. Runtime evidence remains pressure-driven rather than mandatory for structural-only changes, and external-effect receipts remain outside generic Proof-of-Change.

The first B13 qualification attempt exposed a separate real compiler/DX module-identity defect. T5 stopped at that framework boundary; issue #152 was fixed independently and merged through PR #154 as `ebb8184bd6ab5dea9f0fa8b3ed7d9c6d5f765d9a` after CI #578, production #336, and B13 module-identity qualification Run #3 passed. Exact-main CI #579 and production #337 also passed. T5 then resumed on that canonical base without expanding Runtime, Executor, Authz, UoW, persistence, or protobuf business semantics.

Rebased T5.1 behavioral candidate `a102de3c53f1078c8c6cce70e334c91fbbb94bb2` passed CI #580 / run `33954262081` and production #338 / run `33954262031`. Real B13 qualification Run #9 / `33954431712` passed with artifact `9965889476` (`sha256:8070e9876fc5395cf5be4ca82539feceb31b1fb69482c8ea148d0169260ce520`). It independently proved NEW `AUDIT-INFRA-001` blocks at architecture-debt while `go-test=pass`, unchanged historical debt reports as existing without blocking, removal reports as fixed without blocking, repeated fixed attestation is byte-identical, and the qualification restores a clean consumer worktree.

Final pre-integration head `0a742d0cff0b634b5d80660fee2a77441bfdc1e1` passed CI #582 / run `33955577654` and production #340 / run `33955577637`, then PR #148 merged as `2f588ed2e5f40b06c6ce3b7f5803d1c51afceefc`. Exact-main CI #586 / run `33955864123` and production #344 / run `33955864115` both passed. Exact evidence is recorded in `docs/waves/TERMINALIZATION-t5-proof-of-change.md`.

## AX7 default control-state storage — issue #151

Default single-Operation Change Contract and Change Attestation now use the same Git-private path resolver as ChangeSet/remediation. Their logical defaults are `.git/yunka/change-contract.json` and `.git/yunka/change-attestation.json`; Git resolves physical storage for ordinary checkouts, nested project roots, linked worktrees, and submodules. The default `begin -> check -> verify -> check` loop does not require `.gitignore` changes and does not add control artifacts to the source delta.

The implementation changes storage only. Git delta, ownership, placement, semantic, Audit/new-debt, and Go-test gates are not weakened; no filename or directory is exempted from source reconciliation. Explicit `--output` / `--contract` paths remain literal, including historical `.yunka/...` paths. Existing files are not moved or deleted automatically. An old active contract can be selected explicitly; source-tree exports remain subject to normal scope checks. Concurrent projects in one worktree should use distinct explicit Git-private contract/output paths rather than sharing one active default slot.

The permanent issue-151 regression removes the old pressure fixture's blanket `.yunka/` ignore rule and exercises the public default commands, full Go tests, repeatable attestation, clean source delta, native Git layouts, explicit-path compatibility, and rejection of unrelated files. Exact RED/GREEN, framework, production, and consumer qualification records belong to the issue/PR delivery evidence, not a claim of universal defect freedom.

## Contract provenance and inventory path identity — issue #160

**State: COMPLETE / QUALIFIED / MERGED through PR #162.**

The independent substrate/inventory task preserves descriptor-derived file imports and declaration provenance, rebases all inventory source references to project-relative paths, and distinguishes external import names from canonical owned files. It also restores compiler/descriptor helpers accidentally omitted by the initial PR candidate. The root untyped V1 artifact remains byte-compatible; typed manifests use schema v4. PR #162 integrated qualified head `2ccf968aaf05a52d33abc871b327210e05d7f55c` as `8864f6110241c31cd7a28821c4bf7b291fe3eba8`; the merge tree equals the qualified candidate tree.

Real-protoc regression coverage includes independent roots with identical file names, cross-source shared/nested DTO and enum references, include-order selection, external-name collisions, physical aliases/path escapes, source moves preserving OperationPlan/OpenAPI/TypeScript semantics, deterministic artifact checks, legacy compatibility, and read-only project-relative context projections.

The substrate itself does not expose a CLI or change mutation authority. The independent CLI and source-enforcement increments are described below; #161 remains separate. No Runtime/Executor/Authz/UoW or protobuf business DSL changes are introduced here.

## Operation-scoped Agent Context CLI — issue #160

**State: COMPLETE / QUALIFIED / MERGED through PR #163 after PR #162.**

Agent Context schema v6 preserves lightweight, non-compiling bootstrap and adds explicit `--operation <id>` / `--all-operations` source queries. They compile current canonical inputs through the existing projectflow resolver, expose deterministic project-relative source files and separate external import names, and label results `read_only` / `canonical_file_import_closure`. They support proto-root and multi-source inventories plus internal Application Operations. Unknown/blank targets, invalid selectors, stale or broken source and ignored inventory include overrides fail rather than returning broad or partial context.

Tests exercise the public CLI with real protoc, same-named inventory roots, shared/nested DTOs, internal Operations, source moves, deterministic JSON/text, no-protoc bootstrap, and read-only success/failure. PR #163 integrated qualified head `35511cbe27c94c6f836e4edcaf68606518f4ce34` as `14bc2a19b0b2eb25b0efdae8304fd23dda8b4aa1`; its merge tree equals the qualified CLI candidate tree.

The source-enforcement increment adds `declarationFiles`, `messageTypes`, and `enumTypes` to the read-only context while preserving the full `sourceFiles` import closure. A service import closure may include unrelated DTOs and is never itself mutation authority. #161 Service Boundary semantics are unchanged.

## Operation declaration source-scope enforcement — issue #160

**State: COMPLETE / PRODUCTION-QUALIFIED / REAL-CONSUMER SOURCE-QUALIFIED / MERGED through PR #164.**

Plan/begin now derive contract targets from exact Operation/DTO declarations instead of selecting one arbitrary file from the entire proto root. Single-Operation and ChangeSet reconciliation, including create subjects, recompute source bounds from raw immutable Git base input and current canonical compilation whenever protobuf source changes. Explicit path-array changes and broad imports do not widen base authority. The same existing semantic evaluator checks fresh source facts so stale generated JSON cannot hide source-level semantic drift. Message/enum deltas outside the target type graph are blocked even inside allowed files; required shared DTOs and reachable new DTOs inside authorized files are supported.

The source projection is derived, not a second manifest. Git snapshot materialization is private/read-only and uses raw blobs, not archive substitutions or branch checkout. Ordinary/nested projects and canonical source inventories share the same path domain. Unplanned source moves fail closed; all-source lexical slicing, universal custom-option semantics and #161 business boundary review are not claimed. Existing contracts/sets retain their input schemas; check reports are schema v2 and Context is schema v6. Go-only deltas retain quick checks.

Permanent tests cover full-import versus declaration-file separation, shared nested/map/enum DTO changes, unrelated/tampered paths, current-reference widening, co-located unrelated declarations, stale generated semantic evidence, multiple/create subjects, nested inventories, unplanned moves, exact Git blobs, cancellation and path escapes. The full pressure fixture supplies its canonical external DSL include consistently to planning, generation and source checks, without treating framework-owned DSL as a consumer Go generation target. Snapshot regressions also reject composed relative-symlink escapes while admitting contained links. PR #164 integrated qualified head `984faede1a2cfbd5aad1017867d097037d0e2a74` as `25332db2ead8567237352f116d6244f1102b26f1`. This final integration has the same complete tree, `87a2fbc60e759e42c46cbd025ccd4447a911d9ee`, as the qualified candidate; no product files changed during the ordered merges.

## Issue #160 final integrated acceptance

The implementation was integrated in dependency order **#162 -> #163 -> #164**. Exact-main acceptance run `34037811821` rechecked the integrated SHA `25332db2ead8567237352f116d6244f1102b26f1`, proved ordered ancestry and a zero-difference tree against candidate `984faede1a2cfbd5aad1017867d097037d0e2a74`, passed all 16 explicitly identified context/source regression groups without skips, the complete `make verify-production` gate on real MySQL 8.4, deterministic regeneration/dependency checks, and a clean worktree.

The same run replayed the immutable source-scope qualification against `iot-delivery-system@69518dec46bdfaf45cb84a0ee25d64c132b26fc9`. It verified 25 deterministic Operation contexts, the eight-file read/four-file declaration context for `delivery.dashboard.get`, acceptance of required shared-source changes, rejection of unrelated declarations and a tampered unrelated-file allowlist, and unchanged original consumer hashes/Git state. The old CLI accepted the same widened ChangeSet and unrelated edit that the integrated CLI rejected. Consumer edits and explicit DSL input setup occurred only in a disposable copy.

Historical candidate evidence from PRs #162/#163/#164 is preserved for its exact SHA/tree and is inherited by the integrated implementation through verified tree equality, not a narrative assumption. Fresh exact-main results are in run `34037811821` and artifact `issue160-main-acceptance-evidence`; final documentation-only PR checks and issue disposition are recorded on GitHub. No normal CI/protection configuration or runtime/authz/UoW semantics were changed. Independent automated review was not obtained because the review bot reported its usage limit; tests and this integration inspection are not presented as independent approval.

Acceptance is bounded to canonical provenance, Operation context and modeled contract-source/declaration conformance. It does not claim lexical/token-minimal context, a filesystem sandbox, signed task authority, arbitrary custom-option semantics or consumer runtime qualification. Unplanned source moves require a separately scoped migration and fresh task. **Issue #161 remains OPEN and separate**; this delivery does not implement Service Boundary decisions.

## Application governance implementation stream

The user-selected application-organization plan is [APPLICATION-GOVERNANCE-PLAN.md](architecture/APPLICATION-GOVERNANCE-PLAN.md). It is a control-plane/consumer engineering stream, not a new numbered runtime wave. It preserves the separate #161 Service Boundary work and the existing B13 pressure classification.

| Task | Source state | Verification and scope |
| --- | --- | --- |
| AG-00 | Plan archived | Defines AG-01 through AG-09, allowed scope, dependencies, acceptance and rollback; not a claim that all tasks exist |
| AG-01 / AG-01I | Complete / qualified / merged through PR #170 | 11 named Go mechanism scenarios, 13 inventory-mutation cases, exact reviewed fixture fingerprints, 11 definition-binding regressions, anti-false-pass tests and two Linux process-tree cleanup regressions, through existing architecture-check. See [AG-01I evidence](waves/AG-01I-boundary-qualification.md). Candidate qualification and integration identities are recorded on PR #170 and its exact-head delivery run; no consumer refactor or general architecture scanner is claimed |
| AG-02 / AG-02R | Complete / consumer-qualified / merged through Biz PR #16 | TenantLifecycle encapsulation plus strict runtime-readiness and current-read owner-invariant corrections. Actual consumer main `3519e7ee6e51e33984669871e4f32a55a3597d9f`, tree `3f8926b459fb78688fe0f39e9c4acfb6d93b3b47`, revalidated by Biz run `34192294589`. [Exact evidence](waves/AG-02-biz-encapsulation.md); no Yunka runtime/compiler change |
| AG-03 / AG-03R | Complete / consumer-qualified / merged through IoT Delivery PR #7 | Saved-view hidden use cases and actual two-method Repository, legacy error parity and bounded SQLite startup correction. Consumer `bcd20632b666405c3f7a8fe8d53f591c78450087`, tree `0c0d1a2da23de4b7315245faba2103ec7a64377e`, actual-main run `34224052206`. [Exact evidence](waves/AG-03-iot-saved-view.md); no Yunka runtime/compiler change |
| AG-04 | Complete / qualified / merged through [PR #174](https://github.com/hvritual/yunka.io/pull/174), issue #173 completed | Opt-in `yunka audit types` checks explicit factory/contract/caller policy against actual Go type identity and bounded constructor provenance. PASS/FAIL/INCOMPLETE, opaque method/field surfaces and persistent read-only pinned-consumer static qualification. [Contract](architecture/APPLICATION-BOUNDARY-TYPES.md). Accepted main `e844b431c47a8135ba4061773d60e674a343f2f3`, tree `10756708e4c7a382024e0eb722ee28c510fe933b`; final PR and separate main gates all passed. Exact receipt is recorded below |
| AG-05 | Implementation complete; exact qualification/integration record: [PR #176](https://github.com/hvritual/yunka.io/pull/176), issue #175 | Opt-in physical source inventory, explicit module/build profiles, read-only Go-native import policy and full-source/analysis-completeness distinction. [Contract](architecture/APPLICATION-SOURCE-POLICY.md). Real legacy consumer findings remain findings, not invented full-repository PASS. Exact-head qualification, independent review and main acceptance remain required |
| AG-06 through AG-09 | Defined, not delivered | Template integration, migration/debt-growth, batch refactoring and continuous evolution |

The earlier AG-01 head `7c50680f0381505d9cc55a029a2a92baa6b0a140` passed CI run `34119891295` and production run `34119891325`. AG-01I adds test-policy integrity and documentation reconciliation; it must establish its own exact-head qualification rather than inherit the earlier result by narrative. The delivery worker creates a normal Git commit before testing, runs the new candidate with locked Go/protoc and MySQL, and publishes it only after full qualification. PR discussion and Git/action receipts distinguish candidate, reviewed and integrated states; main must be read back before claiming integration.

AG-01I main integration was completed at `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7` with exact-main run `34136019602`. AG-02 then used the consumer's existing runtime/generator pin `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`; current-main CLI placement checks do not imply a consumer runtime upgrade. Biz #17 is the demonstrated consumer last-owner snapshot defect, not a new Yunka UoW defect. The two relevant Biz workflows now retain the readiness and deterministic snapshot regressions in their recurring gates. AG-03 has separately qualified one IoT Delivery saved-view use-case pair while keeping its existing runtime/generator gitlink. The required SQLite startup fix is a consumer correction, not a Yunka UoW defect. AG-04 implements the bounded opt-in typed audit and persistent read-only Biz/IoT static qualification. Final AG-04 head/main `e844b431c47a8135ba4061773d60e674a343f2f3`, tree `10756708e4c7a382024e0eb722ee28c510fe933b`, passed final PR CI `34294613404`, Production `34294613415`, consumer types `34294613417` and separate actual-main push CI `34295287070`, Production `34295287242`, consumer types `34295286293`. Independent review comment `5593875939` reviewed that exact head without major findings; all eight threads were resolved. PR #174 was merged non-force and issue #173 closed completed after main acceptance. Historical earlier revisions remain qualified only by their own runs. AG-05 implements full-source/build-profile import checks with explicit incomplete evidence. Its final reviewed head, non-force integration and actual-main four-gate acceptance are recorded separately in PR #176/#175; source implementation alone does not establish accepted integration. AG-06 templates and later migration/debt work remain undelivered.

AG-05 review hardening preserves three independently reproduced constraints: [CGO tool PATH](waves/AG-05-cgo-toolpath-review.md), [GOROOT overrides](waves/AG-05-goroot-review.md), and [writable cache paths](waves/AG-05-cache-path-review.md). Start the checker with GOROOT unset and keep writable cache/home/temp locations outside the audited root. Failure to establish these prerequisites is INCOMPLETE before invoking tools, not an accepted audit followed by worktree pollution. The full frozen IoT legacy-module finding remains INCOMPLETE and is never replaced by its narrower current-module PASS.

These tests characterize the language boundary, including expected examples of insufficient encapsulation. They do not certify arbitrary Go code, all operating systems, consumer root-UoW behavior, or a same-process/OS security sandbox. Canonical tool downloads are disabled inside fixture execution; this is not a network firewall. Unix process-group cleanup has Linux qualification only; non-Unix execution reports INCOMPLETE until its process-tree backend is implemented and qualified.

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
