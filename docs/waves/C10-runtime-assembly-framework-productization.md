# C10 — Runtime Assembly & Framework Productization

> Document class: **HISTORICAL**  
> Delivery state: **Complete / qualified / merged**  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Original qualified release head: `4c4beb72ffde79d97a28cf5c3b083c756b729d27`  
> Release record: [`C10.5-release-qualification.md`](C10.5-release-qualification.md)  
> Historical note: the original status, sequencing, and planning text below is preserved as authored and is **not** current status truth.

## Status

- State: **Planned**
- Tracking: GitHub issue #42
- Base: C9.9 qualified `main` tree
- Delivery model: five strictly ordered, independently reviewable subwaves

```text
C10.1 Assembly Contract
        ↓
C10.2 Compiler
        ↓
C10.3 Runtime Closure
        ↓
C10.4 Consumer Upgrade
        ↓
C10.5 Release Qualification
```

C10 starts only after the C9.9 baseline/conformance closure. C10 is not a request to broaden framework semantics. Its purpose is to turn the already-completed C7-C9 architecture into a developer-facing application assembly product with deterministic generation, explicit typed wiring, one runtime model, and real-consumer qualification.

## Problem statement

C9 established the correct execution architecture:

- protobuf is the canonical writable contract DSL;
- stable Operation ID is the canonical execution identity;
- `framework/operation.Executor` is the transport-neutral execution path;
- one root authorization decision occurs before the Application boundary;
- root `ExecutionScope` owns UoW/transaction/idempotency lifecycle;
- typed child Operations join the active root scope;
- internal Operations do not require fake REST/RPC exposure;
- typed Application dependencies replace service-locator/reflection composition;
- `core.App`, Platform Provider, Health, Diagnostics, Application Graph, and `yunka dev` already exist as independent correct mechanisms.

The remaining product gap is assembly. A developer still needs too much knowledge of Yunka's internal seams to turn Domain/Application/Operation declarations into a complete runnable process. C10 closes that gap without creating a second runtime or a second business DSL.

## C10 objective

A representative business application should be able to follow this developer loop:

```text
yunka init
    ↓
define developer-owned PO + protobuf contract
    ↓
yunka generate
    ↓
implement handwritten business Application logic
    ↓
yunka check
    ↓
yunka dev
    ↓
Ready / Health / Diagnostics / Graph evidence
```

The developer must not manually reproduce framework plumbing for REST/gRPC registration, root authorization sequencing, transaction/UoW ownership, child Operation execution, capability acquisition, lifecycle wiring, or diagnostic registration.

## Definition of done

C10 is complete only when all of the following are true:

1. **One explicit assembly truth** — project/application assembly facts have a canonical ownership model and do not duplicate protobuf, OperationPlan, module-catalog, or dev-runtime facts.
2. **Deterministic compilation** — identical inputs produce byte-for-byte identical AssemblyPlan/generated Go and a stable digest.
3. **Typed bootstrap** — generated composition uses ordinary Go types and compile-time interfaces; no reflection DI, generic service lookup, dynamic handler registry, or package-global mutable runtime is introduced.
4. **Zero manual transport wiring** — a normal generated application does not hand-register canonical REST/gRPC Operation adapters.
5. **Single execution semantics** — generated external and internal invocations continue through the existing C9 `Executor`/`ExecutionScope`; C10 adds no parallel execution pipeline.
6. **Existing security/transaction invariants preserved** — one root authorization decision, explicit execution policy, one root local UoW, child Operation join semantics, Saga/Outbox transaction joining, and fail-closed behavior remain unchanged.
7. **Runtime closure** — the assembled process participates in existing App Start/Health/Shutdown, Diagnostics, Application Graph, and bounded `yunka dev` readiness/closure.
8. **Real consumer proof** — `hvritual/biz` runs on the C10 path without framework bypass and passes deterministic generation, verification, MySQL pressure, internal-Operation, permission-closure, and shared-UoW assertions.
9. **Release qualification** — exact-tree framework and real-consumer verification are green and the repository returns to zero generated/dependency/worktree drift.

## Architectural invariants retained from C9.9

C10 must preserve these boundaries:

- `context.Context` remains the execution boundary.
- `core.App` remains the application/process lifecycle owner.
- `framework/platform.Provider` remains the owner of named shared infrastructure.
- `gateway/authz` remains the canonical authorization decision boundary.
- `framework/operation.Executor` remains the single canonical Operation execution runtime.
- root `framework/execution.ExecutionScope` remains the Operation UoW/transaction owner.
- `framework/requestscope` remains a typed repository-view/join mechanism, not a second root transaction owner.
- `contracts/proto` remains the canonical RPC/DTO/Application/Operation declaration tree.
- generated contract/runtime artifacts remain derived evidence and are never hand-edited.
- internal Operations remain first-class canonical Operations without invented transport exposure.
- business rules, state machines, semantic DTO/domain mapping, and business orchestration remain handwritten Application code.
- Application/Operation dependencies are explicit facts; naming conventions never create architecture edges.

