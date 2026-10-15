#!/usr/bin/env python3
"""Build a producer CLI bundle and test it against external locked Go modules.

This read-only qualification never tags, publishes, merges or edits a consumer.
The output directory must be outside the producer checkout.
"""
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parents[1]
MODULES = {f"github.com/hvritual/yunka.io/{m}" for m in ("pkg", "framework", "gateway", "infras")}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read_lock(path):
    value = json.loads(path.read_text())
    if value.get("schemaVersion") != 1 or not re.fullmatch(r"[0-9a-f]{40}", value.get("sdkSourceCommit", "")):
        raise ValueError("invalid release lock identity")
    rows = value.get("modules", [])
    if len(rows) != 4 or {m.get("path") for m in rows} != MODULES:
        raise ValueError("release lock must contain the four distinct SDK modules")
    for row in rows:
        if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", row.get("version", "")):
            raise ValueError("exact module version required")
        for key in ("sum", "goModSum"):
            if not re.fullmatch(r"h1:[A-Za-z0-9+/]{43}=", row.get(key, "")):
                raise ValueError("module content and go.mod checksums required")
    return value


def check_download(row, downloaded, source_commit):
    expected = {"Path": row["path"], "Version": row["version"], "Sum": row["sum"], "GoModSum": row["goModSum"]}
    if any(downloaded.get(k) != v for k, v in expected.items()):
        raise ValueError("download identity/checksum mismatch: " + row["path"])
    if downloaded.get("Origin", {}).get("Hash") != source_commit:
        raise ValueError("download source commit mismatch: " + row["path"])


def json_stream(contents):
    decoder = json.JSONDecoder()
    rows = []
    while contents.strip():
        value, end = decoder.raw_decode(contents.lstrip())
        rows.append(value)
        contents = contents.lstrip()[end:]
    return rows


def pack(directory, target):
    # Stable archive metadata allows byte comparison of repeated producer builds.
    with target.open("wb") as raw, gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as zipped:
        with tarfile.open(fileobj=zipped, mode="w") as archive:
            for path in sorted(directory.rglob("*")):
                if not path.is_file() or path.is_symlink():
                    continue
                item = tarfile.TarInfo(path.relative_to(directory).as_posix())
                item.size = path.stat().st_size
                item.mode = 0o755 if path.name == "yunka" else 0o644
                archive.addfile(item, io.BytesIO(path.read_bytes()))


