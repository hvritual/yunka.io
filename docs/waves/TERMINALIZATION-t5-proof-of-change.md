# Terminalization T5 Complete Proof-of-Change

> Document class: **ROADMAP**  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Integration state at this record: **T5 started; T5.1 implementation candidate not yet qualified or merged**

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

The implementation must reuse the existing deterministic Audit implementation and `auditcore.DebtDelta`; it must not shell out to another Yunka CLI process and must not create a second debt model.

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

### T5.1 qualification

Required framework gates:

```text
make verify
make verify-production
```

Required adversarial proof:

```text
clean base -> new PROVEN_VIOLATION
    => FAIL

violating base -> violation fixed, no new debt
    => PASS architecture-debt gate

existing unrelated historical debt unchanged
    => reported as existing, not newly blocking

repeated machine rendering
    => deterministic
```

A real-consumer reverse qualification is required before T5.1 is promoted to complete.

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
