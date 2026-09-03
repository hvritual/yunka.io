#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "$ROOT/tools/module-release.env"
version="${YUNKA_RELEASE_VERSION:-$YUNKA_MODULE_RELEASE}"
fail() { echo "module-release-check: $*" >&2; exit 1; }
check_module() {
  local rel="$1" expected="$2"
  local actual
  actual="$(cd "$ROOT/$rel" && GOWORK=off go list -m -f '{{.Path}}')"
  [[ "$actual" == "$expected" ]] || fail "$rel declares $actual, expected $expected"
  if grep -q '^replace ' "$ROOT/$rel/go.mod"; then
    fail "$rel/go.mod contains a release-invalid replace directive"
  fi
  (cd "$ROOT/$rel" && GOWORK=off go list -m all >/dev/null)
}
check_module pkg "$YUNKA_PKG_MODULE"
check_module framework "$YUNKA_FRAMEWORK_MODULE"
check_module gateway "$YUNKA_GATEWAY_MODULE"
check_module infras "$YUNKA_INFRAS_MODULE"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
GOWORK=off go mod init yunka-release-probe >/dev/null
GOWORK=off go get "$YUNKA_PKG_MODULE@$version"
GOWORK=off go get "$YUNKA_FRAMEWORK_MODULE@$version"
GOWORK=off go get "$YUNKA_GATEWAY_MODULE@$version"
GOWORK=off go get "$YUNKA_INFRAS_MODULE@$version"
GOWORK=off go list -m all >/dev/null
echo "module-release-check: external consumer resolved $version"
