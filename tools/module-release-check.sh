#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
output="${YUNKA_RELEASE_OUT:-$(mktemp -d)/qualification}"
python3 "$ROOT/tools/qualify_consumer_release.py" --out "$output"
echo "module-release-check: evidence and candidate bundle: $output"
