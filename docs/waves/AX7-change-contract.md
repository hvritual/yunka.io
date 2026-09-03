# AX7 — Change Contract and Real-Agent Qualification

> State: AX7.1-AX7.4 production-qualified candidate on PR #138; AX7.5 adversarial pressure in progress
> Issue: #136
> Baseline: `main@5bcc8cb0765516aabd6e70d489c0cca2188390c8`

## Goal

Converge AX1-AX6 into one deterministic Agent change protocol that constrains what may change, reconciles that declaration with the actual Git delta and canonical Yunka semantic delta, and emits machine-readable evidence before merge.

AX7 is a developer/control-plane capability. It does not introduce a second compiler, runtime, business DSL, ownership manifest, or production request-path behavior.

## Ordered delivery

1. **AX7.1 — Change Contract**
   - **QUALIFIED CANDIDATE** on PR #138
   - `yunka change begin` resolves one existing canonical Operation
   - captures base Git identity plus explicitly allowed semantic categories
   - derives editable/generated targets from existing AX4/AX2 facts
   - requires a clean worktree before the baseline is recorded
   - does not copy canonical Manifest/OperationPlan/AssemblyPlan/Application Graph into a second Source of Truth
2. **AX7.2 — Actual Git Delta Reconciliation**
   - **QUALIFIED CANDIDATE** on PR #138
   - `yunka change check` derives tracked and untracked changed paths from Git relative to the recorded base
   - reconciles actual delta against declared editable scope and AX2 ownership
   - broad generated-impact scopes do not authorize handwritten files: AX2 must independently classify a concrete path as `generated-only`
   - fast path remains delta-oriented; no runtime/integration qualification
3. **AX7.3 — Canonical Semantic Delta Reconciliation**
   - **QUALIFIED CANDIDATE** on PR #138
   - compares normalized base/current OperationPlan plus target Application declaration facts
   - rejects undeclared permission, tenant, authentication, transaction, idempotency, composition, dependency, capability, transport, or target contract changes
   - any semantic change to an unrelated Operation/Application is a scope violation
   - uses structured canonical records rather than text diffs of generated artifacts
4. **AX7.4 — Unified Verify and Attestation**
   - **PRODUCTION-QUALIFIED CANDIDATE** on PR #138
   - `yunka change verify` composes Git scope/ownership reconciliation, canonical full `yunka check`, semantic reconciliation, and Go tests when applicable
   - emits `.yunka/change-attestation.json` plus machine-readable diagnostics
   - does not introduce Architecture Delta analysis; that remains pressure-driven
5. **AX7.5 — Real-Agent adversarial pressure qualification**
   - **IN PROGRESS**
   - intentionally induces generated-file edits, scope expansion, security/transaction drift, undeclared capability/composition changes, unrelated semantic edits, and code-organization escape attempts
   - only pressure-proven generic gaps may be promoted into framework work

## Performance constraints

- production runtime hot-path overhead: **0**
- `yunka change check`: delta-oriented; no full runtime/integration qualification
- `yunka change verify`: may invoke canonical full checks, but reconciliation itself reuses canonical models/readback rather than establishing another repository model
- baseline debt is tolerated; AX7 rejects **new undeclared delta**, not every historical architectural imperfection

## AX7.1-AX7.4 qualification evidence

Exact qualified candidate: `1adb35bfc3840beb3480a11e71d0e2bfc0ec24af` on PR #138.

- CI #482 / run `33747031578`: PASS, including toolchain lock, dependency drift, contract reproducibility, contract compatibility, full Verify, and determinism recheck.
- production #240 / run `33747031547`: PASS on MySQL 8.4, including clean-worktree verification.

This evidence qualifies AX7.1-AX7.4 as an unmerged candidate. It does not yet close AX7: AX7.5 pressure must run against the qualified control-plane behavior before PR #138 can become the merge candidate.

## Documentation reconciliation

`docs/STATUS.md` remains the current status authority and must distinguish merged AX1-AX6 from the unmerged AX7.1-AX7.4 qualified candidate plus active AX7.5 pressure. Before PR #138 merges, both this record and `docs/STATUS.md` must be reconciled to the exact pressure result and final candidate evidence.
