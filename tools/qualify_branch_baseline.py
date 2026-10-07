#!/usr/bin/env python3
"""Read-only Git baseline and qualification identity check.

This tool never grants merge authority. GitHub Actions test/security/production
receipts are separate proofs and must match the exact candidate or merged SHA.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.request

FULL_SHA = re.compile(r"(?:[0-9a-f]{40}|[0-9a-f]{64})\Z")
FILES = (
    "tools/dependency-policy.json",
    "tools/toolchain.env",
    "go.work",
    "pkg/go.mod",
    "framework/go.mod",
    "gateway/go.mod",
    "infras/go.mod",
    "app/go.mod",
)
MAIN_GATES = (
    "candidate:ci/verify",
    "candidate:production/verify-production",
    "integrated-main:ci/verify",
    "integrated-main:production/verify-production",
)
COMPAT_GATES = (
    "compat:maintainer-policy",
    "compat:exact-head-ci-security",
    "compat:exact-head-production",
    "compat:separate-integration-review",
)


class BaselineError(ValueError):
    """Invalid or unavailable immutable Git evidence."""


def git(root: Path, *args: str, allow_failure: bool = False) -> subprocess.CompletedProcess:
    result = subprocess.run(
        ["git", "-C", str(root), *args], capture_output=True, check=False
    )
    if result.returncode and not allow_failure:
        raise BaselineError(
            "git " + " ".join(args[:2]) + ": " +
            result.stderr.decode("utf-8", "replace").strip()
        )
    return result


def rev(root: Path, value: str) -> str:
    result = git(root, "rev-parse", "--verify", value + "^{commit}")
    resolved = result.stdout.decode().strip()
    if not FULL_SHA.fullmatch(resolved):
        raise BaselineError("NON_EXACT_GIT_IDENTITY")
    return resolved


def exact_sha(root: Path, value: str) -> str:
    if not FULL_SHA.fullmatch(value):
        raise BaselineError("NON_EXACT_GIT_IDENTITY")
    resolved = rev(root, value)
    if resolved != value:
        raise BaselineError("GIT_IDENTITY_MISMATCH")
    return resolved


def ancestor(root: Path, old: str, new: str) -> bool:
    result = git(root, "merge-base", "--is-ancestor", old, new, allow_failure=True)
    if result.returncode not in (0, 1):
        raise BaselineError("ANCESTRY_EVIDENCE_UNAVAILABLE")
    return result.returncode == 0


def blob_digest(root: Path, sha: str, path: str) -> str | None:
    result = git(root, "show", sha + ":" + path, allow_failure=True)
    if result.returncode:
        return None
    return hashlib.sha256(result.stdout).hexdigest()


def drift(root: Path, main_sha: str, head_sha: str) -> list[dict]:
    result = []
    for name in FILES:
        main_digest = blob_digest(root, main_sha, name)
        candidate_digest = blob_digest(root, head_sha, name)
        result.append({
            "path": name, "mainSHA256": main_digest,
            "candidateSHA256": candidate_digest,
            "matchesMain": main_digest is not None
                           and candidate_digest is not None
                           and main_digest == candidate_digest,
        })
    return result


def maintenance_policy(source: str, branch: str, base_sha: str) -> dict | None:
    if not source.strip():
        return None
    try:
        data = json.loads(source)
    except json.JSONDecodeError as exc:
        raise BaselineError("MALFORMED_MAINTENANCE_POLICY") from exc
    if not isinstance(data, dict) or data.get("schemaVersion") != 1 or not isinstance(data.get("branches"), list):
        raise BaselineError("INVALID_MAINTENANCE_POLICY")
    candidates = [
        item for item in data["branches"]
        if isinstance(item, dict) and item.get("baseRef") == branch
        and item.get("baseSha") == base_sha
    ]
    if len(candidates) > 1:
        raise BaselineError("AMBIGUOUS_MAINTENANCE_POLICY")
    return candidates[0] if candidates else None


def provider_run(repository: str, number: int, token: str,
                 api_url: str = "https://api.github.com") -> tuple[dict, list[dict]]:
    if not token:
        raise BaselineError("PROVIDER_TOKEN_UNAVAILABLE")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise BaselineError("INVALID_REPOSITORY")
    url = api_url.rstrip("/") + "/repos/" + repository + "/actions/runs/" + str(number)
    headers = {"Authorization": "Bearer " + token,
               "Accept": "application/vnd.github+json",
               "X-GitHub-Api-Version": "2022-11-28"}

    def read(target: str) -> dict:
        request = urllib.request.Request(target, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=12) as response:
                return json.load(response)
        except (OSError, ValueError) as exc:
            raise BaselineError("PROVIDER_READBACK_UNAVAILABLE") from exc

    run = read(url)
    jobs = read(url + "/jobs?per_page=100")
    if not isinstance(run, dict) or not isinstance(jobs.get("jobs"), list):
        raise BaselineError("MALFORMED_PROVIDER_RECEIPT")
    if jobs.get("total_count", len(jobs["jobs"])) > len(jobs["jobs"]):
        raise BaselineError("INCOMPLETE_PROVIDER_JOBS")
    return run, jobs["jobs"]


def qualified_run(run: dict, jobs: list[dict], checked_sha: str,
                  base_ref: str, workflow: str, job: str, step: str) -> bool:
    if any((run.get("name") != workflow,
            run.get("head_sha") != checked_sha,
            run.get("status") != "completed",
            run.get("conclusion") != "success",
            run.get("event") != "pull_request")):
        return False
    prs = run.get("pull_requests") or []
    if not any(
        isinstance(pr, dict) and isinstance(pr.get("base"), dict)
        and pr["base"].get("ref") == base_ref for pr in prs
    ):
        return False
    return any(
        item.get("name") == job and item.get("conclusion") == "success"
        and any(s.get("name") == step and s.get("conclusion") == "success"
                for s in item.get("steps", []))
        for item in jobs
    )


def assess(root: Path, mode: str, base_ref: str, base_sha: str,
           checked_sha: str, repository: str,
           trusted_policy: str = "", run_reader=None,
           today: dt.date | None = None) -> dict:
    """Classify immutable evidence; do not mistake a candidate for merged proof."""
    root = root.resolve()
    head = rev(root, "HEAD")
    main = rev(root, "refs/remotes/origin/main")
    report = {
        "schemaVersion": 1,
        "repository": repository,
        "checkedSha": head,
        "checkedTree": rev_tree(root, head),
        "mainSha": main,
        "mainTree": rev_tree(root, main),
        "baseRef": base_ref if mode == "pr" else "main",
        "baseSha": base_sha if mode == "pr" else main,
        "mergeBaseSha": git(root, "merge-base", head, main).stdout.decode().strip(),
        "classification": "UNCLASSIFIED",
        "status": "BLOCKED",
        "blockedReason": None,
        "requiredGates": list(MAIN_GATES if base_ref == "main" or mode == "main" else COMPAT_GATES),
        "verifiedRuns": [],
        "dependencyBaseline": drift(root, main, head),
    }

    def block(code: str, classification: str = "UNCLASSIFIED") -> dict:
        report.update(classification=classification, status="BLOCKED", blockedReason=code)
        return report

    if any(item["mainSHA256"] is None or item["candidateSHA256"] is None
           for item in report["dependencyBaseline"]):
        return block("DEPENDENCY_BASELINE_EVIDENCE_MISSING")
    if exact_sha(root, checked_sha) != head:
        return block("CHECKED_HEAD_MISMATCH")
    if mode == "main":
        if head != main:
            return block("MAIN_READBACK_STALE", "CURRENT_MAIN")
        report.update(classification="CURRENT_MAIN", status="MAIN_COMMIT_IDENTIFIED")
        # This check cannot attest to independent CI/Production runs while running.
        return report
    if mode != "pr" or not base_ref or not base_sha:
        return block("PR_IDENTITY_MISSING")
    base_sha = exact_sha(root, base_sha)
    if not ancestor(root, base_sha, head):
        return block("CANDIDATE_DOES_NOT_DESCEND_BASE")
    if base_ref == "main":
        if main != base_sha or not ancestor(root, main, head):
            return block("STALE_MAIN_BASE", "CURRENT_MAIN")
        report.update(classification="CURRENT_MAIN", status="READY_FOR_CHECKS")
        return report

    policy = maintenance_policy(trusted_policy, base_ref, base_sha)
    if policy is None:
        return block("MAINTENANCE_AUTHORITY_MISSING")
    owner = policy.get("owner")
    if not isinstance(owner, str) or not owner.strip():
        return block("MAINTENANCE_OWNER_MISSING")
    try:
        until = dt.date.fromisoformat(policy["supportUntil"])
    except (KeyError, ValueError, TypeError):
        return block("MAINTENANCE_EXPIRY_INVALID")
    if until < (today or dt.datetime.now(dt.timezone.utc).date()):
        return block("MAINTENANCE_EXPIRED")
    backport = policy.get("securityBackportSha", "")
    if not isinstance(backport, str) or not FULL_SHA.fullmatch(backport) or not ancestor(root, exact_sha(root, backport), head):
        return block("SECURITY_BACKPORT_NOT_IN_CANDIDATE")

    report["classification"] = "MAINTAINED_COMPAT"
    qualification = policy.get("qualification") or {}
    if not isinstance(qualification, dict):
        return block("INDEPENDENT_RUNS_REQUIRED", "MAINTAINED_COMPAT")
    run_ids = (qualification.get("ciRunId"), qualification.get("productionRunId"))
    if any(type(n) is not int or n < 1 for n in run_ids):
        return block("INDEPENDENT_RUNS_REQUIRED", "MAINTAINED_COMPAT")
    if run_reader is None:
        return block("PROVIDER_READBACK_REQUIRED", "MAINTAINED_COMPAT")
    try:
        ci, ci_jobs = run_reader(run_ids[0])
        production, production_jobs = run_reader(run_ids[1])
    except BaselineError:
        return block("PROVIDER_READBACK_UNAVAILABLE", "MAINTAINED_COMPAT")
    if not qualified_run(ci, ci_jobs, head, base_ref, "ci", "verify", "Verify"):
        return block("CI_SECURITY_RECEIPT_MISMATCH", "MAINTAINED_COMPAT")
    if not qualified_run(production, production_jobs, head, base_ref, "production",
                         "verify-production", "Verify production on MySQL 8.4"):
        return block("PRODUCTION_RECEIPT_MISMATCH", "MAINTAINED_COMPAT")
    report["verifiedRuns"] = [
        {"kind": "candidate-ci", "runId": run_ids[0], "headSha": head},
        {"kind": "candidate-production", "runId": run_ids[1], "headSha": head},
    ]
    report.update(status="READY_FOR_COMPAT_REVIEW", blockedReason=None)
    # No compat receipt grants main integration/merge authority.
    return report


def rev_tree(root: Path, sha: str) -> str:
    value = git(root, "rev-parse", "--verify", sha + "^{tree}").stdout.decode().strip()
    if not FULL_SHA.fullmatch(value):
        raise BaselineError("TREE_IDENTITY_MISSING")
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=Path("."))
    parser.add_argument("--mode", choices=("pr", "main"), required=True)
    parser.add_argument("--base-ref", default="")
    parser.add_argument("--base-sha", default="")
    parser.add_argument("--checked-sha", required=True)
    parser.add_argument("--repository", default="hvritual/yunka.io")
    args = parser.parse_args()
    root = args.repo_root.resolve()
    try:
        # Only a repository-admin controlled Actions variable is eligible as a
        # compatibility authority. Never read PR text, candidate policy files
        # or job artifacts as unverified maintenance approval.
        trusted = os.environ.get("YUNKA_MAINTENANCE_POLICY", "")
        token = os.environ.get("GITHUB_TOKEN", "")
        report = assess(root, args.mode, args.base_ref, args.base_sha,
                        args.checked_sha, args.repository, trusted,
                        run_reader=lambda number: provider_run(args.repository, number, token))
    except BaselineError as exc:
        report = {"schemaVersion": 1, "classification": "UNCLASSIFIED",
                  "status": "BLOCKED", "blockedReason": str(exc),
                  "checkedSha": args.checked_sha, "requiredGates": list(MAIN_GATES)}
    print(json.dumps(report, ensure_ascii=False, sort_keys=True, indent=2))
    return 0 if report["status"] in (
        "READY_FOR_CHECKS", "READY_FOR_COMPAT_REVIEW", "MAIN_COMMIT_IDENTIFIED"
    ) else 2


if __name__ == "__main__":
    sys.exit(main())