## Global non-goals

C10 does **not** introduce the following unless an executable C10 blocker proves the smallest required mechanism and the change is separately reviewed:

- generic BPMN/workflow engine;
- distributed database transaction / 2PC;
- generic SQL/data-scope DSL;
- framework-owned Customer/Site/Device or other business resource taxonomy;
- reflection DI/service locator/runtime field injection;
- automatic business-semantic inference from names, HTTP verbs, package paths, or method signatures;
- new universal audit persistence policy;
- new universal cache policy;
- idempotent response/result replay;
- Saga topology DSL/graph expansion solely because FP-C9-005 is open;
- a second REST, RPC, transaction, identity, authorization, or application runtime.

---

# C10.1 — Assembly Contract

## Goal

Define the smallest explicit, deterministic assembly model that answers:

> Which declared Applications, modules, platform capabilities, Operation bindings, and process-level entry surfaces form one runnable Yunka application, and which existing source owns each fact?

C10.1 defines contract/IR ownership only. It must not implement the final runtime bootstrap.

## Inputs

- current `.yunka/project.json` project-level facts;
- canonical contract manifest derived from `contracts/proto`;
- C9 `operation-plans.json`;
- static module catalog descriptors and typed capability requirements;
- existing Application/Operation dependency declarations;
- explicit HTTP/gRPC bindings;
- existing W09 Application Graph evidence model;
- existing `.yunka/dev.json` process/readiness declarations, where those facts are strictly local-runtime concerns.

## Required design decision

C10.1 must explicitly classify every assembly fact as one of:

1. **Developer-authored canonical fact**;
2. **Existing canonical fact reused from another source**;
3. **Derived assembly fact**;
4. **Environment/runtime-local fact** that must not enter committed assembly truth.

A new writable DSL is forbidden for facts already owned by protobuf, module descriptors, project config, or dev runtime.

## Target output

Introduce a leaf-safe immutable AssemblyPlan model, conceptually parallel to `pkg/operationplan`, with deterministic:

- normalize;
- validate;
- digest;
- load/save;
- inspect;
- provenance/evidence.

The committed derived artifact should expose, at minimum:

- assembly identity/schema version;
- selected Application identities;
- Application dependency closure;
- module/capability requirements;
- canonical Operation inventory used by the assembly;
- external transport bindings as references to existing explicit contract bindings;
- internal Operation dependencies;
- required platform capability names/types;
- generated bootstrap targets;
- evidence/provenance for every edge/fact.

The exact artifact path is fixed in C10.1 after ownership tests prove it does not become a second contract source of truth. The preferred model is a deterministic generated `assembly-plan.json` consumed by C10.2.

## Expected change areas

Allowed/expected:

- `pkg/assemblyplan/**` or the final leaf-safe AssemblyPlan package;
- `pkg/contract/**` compiler integration;
- `app/cmd/contract/**` and/or a narrowly scoped assembly inspect/check command;
- `app/cmd/project/**` only for genuinely project-owned facts;
- deterministic generated fixtures/artifacts;
- architecture/contract tests;
- `docs/waves/**`.

Forbidden in C10.1:

- changes to authorization semantics;
- changes to `ExecutionScope`/transaction/idempotency behavior;
- generated runtime bootstrap;
- server startup or process supervision changes;
- consumer migration.

## Validation gate

C10.1 is complete only when:

- same canonical inputs produce the same AssemblyPlan bytes and digest;
- missing/unknown Application references fail closed;
- missing capability/module requirements fail closed;
- dependency cycles fail closed;
- external bindings are reused, never inferred;
- internal Operations remain transport-free unless explicitly externally bound;
- no duplicated writable contract fact is introduced;
- AssemblyPlan generation/check leaves the worktree clean on a second run;
- architecture tests prove AssemblyPlan is data/IR, not a runtime registry or service locator.

## Primary risk

**Risk:** C10 creates another manifest that competes with protobuf/project/module/dev sources.

**Control:** AssemblyPlan is derived wherever possible; C10.1 must document ownership for every field and reject duplicated source-of-truth semantics.

## Rollback

Delete the AssemblyPlan experiment without changing C9 runtime behavior. C10.1 must be independently removable before C10.2 consumes it.

