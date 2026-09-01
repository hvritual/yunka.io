# C11 — Developer Experience Productization

> Document class: **HISTORICAL**  
> Delivery state: **Complete / production-qualified**  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Final C11 qualified head: `52b67bc0cc78ebad4367b255f2a25bce4e729159` (issue #60)  
> Post-C11 five-gap convergence: qualified `a0cf52d2fdc8e5fd2b361b8c2640dfa60184b1b4`, merged as current `main@24089df35f945abb42ddb00731283f4d878d8424`  
> Historical note: the original status, sequencing, and planning text below is preserved as authored and is **not** current status truth.

## Status

- State: **In progress**
- Base: `main@257660d5c57ca58647020dc9cf8199f438f5fc96`
- C11.1: **Complete / qualified**
- C11.1 exact head: `f0836e0c69af4ddf0f84f42e3422a829da47d0a5`
- Delivery model: ordered, independently reviewable DX subwaves

```text
C11.1 Happy Path          ✅
        ↓
C11.2 Project Profile
        ↓
C11.3 Diagnostics UX
        ↓
C11.4 Fast Feedback
        ↓
C11.5 Dev Runtime UX
        ↓
C11.6 Discoverability
        ↓
C11.7 Real Consumer Gate
```

C11 starts from the merged C10 Runtime Assembly / Framework Productization baseline. It does not broaden framework execution semantics. Its purpose is to reduce how much Yunka architecture a developer must understand in order to create, generate, validate, run, and diagnose an application correctly.

## Problem statement

C7-C10 established the correct architecture:

- protobuf is the canonical writable business contract DSL;
- OperationPlan is the canonical operation execution projection;
- AssemblyPlan is derived assembly truth;
- generated typed Go owns structural Runtime Assembly;
- `framework/operation.Executor` is the single canonical Operation runtime;
- root `ExecutionScope` owns transaction/UoW/idempotency lifecycle;
- `core.App` is the only process lifecycle owner;
- Platform Provider owns named shared infrastructure;
- Health, Diagnostics, Application Graph and `yunka dev` provide runtime evidence and closure;
- real consumer `hvritual/biz` has qualified the C10 path.

The remaining gap is developer workflow. The current CLI exposes expert-level framework seams directly (`contract`, `assembly`, `graph`, `dev plan/run/status`, repeated source/output flags). A correct application can be built, but a normal developer still needs to understand too many internal stages and paths.

C11 therefore converts architectural correctness into a correct-by-default developer path.

## C11 objective

The normal developer loop must converge to:

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
Ready / Health / Diagnostics / Graph evidence
```

Expert commands remain available for focused inspection and troubleshooting, but ordinary application development must not require manual orchestration of framework compiler stages.

## Definition of done

C11 is complete only when all of the following are true:

1. **Four-command happy path** — normal development is centered on `yunka init`, `yunka generate`, `yunka check`, and `yunka dev`.
2. **Zero routine compiler flags** — a normal configured project can generate and check without repeating contract/module/assembly paths or Go import paths on every invocation.
3. **No second source of truth** — DX configuration locates or defaults existing canonical facts; it never duplicates protobuf, OperationPlan, module descriptor, AssemblyPlan, or runtime closure semantics.
4. **Correct-by-default generation** — the facade reuses the existing deterministic contract/module/assembly compilers and preserves second-run zero drift.
5. **Fast local feedback** — `yunka check` is intentionally narrower than production qualification and provides a bounded edit/validate loop.
6. **Actionable diagnostics** — core DX failures identify the failed stage and provide a concrete remediation action; machine-readable output remains available where applicable.
7. **Single runtime** — `yunka dev` continues to reuse the existing devruntime/closure model; C11 introduces no second supervisor or readiness model.
8. **Expert escape hatch** — `yunka contract`, `yunka assembly`, `yunka graph`, `yunka inspect`, `yunka dependency`, and explicit `yunka dev plan|run|status` remain valid advanced interfaces.
9. **Real consumer proof** — `hvritual/biz` can adopt the happy path without bypassing C10 Runtime Assembly or weakening C9 execution/security/transaction invariants.

## Architectural invariants

C11 must preserve these boundaries:

- `context.Context` remains the execution boundary.
- `core.App` remains the only lifecycle owner.
- `framework/platform.Provider` remains the process-owned capability provider.
- `gateway/authz` remains the canonical root authorization boundary.
- `framework/operation.Executor` remains the sole canonical Operation execution runtime.
- root `framework/execution.ExecutionScope` remains the root UoW/transaction/idempotency owner.
- protobuf remains the canonical writable business contract DSL.
- generated contract/operation/assembly artifacts remain derived and are never hand-edited.
- module descriptors remain the canonical module capability/dependency declaration.
- `.yunka/dev.json` remains the explicit local process/readiness/runtime-closure declaration.
- no developer-facing convenience command may infer business semantics from names, package paths, verbs, or method signatures.

## Global non-goals

C11 does **not** introduce:

- a second contract or assembly DSL;
- reflection DI or service locator behavior;
- automatic business logic generation;
- implicit authorization, transaction, idempotency, Saga or Outbox semantics;
- runtime listener/secret/environment discovery by convention;
- a second process supervisor, health model or diagnostics model;
- broad framework primitive expansion unrelated to an executable DX blocker.

---

# C11.1 — Happy Path

## Status

- State: **Complete / qualified**
- Exact qualified head: `f0836e0c69af4ddf0f84f42e3422a829da47d0a5`
- CI: run `33311070919` — full verify and determinism PASS
- Production: run `33311070913` — MySQL 8.4 `verify-production` and clean worktree PASS

## Goal achieved

C11.1 introduced the smallest top-level developer facade over the existing canonical compiler and validation paths:

```text
yunka generate
    -> canonical contract generation
    -> module validation
    -> canonical C10 Assembly generation

yunka check
    -> canonical contract drift checks
    -> module validation
    -> canonical C10 Assembly drift checks
```

The facade does not own a second compiler or runtime.

## Qualified behavior

- `yunka generate` and `yunka check` are top-level commands.
- Conventional project defaults resolve `contracts/sources.json` or `contracts/proto`, `contracts/generated`, `modules`, and generated Go root `internal`.
- Generated Go root import is derived from `go.mod` as `<go-module>/internal`; the compiler emits Application/Assembly child packages beneath that root.
- `yunka check` is read-only and fails closed on generated drift.
- Second generation is byte-stable.
- A permanent equivalence test compiles the same typed Application/module fixture through both the happy path and expert `contract generate + assembly generate` path and requires identical per-file SHA snapshots.
- Both happy-path and expert check paths pass on the equivalent outputs.
- Existing expert commands remain unchanged.

## Preserved boundaries

C11.1 changes no authorization, OperationPlan, Executor, ExecutionScope, UoW, transaction, idempotency, Runtime Assembly ownership, Platform lifecycle ownership, or runtime supervision semantics.

---

# C11.2 — Project Profile

## Goal

Centralize developer-workflow defaults in a narrowly scoped project profile so routine commands do not require repeated path/import flags, while keeping architecture truth in its existing canonical owners.

## Configuration ownership rule

`.yunka/project.json` may own only developer-workflow/location defaults such as:

- project display identity;
- contract source inventory or proto-root location;
- contract generated output location;
- module root location;
- generated Go root location;
- generated Go root import when it cannot be derived safely from `go.mod`;
- local dev manifest location/default target references.

It must **not** own or duplicate:

- Domain/Application/Operation declarations;
- Application or Operation dependencies;
- execution/security/transaction/idempotency policy;
- module capability/dependency declarations;
- AssemblyPlan relationships;
- local process/readiness declarations already owned by `.yunka/dev.json`.

## Required C11.2 behavior

1. Upgrade the project config schema in a backward-compatible, deterministic way.
2. `yunka init` creates the profile with conservative defaults and derives the Go import root when possible.
3. `yunka generate` / `yunka check` load the profile first, then resolve canonical facts through configured locations.
4. Conventional defaults remain compatible for existing C11.1 projects without a migrated profile.
5. Missing or conflicting paths fail with explicit configuration-stage errors rather than silently falling back to unrelated locations.
6. Profile serialization is deterministic and contains no secrets.
7. Expert command flags remain valid escape hatches and are not silently rewritten by the profile.

## C11.2 non-goals

- no second contract/assembly/runtime DSL;
- no business-semantic configuration;
- no secret storage;
- no environment deployment topology;
- no automatic module/Application discovery beyond existing explicit generated/module facts;
- no change to C7-C10 semantic owners.

---

# C11.3 — Diagnostics UX

Introduce stable stage/error codes and actionable remediation metadata consumable by text CLI, JSON output, CI and AI agents.

# C11.4 — Fast Feedback

Add bounded fast checks and optional incremental generation using canonical input digests. Incremental behavior is an optimization only; full deterministic generation remains authoritative.

# C11.5 — Dev Runtime UX

Make `yunka dev` the zero-argument happy path over the existing devruntime/closure model while preserving explicit `plan`, `run`, `status`, target and expert flags.

# C11.6 — Discoverability

Converge CLI taxonomy, help, examples, deprecation messaging and `explain`-style diagnostic discovery without removing expert capabilities prematurely.

# C11.7 — Real Consumer Gate

Qualify the complete DX path against `hvritual/biz` from a clean checkout, including generation, check, runtime readiness/closure and unchanged C9/C10 semantic gates.