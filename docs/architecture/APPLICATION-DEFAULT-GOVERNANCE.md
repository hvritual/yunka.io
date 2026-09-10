# AG-06.2 — Default Application governance workflow

> Document class: **CONTRACT**
> Current state authority: [`../STATUS.md`](../STATUS.md)
> Parent: [#178](https://github.com/hvritual/yunka.io/issues/178)
> Delivery task: [#180](https://github.com/hvritual/yunka.io/issues/180)

## Purpose

AG-06.1 proved the sealed-v1 Application implementation layout. AG-06.2 makes the
same layout and existing AG-04/AG-05 checks part of the ordinary authoring path so
developers and agents do not have to guess paths or hand-copy governance facts.
It does not introduce a second Application/Operation registry. Canonical business
identity remains in the existing contract/compiler/Operation graph; this contract
only defines deterministic physical projection and default developer-owned policy.

## Initialized source-policy baseline

For a Go project, `yunka init` creates `.yunka/source-policy.json` if it is missing.
It never overwrites an existing file. Projects without `go.mod` do not receive a
policy that could incorrectly claim Go coverage.

The baseline is deliberately broad and non-semantic:

- one `linux/amd64`, `CGO=false` profile;
- the root Go module with workspace disabled;
- one production component rooted at `.` with external imports allowed;
- no exclusions and no test-support exceptions.

This is an inventory/completeness baseline, not a domain/layer policy. Teams may
refine it explicitly. `yunka audit source --root . --format agent-json` uses this
initialized path when `--policy` is omitted; an explicit `--policy` remains fully
supported. Missing/invalid policy or incomplete analysis still fails closed.

## Context and ownership

`yunka context` schema v7 exposes:

- location `source-policy` -> `.yunka/source-policy.json`;
- the read-only implementation-plan command;
- the explicit implementation-apply command;
- the default source-audit command.

`yunka ownership` classifies the initialized policy as editable
`developer-governance`. This does not authorize source code or generated artifacts.

## One derived sealed-v1 layout

`projectflow.DescribeImplementationLayout` is the shared physical projection used
by the starter and change planner. For canonical Domain/Application/Go-method facts
it derives:

```text
<generatedGoRoot>/<domain>/application/<application>/
  build.go
  architecture.types.json
  internal/usecase/<lower-method>_handler.go
```

The function is not persisted and owns no Application identity. Generated ports
remain where the compiler already owns them. Business-key grammar remains owned by
contract lint; this projection only rejects values that cannot form contained Go
paths.

## Change planning

For an existing sealed-v1 Application, `yunka change plan` uses canonical
`applicationMethod` to point at the exact existing handler. It does not ask an
agent to invent a filename. Ownership must still confirm that target is editable.

The planner fails closed when:

- only one of `build.go` / `architecture.types.json` exists;
- the sealed Application exists but the canonical handler is absent;
- the expected files are symlinks or non-regular files;
- canonical physical layout cannot be derived.

For `intent=both`, canonical contract-source re-resolution replaces only provisional
contract targets; it must retain a separately proven sealed handler target.

Applicable gates are derived from actual project state:

- existing ownership/generate/check/runtime gates;
- default source audit when `.yunka/source-policy.json` exists;
- type audit when the sealed Application type policy exists;
- Go tests when the project has a Go module.

No gate is treated as evidence before it runs.

## Ordinary Operation additions

`yunka add operation` detects the existing sealed-v1 marker pair before mutation.
If sealed governance exists, it mutates the canonical contract only and does **not**
create a competing legacy flat implementation landing file. Next actions point to
existing type/source checks. A partial marker pair is rejected before contract
mutation.

Projects without a sealed starter retain the legacy landing behavior for backward
compatibility. AG-06.2 therefore does not force existing consumers into a migration.

## Boundaries

AG-06.2 does **not**:

- infer business layers/components into source policy;
- create a second Application registry or path manifest;
- rewrite existing handlers into sealed layout;
- generate dependency-bearing/composed Application implementations;
- change Runtime, Executor, authorization, UoW, database, transport or generated
  contract semantics;
- certify all build profiles from the default single-profile baseline.

Typed dependency templates and consecutive contract-evolution qualification remain
AG-06.3. Structural migration/debt growth remains AG-07.

## Acceptance

The permanent AG-06 template workflow must execute both AG-06.1 and AG-06.2 named
regressions. AG-06.2 requires positive/negative evidence for init idempotency and
preservation, context discovery, ownership, default source-policy lookup, shared
layout, exact change-plan handler/gates, fail-closed partial state, intent=both
preservation, and sealed/legacy add-operation compatibility.

Exact candidate CI, Production/MySQL, AG-05 source-policy and AG-06 qualification
must pass before non-force integration. The same applicable four gates must run
against actual `main` before this increment is marked integrated. External code
review is not a required gate under the current user-selected repository policy.
