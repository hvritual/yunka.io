# Yunka Current Status

> Document class: **STATUS**  
> Authority: current framework/wave/release status  
> Last reconciled repository baseline: `main@24089df35f945abb42ddb00731283f4d878d8424`  
> Governance: [`DOCUMENTATION_GOVERNANCE.md`](DOCUMENTATION_GOVERNANCE.md)

## Current summary

Yunka has no active numbered framework wave after the completed C11 Developer Experience Productization and the subsequent five-gap DX convergence. The current merged baseline is `main@24089df35f945abb42ddb00731283f4d878d8424`.

| Area | Current state | Qualified / release evidence | Relationship to current `main` |
| --- | --- | --- | --- |
| C9 Operation Contract / execution closure | **Complete / merged** | C9.9 merge `e091baff2730e04b402710606f655b3a9cd630b7` | Inherited by later qualified releases |
| C10 Runtime Assembly & Framework Productization | **Complete / qualified / merged** | qualified release head `4c4beb72ffde79d97a28cf5c3b083c756b729d27`; post-merge release record in `PROJECT_MEMORY.md` / `C10.5-release-qualification.md` | Inherited by C11 and current `main` |
| C11 Developer Experience Productization | **Complete / production-qualified** | final qualified C11 head `52b67bc0cc78ebad4367b255f2a25bce4e729159`; issue #60 closed complete | Base of post-C11 convergence |
| Post-C11 five-gap DX convergence | **Complete / qualified / merged** | qualified candidate `a0cf52d2fdc8e5fd2b361b8c2640dfa60184b1b4`; PR #104 merge `24089df35f945abb42ddb00731283f4d878d8424` | **Current `main`** |
| Active next framework wave | **None selected** | — | New work must start as a separately scoped, pressure-driven task/wave |

## Current developer workflow

The current correct-by-default developer path is:

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

After PR #104, top-level generation/check owns the canonical project closure, including managed Domain generation/check, standard protobuf Go/gRPC generation/checking, Provider preflight, Contract, Module, and Assembly stages. `yunka init` idempotently adopts the safe managed project shape for new/migrated projects. Qualified legacy consumers retain explicit compatibility/adoption boundaries rather than receiving silently changed runtime semantics.

## Release evidence currently inherited

### C10

- C10 is complete, qualified, and merged.
- Original qualified release source head: `4c4beb72ffde79d97a28cf5c3b083c756b729d27`.
- The C10 release preserves one canonical C9 Executor / `ExecutionScope`, one `core.App` lifecycle owner, and Platform Provider ownership.
- `docs/waves/C10-runtime-assembly-framework-productization.md` is the original historical roadmap, not current status authority.

### C11

- C11 issue #60 is closed as complete.
- Final qualified C11 head: `52b67bc0cc78ebad4367b255f2a25bce4e729159`.
- Final qualified real consumer: `hvritual/biz@df3a8ed8d0bf1c1eb0e75889675bd7291b0dd1d8`.
- C11 preserved protobuf as the writable contract DSL, OperationPlan/AssemblyPlan as derived projections, the canonical Operation Executor, root `ExecutionScope`, `core.App`, and Platform Provider semantics.

### Post-C11 DX convergence

PR #104 closed the five remaining developer-workflow gaps without relaxing compiler/runtime/security/transaction/idempotency semantics.

- Qualified candidate: `a0cf52d2fdc8e5fd2b361b8c2640dfa60184b1b4`.
- Merge commit/current main: `24089df35f945abb42ddb00731283f4d878d8424`.
- Qualified real consumer: `hvritual/biz@df3a8ed8d0bf1c1eb0e75889675bd7291b0dd1d8`.
- Qualification recorded by PR #104: CI/full verify/determinism PASS, production/MySQL 8.4/clean worktree PASS, and C11.7-C real-consumer PASS.

## Known deferred pressure

These are not current release blockers but remain explicit deferred facts:

- `FP-C9-005` — safe remote Saga topology evidence in the Application Graph: **OPEN / DEFERRED**.
- Durable idempotency duplicate suppression is qualified; response/result replay remains outside the current idempotency contract unless future pressure justifies a separate mechanism.

A deferred item does not become an active framework wave merely because it is listed here. It requires new pressure/scope selection.

## Status authority rules

- Use this file for answers to "what is currently complete, active, qualified, or deferred?"
- Use `PROJECT_MEMORY.md` for durable architecture/governance invariants.
- Use `README.md` and current authoring guides for current developer-facing behavior.
- Use release/qualification records for exact evidence.
- Treat historical roadmap status blocks in `docs/waves/**` as historical snapshots unless their classification explicitly says they are current.

When `main` or the selected active wave changes, reconcile this file in the same release/governance flow rather than duplicating current status across historical roadmaps.