---

# C10.2 — Compiler

## Goal

Compile the C10.1 AssemblyPlan into deterministic typed Go composition so that framework wiring becomes generated structure rather than handwritten framework knowledge.

C10.2 is structural code generation. It must not generate business behavior.

## Compiler responsibilities

The compiler may generate typed code for:

- App/kernel bootstrap options;
- static module descriptor/catalog composition;
- aggregate platform capability requirements;
- typed Application dependency ports;
- typed Application implementation/factory binding seams;
- canonical Operation executor binding;
- REST adapter registration for explicitly HTTP-bound Operations;
- gRPC adapter registration for explicitly RPC-bound Operations;
- internal child capability wiring without external transport exposure;
- route/RPC inventory evidence needed by existing diagnostics/graph surfaces;
- compile-time checks that required handwritten business implementations are supplied.

## Required handwritten boundary

Developers continue to own:

- PO structs/persistence-specific declarations;
- protobuf business contracts;
- Application use-case implementations;
- business invariants/state machines;
- domain-specific authorized-scope interpretation;
- complex queries and semantic mapping;
- product-specific configuration values/secrets.

Generated code may require these implementations through typed constructors/interfaces, but must never synthesize default business behavior to make compilation pass.

## Expected change areas

Allowed/expected:

- contract/assembly compiler and code generation packages;
- generated fixture packages;
- typed bootstrap/binding interfaces;
- narrowly required `framework/kernel`/catalog constructor seams if current APIs cannot express generated composition without handwritten glue;
- REST/gRPC generated registration glue that delegates to existing canonical adapters;
- CLI `generate/check` integration;
- architecture/determinism tests.

Forbidden in C10.2:

- reflection-based constructor discovery;
- package scanning at runtime;
- string-based Application/service lookup;
- implicit module/Application activation from package names;
- duplicate authorization logic;
- duplicate transaction/UoW lifecycle;
- new RPC/REST execution runtime;
- business CRUD/use-case implementation generation beyond already-approved Domain compiler boundaries.

## Validation gate

C10.2 is complete only when:

- a representative multi-Domain/multi-Application fixture compiles from generated assembly;
- missing handwritten Application binding is a compile/check failure rather than runtime discovery;
- generated REST and gRPC adapters target the same canonical Operation/Executor semantics;
- internal Operations generate typed child capabilities but no fake transport registration;
- second generation is byte-for-byte zero drift;
- generation order is independent of source enumeration order;
- architecture policy rejects reflection/service-locator/hidden registry regressions;
- existing C9 security, transaction, idempotency, permission-closure, and OperationPlan tests remain green.

## Primary risk

**Risk:** codegen absorbs Application semantics and becomes a low-code business framework.

**Control:** generated code owns only structural assembly. Business decisions remain handwritten and compiler tests must prove no default business implementation is emitted.

## Rollback

Generated assembly can be disabled while retaining the C9 manual typed composition path until C10.4 proves the replacement against a real consumer.

---

# C10.3 — Runtime Closure

## Goal

Connect generated assembly to the existing runtime mechanisms so the generated application becomes a complete lifecycle participant without introducing a second runtime.

The closure target is:

```text
AssemblyPlan
   ↓
Generated typed bootstrap
   ↓
core.App / kernel.New
   ↓
Platform Provider
   ↓
Applications + Executor
   ↓
REST / gRPC / internal Operations
   ↓
Health + Diagnostics + Application Graph
   ↓
yunka dev readiness / closure
```

## Scope

C10.3 must prove the generated bootstrap can:

- construct one isolated `core.App` instance;
- prepare each named shared platform capability once;
- start typed modules/servers in deterministic order;
- expose only explicitly bound REST/gRPC Operations;
- execute all external Operations through the canonical C9 Executor;
- execute local child Operations through typed `ExecuteChildTyped` semantics;
- expose the existing App Health model to transport and local dev readiness;
- publish existing safe Diagnostics and runtime inventory;
- contribute declared/observed evidence to the existing Application Graph;
- shut down in reverse dependency order with bounded graceful shutdown;
- integrate with `yunka dev plan|run|status --closure` without making the dev runtime infer commands or business architecture.

## Developer runtime rule

`yunka dev` remains an explicit local process orchestrator. C10.3 may consume generated assembly/health/graph facts to validate readiness and closure, but it must not infer shell commands, environment secrets, deployment topology, or restart policy from application code.

## Expected change areas

Allowed/expected:

