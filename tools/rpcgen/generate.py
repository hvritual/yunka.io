#!/usr/bin/env python3
"""Deterministic standard protobuf/gRPC generation for the canonical RPC root."""
from __future__ import annotations

import argparse
import filecmp
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

MANAGED_ROOTS = (
    Path("gateway/rpc/meta"),
    Path("pkg/rpcmeta/legacy"),
    Path("pkg/contractdsl/v1"),
)
PROTO_FILES = (
    "yunka/dsl/v1/options.proto",
    "gateway/api_common.proto",
    "gateway/common.proto",
    "gateway/gateway.proto",
    "legacy/api.proto",
    "legacy/api_common.proto",
    "legacy/common.proto",
    "legacy/unit.proto",
)


def parser() -> argparse.ArgumentParser:
    value = argparse.ArgumentParser()
    value.add_argument("--repo-root", required=True)
    value.add_argument("--protoc", required=True)
    value.add_argument("--protoc-gen-go", required=True)
    value.add_argument("--protoc-gen-go-grpc", required=True)
    value.add_argument("--check", action="store_true")
    return value


def resolve_executable(value: str, label: str) -> Path:
    candidate = Path(value).expanduser()
    if candidate.is_file():
        return candidate.resolve()
    resolved = shutil.which(value)
    if resolved:
        return Path(resolved).resolve()
    raise RuntimeError(f"{label} is not executable or discoverable on PATH: {value}")


def protoc_include(protoc: Path) -> Path:
    candidates = (
        protoc.parent.parent / "include",
        protoc.parent / "include",
    )
    for candidate in candidates:
        if (candidate / "google" / "protobuf" / "descriptor.proto").is_file():
            return candidate.resolve()
    raise RuntimeError(f"locked protoc standard include directory not found next to {protoc}")


def require_file(path: Path, label: str) -> None:
    if not path.is_file():
        raise RuntimeError(f"{label} is not a file: {path}")


def generated_files(root: Path) -> dict[Path, Path]:
    result: dict[Path, Path] = {}
    for managed in MANAGED_ROOTS:
        source_root = root / managed
        if not source_root.exists():
            continue
        for path in source_root.rglob("*.pb.go"):
            relative = path.relative_to(root)
            if path.is_symlink():
                raise RuntimeError(f"generated output may not be a symlink: {relative}")
            result[relative] = path
    return result


def compare_outputs(repo: Path, staged: Path) -> list[str]:
    expected = generated_files(staged)
    actual = generated_files(repo)
    messages: list[str] = []
    for relative in sorted(expected.keys() | actual.keys()):
        left = expected.get(relative)
        right = actual.get(relative)
        if left is None:
            messages.append(f"stale generated file: {relative}")
        elif right is None:
            messages.append(f"missing generated file: {relative}")
        elif not filecmp.cmp(left, right, shallow=False):
            messages.append(f"generated content drift: {relative}")
    return messages


def install_outputs(repo: Path, staged: Path) -> None:
    expected = generated_files(staged)
    actual = generated_files(repo)
    for relative in sorted(actual.keys() - expected.keys()):
        actual[relative].unlink()
    for relative, source in sorted(expected.items()):
        target = repo / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        temporary = target.with_name(target.name + ".tmp")
        shutil.copyfile(source, temporary)
        os.chmod(temporary, 0o644)
        os.replace(temporary, target)


def main() -> int:
    args = parser().parse_args()
    repo = Path(args.repo_root).resolve()
    proto_root = repo / "contracts" / "proto"
    protoc = resolve_executable(args.protoc, "protoc")
    include_root = protoc_include(protoc)
    protoc_gen_go = resolve_executable(args.protoc_gen_go, "protoc-gen-go")
    protoc_gen_go_grpc = resolve_executable(args.protoc_gen_go_grpc, "protoc-gen-go-grpc")
    for name in PROTO_FILES:
        require_file(proto_root / name, f"canonical proto {name}")

    with tempfile.TemporaryDirectory(prefix="yunka-rpc-generate-") as directory:
        staged = Path(directory)
        command = [
            str(protoc),
            f"-I{proto_root}",
            f"-I{include_root}",
            f"--plugin=protoc-gen-go={protoc_gen_go}",
            f"--plugin=protoc-gen-go-grpc={protoc_gen_go_grpc}",
            f"--go_out={staged}",
            "--go_opt=module=yunka.io",
            f"--go-grpc_out={staged}",
            "--go-grpc_opt=module=yunka.io,require_unimplemented_servers=false",
            *PROTO_FILES,
        ]
        subprocess.run(command, cwd=repo, check=True)
        if args.check:
            drift = compare_outputs(repo, staged)
            if drift:
                for item in drift:
                    print(f"RPC DRIFT: {item}", file=sys.stderr)
                return 1
            print(f"rpc-check: {len(generated_files(staged))} generated files are current")
            return 0
        install_outputs(repo, staged)
        print(f"rpc-generate: wrote {len(generated_files(staged))} generated files")
        return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode) from error
    except Exception as error:
        print(f"rpc generation failed: {error}", file=sys.stderr)
        raise SystemExit(1) from error
