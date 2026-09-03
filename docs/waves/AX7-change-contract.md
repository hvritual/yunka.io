# AX7 — Change Contract and Real-Agent Qualification

> State: AX7.1 implementation in progress
> Issue: #136
> Baseline: `main@5bcc8cb0765516aabd6e70d489c0cca2188390c8`

## Goal

Converge AX1-AX6 into one deterministic Agent change protocol that constrains what may change, reconciles that declaration with the actual Git delta and canonical Yunka semantic delta, and emits machine-readable evidence before merge.

AX7 is a developer/control-plane capability. It does not introduce a second compiler, runtime, business DSL, ownership manifest, or production request-path behavior.

## Ordered delivery

1. **AX7.1 — Change Contract**
   - add `yunka change begin`
   - resolve an existing canonical Operation
   - capture base Git identity plus allowed semantic categories
   - derive editable/generated targets from existing AX4/AX2 facts
   - do not copy canonical Manifest/OperationPlan/AssemblyPlan/Application Graph into a second Source of Truth
2. **AX7.2 — Actual Git Delta Reconciliation**
   - add `yunka change check`
   - derive changed paths from Git relative to the recorded base
   - reconcile the actual delta against declared editable scope and AX2 ownership
   - keep the fast path delta-oriented; no runtime/integration qualification
3. **AX7.3 — Canonical Semantic Delta Reconciliation**
   - compare before/after canonical contract facts
   - reject undeclared permission, tenant, authn, transaction, idempotency, composition, dependency, capability, or transport changes
   - prefer stable normalized semantic records instead of text diffs of generated artifacts
4. **AX7.4 — Unified Verify and Attestation**
   - add `yunka change verify`
   - compose scope/ownership/semantic reconciliation with existing generate/check verification gates
   - emit stable agent diagnostics and a machine-readable attestation
   - no Architecture Delta analyzer unless pressure proves a remaining generic gap
5. **AX7.5 — Real-Agent adversarial pressure qualification**
   - starts only after AX7.4 qualifies
   - intentionally induce generated-file edits, scope expansion, security/transaction drift, undeclared capability/composition changes, and unrelated refactors
   - promote only pressure-proven generic gaps

## Performance constraints

- production runtime hot-path overhead: **0**
- `yunka change check`: delta-oriented; no full runtime/integration qualification
- `yunka change verify`: may invoke canonical full checks, but reconciliation itself must reuse canonical in-memory models and fingerprints rather than rescan or duplicate the repository model
- baseline debt is tolerated; AX7 rejects **new undeclared delta**, not every historical architectural imperfection

## Documentation reconciliation

`docs/STATUS.md` must reflect the exact AX7 phase that has qualified on the branch/main state. No phase may be marked complete before its executable qualification evidence exists.