def qualify(out):
    if out == ROOT or ROOT in out.parents or out.exists():
        raise ValueError("choose a new output directory outside the checkout")
    lock = read_lock(ROOT / "tools/consumer-release-lock.json")
    go = shutil.which("go")
    if not go:
        raise ValueError("locked Go toolchain is required")
    env = dict(os.environ, GOWORK="off", GOTOOLCHAIN="local", GOENV="off", GOFLAGS="")
    for key in ("GOROOT", "GOOS", "GOARCH"):
        env.pop(key, None)
    out.mkdir(parents=True)
    (out / "logs").mkdir()
    receipts = []

    def run(label, args, cwd=ROOT, expected=0):
        result = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True, timeout=900)
        (out / "logs" / (label + ".log")).write_text(result.stdout + result.stderr)
        receipts.append({"label": label, "argv": list(map(str, args)), "exitCode": result.returncode, "expectedExitCode": expected})
        (out / "commands.json").write_text(json.dumps(receipts, indent=2) + "\n")
        if result.returncode != expected:
            raise RuntimeError(f"{label}: unexpected exit {result.returncode}\n" + (result.stdout + result.stderr)[-4000:])
        return result.stdout.strip()

    before = run("clean-before", ["git", "status", "--porcelain", "--untracked-files=all"])
    if before:
        raise ValueError("producer must be committed and clean")
    commit = run("commit", ["git", "rev-parse", "HEAD"])
    tree = run("tree", ["git", "rev-parse", "HEAD^{tree}"])
    delta = run("sdk-source-delta", ["git", "diff", "--name-only", lock["sdkSourceCommit"], commit, "--",
                "pkg", "framework", "gateway", "infras", "contracts/proto",
                ":(exclude)**/go.mod", ":(exclude)**/go.sum", ":(exclude)**/*.md"])
    if delta:
        raise ValueError("SDK source differs from downloaded lock; qualify a new source version: " + delta)
    toolchain = dict(line.split("=", 1) for line in (ROOT / "tools/toolchain.env").read_text().splitlines() if "=" in line and not line.startswith("#"))
    if run("go-version", [go, "env", "GOVERSION"]) != "go" + toolchain["GO_VERSION"]:
        raise ValueError("Go version differs from tools/toolchain.env")
    if run("go-platform", [go, "env", "GOOS", "GOARCH"]) != "linux\namd64":
        raise ValueError("this candidate bundle qualifies linux/amd64 only")
    if run("protoc-version", ["protoc", "--version"]) != "libprotoc " + toolchain["PROTOC_VERSION"]:
        raise ValueError("protoc version differs from tools/toolchain.env")
    for exe, key in (("protoc-gen-go", "PROTOC_GEN_GO_VERSION"), ("protoc-gen-go-grpc", "PROTOC_GEN_GO_GRPC_VERSION")):
        if run(exe + "-version", [exe, "--version"]).split()[-1].removeprefix("v") != toolchain[key].removeprefix("v"):
            raise ValueError("protobuf plugin version mismatch")

    downloads = []
    for row in lock["modules"]:
        name = row["path"].rsplit("/", 1)[1]
        download = json.loads(run("download-" + name, [go, "mod", "download", "-json", row["path"] + "@" + row["version"]], out))
        check_download(row, download, lock["sdkSourceCommit"])
        downloads.append(download)
    (out / "downloads.json").write_text(json.dumps(downloads, indent=2) + "\n")

    # Compile each producer module without the workspace that hid the old CLI graph.
    for name in ("framework", "gateway", "infras"):
        run("isolated-build-" + name, [go, "build", "-p=4", "-mod=readonly", "./..."], ROOT / name)
    scanner = [go, "run", "golang.org/x/vuln/cmd/govulncheck@" + toolchain["GOVULNCHECK_VERSION"], "./..."]
    for name in ("pkg", "framework", "gateway", "infras", "app"):
        run("isolated-security-" + name, scanner, ROOT / name)
    bundle = out / "bundle"
    (bundle / "bin").mkdir(parents=True)
    version = "0.0.0-dev+" + commit[:12]
    binary = bundle / "bin/yunka"
    # Source/type audit intentionally trusts the checker's compiled-in GOROOT.
    # -trimpath erases it and makes that existing fail-closed contract unusable.
    # Keep the root explicit as an installation prerequisite, never override it.
    required_go_root = run("go-root", [go, "env", "GOROOT"])
    build = [go, "build", "-p=4", "-mod=readonly", "-buildvcs=true", "-ldflags=-X main.releaseVersion=" + version, "-o"]
    run("cli-build", build + [str(binary), "./cmd"], ROOT / "app")
    second = out / "yunka-second"
    run("cli-rebuild", build + [str(second), "./cmd"], ROOT / "app")
    if sha(binary) != sha(second):
        raise ValueError("CLI rebuild is not byte-identical")
    if run("cli-version", [str(binary), "--version"]) != "yunka version " + version:
        raise ValueError("CLI version does not identify producer commit")
    build_info = run("cli-build-info", [go, "version", "-m", str(binary)]).splitlines()[1:]
    if not any("vcs.revision=" + commit in line for line in build_info) or any("vcs.modified=true" in line for line in build_info):
        raise ValueError("CLI VCS identity missing or dirty")
    shutil.copytree(ROOT / "contracts/proto", bundle / "include")
    shutil.copyfile(ROOT / "tools/consumer-release-lock.json", bundle / "sdk-lock.json")
    licenses = [p for p in ROOT.iterdir() if p.is_file() and p.name.upper().startswith(("LICENSE", "COPYING", "NOTICE"))]
    for path in licenses:
        shutil.copyfile(path, bundle / path.name)
    manifest = {"schemaVersion": 1, "status": "CANDIDATE_NOT_PUBLISHED", "cliModule": "yunka.io/app", "version": version,
                "producerCommit": commit, "producerTree": tree, "sdkLock": lock, "toolchain": toolchain,
                "requiredGoRoot": required_go_root,
                "platform": "linux/amd64", "toolSha256": {name: sha(Path(shutil.which(name))) for name in ("go", "protoc", "protoc-gen-go", "protoc-gen-go-grpc")},
                "buildInfo": build_info, "repositoryLicenseFiles": [p.name for p in licenses],
                "files": {p.relative_to(bundle).as_posix(): sha(p) for p in sorted(bundle.rglob("*")) if p.is_file()}}
    (bundle / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    archive = out / "yunka-linux-amd64.tar.gz"
    pack(bundle, archive)
    # Consume the packaged bytes in a distinct directory; no producer proto path.
    installed = out / "installed"
    with tarfile.open(archive) as source:
        source.extractall(installed, filter="data")
    cli = installed / "bin/yunka"
    cli.chmod(0o755)
    for path, expected in manifest["files"].items():
        if sha(installed / path) != expected:
            raise ValueError("bundle file mismatch: " + path)
    consumer = out / "consumer"
    shutil.copytree(ROOT / "tools/release-consumer", consumer)
    requires = "\n".join("\t" + m["path"] + " " + m["version"] for m in lock["modules"])
    language = re.search(r"(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$", (ROOT / "app/go.mod").read_text())
    if not language:
        raise ValueError("producer language directive missing")
    (consumer / "go.mod").write_text("module example.com/releaseconsumer\n\ngo " + language[1] + "\n\nrequire (\n" + requires + "\n)\n")
    run("consumer-module", [str(cli), "module", "new", "--name", "catalog", "--root", str(consumer / "modules"), "--no-config", "--no-logger"], consumer)
    run("consumer-init", [str(cli), "init", "--root", str(consumer)], consumer)
    args = ["--root", str(consumer), "--proto-path", str(installed / "include"), "--full", "--format", "json"]
    run("consumer-generate-first", [str(cli), "generate"] + args, consumer)

    def snapshot():
        return {p.relative_to(consumer).as_posix(): sha(p) for p in sorted(consumer.rglob("*")) if p.is_file()}

    first = snapshot()
    run("consumer-generate-second", [str(cli), "generate"] + args, consumer)
    if snapshot() != first:
        raise ValueError("second generation drift")
    run("consumer-check", [str(cli), "check"] + args, consumer)
    if snapshot() != first:
        raise ValueError("check modified consumer bytes")
    run("consumer-tidy", [go, "mod", "tidy"], consumer)
    graph = json_stream(run("consumer-graph", [go, "list", "-m", "-json", "all"], consumer))
    for row in lock["modules"]:
        actual = next(m for m in graph if m["Path"] == row["path"])
        if actual.get("Version") != row["version"] or actual.get("Sum") != row["sum"] or "Replace" in actual:
            raise ValueError("consumer selected a different SDK graph")
    if any("Replace" in m for m in graph):
        raise ValueError("consumer replacement is not allowed")
    run("consumer-verify", [go, "mod", "verify"], consumer)
    # This disposable source fixture has no Git history. Producer stamping above
    # remains mandatory; do not discover an unrelated parent checkout for it.
    run("consumer-build", [go, "build", "-buildvcs=false", "-mod=readonly", "./..."], consumer)
    run("consumer-tests", [go, "test", "-buildvcs=false", "-mod=readonly", "-count=1", "-v", "./..."], consumer)
    run("consumer-vet", [go, "vet", "-mod=readonly", "./..."], consumer)
    run("consumer-security", scanner, consumer)
    audit = json.loads(run("consumer-source-audit", [str(cli), "audit", "source", "--root", str(consumer), "--format", "agent-json", "--timeout", "10m"], consumer))
    if audit.get("status") != "PASS" or not audit.get("sourceUnchanged"):
        raise ValueError("packaged source audit did not establish read-only completeness")
    generated = consumer / "contracts/generated/operation-plans.json"
    original = generated.read_bytes()
    generated.write_bytes(b"{}\n")
    drifted = snapshot()
    rejection = json.loads(run("consumer-reject-generated-drift", [str(cli), "check"] + args, consumer, expected=1))
    if rejection.get("ok") is not False or "YUNKA-DX-CONTRACT-002" not in json.dumps(rejection):
        raise ValueError("negative control failed for a reason other than contract drift")
    if snapshot() != drifted:
        raise ValueError("failed check repaired consumer")
    generated.write_bytes(original)
    if run("clean-after", ["git", "status", "--porcelain", "--untracked-files=all"]):
        raise ValueError("qualification modified producer")
    report = {"status": "PASS", "scope": "packaged CLI plus independent four-module fixture; not cs-biz runtime or issue234 completion",
              "producerCommit": commit, "producerTree": tree, "archive": archive.name, "archiveSha256": sha(archive),
              "manifestSha256": sha(bundle / "manifest.json"), "sdkLockSha256": sha(ROOT / "tools/consumer-release-lock.json"),
              "generatedFirstSecond": first, "consumerGraph": graph, "commands": receipts}
    (out / "qualification.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({k: report[k] for k in ("status", "producerCommit", "archiveSha256", "scope")}, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path)
    qualify(parser.parse_args().out.resolve())
