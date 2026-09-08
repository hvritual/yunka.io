#!/usr/bin/env python3
"""Read-only real-consumer type qualification; run on disposable pinned checkouts.

The caller prepares the dependency cache. This script executes only the checker,
not consumer business code. It temporarily changes test-copy source for controlled
negatives, restores bytes in finally blocks and refuses dirty starting checkouts.
It never commits, pushes, expands permissions or modifies generator/runtime pins.
"""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

PINS = {
    "biz": "3519e7ee6e51e33984669871e4f32a55a3597d9f",
    "iot": "bcd20632b666405c3f7a8fe8d53f591c78450087",
}


def git(root: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def run(checker: Path, root: Path, policy: Path, output: Path, name: str,
        status: str, expected_rule: str | None = None) -> dict:
    proc = subprocess.run([str(checker), "audit", "types", "--root", str(root),
                           "--policy", str(policy), "--format", "agent-json", "--timeout", "4m"],
                          capture_output=True, text=True, timeout=270, check=False)
    (output / (name + ".json")).write_text(proc.stdout)
    (output / (name + ".stderr")).write_text(proc.stderr)
    report = json.loads(proc.stdout)
    assert report["status"] == status, (name, report)
    assert (proc.returncode == 0) == (status == "PASS"), (name, proc.returncode)
    assert report["checkedFactories"] == 1 and report["checkedReferences"] > 0, report
    if expected_rule:
        assert any(f["rule"] == expected_rule for f in report["findings"]), (name, expected_rule, report)
    else:
        assert not report["findings"], (name, report)
    print(f"{name}: {status}; factories={report['checkedFactories']} references={report['checkedReferences']}", flush=True)
    return report


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--consumer", choices=tuple(PINS), required=True)
    p.add_argument("--checkout", type=Path, required=True)
    p.add_argument("--checker", type=Path, required=True)
    p.add_argument("--policy", type=Path, required=True)
    p.add_argument("--output", type=Path, required=True)
    a = p.parse_args()
    checkout = a.checkout.resolve()
    assert git(checkout, "rev-parse", "HEAD") == PINS[a.consumer], "wrong consumer commit"
    assert not git(checkout, "status", "--porcelain", "--untracked-files=all"), "dirty consumer"
    root = checkout if a.consumer == "biz" else checkout / "backend-yunka"
    output = a.output.resolve()
    assert not output.is_relative_to(checkout), "evidence must be outside source"
    output.mkdir(parents=True, exist_ok=True)
    checker, policy = a.checker.resolve(), a.policy.resolve()
    before = git(checkout, "rev-parse", "HEAD^{tree}")
    positive = run(checker, root, policy, output, "original", "PASS")
    second = run(checker, root, policy, output, "repeat", "PASS")
    assert positive == second, "nondeterministic type report"
    if a.consumer == "biz":
        target = root / "internal/access/application/tenantlifecycle/internal/usecase/service.go"
        original = target.read_bytes()
        try:
            target.write_bytes(original + b"\nfunc (*service) AG04ForbiddenEscape() {}\n")
            run(checker, root, policy, output, "extra-method", "FAIL", "AG-TYPE-003")
        finally:
            target.write_bytes(original)
        factory = "github.com/hvritual/biz/internal/access/application/tenantlifecycle"
        invocation = "_, _ = owner.Build(nil, nil)"
    else:
        target = root / "internal/delivery/saved_view_service.go"
        original = target.read_bytes()
        needle = b"newSavedViewRepository(service.repository)"
        assert original.count(needle) == 1, "mutation anchor drift"
        try:
            target.write_bytes(original.replace(needle, b"service.repository"))
            run(checker, root, policy, output, "wide-object", "FAIL", "AG-TYPE-003")
        finally:
            target.write_bytes(original)
        factory = "github.com/hvritual/iot-delivery-system/backend-yunka/internal/delivery/application/savedview"
        invocation = "_, _ = owner.Build(nil, nil, nil)"
    unauthorized = root / "internal/ag04qualificationnegative"
    assert not unauthorized.exists(), "negative package already exists"
    unauthorized.mkdir()
    try:
        (unauthorized / "probe.go").write_text(
            f'package ag04qualificationnegative\nimport owner "{factory}"\n'
            f'func Probe() {{ {invocation} }}\n')
        # IoT's nil injected interface is additionally incomplete. Both cases
        # must still prove the exact unauthorized factory-use rule, not merely
        # fail to load/type-check a broken source package.
        run(checker, root, policy, output, "unauthorized-factory",
            "INCOMPLETE" if a.consumer == "iot" else "FAIL", "AG-TYPE-001")
    finally:
        (unauthorized / "probe.go").unlink()
        unauthorized.rmdir()
    restored = run(checker, root, policy, output, "restored", "PASS")
    assert restored == positive, "restored evidence differs"
    assert not git(checkout, "status", "--porcelain", "--untracked-files=all"), "consumer source changed"
    assert git(checkout, "rev-parse", "HEAD^{tree}") == before
    receipt = dict(schemaVersion=1, consumer=a.consumer, head=PINS[a.consumer], tree=before,
                   checkerSHA256=hashlib.sha256(checker.read_bytes()).hexdigest(),
                   policySHA256=hashlib.sha256(policy.read_bytes()).hexdigest(),
                   result="PASS", scope="typed-boundary-only-not-runtime", sourceRestored=True)
    (output / "RECEIPT.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt, sort_keys=True), flush=True)


if __name__ == "__main__":
    main()