- generated bootstrap/runtime package;
- `framework/core`, `framework/kernel`, `framework/platform` only where a minimal typed assembly seam is required;
- gateway REST/gRPC registration composition;
- `framework/diagnostics` and Application Graph adapters using existing evidence contracts;
- `pkg/devruntime/**` and `app/cmd/dev/**` for assembly-aware closure/readiness;
- integration/runtime tests.

Forbidden in C10.3:

- parallel lifecycle owner;
- hidden global App/runtime singleton;
- new service locator;
- implicit nested transaction management;
- new authorization phase;
- production secret persistence in assembly/runtime state;
- automatic shell command inference;
- automatic process restart policy.

## Validation gate

C10.3 is complete only when an assembled fixture proves:

1. generated bootstrap starts successfully;
2. readiness remains false until required capabilities/modules are healthy;
3. explicitly bound REST and gRPC Operations are reachable;
4. internal-only Operations are not externally reachable;
5. one root protected request performs one authorization decision;
6. one root transactional Operation owns one local UoW;
7. child Operations join that scope and cannot escalate transaction mode;
8. diagnostics expose safe assembly/runtime evidence without credentials or caller scopes;
9. graph closure records declared and observed evidence correctly;
10. graceful shutdown occurs in reverse dependency order;
11. `yunka dev --closure` detects incomplete ownership/readiness rather than silently continuing;
12. existing `make verify` remains green.

## Primary risk

**Risk:** generated assembly becomes a second runtime layered beside `core.App`/Executor.

**Control:** C10.3 may only adapt to existing lifecycle/execution interfaces. Architecture checks must block alternative runtime ownership.

## Rollback

Retain generated AssemblyPlan/codegen but fall back to explicit typed C9 bootstrap while correcting runtime adapters. No C9 runtime contract is removed before C10.4 consumer proof.

---

# C10.4 — Consumer Upgrade

## Goal

Use the real `hvritual/biz` repository to prove C10 removes framework plumbing without weakening business/runtime semantics.

C10.4 is primarily a **consumer pressure wave**. Framework changes are allowed only when the real migration demonstrates a concrete generic assembly defect.

## Consumer migration target

Migrate the existing real cross-Application Device Transfer slice and surrounding generated applications to the C10 assembly path while preserving the already-proven C9.8/C9.9 truth:

```text
device_transfer (root)
  -> site.validate_transfer_target (internal child Operation)
  -> business decision
  -> device.update (child Operation)
```

Required invariants remain:

- no fake Site RPC/REST for the internal validation Operation;
- generated typed child capability execution;
- transitive permission closure remains explicit and correct;
- one root authorization decision;
- one root local UoW/transaction;
- no direct cross-Application repository/service-locator bypass;
- internal-only DTOs do not leak into external OpenAPI/TypeScript projections;
- durable idempotency/Outbox/Saga semantics remain unchanged where used.

## Migration measurement

C10.4 must maintain a before/after inventory of handwritten framework plumbing. The target is not an arbitrary percentage reduction; the hard target is elimination of these categories where they are purely structural:

- manual canonical REST registration;
- manual canonical gRPC registration;
- manual framework Application dependency wiring;
- manual root authorization sequencing;
- manual root requestscope/transaction orchestration;
- manual child Operation runtime wiring;
- duplicate lifecycle/health/diagnostic registration.

Business-specific constructors, policy interpretation, configuration, and use-case logic remain legitimate handwritten code.

## Framework Pressure classification

Every migration workaround must be classified before Yunka changes:

- **P0 Compiler defect** — declared assembly cannot generate an already-supported C9 behavior;
- **P1 Missing generic assembly seam** — repeated structural wiring remains and cannot be expressed through typed C10 bindings;
- **Consumer-specific** — belongs in Biz and must not enter Yunka;
- **Deferred framework pressure** — potentially generic but requires a second independent business proof before adding new semantics.

A consumer-specific workaround must not be encoded as a Yunka DSL feature merely to make C10.4 look cleaner.

## Validation gate

C10.4 is complete only when the real consumer:

- regenerates from the C10 compiler;
- produces zero drift on a second generation;
- passes its standard `make verify`/equivalent gate;
- passes MySQL 8.4 pressure/integration;
- passes cross-Application shared-UoW assertions;
- passes internal-Operation/no-external-exposure assertions;
- passes authorization/permission-closure assertions;
- has no manual canonical REST/gRPC registration remaining for migrated Operations;
- has no Application-level framework transaction/authz bypass;
- records any remaining framework pressure explicitly rather than silently extending C10 scope.

## Primary risk

**Risk:** Yunka absorbs `hvritual/biz` product-specific structure during migration.

