# Branch baseline qualification

> Document class: **CURRENT**  
> Tracking: [Issue #228](https://github.com/hvritual/yunka.io/issues/228)  
> Current status authority: [STATUS.md](STATUS.md)

## Contract

`tools/qualify_branch_baseline.py` is a read-only Git identity evaluator.
It reports classification and required downstream gates, not dependency repair,
merge approval, completed CI, or universal security of the checked code.

| Classification | Meaning |
| --- | --- |
| `CURRENT_MAIN` | The PR targets main, its exact base equals the fetched live main, and checked HEAD descends from that base. |
| `MAINTAINED_COMPAT` | A non-main base has an explicit maintenance record, responsible owner, active support horizon, reachable security-backport commit and independently checked exact-candidate receipts. |
| `UNCLASSIFIED` | Required identity or maintenance evidence is absent, stale or inconsistent. The report is blocked. |

`READY_FOR_CHECKS` is identity eligibility, not CI acceptance.
`READY_FOR_COMPAT_REVIEW` is only maintained-branch eligibility, never current-main acceptance.
`MAIN_COMMIT_IDENTIFIED` proves that the inspected commit equals live main; it does not verify workflows still running for that commit.

## Immutable dependency and receipt evidence

The report includes checked/main SHA and tree, PR base, merge-base, dependency
file digests, required gates and blocking reason. Each release's own existing
`tools/dependency-policy.json` supplies its module inventory. An older release
is not required to contain modules introduced later. The report compares the
union of the two inventories and retains existing `go.sum` and `go.work.sum`
identities, including checksum-only changes. Missing required files fail;
optional checksum files absent on both sides are not fabricated.

Dependency difference is not automatically a vulnerability. The existing
`make vuln`, normal CI and Production suites own security and execution proof.
The `requiredGates` list is the minimum; all other applicable repository
source/type/template and consumer gates remain independently required.

For compatibility eligibility the Actions API receipt must match the repository,
canonical workflow path, exact head SHA, exact PR base ref and SHA, and PR head.
The workflow, required job and required verification step must all have
completed successfully. A matching workflow display name or branch name alone
is not evidence of the reviewed candidate. Missing provider evidence fails
closed. Original failed runs remain historical evidence, not current approval.

## Workflow and trust boundary

`.github/workflows/branch-baseline-qualification.yml` runs on PRs and main pushes.
It fetches live main, executes real Git-history regressions, writes the report
outside tracked source and retains it as an artifact. Normal CI, Production,
vulnerability scanning and post-integration acceptance remain separate.

An optional repository-admin-controlled Actions variable,
`YUNKA_MAINTENANCE_POLICY`, supplies compatibility eligibility:

```json
{
  "schemaVersion": 1,
  "branches": [{
    "baseRef": "compat/example",
    "baseSha": "<exact reviewed PR base SHA>",
    "owner": "<responsible maintainer>",
    "supportUntil": "<reviewed YYYY-MM-DD support end>",
    "securityBackportSha": "<reviewed ancestor commit>",
    "qualification": {
      "ciRunId": 123,
      "productionRunId": 456
    }
  }]
}
```

This is a schema example, not an approved maintenance record or real run IDs.
The referenced CI and Production executions must independently exist for the
exact candidate. The tool does not create those runs or activate compatibility
maintenance. A maintenance record or reachable backport alone is insufficient.
The standard production workflow currently targets main PRs; maintaining a
compatibility line therefore also requires an independently reviewed means to
obtain its exact-candidate production evidence, not a fabricated receipt.

PR prose and candidate files are not maintenance approval inputs. In local use,
an environment variable is still caller-provided; local output is not proof of
repository-admin authorization. The workflow and verifier are reviewable code,
not a security sandbox against an actor allowed to rewrite their implementation.

## Local invocation

After refreshing `origin/main` and checking out the exact candidate:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 tools/test_qualify_branch_baseline.py -v
python3 tools/qualify_branch_baseline.py --mode pr \
  --base-ref main --base-sha "$BASE_SHA" --checked-sha "$HEAD_SHA"
```

Complete Git history and the current remote-tracking ref are required. Exit 0
means the reported identity/eligibility state, not fully accepted delivery.
Exit 2 means blocked or incomplete evidence. For integrated identity readback,
use `--mode main --checked-sha "$ACTUAL_MAIN_SHA"`; then inspect the separate
actual-main CI and Production receipts. No equivalent-tree comparison is
reported as an additional execution.

## Governance limits

GitHub required-status rulesets and administrator approval are separate from
creating a check. This task does not activate repository rulesets, authorize a
compatibility line, upgrade consumer pins, or make a passing branch check
sufficient to merge. A qualifying PR must still pass its applicable exact-head
checks and follow the existing non-force integration and live-main readback
policy. No changes to Runtime, Executor, Authz, UoW or C9 behavior are authorized
by classification.
