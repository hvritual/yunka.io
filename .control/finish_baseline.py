"""Apply bounded changes to the existing branch-qualification implementation."""
from pathlib import Path

source_path = Path('tools/qualify_branch_baseline.py')
source = source_path.read_text()

def replace_once(old, new):
    global source
    if source.count(old) != 1:
        raise SystemExit('Source anchor changed: ' + old[:120])
    source = source.replace(old, new, 1)

replace_once('from pathlib import Path\n', 'from pathlib import Path, PurePosixPath\n')
replace_once('["git", "-C", str(root), *args], capture_output=True, check=False', '["git", "-C", str(root), *args], capture_output=True, check=False,\n        timeout=30, env=dict(os.environ, GIT_NO_REPLACE_OBJECTS="1")')
start = source.index('def drift(')
end = source.index('\ndef maintenance_policy(', start)
source = source[:start] + '''def dependency_inventory(root: Path, sha: str) -> tuple[set[str], set[str]]:
    """Read each release's own module inventory; old releases need not own infras."""
    try:
        policy = json.loads(git(root, "show", sha + ":tools/dependency-policy.json").stdout)
        modules = policy["moduleFiles"]
    except (ValueError, KeyError, TypeError) as exc:
        raise BaselineError("DEPENDENCY_POLICY_INVALID") from exc
    if not isinstance(modules, list) or not modules:
        raise BaselineError("DEPENDENCY_POLICY_INVALID")
    required = {"tools/dependency-policy.json", "tools/toolchain.env", "go.work"}
    optional = {"go.work.sum"}
    for name in modules:
        if (not isinstance(name, str) or not name or "\\\\" in name
                or PurePosixPath(name).is_absolute() or ".." in name.split("/")
                or str(PurePosixPath(name)) != name
                or PurePosixPath(name).name not in {"go.mod", "go.work"}):
            raise BaselineError("DEPENDENCY_POLICY_INVALID")
        required.add(name)
        if name.endswith("go.mod"):
            optional.add(name[:-3] + "sum")
    return required, optional


def drift(root: Path, main_sha: str, head_sha: str) -> list[dict]:
    left, left_optional = dependency_inventory(root, main_sha)
    right, right_optional = dependency_inventory(root, head_sha)
    result = []
    for name in sorted(left | right | left_optional | right_optional):
        main_digest = blob_digest(root, main_sha, name)
        candidate_digest = blob_digest(root, head_sha, name)
        if main_digest is None and candidate_digest is None and name not in left | right:
            continue  # A pure-Go module legitimately may have no go.sum.
        result.append({
            "path": name, "mainSHA256": main_digest,
            "candidateSHA256": candidate_digest,
            "mainRequired": name in left, "candidateRequired": name in right,
            "matchesMain": main_digest is not None
                           and candidate_digest is not None
                           and main_digest == candidate_digest,
        })
    return result

''' + source[end:]
replace_once('    if not isinstance(data, dict) or data.get("schemaVersion") != 1 or not isinstance(data.get("branches"), list):', '    if not isinstance(data, dict) or type(data.get("schemaVersion")) is not int or data.get("schemaVersion") != 1 or not isinstance(data.get("branches"), list):')
replace_once('    if not isinstance(run, dict) or not isinstance(jobs.get("jobs"), list):', '    if (not isinstance(run, dict) or run.get("id") != number\n            or not isinstance(jobs, dict) or not isinstance(jobs.get("jobs"), list)):')
start = source.index('def qualified_run(')
end = source.index('\ndef assess(', start)
source = source[:start] + '''def qualified_run(run: dict, jobs: list[dict], checked_sha: str,
                  base_ref: str, base_sha: str, repository: str,
                  workflow: str, job: str, step: str) -> bool:
    """Bind a provider receipt to repository, workflow path and exact PR pair.

    A successful workflow with the same display name or branch name is not
    interchangeable with the actual workflow/base/head under review.
    """
    if not isinstance(run, dict) or not isinstance(jobs, list):
        return False
    repo = run.get("repository")
    if not isinstance(repo, dict) or repo.get("full_name") != repository:
        return False
    if any((run.get("name") != workflow,
            run.get("path") != ".github/workflows/" + workflow + ".yml",
            run.get("head_sha") != checked_sha,
            run.get("status") != "completed",
            run.get("conclusion") != "success",
            run.get("event") != "pull_request")):
        return False
    prs = run.get("pull_requests")
    if not isinstance(prs, list):
        return False
    if not any(
        isinstance(pr, dict) and isinstance(pr.get("base"), dict)
        and isinstance(pr.get("head"), dict)
        and pr["base"].get("ref") == base_ref
        and pr["base"].get("sha") == base_sha
        and pr["head"].get("sha") == checked_sha for pr in prs
    ):
        return False
    for item in jobs:
        if not isinstance(item, dict) or not isinstance(item.get("steps"), list):
            return False
        if (item.get("name") == job and item.get("status") == "completed"
                and item.get("conclusion") == "success"
                and any(isinstance(s, dict) and s.get("name") == step
                        and s.get("status") == "completed"
                        and s.get("conclusion") == "success"
                        for s in item["steps"])):
            return True
    return False

''' + source[end:]
replace_once('    if any(item["mainSHA256"] is None or item["candidateSHA256"] is None\n           for item in report["dependencyBaseline"]):', '    if any((item["mainRequired"] and item["mainSHA256"] is None)\n           or (item["candidateRequired"] and item["candidateSHA256"] is None)\n           for item in report["dependencyBaseline"]):')
replace_once('qualified_run(ci, ci_jobs, head, base_ref, "ci", "verify", "Verify")', 'qualified_run(ci, ci_jobs, head, base_ref, base_sha, repository,\n                         "ci", "verify", "Verify")')
replace_once('qualified_run(production, production_jobs, head, base_ref, "production",', 'qualified_run(production, production_jobs, head, base_ref, base_sha, repository, "production",')
replace_once('    except BaselineError as exc:\n        report = ', '    except (BaselineError, OSError, subprocess.TimeoutExpired) as exc:\n        report = ')
replace_once('        "verifiedRuns": [],\n', '        "requiredGatesScope": "minimum; applicable existing workflows remain independently required",\n        "verifiedRuns": [],\n')
source_path.write_text(source)

