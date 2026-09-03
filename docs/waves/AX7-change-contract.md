# AX7 — Change Contract and Real-Agent Qualification

> State: AX7.1-AX7.5 production-qualified candidate on PR #138; merge pending
> Issue: #136
> Baseline: `main@5bcc8cb0765516aabd6e70d489c0cca2188390c8`

## Goal

Converge AX1-AX6 into one deterministic Agent change protocol that constrains what may change, reconciles that declaration with the actual Git delta and canonical Yunka semantic delta, and emits machine-readable evidence before merge.

AX7 is a developer/control-plane capability. It does not introduce a second compiler, runtime, business DSL, ownership manifest, or production request-path behavior.

## Ordered delivery

1. **AX7.1 — Change Contract**
   - **PRODUCTION-QUALIFIED CANDIDATE** on PR #138
   - `yunka change begin` resolves one existing canonical Operation
   - captures base Git identity plus explicitly allowed semantic categories
   - derives editable/generated targets from existing AX4/AX2 facts
   - requires a clean worktree before the baseline is recorded
   - does not copy canonical Manifest/OperationPlan/AssemblyPlan/Application Graph into a second Source of Truth
2. **AX7.2 — Actual Git Delta Reconciliation**
   - **PRODUCTION-QUALIFIED CANDIDATE** on PR #138
   - `yunka change check` derives tracked and untracked changed paths from Git relative to the recorded base
   - reconciles actual delta against declared editable scope and AX2 ownership
   - broad generated-impact scopes do not authorize handwritten files: AX2 must independently classify a concrete path as `generated-only`
   - pressure-proven placement closure: broad Application scopes authorize modification/deletion of existing handwritten files, but newly added/renamed/copied handwritten destinations require an exact `EditablePaths` declaration captured before mutation
   - fast path remains delta-oriented; no runtime/integration qualification
3. **AX7.3 — Canonical Semantic Delta Reconciliation**
   - **PRODUCTION-QUALIFIED CANDIDATE** on PR #138
   - compares normalized base/current OperationPlan plus target Application declaration facts
   - rejects undeclared permission, tenant, authentication, transaction, idempotency, composition, dependency, capability, transport, or target contract changes
   - any semantic change to an unrelated Operation/Application is a scope violation
   - uses structured canonical records rather than text diffs of generated artifacts
4. **AX7.4 — Unified Verify and Attestation**
   - **PRODUCTION-QUALIFIED CANDIDATE** on PR #138
   - `yunka change verify` composes Git scope/ownership reconciliation, canonical full `yunka check`, semantic reconciliation, and Go tests when applicable
   - emits `.yunka/change-attestation.json` plus machine-readable diagnostics
   - does not add a general Architecture Delta analyzer
5. **AX7.5 — Real-Agent adversarial pressure qualification**
   - **COMPLETE / PRESSURE-QUALIFIED CANDIDATE**
   - real temporary consumer is built through `yunka add application`, `yunka add operation`, declarative module authoring, canonical `yunka generate`, and a real Git baseline
   - pressure covers out-of-envelope handwritten files, generated-artifact tampering, tenant/permission/transaction drift, undeclared capability drift, unrelated Operation semantic drift, and same-Application code-organization escape attempts
   - only the pressure-proven generic placement escape was promoted; no AST/SSA/DDD analyzer or repository-wide Architecture Delta engine was added

## Pressure discovery and closure

The first strict placement pressure candidate `38036a66e0b264a87526f18aaa8494bea6355a28` intentionally preserved a failing case. CI #489 / run `33749552545` failed in `TestAX7PlacementPressureRequiresExplicitNewHandwrittenPath` with the exact evidence:

```text
path: internal/tenant/application/global_helper.go
status: A
class: editable
owner: developer-code
violations: []
```

This proved that AX2 ownership plus a broad Application editable scope answered “may this developer-owned path be edited?” but did not answer “may this task introduce this new architectural surface?”.

The minimal closure does not add a general architecture linter. Git reconciliation now distinguishes baseline placement from new placement:

```text
existing handwritten M/D inside declared Application scope
    -> eligible for normal AX2 ownership reconciliation

new A/R/C handwritten destination inside declared Application scope
    -> requires exact Change Contract EditablePaths evidence
    -> otherwise placement violation

explicit exact new path
    -> still must pass AX2 ownership
```

This keeps baseline debt tolerated, blocks new undeclared code-organization surface, and preserves an explicit escape hatch for planned new files.

## Qualification evidence

### AX7.1-AX7.4 initial candidate

Exact candidate `1adb35bfc3840beb3480a11e71d0e2bfc0ec24af`:

- CI #482 / run `33747031578`: PASS, including toolchain lock, dependency drift, contract reproducibility, contract compatibility, full Verify, and determinism recheck.
- production #240 / run `33747031547`: PASS on MySQL 8.4, including clean-worktree verification.

### AX7.5 final behavioral candidate

Exact behavioral candidate `45526e6f79f5e9b430f30f641080738e45c5b72a` includes the pressure-proven placement closure and both negative/positive placement qualification tests.

- CI #491 / run `33749854560`: PASS, including full Verify and determinism recheck.
- production #249 / run `33749854762`: PASS on MySQL 8.4, including clean-worktree verification.

The pressure suite also confirms that undeclared capability changes are rejected by AX7 semantic reconciliation rather than incorrectly requiring contract/assembly generation itself to reject a valid capability declaration.

## Performance constraints

- production runtime hot-path overhead: **0**
- `yunka change check`: delta-oriented; no full runtime/integration qualification
- placement closure uses Git change status plus exact-path membership; it does not scan Go AST/SSA or the whole repository
- `yunka change verify`: may invoke canonical full checks, but reconciliation itself reuses canonical models/readback rather than establishing another repository model
- baseline debt is tolerated; AX7 rejects **new undeclared delta**, not every historical architectural imperfection

## Documentation reconciliation

This record describes the unmerged PR #138 candidate. `docs/STATUS.md` must distinguish merged AX1-AX6 from the production-qualified, pressure-closed AX7 candidate until PR #138 merges. After merge, status may be advanced to AX1-AX7 merged only after the merged main ref is verified.
