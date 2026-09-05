# Terminalization T5 Complete Proof-of-Change

> Document class: **ROADMAP**  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Integration state at this record: **T5.1 behavior complete / production-qualified / real-consumer pressure-qualified / integration pending**

## Goal

T5 closes the remaining gap between bounded mutation and a machine-verifiable final Proof-of-Change. It extends the existing AX7/T0-T4 control plane; it does not create another compiler, runtime, authorization path, transaction owner, architecture Source of Truth, or business-semantic inference layer.

The terminal proof model is:

```text
Mutation Proof
Ownership Proof
Placement Proof
Semantic Proof
Audit / New-Debt Proof
Generated Proof
Behavior Proof
Runtime Proof when explicitly required
```

The first and highest-priority closure is to make architecture debt delta part of the existing Change Attestation.

## T5.1 — Architecture debt proof in Change Attestation

`yunka change verify` already composes Git scope/ownership/placement reconciliation, canonical `yunka check`, semantic reconciliation, and `go test ./...` when applicable.

T5.1 adds the T2 debt comparison on the immutable Change Contract base:

```text
yunka audit --base ChangeContract.BaseSHA
```

The implementation reuses the existing deterministic Audit implementation and `auditcore.DebtDelta`; it does not shell out to another Yunka CLI process and does not create a second debt model.

The Change Attestation machine contract is extended with:

```json
{
  "architectureDebt": {
    "existing": [],
    "new": [],
    "fixed": []
  }
}
```

Conformance rule:

```text
new proven violation > 0
    -> architecture-debt gate FAIL
    -> Change Attestation Conformant = false

existing historical proven debt only
    -> reported, not newly blocking

fixed proven debt
    -> reported, not blocking
```

Only `proven_violation` findings participate in the T2 debt delta. Evidence observations are not promoted into merge-blocking debt.

Audit/New-Debt Proof runs only after canonical `yunka check` succeeds. If canonical generated/contract evidence is invalid, the architecture-debt gate is skipped because the Audit result would not be trustworthy; the failed canonical check already makes the attestation non-conformant.

### T5.1 code boundary

Allowed implementation surface:

```text
app/cmd/change/**
docs/STATUS.md
docs/waves/TERMINALIZATION-t5-proof-of-change.md
related tests
```

Explicitly forbidden for T5.1 unless separate real consumer pressure proves a generic gap:

```text
framework runtime semantics
framework/operation Executor semantics
gateway/authz ownership or policy semantics
root ExecutionScope / UoW semantics
protobuf business DSL
persistence semantics
second architecture Source of Truth
```

## Qualification record

### Stop-condition closure before qualification

The first B13 replay exposed a real compiler/DX compatibility gap rather than a T5 debt-rule failure: historical consumers could contain both `yunka.io/{framework,gateway,pkg}` and canonical `github.com/hvritual/yunka.io/{framework,gateway,pkg}` identities, and the historical vendored Yunka DSL could recreate the legacy identity through protobuf `option go_package`.

T5 was paused under the Terminalization stop condition. The gap was tracked as issue #152 and fixed independently by PR #154. The minimal closure added explicit module-identity inspection/migration for Go imports, Go module/workspace tokens, and protobuf `go_package`, plus fail-closed `generate/check` preflight. It did not expand protobuf business semantics or runtime/compiler ownership. Exact #152 candidate `5ebb03c739829e3ae960b31d00bd5d9040c82f7e` passed CI #578, production #336, and B13 module-identity Run #3. PR #154 merged as `ebb8184bd6ab5dea9f0fa8b3ed7d9c6d5f765d9a`; exact-main CI #579 and production #337 also passed.

The T5 patch was then replayed onto canonical `main@ebb8184bd6ab5dea9f0fa8b3ed7d9c6d5f765d9a` without scope expansion. Rebased behavioral candidate:

```text
a102de3c53f1078c8c6cce70e334c91fbbb94bb2
```

Relative to that canonical main it remains exactly four T5 files, 438 additions and 11 deletions.

### Framework qualification

