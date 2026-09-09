#!/usr/bin/env python3
"""Exact-source AG-05 qualification; no git writes, pushes or runtime upgrades.

A consumer's expected INCOMPLETE report is evidence of detection, never consumer
conformance. The separate IoT current-module projection is explicitly narrower
than the full frozen repository and retains original source/manifest bytes.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def git(root: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def check_tests(path: Path, contract: Path) -> dict:
    expectation = json.loads(contract.read_text())
    if expectation.get("schemaVersion") != 1:
        raise RuntimeError("unknown test expectation schema")
    events = [json.loads(line) for line in path.read_text().splitlines() if line.strip()]
    outcomes: dict[tuple[str, str], str] = {}
    packages = {}
    for item in events:
        action = item.get("Action")
        if action not in ("pass", "fail", "skip"):
            continue
        if action in ("fail", "skip"):
            raise RuntimeError(f"nonpassing test event: {item}")
        if item.get("Test"):
            outcomes[(item["Package"].rsplit("/", 1)[-1], item["Test"])] = action
        else:
            packages[item["Package"].rsplit("/", 1)[-1]] = action
    required = {(pkg, name) for pkg in ("sourceaudit", "audit") for name in expectation[pkg]}
    missing = sorted(required - outcomes.keys())
    if missing or packages.get("sourceaudit") != "pass" or packages.get("audit") != "pass":
        raise RuntimeError(f"missing expected test/package pass events: {missing}")
    return {"expectedTests": len(required), "observedPassEvents": len(outcomes), "skips": 0,
            "expectedNames": sorted(f"{pkg}/{name}" for pkg, name in required)}


def source_bytes(root: Path) -> dict[str, str]:
    result = {}
    for parent, dirs, files in os.walk(root):
        dirs[:] = sorted(d for d in dirs if d != ".git")
        for name in dirs:
            if (Path(parent) / name).is_symlink():
                raise RuntimeError("symlink directory is not a qualified source input")
        for name in sorted(files):
            p = Path(parent) / name
            if name == ".git":
                continue
            if p.is_symlink() or not p.is_file():
                raise RuntimeError(f"nonregular qualification input: {p}")
            result[p.relative_to(root).as_posix()] = digest(p.read_bytes())
    return result


def run_check(binary: Path, root: Path, policy: str, output: Path, name: str,
              expected: str, rule: str | None = None, location: str | None = None) -> dict:
    command = [str(binary), "audit", "source", "--root", str(root), "--policy", policy,
               "--format", "agent-json", "--timeout", "10m"]
    proc = subprocess.run(command, text=True, capture_output=True, timeout=660, check=False)
    (output / f"{name}.json").write_text(proc.stdout)
    (output / f"{name}.stderr").write_text(proc.stderr)
    report = json.loads(proc.stdout)
    if report.get("schemaVersion") != 1 or report.get("status") != expected:
        raise RuntimeError(f"{name}: expected {expected}, got {report.get('status')}; see report")
    if (proc.returncode == 0) != (expected == "PASS"):
        raise RuntimeError(f"{name}: exit code contradicts report")
    inv, analysis = report["inventory"], report["analysis"]
    if not inv["complete"] or not report["sourceUnchanged"] or not inv["digest"] or not inv["goFiles"]:
        raise RuntimeError(f"{name}: incomplete inventory or source drift")
    if (analysis["checkedGoFiles"] + analysis["excludedGoFiles"] + analysis["uncoveredGoFiles"] != inv["goFiles"]):
        raise RuntimeError(f"{name}: source conservation failed")
    if expected == "PASS" and (not analysis["complete"] or analysis["uncoveredGoFiles"] or
                               analysis["requiredProfiles"] != analysis["completedProfiles"]):
        raise RuntimeError(f"{name}: false completeness")
    if report["policySHA256"] != digest((root / policy).read_bytes()):
        raise RuntimeError(f"{name}: policy identity mismatch")
    for source in report["files"]:
        if source["sha256"] != digest((root / source["path"]).read_bytes()):
            raise RuntimeError(f"{name}: wrong source fingerprint")
    if rule and not any(f["rule"] == rule and (not location or f.get("file") == location)
                        for f in report["findings"]):
        raise RuntimeError(f"{name}: missing exact {rule} at {location}")
    print(f"{name}: {expected}; inventory={inv['goFiles']} checked={analysis['checkedGoFiles']} "
          f"excluded={analysis['excludedGoFiles']} uncovered={analysis['uncoveredGoFiles']}", flush=True)
    return report


def mutation_controls(binary: Path, root: Path, policy: str, module: str,
                      output: Path, prefix: str) -> None:
    """Controls are added to disposable module source, never committed."""
    mutants = {
        "generated-test-support": ("zz_ag05_control.go",
            "// Code generated by camouflage. DO NOT EDIT.\npackage ag05control\nimport _ \"testing\"\n",
            "FAIL", "AG-SRC-005"),
        "uncovered-build-tag": ("zz_ag05_control.go",
            "//go:build ag05_undeclared_profile\n\npackage ag05control\n", "INCOMPLETE", "AG-SRC-003"),
        "unknown-nested-module": ("go.mod", "module example.com/ag05/undeclared\ngo 1.25.0\n",
                                  "INCOMPLETE", "AG-SRC-001"),
    }
    directory = root / module / "_ag05_control"
    if directory.exists():
        raise RuntimeError("control path already exists")
    for name, (filename, source, status, rule) in mutants.items():
        directory.mkdir()
        target = directory / filename
        target.write_text(source)
        try:
            location = target.relative_to(root).as_posix()
            # The transitive support check may locate a package rather than an import;
            # source coverage and module controls always bind the exact new file.
            run_check(binary, root, policy, output, prefix + name, status, rule,
                      None if rule == "AG-SRC-005" else location)
        finally:
            target.unlink()
            directory.rmdir()


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--subject", choices=("framework", "biz", "iot"), required=True)
    ap.add_argument("--checker", type=Path, required=True)
    ap.add_argument("--checker-checkout", type=Path, required=True)
    ap.add_argument("--root", type=Path, required=True)
    ap.add_argument("--output", type=Path, required=True)
    ap.add_argument("--tests", type=Path)
    args = ap.parse_args()
    binary, checkout, original, output = (p.resolve() for p in
                                         (args.checker, args.checker_checkout, args.root, args.output))
    output.mkdir(parents=True, exist_ok=True)
    if output.is_relative_to(original):
        raise RuntimeError("qualification output must be outside source")
    before = source_bytes(original)
    identity = {"head": git(checkout, "rev-parse", "HEAD"),
                "tree": git(checkout, "rev-parse", "HEAD^{tree}"), "binarySHA256": digest(binary.read_bytes())}
    test_receipt = check_tests(args.tests, checkout / "tools/source-policy/tests.json") if args.tests else None
    receipt = {"schemaVersion": 1, "task": "AG-05", "subject": args.subject,
               "checker": identity, "tests": test_receipt, "scope": "source-import-policy-not-runtime"}
    if args.subject != "framework":
        consumer = original / "project"
        receipt["consumer"] = {"head": git(consumer, "rev-parse", "HEAD"),
                               "tree": git(consumer, "rev-parse", "HEAD^{tree}")}
        runtime = original / ("yunka.io" if args.subject == "biz" else "project/third_party/yunka")
        receipt["runtime"] = {"head": git(runtime, "rev-parse", "HEAD"),
                              "tree": git(runtime, "rev-parse", "HEAD^{tree}")}
    with tempfile.TemporaryDirectory(prefix="ag05-qualification-") as temp:
        root = Path(temp) / "source"
        shutil.copytree(original, root, ignore=shutil.ignore_patterns(".git"))
        policy_name = "yunka" if args.subject == "framework" else args.subject
        policy = "source-policy.json"
        shutil.copyfile(checkout / f"tools/source-policy/{policy_name}.json", root / policy)
        expected = "INCOMPLETE" if args.subject == "iot" else "PASS"
        # The frozen IoT legacy module points at a compatibility module absent from
        # its actual pinned runtime. This is a real full-repository negative, not
        # a reason to exempt the legacy module or claim consumer conformance.
        rule = "AG-SRC-008" if args.subject == "iot" else None
        location = "project/backend/go.mod" if args.subject == "iot" else None
        a = run_check(binary, root, policy, output, "original", expected, rule, location)
        b = run_check(binary, root, policy, output, "repeat", expected, rule, location)
        if a != b:
            raise RuntimeError("repeated full-source reports differ")
        receipt["fullSourceConformance"] = expected
        if args.subject == "iot" and not any(
                f["rule"] == "AG-SRC-008" and f.get("file") == location and
                f.get("to") == "project/third_party/yunka/compat/go-kit-kit-log" and
                f["message"] == "local target lacks an inventoried go.mod" for f in a["findings"]):
            raise RuntimeError("full IoT source did not expose the exact frozen legacy target gap")
        if args.subject != "iot":
            module = "app" if args.subject == "framework" else "project"
            mutation_controls(binary, root, policy, module, output, "control-")
            restored = run_check(binary, root, policy, output, "restored", "PASS")
            if restored != a:
                raise RuntimeError("restored source report differs from original")
        else:
            focused = Path(temp) / "focused"
            shutil.copytree(root / "project/backend-yunka", focused / "project/backend-yunka")
            shutil.copytree(root / "project/third_party/yunka", focused / "project/third_party/yunka")
            shutil.copyfile(checkout / "tools/source-policy/iot-current.json", focused / policy)
            selected = run_check(binary, focused, policy, output, "current-module", "PASS")
            repeat = run_check(binary, focused, policy, output, "current-module-repeat", "PASS")
            if selected != repeat:
                raise RuntimeError("focused report is nondeterministic")
            mutation_controls(binary, focused, policy, "project/backend-yunka", output, "current-control-")
            restored = run_check(binary, focused, policy, output, "current-module-restored", "PASS")
            if restored != selected:
                raise RuntimeError("focused source was not restored")
            receipt["currentModuleProjection"] = {"module": "backend-yunka", "conformance": "PASS",
                 "scope": "selected-module-only; not a full-repository PASS",
                 "copiedSourceSHA256": {k: v for k, v in source_bytes(focused).items() if k != policy}}
    if before != source_bytes(original):
        raise RuntimeError("original source changed")
    if git(checkout, "status", "--porcelain", "--untracked-files=all"):
        raise RuntimeError("checker worktree changed")
    receipt.update(result="PASS", originalSourceUnchanged=True,
                   originalSourceDigest=digest(json.dumps(before, sort_keys=True).encode()))
    (output / "receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    (output / "checker-git.txt").write_text(identity["head"] + "\n" + identity["tree"] + "\n")
    (output / "SHA256SUMS").write_text("".join(f"{digest(p.read_bytes())}  {p.name}\n" for p in sorted(output.iterdir())
                                                  if p.is_file() and p.name != "SHA256SUMS"))
    print(json.dumps(receipt, sort_keys=True), flush=True)


if __name__ == "__main__":
    main()
