# Bounded engineering-quality migration

> Document class: **CURRENT**
>
> Authority: bounded structural-migration protocol
>
> Delivery/status authority: [`../STATUS.md`](../STATUS.md)

## Purpose

Historical reviewability debt is migrated incrementally. A migration is not permission to rewrite a repository, change business behavior, weaken tests, or move handwritten behavior into generated ownership.

The migration contract composes existing Yunka authorities:

- source coverage remains owned by `.yunka/source-policy.json` and `sourceaudit`;
- deterministic findings and `existing / new / fixed` debt remain owned by Audit;
- engineering-quality blocking authority remains owned by `.yunka/engineering-quality.json`;
- semantic judgment remains human/advisory;
- Git source bytes and the immutable baseline remain the candidate facts.

No second finding registry or consumer-specific ruleset is introduced.

## Workflow

Before a structural refactor:

```text
yunka change review migration plan \
  --base HEAD \
  --recipe <recipe> \
  --path <exact-existing-or-planned-path> ... \
  --problem ... \
  --current-concept ... \
  --desired-ownership ... \
  --why ... \
  --what ... \
  --boundary ...
```

Planning requires a clean exact HEAD, PASS source coverage, a present engineering-quality policy, developer-owned targets, and the deterministic/advisory finding evidence required by the selected recipe.

After the bounded change:

```text
yunka change review migration check
```

The check produces the migration human-review packet. It reconciles all changed paths against the declared path set, re-runs source coverage, computes Audit debt against the immutable base, and proves structural deltas.

## Structural NONE proof

A migration passes automatically only when the review packet proves:

```text
behavior delta       = NONE
public API delta     = NONE
persistence delta    = NONE
generated-code delta = NONE
new blocking debt    = 0
```

`behavior = NONE` is deliberately conservative. Yunka parses developer-owned production Go syntax across the touched package directories, removes file-location and comment identity, canonicalizes declarations/imports, and compares the package-level production fingerprint before and after the change. Moving unchanged declarations between semantic files therefore remains `NONE`; changing executable production syntax does not.

This syntax proof supplements tests. It does not claim universal semantic equivalence for arbitrary refactors.

The public API fingerprint is independently compared. Persistence and generated deltas are derived from the exact Git reconciliation and existing path/ownership facts.

## Recipes

### `generic-container-split`

Use when an in-scope `AUDIT-NAME-002` observation identifies a low-information aggregate file and semantic concepts can be separated without changing declarations. The file split must preserve the production/API fingerprints.

Example evidence case: `hvritual/biz/internal/access/domain/model.go`. The framework does not special-case that path; the qualification applies the generic recipe to a temporary copy of the real consumer.

### `durable-test-rename`

Use when `AUDIT-NAME-001` identifies durable test identity derived from delivery/task history. Rename the test/file by the behavior or invariant it proves. Production/API fingerprints must remain unchanged and the historical naming finding must move to `fixed` debt.

### `package-documentation`

Use for in-scope `AUDIT-DOC-001` / `AUDIT-DOC-002`. Adding durable package/contract documentation must not alter production syntax, public API, persistence, or generated ownership; the deterministic documentation finding must become fixed.

### `abstraction-simplification`

Use to bound a human/advisory finding about an unjustified abstraction. The recipe does not promote semantic judgment into deterministic authority. If simplifying the abstraction changes production syntax, the automatic `NONE` proof fails; use a separately authorized behavior-changing change contract rather than weakening the migration gate.

## Coverage first

If baseline or candidate source coverage is not explicit and PASS, migration stops. Coverage policy changes are not smuggled into the structural refactor. Establish or repair governance coverage first, then start a new migration baseline.

## Touched-scope rule

The plan declares exact existing and planned paths. New paths must share a package directory with an existing developer-owned target. Any changed path outside the declared set is a violation.

Historical debt outside the touched scope remains visible as historical debt. The migration does not require repository-wide cleanup. New blocking debt is never grandfathered because similar historical debt exists.

## Review packet

The packet exposes:

- immutable base and current HEAD;
- plan digest, recipes, touched and changed paths;
- baseline finding inventory;
- behavior/API/persistence/generated deltas;
- deterministic debt counts and blocking-new count;
- source-policy identity and coverage status;
- WHY / WHAT / BOUNDARY / PROOF;
- fail-closed violations and conformance.

Relevant framework/consumer tests and ordinary CI/Production remain required delivery evidence. The migration packet is review evidence, not merge authority.

## Consumer qualification

Qualification must use the same contract without repository-specific checker logic:

- Biz: split the real access-domain aggregate into semantic files in a temporary clone and run the package tests plus migration check.
- IoT Delivery: rename the historical task-named SQLite startup regression in a temporary clone and run the package test plus migration check.

The original consumer checkouts remain read-only. Consumer paths exist only in qualification fixtures; the migration engine itself contains no Biz/IoT path rules.

## Source-root and qualification boundaries

`plan --root <project> --coverage-root <ancestor>` supports a project whose
local Go dependencies live beside it. A relative coverage root is resolved
from the project root, and the selected ancestor must contain the project.
The plan stores project-relative coverage identity, not an absolute runner
or workstation path. `check --root <project>` reconstructs that same coverage
root from the plan; it does not accept a replacement coverage policy.

The recurring qualification uses these immutable source pairs:

| Case | Consumer commit | Runtime commit |
| --- | --- | --- |
| Biz access-domain aggregate split | `3519e7ee6e51e33984669871e4f32a55a3597d9f` | `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` |
| IoT Delivery SQLite regression rename | `bcd20632b666405c3f7a8fe8d53f591c78450087` | `057ebcf88a87303eb633eb6e604d306f633dfac0` |

Biz uses a disposable exact checkout and sibling runtime. IoT uses an exact
current-module projection of `backend-yunka` and the pinned runtime at
`third_party/yunka`, with relative-file/mode/content digests checked against
the original sources before governance setup. It does not claim full legacy
IoT repository source-policy PASS. The source policy and explicit domain
ownership are established and committed before a new migration baseline.
Dependency-cache preparation also precedes the offline source audit and must
leave source unchanged.

The workflow `.github/workflows/quality-migration-qualification.yml` supplies
all five inputs to `TestQualityMigrationRealConsumersUseSameContract`:
`YUNKA_MIGRATION_FRAMEWORK_ROOT`, `YUNKA_MIGRATION_CONSUMER_BIZ`,
`YUNKA_MIGRATION_BIZ_RUNTIME`, `YUNKA_MIGRATION_CONSUMER_IOT`, and
`YUNKA_MIGRATION_IOT_RUNTIME`. Partial configuration fails; an ordinary test
run with all five unset skips the real-consumer fixture and is not evidence
of real-consumer qualification. The configured workflow runs
`go test ./cmd/change -run '^TestQualityMigration' -count=1 -v` from `app`.

Consumer mutations remain inside disposable copies. The engine's NONE syntax
proof is bounded review evidence, not proof of arbitrary semantic equivalence
or permission to weaken tests. Relevant consumer tests and original-checkout
cleanliness are independent assertions, including command failure propagation
in the workflow's Git readback checks.