Exact T5.1 behavioral candidate `a102de3c53f1078c8c6cce70e334c91fbbb94bb2` passed:

```text
CI #580          run 33954262081   PASS
production #338  run 33954262031   PASS
```

Production qualification includes the existing MySQL 8.4 gate and clean-worktree verification.

### Real B13 reverse qualification

Real-consumer qualification used preserved B13 pressure SHA:

```text
506e9c117822855db318f8b4b6689d318a62ded1
```

Successful qualification:

```text
workflow: terminalization-t5-proof-b13
run:      #9 / 33954431712
artifact: 9965889476
sha256:   8070e9876fc5395cf5be4ca82539feceb31b1fb69482c8ea148d0169260ce520
```

The qualification uses the canonical #152 migration path. It does not manufacture legacy alias modules, hand-edit generated files, or use `--skip-tests`.

It proved the following machine states on the real consumer:

```text
canonicalized B13
    module identity findings = 0
    yunka check              = PASS
    go test ./...            = PASS

NEW proven debt
    rule                     = AUDIT-INFRA-001
    git-delta                = PASS
    yunka-check              = PASS
    semantic-delta           = PASS
    architecture-debt        = FAIL (existing=0 new=1 fixed=0)
    go-test                  = PASS
    attestation conformant   = false

EXISTING proven debt
    rule                     = AUDIT-INFRA-001
    architecture-debt        = PASS (existing=1 new=0 fixed=0)
    go-test                  = PASS
    attestation conformant   = true

FIXED proven debt
    rule                     = AUDIT-INFRA-001
    architecture-debt        = PASS (existing=0 new=0 fixed=1)
    go-test                  = PASS
    attestation conformant   = true
```

The successful fixed-debt attestation was rendered twice from an unchanged state and was byte-identical. Both copies have SHA256:

```text
f49c5bdb2b90f04d432c51dc8c48f44dfc4497e3439ca7a8ea155529d71c586a
```

Pre-migration inspection/check remained read-only, the canonical qualification baseline was clean, and the workflow restored the consumer branch to a zero-dirty final status.

Framework unit coverage additionally proves the `AUDIT-AUTH-001` new-debt path. The B13 pressure intentionally uses `AUDIT-INFRA-001` so the architecture-debt gate is isolated from Biz's existing authorization-boundary test; `go test ./...` therefore stays PASS while the new-debt gate independently blocks the change.

### Qualification disposition

T5.1 behavioral implementation is now:

```text
COMPLETE
PRODUCTION-QUALIFIED
REAL-CONSUMER PRESSURE-QUALIFIED
INTEGRATION PENDING
```

The final PR head must still pass CI and production after this documentation reconciliation before integration. T5.1 is not recorded as merged until canonical integration and exact-main post-merge qualification complete.

## Behavior Proof

`go test ./...` remains the current generic behavior proof. T5 does not introduce a generic Test DSL merely to make the proof list look more complete.

## Runtime Proof

AX6 already exposes canonical runtime evidence through:

```text
yunka dev --event-format jsonl
```

T5 does not immediately make runtime proof mandatory for every change.

```text
structural-only change
    -> runtime proof optional

runtime-affecting change
    -> runtime proof required only after an explicit, deterministic classification contract exists
```

Whether a change is runtime-affecting must not be guessed by an Agent. A later T5 delivery may add an explicit ChangeSet classification if real pressure justifies it. Until then, runtime evidence attachment remains pressure-driven rather than a T5.1 requirement.

## External effects

External-effect completion remains outside generic Proof-of-Change. Trace/runtime evidence can establish technical execution and causality; it is not an authoritative external-effect receipt. Authoritative readback/reconciliation remains a consumer/domain concern unless later repeated pressure proves a generic framework contract.

## Stop condition

T5 work stops immediately and is reclassified as a separate framework-pressure item if completion requires any of the following:

```text
new Runtime semantics
new Executor path
new authorization ownership
new root transaction/UoW semantics
new protobuf business semantics
second architecture truth store
```

The smallest proven generic fix must then be qualified separately before T5 resumes.
