"""Read-only public CLI qualification; no consumer source or runtime changes."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

binary, old_binary, framework, consumer, evidence = map(Path, sys.argv[1:])
root = consumer / "backend-yunka"
evidence.mkdir(parents=True, exist_ok=True)

def run(*args, checked=True):
    result = subprocess.run([str(binary), "context", "--root", str(root), *args], capture_output=True, timeout=120)
    if checked and result.returncode != 0:
        raise RuntimeError(result.stderr.decode())
    return result

def snapshot():
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob("*") if p.is_file() and not p.is_symlink()}

before = snapshot()
old = subprocess.run([str(old_binary), "context", "--root", str(root), "--operation", "issue160.probe", "--json"], capture_output=True, timeout=120)
(evidence / "cli-red.log").write_bytes(old.stdout + old.stderr)
assert old.returncode != 0 and b"flag provided but not defined" in old.stdout + old.stderr

bootstrap = run("--json")
boot = json.loads(bootstrap.stdout)
assert boot["schemaVersion"] == 5 and "contractContext" not in boot
assert boot["agentProtocol"]["operationContext"] == "yunka context --operation <operation> --json"
extra = ["--proto-path", str(framework / "contracts/proto")]
first = run("--all-operations", *extra, "--json")
again = run("--all-operations", *extra, "--json")
assert first.stdout == again.stdout
all_value = json.loads(first.stdout)
contract = all_value["contractContext"]
assert contract["authority"] == "read_only"
assert contract["scope"] == "canonical_file_import_closure"
operations = contract["operations"]
plans = json.loads((root / "contracts/generated/operation-plans.json").read_text())
expected_ids = sorted(p["operationId"] for p in plans["operations"])
assert [p["operationId"] for p in operations] == expected_ids
assert len(operations) == 25
for op in operations:
    one = run("--operation", op["operationId"], *extra, "--json")
    value = json.loads(one.stdout)
    assert value["contractContext"]["operations"] == [op]
    assert op["sourceFiles"] == sorted(set(op["sourceFiles"]))
    for source in op["sourceFiles"]:
        path = root / source
        assert not Path(source).is_absolute() and path.is_file()
        assert path.resolve().is_relative_to(root.resolve())
    assert str(root).encode() not in one.stdout
    assert b'"editablePaths"' not in one.stdout
text = run("--operation", expected_ids[0], *extra)
assert b"authority=read_only" in text.stdout
for flags in [["--operation", "not.a.current.operation"], ["--operation", ""], ["--operation", expected_ids[0], "--all-operations"]]:
    rejected = run(*flags, *extra, "--json", checked=False)
    assert rejected.returncode != 0 and not rejected.stdout
assert before == snapshot(), "CLI mutated pinned consumer"
assert not subprocess.check_output(["git", "-C", str(consumer), "status", "--porcelain"])
(evidence / "all-contexts.json").write_bytes(first.stdout)
(evidence / "single-context.txt").write_bytes(text.stdout)
report = {
    "consumer": subprocess.check_output(["git", "-C", str(consumer), "rev-parse", "HEAD"], text=True).strip(),
    "framework": subprocess.check_output(["git", "-C", str(framework), "rev-parse", "HEAD"], text=True).strip(),
    "schemaVersion": 5, "operations": len(operations),
    "allAndSingleAgree": True, "deterministic": True, "sourcePathsExist": True,
    "readOnly": True, "unknownAndInvalidFailClosed": True,
    "scope": "public context CLI / source provenance only; not consumer runtime or ChangeSet enforcement"
}
(evidence / "consumer.json").write_text(json.dumps(report, indent=2) + "\n")
print(json.dumps(report))
