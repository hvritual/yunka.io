# R0 — C9.8 Release Closure

## Status

- State: **Release qualification / validation**
- Framework baseline: `main@407193d0b53f5fdbe2aad5c4ab152aba92d61097`
- Baseline tree: `d2454c393e9c19379a71980154d4e354bb4fce22`
- Tracking: GitHub issue #36
- Delivery branch: `agent/r0-c9-8-release-closure`
- Delivery PR: #37
- Planned release candidate: `v0.9.0-rc.1`

## Objective

R0 freezes C9.8 as the first release-candidate framework baseline before any C10 or other framework-surface extension begins.

R0 is not a feature wave. It closes release evidence, makes production verification durable, records the final C9 architecture boundary, removes stale control-plane state, and publishes an RC only after the exact release tree has passed all required gates.

## Frozen architecture

The release candidate is based on the C9 execution model:

```text
PB DSL
  -> Contract Compiler
  -> immutable OperationPlan
  -> Unified Operation Executor
       -> metadata
       -> security
       -> idempotency begin
       -> root ExecutionScope / UoW
       -> Application / typed child Operations
       -> Saga / Outbox staging when declared
       -> transaction finalize
       -> idempotency finalize
       -> outcome / diagnostics / graph evidence
```

C9.8 additionally establishes that a canonical business Operation does not require an RPC/HTTP binding. Application-level internal Operations remain first-class OperationPlan/Application Graph facts and may be exposed only through generated typed local child capabilities.

## R0 architecture freeze

Until R0 closes, do not add:

- C10 functionality;
- new `ExecutionPolicy` mechanisms;
- generic Customer/Site/Device scope taxonomies in the framework;
- generic SQL/data-scope DSLs;
- workflow/BPMN engines;
- distributed transaction/2PC mechanisms;
- business-semantic inference from HTTP verbs, method names, package names, or transport bindings.

A code change during R0 is allowed only when an actual closure gate proves that the release baseline is incorrect or incomplete.

## Evidence already established

### C9.7 production closure

GitHub Actions run `33173605998` passed the C9.7 production gate on MySQL 8.4, including full `make verify-production`.

### C9.8 real business pressure

GitHub Actions run `33176909258` passed the real `hvritual/biz` cross-Application MySQL 8.4 pressure slice. The pressure result closed the shared ExecutionScope/UoW mechanism and proved generated child Operation composition without a framework bypass.

Pressure disposition at the C9.8 boundary:

- `FP-C9-006`: closed by the second real cross-Application business slice;
- `FP-C9-001`: resolved by canonical application-level internal Operations with optional transport bindings;
- `FP-C9-005`: intentionally deferred because no repeated Saga-topology pressure justified new framework surface.

### Final C9.8 source evidence

The final PR head `397b65b6da7abb665c91effaa685f4e7b4cdd937` and merged `main@407193d0b53f5fdbe2aad5c4ab152aba92d61097` share the same product tree `d2454c393e9c19379a71980154d4e354bb4fce22`.

The final cleanup after the C9.8 production-verification candidate removed only temporary validation workflows/build helpers and the temporary C9.8 branch trigger. It did not change framework/product source semantics.

## Current hosted-runner validation debt

The exact final C9.8/R0 tree has not yet received a successful hosted-runner closeout because GitHub-hosted jobs are currently failing before any workflow step executes.

Observed during R0:

- final C9.8 PR CI run `33187074493`, attempt 2: `verify` failed with `steps=null`;
- final C9.8 `main` CI run `33187119857`, attempt 2: `verify` failed with `steps=null`;
- C9.8 production run `33185292431`, attempt 2: `verify-production` failed with `steps=null`;
- R0 PR-head CI run `33201911506`: explicit re-run still failed with `steps=null`;
- R0 PR-head production run `33201911509`: explicit re-run still failed with `steps=null`;
- `hvritual/biz` C9.8 source-export run `33185310303`, attempt 3: `export` failed with `steps=null`.

The workflow files are registered and runs/jobs are created, but no executable step/log is produced. These are unresolved release gates. They are not treated as passing evidence and are not interpreted as code-test failures because no test or setup step executed.

