#!/usr/bin/env python3
"""Adversarial Git fixtures for exact-identity qualification."""
from __future__ import annotations

import datetime as dt
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import qualify_branch_baseline as baseline

TODAY = dt.date(2026, 10, 8)


def invoke(root: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", "-C", str(root), *args], capture_output=True, text=True, check=True
    )
    return result.stdout.strip()


def commit(root: Path, name: str, contents: str) -> str:
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(contents, encoding="utf-8")
    invoke(root, "add", name)
    invoke(root, "commit", "-qm", "Update " + name)
    return invoke(root, "rev-parse", "HEAD")


class RepositoryFixture(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        invoke(self.root, "init", "-q", "--initial-branch=main")
        invoke(self.root, "config", "user.email", "fixture@example.invalid")
        invoke(self.root, "config", "user.name", "Baseline Fixture")
        # All required manifest identities must be present, even in a synthetic project.
        for file in baseline.FILES:
            path = self.root / file
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("baseline\n", encoding="utf-8")
        invoke(self.root, "add", ".")
        invoke(self.root, "commit", "-qm", "Canonical baseline")
        self.main_sha = invoke(self.root, "rev-parse", "HEAD")
        self.publish_main()

    def publish_main(self) -> None:
        self.main_sha = invoke(self.root, "rev-parse", "main")
        invoke(self.root, "update-ref", "refs/remotes/origin/main", self.main_sha)

    def feature(self) -> str:
        invoke(self.root, "switch", "-q", "-c", "feature/current-main")
        return commit(self.root, "docs/change.txt", "candidate")

    def assess(self, *, mode: str = "pr", base_ref: str = "main",
               base_sha: str | None = None, sha: str | None = None,
               trusted: str = "", reader=None) -> dict:
        return baseline.assess(
            self.root, mode, base_ref, base_sha or self.main_sha,
            sha or invoke(self.root, "rev-parse", "HEAD"),
            "hvritual/yunka.io", trusted, reader, today=TODAY
        )

    def legacy(self) -> tuple[str, str]:
        # Divergence is real: main advances independently of a compat branch.
        invoke(self.root, "switch", "-q", "-c", "compat/example")
        compat_base = commit(self.root, "pkg/go.mod", "old grpc baseline\n")
        invoke(self.root, "switch", "-q", "main")
        commit(self.root, "docs/current.txt", "new-main")
        self.publish_main()
        invoke(self.root, "switch", "-q", "compat/example")
        invoke(self.root, "switch", "-q", "-c", "fix/example")
        head = commit(self.root, "docs/compat.txt", "compat fix")
        return compat_base, head

    @staticmethod
    def policy(base_sha: str, backport: str, *, expiry: str = "2099-12-31",
               ci: int | None = None, production: int | None = None) -> str:
        values = {
            "schemaVersion": 1,
            "branches": [{
                "baseRef": "compat/example", "baseSha": base_sha,
                "owner": "framework-maintainers", "supportUntil": expiry,
                "securityBackportSha": backport,
                "qualification": {
                    "ciRunId": ci, "productionRunId": production
                },
            }],
        }
        return json.dumps(values)


def successful_run(sha: str, workflow: str, job_name: str,
                   step_name: str, branch: str = "compat/example") -> tuple[dict, list[dict]]:
    return (
        {
            "name": workflow, "head_sha": sha, "status": "completed",
            "conclusion": "success", "event": "pull_request",
            "pull_requests": [{"base": {"ref": branch}}],
        },
        [{
            "name": job_name, "conclusion": "success",
            "steps": [{"name": step_name, "conclusion": "success"}],
        }],
    )


class BranchIdentityTests(RepositoryFixture):
    def test_current_main_candidate_is_ready_for_checks_not_merged(self) -> None:
        sha = self.feature()
        data = self.assess(sha=sha)
        self.assertEqual(data["classification"], "CURRENT_MAIN")
        self.assertEqual(data["status"], "READY_FOR_CHECKS")
        self.assertEqual(data["checkedSha"], sha)
        self.assertEqual(data["mainSha"], self.main_sha)
        self.assertEqual(data["mergeBaseSha"], self.main_sha)
        self.assertIn("integrated-main:production/verify-production", data["requiredGates"])
        self.assertEqual(data["verifiedRuns"], [])
        self.assertTrue(all(d["matchesMain"] for d in data["dependencyBaseline"]))

    def test_missing_dependency_manifest_is_incomplete_evidence(self) -> None:
        sha = self.feature()
        (self.root / "go.work").unlink()
        invoke(self.root, "add", "-u")
        invoke(self.root, "commit", "-qm", "Remove required manifest")
        sha = invoke(self.root, "rev-parse", "HEAD")
        data = self.assess(sha=sha)
        self.assertEqual(data["classification"], "UNCLASSIFIED")
        self.assertEqual(data["blockedReason"], "DEPENDENCY_BASELINE_EVIDENCE_MISSING")
        self.assertFalse(next(x for x in data["dependencyBaseline"]
                              if x["path"] == "go.work")["matchesMain"])

    def test_main_moving_invalidates_an_older_candidate(self) -> None:
        old_sha = self.main_sha
        sha = self.feature()
        invoke(self.root, "switch", "-q", "main")
        commit(self.root, "docs/new-main.txt", "new main")
        self.publish_main()
        invoke(self.root, "switch", "-q", "feature/current-main")
        data = self.assess(base_sha=old_sha, sha=sha)
        self.assertEqual(data["classification"], "CURRENT_MAIN")
        self.assertEqual(data["status"], "BLOCKED")
        self.assertEqual(data["blockedReason"], "STALE_MAIN_BASE")

    def test_unrelated_or_forged_base_is_rejected(self) -> None:
        sha = self.feature()
        invoke(self.root, "switch", "-q", "--orphan", "unrelated")
        unrelated_sha = commit(self.root, "docs/unrelated", "orphan")
        invoke(self.root, "switch", "-q", "feature/current-main")
        data = self.assess(base_sha=unrelated_sha, sha=sha)
        self.assertEqual(data["status"], "BLOCKED")
        self.assertEqual(data["blockedReason"], "CANDIDATE_DOES_NOT_DESCEND_BASE")

    def test_wrong_checkout_cannot_claim_requested_head(self) -> None:
        self.feature()
        data = self.assess(sha=self.main_sha)
        self.assertEqual(data["blockedReason"], "CHECKED_HEAD_MISMATCH")

    def test_live_main_readback_does_not_claim_post_merge_workflows(self) -> None:
        data = self.assess(mode="main", sha=self.main_sha)
        self.assertEqual(data["status"], "MAIN_COMMIT_IDENTIFIED")
        self.assertEqual(data["verifiedRuns"], [])
        self.assertIn("integrated-main:ci/verify", data["requiredGates"])

    def test_read_only_and_deterministic(self) -> None:
        sha = self.feature()
        before = invoke(self.root, "status", "--porcelain", "--untracked-files=all")
        first = self.assess(sha=sha)
        second = self.assess(sha=sha)
        after = invoke(self.root, "status", "--porcelain", "--untracked-files=all")
        self.assertEqual(before, after)
        self.assertEqual(json.dumps(first, sort_keys=True),
                         json.dumps(second, sort_keys=True))


class CompatibilityTests(RepositoryFixture):
    def test_legacy_branch_is_never_inferred_from_its_name(self) -> None:
        base, head = self.legacy()
        report = self.assess(base_ref="compat/example", base_sha=base, sha=head)
        self.assertEqual(report["classification"], "UNCLASSIFIED")
        self.assertEqual(report["blockedReason"], "MAINTENANCE_AUTHORITY_MISSING")
        self.assertNotEqual(report["status"], "READY_FOR_CHECKS")
        self.assertNotEqual(report["mergeBaseSha"], self.main_sha)
        self.assertFalse(next(x for x in report["dependencyBaseline"] if x["path"] == "pkg/go.mod")["matchesMain"])

    def test_malformed_or_ambiguous_policy_is_rejected(self) -> None:
        base, head = self.legacy()
        with self.assertRaisesRegex(baseline.BaselineError, "MALFORMED_MAINTENANCE_POLICY"):
            self.assess(base_ref="compat/example", base_sha=base, sha=head, trusted="{")
        p = json.loads(self.policy(base, base))
        p["branches"].append(p["branches"][0])
        with self.assertRaisesRegex(baseline.BaselineError, "AMBIGUOUS_MAINTENANCE_POLICY"):
            self.assess(base_ref="compat/example", base_sha=base, sha=head,
                        trusted=json.dumps(p))

    def test_expired_or_unrelated_maintainer_record_does_not_grant_compatibility(self) -> None:
        base, head = self.legacy()
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, base, expiry="2020-01-01"))
        self.assertEqual(data["blockedReason"], "MAINTENANCE_EXPIRED")
        record = json.loads(self.policy(base, base))
        record["branches"][0]["baseSha"] = self.main_sha
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=json.dumps(record))
        self.assertEqual(data["blockedReason"], "MAINTENANCE_AUTHORITY_MISSING")

    def test_malformed_backport_and_run_record_do_not_raise(self) -> None:
        base, head = self.legacy()
        record = json.loads(self.policy(base, base))
        record["branches"][0]["securityBackportSha"] = 17
        invalid = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                              trusted=json.dumps(record))
        self.assertEqual(invalid["blockedReason"], "SECURITY_BACKPORT_NOT_IN_CANDIDATE")
        record["branches"][0]["securityBackportSha"] = base
        record["branches"][0]["qualification"] = ["untrusted", "runs"]
        invalid = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                              trusted=json.dumps(record))
        self.assertEqual(invalid["blockedReason"], "INDEPENDENT_RUNS_REQUIRED")

    def test_backport_must_be_in_exact_candidate_history(self) -> None:
        base, head = self.legacy()
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, self.main_sha))
        self.assertEqual(data["blockedReason"], "SECURITY_BACKPORT_NOT_IN_CANDIDATE")

    def test_owner_and_backport_without_runs_are_incomplete(self) -> None:
        base, head = self.legacy()
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, base))
        self.assertEqual(data["classification"], "MAINTAINED_COMPAT")
        self.assertEqual(data["blockedReason"], "INDEPENDENT_RUNS_REQUIRED")

    def test_candidate_scoped_stale_or_wrong_receipt_is_rejected(self) -> None:
        base, head = self.legacy()
        reader = lambda number: successful_run(
            self.main_sha if number == 101 else head,
            "ci" if number == 101 else "production",
            "verify" if number == 101 else "verify-production",
            "Verify" if number == 101 else "Verify production on MySQL 8.4",
        )
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, base, ci=101, production=102),
                           reader=reader)
        self.assertEqual(data["classification"], "MAINTAINED_COMPAT")
        self.assertEqual(data["blockedReason"], "CI_SECURITY_RECEIPT_MISMATCH")

    def test_a_record_for_another_branch_is_rejected(self) -> None:
        base, head = self.legacy()
        reader = lambda number: successful_run(
            head, "ci" if number == 101 else "production",
            "verify" if number == 101 else "verify-production",
            "Verify" if number == 101 else "Verify production on MySQL 8.4",
            branch="main",
        )
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, base, ci=101, production=102),
                           reader=reader)
        self.assertEqual(data["blockedReason"], "CI_SECURITY_RECEIPT_MISMATCH")

    def test_live_exact_head_receipts_allow_review_but_not_main_acceptance(self) -> None:
        base, head = self.legacy()
        reader = lambda number: successful_run(
            head, "ci" if number == 101 else "production",
            "verify" if number == 101 else "verify-production",
            "Verify" if number == 101 else "Verify production on MySQL 8.4",
        )
        data = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                           trusted=self.policy(base, base, ci=101, production=102),
                           reader=reader)
        self.assertEqual(data["classification"], "MAINTAINED_COMPAT")
        self.assertEqual(data["status"], "READY_FOR_COMPAT_REVIEW")
        self.assertEqual(data["verifiedRuns"], [
            {"kind": "candidate-ci", "runId": 101, "headSha": head},
            {"kind": "candidate-production", "runId": 102, "headSha": head},
        ])
        self.assertNotEqual(data["status"], "MAIN_COMMIT_IDENTIFIED")


if __name__ == "__main__":
    unittest.main()
