#!/usr/bin/env python3
"""Read back exact Git-bound generated HTTP/gRPC semantic conformance evidence.

This is an independent evidence-integrity check, not an HTTP/gRPC interpreter.
The canonical Go contract tests own behavioral assertions and mandatory cases.
"""
import json
from pathlib import Path
import re
import subprocess
import sys


def git_revision(revision: str) -> str:
    return subprocess.check_output(
        ["git", "rev-parse", "--verify", revision], text=True
    ).strip()


def verify_evidence(document: dict, candidate: str, tree: str) -> str:
    if document.get("schemaVersion") != 1:
        raise ValueError("unsupported protocol evidence schema")
    if document.get("candidateSHA") != candidate or document.get("candidateTree") != tree:
        raise ValueError("matrix does not identify the exact checked-out commit/tree")
    for field in ("protobufSHA256", "descriptorSHA256", "generatedSHA256"):
        if re.fullmatch(r"[0-9a-f]{64}", document.get(field, "")) is None:
            raise ValueError(f"missing/invalid {field}")
    if document.get("generatedFileCount", 0) < 1:
        raise ValueError("matrix does not prove generated adapters exist")
    cases = document.get("cases")
    if not isinstance(cases, list) or len(cases) < 30:
        raise ValueError("generated semantic case inventory incomplete")
    if document.get("caseCount") != len(cases):
        raise ValueError("caseCount differs from actual evidence")
    if len({case["case"] for case in cases}) != len(cases):
        raise ValueError("duplicate conformance case identity")
    unsupported = 0
    for item in cases:
        if not item.get("diagnostic") or not isinstance(item.get("supported"), bool):
            raise ValueError("case lacks an explicit support boundary or diagnostic")
        if item.get("request-equivalent") is False or item.get("response-equivalent") is False:
            raise ValueError(f"semantic divergence: {item['case']}")
        if not item["supported"]:
            unsupported += 1
            if item.get("request-equivalent") is not None or item.get("response-equivalent") is not None:
                raise ValueError("unsupported compiler case falsely claims runtime equivalence")
            if "UNSUPPORTED_HTTP_BINDING" not in item["diagnostic"]:
                raise ValueError("unsupported case lacks actual compilation rejection")
    if not unsupported:
        raise ValueError("unsupported compiler negatives were omitted")
    return (
        f"HTTP_GRPC_CONFORMANCE_ACCEPTED commit={candidate} tree={tree} "
        f"generated={document['generatedSHA256']} "
        f"cases={len(cases)} unsupported={unsupported}"
    )


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: verify_http_grpc_conformance.py <matrix.json>", file=sys.stderr)
        return 2
    try:
        data = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
        print(verify_evidence(data, git_revision("HEAD"), git_revision("HEAD^{tree}")))
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as exc:
        print(f"HTTP_GRPC_CONFORMANCE_REJECTED: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