GitHub public status currently reports Actions operational. Its recent incident history nevertheless records Aug 24-27 failures/delays in Actions job startup. Private repositories can also be blocked before runner execution when Actions billing/minute/budget entitlement is exhausted. Repository tooling available to R0 cannot read the account billing state, so the present root cause remains classified only as an external runner/Actions-entitlement blocker rather than a proven billing or GitHub-service incident.

## Durable production gate

R0 replaces the historical branch-specific `c9-production.yml` with `.github/workflows/production.yml`.

The durable production workflow:

- runs for pull requests targeting `main`;
- runs for pushes to `main`;
- supports explicit `workflow_dispatch` revalidation;
- uses `contents: read` only;
- uses the repository-locked Go and protoc toolchain;
- runs against MySQL 8.4;
- executes the canonical `make verify-production` target;
- requires a clean worktree after verification.

Normal `.github/workflows/ci.yml` remains the deterministic read-only standard gate. R0 therefore establishes two permanent evidence classes:

```text
ci
  -> dependency/contract determinism
  -> architecture/security/operation gates
  -> test/race/vet/vuln/build

production
  -> everything in make verify
  -> real MySQL integration
  -> clean worktree
```

## GitHub repository-policy limitation

At R0 start, repository ruleset access for this private personal repository returns GitHub's platform error requiring GitHub Pro or a public repository. `main` must therefore not be represented as platform-protected when that capability is unavailable.

Until the repository/account supports required-check rulesets, release governance is enforced through:

1. the read-only `ci` workflow;
2. the read-only `production` workflow;
3. the R0 release checklist;
4. no RC tag/release before exact-tree gates are green.

If repository rulesets later become available, required `ci` and `production` checks should be promoted into the platform-level branch policy without changing their semantic gates.

## GitHub control-plane cleanup

R0 treats GitHub state as machine-readable project context. Stale implementation trackers must not remain open after their work is present in `main`.

At R0 start the following were closed as completed/superseded:

- C2 toolchain determinism;
- C7 explicit composition and C7.1/C7.2 trackers;
- C8.3 Gateway authorization convergence;
- the corrected Outbox SKIP LOCKED stress-test issue;
- C9 implementation tracker;
- C9.8 real cross-Application pressure tracker;
- the obsolete generated use-case policy PR whose framework-level resource taxonomy conflicts with the current domain-owned opaque-scope boundary.

Issue #36 is the release-closure tracker and PR #37 is the isolated R0 delivery branch review surface.

## Completion gates

R0 is complete only when all of the following are true on the exact release tree:

1. `ci` is green.
2. `production` is green on MySQL 8.4.
3. `make verify` and `make verify-production` introduce zero dependency/generated/worktree drift.
4. The real `hvritual/biz` C9.8 consumer regenerates and passes its cross-Application pressure suite against the exact Yunka release tree.
5. Internal-only Operations remain absent from external OpenAPI/TypeScript transport projections unless reachable from a real externally bound service method.
6. No Application/business code bypasses the unified Executor to own authorization, transaction lifecycle, or Operation-level idempotency.
7. R0 documentation and durable project memory match the released architecture.
8. No known release-blocking verification debt remains.
9. Only then may `v0.9.0-rc.1` be tagged/released.

## Release candidate boundary

`v0.9.0-rc.1` freezes the reviewed shape of:

- protobuf DSL declarations used by the current framework;
- Contract Manifest / OperationPlan derived evidence;
- Application Port and generated child-capability conventions;
- Unified Executor semantic ordering;
- Gateway authorization ownership;
- Module Catalog / Platform Provider / RequestScope / ExecutionScope ownership boundaries;
- standard RPC runtime and deterministic contract generation;
- read-only CI and production verification semantics.

The RC does not promise v1 API stability. Breaking changes after the RC require explicit compatibility/migration treatment rather than silent generator/runtime drift.

## Rollback

R0 itself should not alter C9.8 product semantics. If the new durable workflow or documentation is incorrect, revert the R0-only change. If an actual validation gate exposes a product defect, fix the smallest proven defect on the R0 branch, rerun the complete closure gates, and document the changed release tree before tagging.