test_path = Path('tools/test_qualify_branch_baseline.py')
tests = test_path.read_text()
anchor = '        invoke(self.root, "add", ".")\n'
assert tests.count(anchor) == 1
tests = tests.replace(anchor, '''        (self.root / "tools/dependency-policy.json").write_text(
            json.dumps({"schemaVersion": 3, "moduleFiles": list(baseline.FILES[2:])}),
            encoding="utf-8",
        )
''' + anchor, 1)
tests = tests.replace('step_name: str, branch: str = "compat/example")', 'step_name: str, branch: str = "compat/example",\n                   base_sha: str = "")')
assert '"pull_requests": [{"base": {"ref": branch}}],' in tests
tests = tests.replace('"pull_requests": [{"base": {"ref": branch}}],', '''"path": ".github/workflows/" + workflow + ".yml",
            "repository": {"full_name": "hvritual/yunka.io"},
            "pull_requests": [{"base": {"ref": branch, "sha": base_sha},
                               "head": {"sha": sha}}],''')
tests = tests.replace('"name": job_name, "conclusion": "success",', '"name": job_name, "status": "completed", "conclusion": "success",')
tests = tests.replace('"steps": [{"name": step_name, "conclusion": "success"}],', '"steps": [{"name": step_name, "status": "completed", "conclusion": "success"}],')
tests = tests.replace('            "Verify" if number == 101 else "Verify production on MySQL 8.4",\n', '            "Verify" if number == 101 else "Verify production on MySQL 8.4",\n            base_sha=base,\n')
anchor = '    def test_main_moving_invalidates_an_older_candidate(self) -> None:\n'
assert tests.count(anchor) == 1
tests = tests.replace(anchor, '''    def test_checksum_only_changes_are_visible(self) -> None:
        self.feature()
        sha = commit(self.root, "pkg/go.sum", "new dependency checksum\\n")
        report = self.assess(sha=sha)
        evidence = next(item for item in report["dependencyBaseline"]
                        if item["path"] == "pkg/go.sum")
        self.assertIsNone(evidence["mainSHA256"])
        self.assertIsNotNone(evidence["candidateSHA256"])
        self.assertFalse(evidence["matchesMain"])
        self.assertEqual(report["status"], "READY_FOR_CHECKS")

''' + anchor, 1)
anchor = '    def test_backport_must_be_in_exact_candidate_history(self) -> None:\n'
assert tests.count(anchor) == 1
tests = tests.replace(anchor, '''    def test_old_release_module_inventory_does_not_require_new_modules(self) -> None:
        base, head = self.legacy()
        policy_path = self.root / "tools/dependency-policy.json"
        policy = json.loads(policy_path.read_text())
        policy["moduleFiles"].remove("infras/go.mod")
        policy_path.write_text(json.dumps(policy))
        (self.root / "infras/go.mod").unlink()
        invoke(self.root, "add", "-A")
        invoke(self.root, "commit", "-qm", "Retain old release module inventory")
        head = invoke(self.root, "rev-parse", "HEAD")
        report = self.assess(base_ref="compat/example", base_sha=base, sha=head)
        self.assertEqual(report["classification"], "UNCLASSIFIED")
        self.assertEqual(report["blockedReason"], "MAINTENANCE_AUTHORITY_MISSING")
        evidence = next(item for item in report["dependencyBaseline"]
                        if item["path"] == "infras/go.mod")
        self.assertFalse(evidence["candidateRequired"])
        self.assertIsNone(evidence["candidateSHA256"])

    def test_same_branch_receipts_from_another_base_are_rejected(self) -> None:
        base, head = self.legacy()
        def reader(number):
            return successful_run(
                head, "ci" if number == 101 else "production",
                "verify" if number == 101 else "verify-production",
                "Verify" if number == 101 else "Verify production on MySQL 8.4",
                base_sha=self.main_sha,
            )
        report = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                             trusted=self.policy(base, base, ci=101, production=102),
                             reader=reader)
        self.assertEqual(report["blockedReason"], "CI_SECURITY_RECEIPT_MISMATCH")

    def test_workflow_display_name_and_foreign_repository_are_not_authority(self) -> None:
        base, head = self.legacy()
        for changed in ("workflow", "repository", "pr_head", "job_status", "step_status"):
            with self.subTest(changed=changed):
                def reader(number):
                    run, jobs = successful_run(
                        head, "ci" if number == 101 else "production",
                        "verify" if number == 101 else "verify-production",
                        "Verify" if number == 101 else "Verify production on MySQL 8.4",
                        base_sha=base,
                    )
                    if changed == "workflow":
                        run["path"] = ".github/workflows/unrelated.yml"
                    elif changed == "repository":
                        run["repository"]["full_name"] = "other/repository"
                    elif changed == "pr_head":
                        run["pull_requests"][0]["head"]["sha"] = base
                    elif changed == "job_status":
                        jobs[0]["status"] = "in_progress"
                    else:
                        jobs[0]["steps"][0]["status"] = "in_progress"
                    return run, jobs
                report = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                                     trusted=self.policy(base, base, ci=101, production=102),
                                     reader=reader)
                self.assertEqual(report["blockedReason"], "CI_SECURITY_RECEIPT_MISMATCH")

    def test_malformed_provider_receipt_does_not_raise(self) -> None:
        base, head = self.legacy()
        report = self.assess(base_ref="compat/example", base_sha=base, sha=head,
                             trusted=self.policy(base, base, ci=101, production=102),
                             reader=lambda number: (None, ["invalid"]))
        self.assertEqual(report["blockedReason"], "CI_SECURITY_RECEIPT_MISMATCH")

''' + anchor, 1)
test_path.write_text(tests)
