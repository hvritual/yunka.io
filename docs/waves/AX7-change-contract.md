# AX7 — Change Contract and Real-Agent Qualification

> State: AX7.1-AX7.4 implementation candidate on PR #138; qualification pending; AX7.5 not started
> Issue: #136
> Baseline: `main@5bcc8cb0765516aabd6e70d489c0cca2188390c8`

## Goal

Converge AX1-AX6 into one deterministic Agent change protocol that constrains what may change, reconciles that declaration with the actual Git delta and canonical Yunka semantic delta, and emits machine-readable evidence before merge.

AX7 is a developer/control-plane capability. It does not introduce a second compiler, runtime, business DSL, ownership manifest, or production request-path behavior.

## Ordered delivery

1. **AX7.1 — Change Contract**
   - implementation candidate exists in PR #138
   - `yunka change begin` resolves one existing canonical Operation
   - captures base Git identity plus explicitly allowed semantic categories
   - derives editable/generated targets from existing AX4/AX2 facts
   - requires a clean worktree before the baseline is recorded
   - does not copy canonical Manifest/OperationPlan/AssemblyPlan/Application Graph into a second Source of Truth
2. **AX7.2 — Actual Git Delta Reconciliation**
   - implementation candidate exists in PR #138
   - `yunka change check` derives tracked and untracked changed paths from Git relative to the recorded base
   - reconciles actual delta against declared editable scope and AX2 ownership
   - broad generated-impact scopes do not authorize handwritten files: AX2 must independently classify a concrete path as `generated-only`
   - fast path remains delta-oriented; no runtime/integration qualification
3. **AX7.3 — Canonical Semantic Delta Reconciliation**
   - implementation candidate exists in PR #138
   - compares normalized base/current OperationPlan plus target Application declaration facts
   - rejects undeclared permission, tenant, authentication, transaction, idempotency, composition, dependency, capability, transport, or target contract changes
   - any semantic change to an unrelated Operation/Application is a scope violation
   - uses structured canonical records rather than text diffs of generated artifacts
4. **AX7.4 — Unified Verify and Attestation**
   - implementation candidate exists in PR #138
   - `yunka change verify` composes Git scope/ownership reconciliation, canonical full `yunka check`, semantic reconciliation, and Go tests when applicable
   - emits `.yunka/change-attestation.json` plus machine-readable diagnostics
   - does not introduce Architecture Delta analysis; that remains pressure-driven
   - **not qualified yet**: PR #138 CI/production evidence is required before this phase can be called complete
5. **AX7.5 — Real-Agent adversarial pressure qualification**
   - **not started**
   - starts only after AX7.4 qualifies
   - will intentionally induce generated-file edits, scope expansion, security/transaction drift, undeclared capability/composition changes, and unrelated refactors
   - only pressure-proven generic gaps may be promoted into framework work

## Performance constraints

- production runtime hot-path overhead: **0**
- `yunka change check`: delta-oriented; no full runtime/integration qualification
- `yunka change verify`: may invoke canonical full checks, but reconciliation itself reuses canonical models/readback rather than establishing another repository model
- baseline debt is tolerated; AX7 rejects **new undeclared delta**, not every historical architectural imperfection

## Qualification evidence

Current candidate branch: `ax7/change-contract` / PR #138.

No AX7.1-AX7.4 completion claim is made until the exact PR candidate passes repository qualification. After that evidence exists, this document and `docs/STATUS.md` must be reconciled before AX7.5 starts.

## Documentation reconciliation

`docs/STATUS.md` remains the merged framework-status authority and currently correctly reports AX7 as not started on `main`. This branch document records the unmerged candidate state. Before any AX7 candidate merges, `docs/STATUS.md` must be updated in the same delivery flow to match the exact qualified/pressure-tested state.
