#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path


def extract_method(source: str, receiver: str, method: str) -> str:
    pattern = re.compile(
        rf"func\s*\(\s*[A-Za-z_]\w*\s+\*\s*{re.escape(receiver)}\s*\)\s*{re.escape(method)}\s*\("
    )
    match = pattern.search(source)
    if match is None:
        raise RuntimeError(f"method not found: (*{receiver}).{method}")
    start = match.start()
    brace = source.find("{", match.end())
    if brace < 0:
        raise RuntimeError(f"method body not found: (*{receiver}).{method}")

    depth = 0
    state = "code"
    escaped = False
    index = brace
    while index < len(source):
        ch = source[index]
        nxt = source[index + 1] if index + 1 < len(source) else ""
        if state == "code":
            if ch == "/" and nxt == "/":
                state = "line-comment"
                index += 2
                continue
            if ch == "/" and nxt == "*":
                state = "block-comment"
                index += 2
                continue
            if ch == '"':
                state = "string"
                escaped = False
            elif ch == "'":
                state = "rune"
                escaped = False
            elif ch == "`":
                state = "raw"
            elif ch == "{":
                depth += 1
            elif ch == "}":
                depth -= 1
                if depth == 0:
                    fragment = source[start : index + 1]
                    lines = [line.rstrip() for line in fragment.replace("\r\n", "\n").split("\n")]
                    return "\n".join(lines).strip() + "\n"
        elif state == "line-comment":
            if ch == "\n":
                state = "code"
        elif state == "block-comment":
            if ch == "*" and nxt == "/":
                state = "code"
                index += 2
                continue
        elif state in {"string", "rune"}:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif (state == "string" and ch == '"') or (state == "rune" and ch == "'"):
                state = "code"
        elif state == "raw":
            if ch == "`":
                state = "code"
        index += 1
    raise RuntimeError(f"unterminated method body: (*{receiver}).{method}")


def digest_method(source: str, receiver: str, method: str) -> str:
    return hashlib.sha256(extract_method(source, receiver, method).encode("utf-8")).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=".")
    parser.add_argument("--baseline", default="tools/rpc-consumer-abi.json")
    args = parser.parse_args()
    root = Path(args.repo_root).resolve()
    baseline_path = root / args.baseline
    baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    if baseline.get("schemaVersion") != 1:
        raise RuntimeError("unsupported RPC consumer ABI baseline")
    checked = 0
    for item in baseline.get("consumers", []):
        path = root / item["path"]
        source = path.read_text(encoding="utf-8")
        receiver = item["receiver"]
        for method, expected in sorted(item["methods"].items()):
            actual = digest_method(source, receiver, method)
            if actual != expected:
                raise RuntimeError(
                    f"business method drift: {item['path']} (*{receiver}).{method} actual={actual} expected={expected}"
                )
            checked += 1
    print(f"rpc-consumer-abi: checked={checked} status=ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