**Control:** consumer pressure classification is mandatory; new semantics require evidence beyond one consumer unless they are direct correctness defects in already-declared C10 behavior.

## Rollback

The consumer remains able to return to the qualified C9.9 generated/manual typed assembly path until C10.5. No irreversible consumer cutover is treated as release evidence before zero-drift and MySQL pressure are green.

---

# C10.5 — Release Qualification

## Goal

Qualify one exact C10 framework tree and one exact real-consumer generated tree as the release baseline. C10.5 is a closure/fact wave, not a feature wave.

## Freeze rule

Once C10.5 starts, no new framework mechanism may enter the release candidate. A failing gate may cause only:

- a deterministic generated-artifact correction;
- a documentation/fact correction;
- the smallest code fix directly required by a failing C10 contract/invariant.

Any broader feature returns to a new post-C10 pressure wave.

## Framework release gate

Required evidence:

- locked toolchain check;
- dependency/architecture/module/security/operation/domain/DSL/RPC/contract checks;
- deterministic AssemblyPlan generation/check;
- deterministic generated bootstrap generation/check;
- full `make verify`;
- full `make verify-production` against real MySQL 8.4;
- zero dependency/generated/worktree drift after verification;
- no C10 temporary builder/control artifacts in the final tree.

## Real consumer release gate

Required evidence on an exact committed Biz generated tree:

- C10 regenerate;
- second regeneration zero drift;
- standard consumer verification;
- MySQL 8.4 pressure/integration;
- C9.8/C9.9 internal Operation and cross-Application invariants;
- C10 zero-manual-transport-wiring assertions for the migrated slice;
- no framework bypass introduced to make the migration pass.

## Repository truth reconciliation

Before completion, reconcile:

```text
main code truth
== generated assembly truth
== contract/OperationPlan truth
== docs truth
== PROJECT_MEMORY truth
== C10 issue/PR truth
== consumer generated truth
== consumer Pressure truth
== verification truth
```

## Hosted-runner rule

A GitHub Actions job that terminates before any workflow step (`steps=null`) is neither passing evidence nor a product-test failure. If native Yunka hosted runners remain unavailable, C10.5 may use an independently executed exact-tree substitute only when it runs the canonical locked commands to completion on the exact candidate tree and the evidence is recorded. The release record must distinguish infrastructure unavailability from executable verification.

## Completion gate

C10 is Complete only when:

1. C10.1-C10.4 are individually closed;
2. exact C10 framework tree passes production verification;
3. exact real consumer tree passes regenerate/zero-drift/verify/MySQL pressure;
4. generated assembly has no manual post-generation edits;
5. current README/wave docs/PROJECT_MEMORY describe the same runtime architecture;
6. issue #42 is reconciled with the final subwave disposition;
7. no temporary control/build mechanism remains;
8. the qualified C10 tree is merged through normal repository review/merge flow;
9. any deferred pressure is explicitly recorded and kept out of C10 runtime semantics.

---

# Delivery grouping

## MVP Roadmap

**C10.1 → C10.2 → C10.3**

Outcome: a new project can move from explicit project/contract facts to a generated typed application bootstrap and reach healthy local runtime closure without manual canonical transport wiring.

## Full Runtime Roadmap

**C10.4**

Outcome: the real Biz consumer proves the assembly model works under existing cross-Application, authorization, transaction, internal Operation, and MySQL pressure rather than only in framework fixtures.

## Production Roadmap

**C10.5**

Outcome: exact framework + consumer trees, deterministic generation, production MySQL verification, docs/memory/Pressure reconciliation, and a release-qualified baseline.

# Global execution rules

- Work from the latest reviewed `main` baseline for each subwave.
- One subwave = one independently reviewable task branch/PR unless an explicit split is required by risk.
- Do not stack new semantics on an unmerged/failed previous subwave.
- Every compiler/runtime change requires architecture + deterministic generation tests in the same subwave.
- Generated files are outputs, not hand-edit surfaces.
- Normal CI remains read-only and deterministic.
- `main` is not moved directly as part of implementation work; merge remains a separate explicit repository action.
- Real-consumer evidence is mandatory before declaring framework DX/productization complete.
- Framework Pressure, not speculative completeness, determines post-C10 feature growth.

# C10 success statement

C10 succeeds when an ordinary Go developer can understand Yunka primarily through four concepts:

```text
PO
Contract
Application
Operation
```

and can rely on the framework/compiler to correctly assemble the remaining infrastructure/runtime mechanics without learning or reimplementing Yunka's internal lifecycle, authorization, transaction, transport, capability, diagnostics, and cross-Application wiring seams.