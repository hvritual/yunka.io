# C11 — Developer Experience Productization

## Status

- State: **In progress**
- Base: `main@257660d5c57ca58647020dc9cf8199f438f5fc96`
- First delivery branch: `agent/c11-1-developer-happy-path`
- Delivery model: ordered, independently reviewable DX subwaves

```text
C11.1 Happy Path
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

## Goal

Introduce the smallest top-level developer facade that makes the existing deterministic compiler and validation stages usable without requiring developers to manually orchestrate them.

The target first slice is:

```text
yunka generate
    -> existing canonical generation stages

yunka check
    -> existing canonical drift/structure checks
```

C11.1 must be an orchestration/facade layer. It must not fork compiler semantics.

## Required behavior

### `yunka generate`

The command must:

1. resolve project-local defaults from existing project facts where they already exist;
2. invoke the canonical contract generation path;
3. invoke module validation as a prerequisite when modules are present;
4. invoke the canonical C10 Assembly generation path when assembly inputs are configured/resolvable;
5. preserve deterministic output and second-run zero drift;
6. print stage-oriented output so a developer knows what was generated or skipped;
7. fail immediately at the first blocking canonical error with stage context.

### `yunka check`

The command must:

1. remain read-only;
2. validate the same canonical project facts used by `yunka generate`;
3. run contract lint/drift checks;
4. run module structure/topology validation when modules are present;
5. run C10 Assembly drift checks when assembly inputs are configured/resolvable;
6. avoid production-only heavy gates such as race, vulnerability scanning and MySQL pressure;
7. return a non-zero exit code on blocking drift or invalid structure.

## C11.1 configuration boundary

C11.1 may reuse the existing `.yunka/project.json` and Go module facts, but it must not prematurely expand the project schema into a second architecture model. If the current project config cannot express a required path without duplication, C11.1 may use conservative conventional defaults and defer the generalized profile model to C11.2.

Preferred defaults:

```text
contract source inventory  contracts/sources.json (when present)
canonical proto root       contracts/proto
contract generated output  contracts/generated
module root                modules
assembly Go output         internal/assembly
assembly Go import         <go module>/internal/assembly
```

Any automatic value must be derived only from explicit filesystem/Go module facts and must remain overridable by the existing expert commands.

## Expected change areas

Allowed/expected:

- `app/cmd/generate/**`;
- `app/cmd/check/**`;
- `app/cmd/yunka.go`;
- narrowly reusable orchestration helpers under `app/cmd/**` or a leaf-safe package;
- tests proving the facade delegates to canonical generation/check semantics;
- `docs/waves/**`.

Conditionally allowed only when required by a concrete blocker:

- small exported helper seams in `app/cmd/contract/**` or `app/cmd/assembly/**` that remove duplicate orchestration logic without changing semantics;
- small project-config read helpers.

Forbidden in C11.1:

- changes to authorization semantics;
- changes to OperationPlan or Executor semantics;
- changes to `ExecutionScope`, UoW, transaction or idempotency semantics;
- changes to Runtime Assembly ownership;
- changes to Platform lifecycle ownership;
- new reflection/service-locator composition;
- new runtime supervision/readiness semantics;
- generated business logic.

## Validation gate

C11.1 is complete only when:

- `yunka generate` is a top-level command;
- `yunka check` is a top-level command;
- both commands work from a repository root without repeating routine compiler flags for the representative fixture;
- generated artifacts are identical to the existing expert command path;
- `yunka generate` followed by a second `yunka generate` produces zero tracked drift;
- `yunka check` is read-only and passes on the generated tree;
- intentional contract or assembly drift is rejected;
- existing `yunka contract ...`, `yunka assembly ...`, `yunka module ...`, and `yunka dev ...` behavior remains compatible;
- architecture/contract/runtime qualification tests remain green;
- no C7-C10 semantic owner is duplicated.

## Primary risks

### Risk 1 — convenience facade becomes a second compiler

**Control:** facade code must call the existing canonical compiler/generator/check implementations or extracted shared helpers; semantic reimplementation is forbidden.

### Risk 2 — conventional defaults become hidden architecture inference

**Control:** defaults are restricted to filesystem/output locations and Go module import derivation. Business/Application/Operation/module dependency facts remain explicit canonical inputs.

### Risk 3 — `yunka check` becomes as slow as production verification

**Control:** C11.1 explicitly limits `check` to developer-loop structural/drift validation. `make verify` and `make verify-production` remain higher assurance gates.

## Rollback

C11.1 is additive. Remove the top-level facade commands and their orchestration helpers; all existing expert commands and C10 runtime behavior remain unchanged.

---

# C11.2 — Project Profile

Centralize developer-workflow defaults in a narrowly scoped project profile so routine commands do not require repeated path/import flags. The profile may locate canonical facts but must not duplicate them.

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
