# Branch baseline qualification

> Document class: **CURRENT**
> Tracking: Issue #228
> Current status authority: `docs/STATUS.md`

## Contract

`tools/qualify_branch_baseline.py` is a read-only Git identity evaluator.
It reports a classification and required downstream gates. It does not
repair dependencies, approve a merge, or claim that CI has passed.

- `CURRENT_MAIN`: an exact PR base at live `origin/main`, with HEAD descending from that base.
- `MAINTAINED_COMPAT`: an explicitly maintained non-main base with named owner, support horizon, reachable security backport, and matching independent CI/Production receipts.
- `UNCLASSIFIED`: missing, stale or inconsistent evidence; fail closed.

`READY_FOR_CHECKS` is only permission to run existing checks; it is not acceptance.
`MAIN_COMMIT_IDENTIFIED` proves Git identity but not post-merge workflow success.

## Evidence

The JSON output identifies checked and main SHA/tree, PR base, merge-base,
dependency baseline file digests, the required gates and any blocking reason.
Dependency drift is not automatically a security finding; the existing
`make vuln`, normal CI and Production suites retain that authority.

## Workflow

`.github/workflows/branch-baseline-qualification.yml` runs on PRs and main pushes.
It fetches live main, tests adversarial Git histories, executes the read-only
classifier and stores its JSON output as a CI artifact.

An optional repository-admin-controlled Actions variable named
`YUNKA_MAINTENANCE_POLICY` supplies compatibility policy; PR text and
candidate-controlled files cannot grant compatibility authority.
The maintenance record binds branch/base, owner, support-until date,
security backport SHA, and independently validated exact-head run IDs.
The GitHub Actions API must confirm candidate identity and successful
CI Verify / Production Verify jobs. No data is accepted as proof solely
because an AI, PR author or document reports success.

## Local invocation

```bash
python3 tools/test_qualify_branch_baseline.py -v
python3 tools/qualify_branch_baseline.py --mode pr \
  --base-ref main --base-sha "$BASE_SHA" --checked-sha "$HEAD_SHA"
```

This command requires complete Git history and a fresh `origin/main`.
Use `--mode main` with the actual integrated SHA for post-integration
identity readback; verify subsequent CI and Production receipts separately.

## Governance limits

GitHub required-status rulesets are not changed by this task. Their
activation and admin approval are separate from generating a check.
Neither an old compatibility branch nor a passing baseline check can
substitute for qualification against the current main branch.
