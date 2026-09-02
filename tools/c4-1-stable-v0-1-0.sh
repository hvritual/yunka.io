#!/usr/bin/env bash
set -euo pipefail

BRANCH="${BRANCH:-release/c4-1-v0.1.0}"
STABLE_VERSION="${STABLE_VERSION:-v0.1.0}"
MAIN_MERGE_SHA="${MAIN_MERGE_SHA:-94113e9fb9fd021f773efd626dba49489b5b2071}"
PKG_QUALIFIED_SHA="${PKG_QUALIFIED_SHA:-abdcc78c571b68e3ece4fed0014c20f75cf163ec}"
PKG_STABLE_SHA="${PKG_STABLE_SHA:-$MAIN_MERGE_SHA}"
export GONOPROXY="${GONOPROXY:-github.com/hvritual/yunka.io/*}"
export GONOSUMDB="${GONOSUMDB:-github.com/hvritual/yunka.io/*}"

log() { printf '\n==> %s\n' "$*"; }
tag_sha() { git ls-remote --tags origin "refs/tags/$1/${STABLE_VERSION}" | awk '{print $1}'; }
tag_exists() { git ls-remote --exit-code --tags origin "refs/tags/$1/${STABLE_VERSION}" >/dev/null 2>&1; }

log "preflight"
git config user.name 'github-actions[bot]'
git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
git merge-base --is-ancestor "$MAIN_MERGE_SHA" HEAD
git merge-base --is-ancestor "$PKG_QUALIFIED_SHA" "$MAIN_MERGE_SHA"
test "$PKG_STABLE_SHA" = "$MAIN_MERGE_SHA"
make toolchain-check
make rpc-tools
make rpc-toolchain-check
test -z "$(git grep -n '^replace ' -- pkg/go.mod framework/go.mod gateway/go.mod || true)"
test -z "$(git status --porcelain)"

log "publish pkg ${STABLE_VERSION}"
git diff --exit-code "$PKG_QUALIFIED_SHA" "$PKG_STABLE_SHA" -- pkg
if tag_exists pkg; then
  actual="$(tag_sha pkg)"
  test "$actual" = "$PKG_STABLE_SHA" || { echo "pkg stable tag points to unexpected sha: $actual" >&2; exit 1; }
else
  git tag "pkg/${STABLE_VERSION}" "$PKG_STABLE_SHA"
  git push origin "refs/tags/pkg/${STABLE_VERSION}"
fi
test "$(tag_sha pkg)" = "$PKG_STABLE_SHA"
(
  cd pkg
  GOWORK=off go mod tidy -diff
  GOWORK=off go list -m all >/dev/null
  GOWORK=off go test ./...
  GOWORK=off go vet ./...
  GOWORK=off go build ./...
)
tmp="$(mktemp -d)"
(
  cd "$tmp"
  GOWORK=off go mod init yunka-pkg-stable-probe >/dev/null
  GOPROXY=direct GOWORK=off go get "github.com/hvritual/yunka.io/pkg@${STABLE_VERSION}"
  test "$(GOPROXY=direct GOWORK=off go list -m -f '{{.Version}}' github.com/hvritual/yunka.io/pkg)" = "$STABLE_VERSION"
)
rm -rf "$tmp"

log "publish framework ${STABLE_VERSION}"
if tag_exists framework; then
  framework_sha="$(tag_sha framework)"
  git merge-base --is-ancestor "$framework_sha" HEAD
  grep -q "github.com/hvritual/yunka.io/pkg ${STABLE_VERSION}" framework/go.mod
else
  (cd framework && go mod edit -require="github.com/hvritual/yunka.io/pkg@${STABLE_VERSION}")
  (cd framework && GOPROXY=direct GOWORK=off go mod tidy)
  grep -q "github.com/hvritual/yunka.io/pkg ${STABLE_VERSION}" framework/go.mod
  test -z "$(grep '^replace ' framework/go.mod || true)"
  (
    cd framework
    GOPROXY=direct GOWORK=off go mod tidy -diff
    GOPROXY=direct GOWORK=off go list -m all >/dev/null
    GOPROXY=direct GOWORK=off go test ./...
    GOPROXY=direct GOWORK=off go vet ./...
    GOPROXY=direct GOWORK=off go build ./...
  )
  git add framework/go.mod framework/go.sum
  git commit -m 'release: pin framework to pkg v0.1.0'
  git push origin "HEAD:${BRANCH}"
  framework_sha="$(git rev-parse HEAD)"
  git tag "framework/${STABLE_VERSION}" "$framework_sha"
  git push origin "refs/tags/framework/${STABLE_VERSION}"
fi
test "$(tag_sha framework)" = "$framework_sha"

log "publish gateway ${STABLE_VERSION}"
if tag_exists gateway; then
  gateway_sha="$(tag_sha gateway)"
  git merge-base --is-ancestor "$gateway_sha" HEAD
  grep -q "github.com/hvritual/yunka.io/pkg ${STABLE_VERSION}" gateway/go.mod
  grep -q "github.com/hvritual/yunka.io/framework ${STABLE_VERSION}" gateway/go.mod
else
  (cd gateway && go mod edit -require="github.com/hvritual/yunka.io/pkg@${STABLE_VERSION}")
  (cd gateway && go mod edit -require="github.com/hvritual/yunka.io/framework@${STABLE_VERSION}")
  (cd gateway && GOPROXY=direct GOWORK=off go mod tidy)
  grep -q "github.com/hvritual/yunka.io/pkg ${STABLE_VERSION}" gateway/go.mod
  grep -q "github.com/hvritual/yunka.io/framework ${STABLE_VERSION}" gateway/go.mod
  test -z "$(grep '^replace ' gateway/go.mod || true)"
  (
    cd gateway
    GOPROXY=direct GOWORK=off go mod tidy -diff
    GOPROXY=direct GOWORK=off go list -m all >/dev/null
    GOPROXY=direct GOWORK=off go test ./...
    GOPROXY=direct GOWORK=off go vet ./...
    GOPROXY=direct GOWORK=off go build ./...
  )
  git add gateway/go.mod gateway/go.sum
  git commit -m 'release: pin gateway to stable Yunka v0.1.0'
  git push origin "HEAD:${BRANCH}"
  gateway_sha="$(git rev-parse HEAD)"
  git tag "gateway/${STABLE_VERSION}" "$gateway_sha"
  git push origin "refs/tags/gateway/${STABLE_VERSION}"
fi
test "$(tag_sha gateway)" = "$gateway_sha"

log "converge repository bookkeeping"
(cd app && go mod edit -require="github.com/hvritual/yunka.io/pkg@${STABLE_VERSION}")
(cd app && GOPROXY=direct GOWORK=off go mod tidy)
python3 - <<'PY'
from pathlib import Path
path = Path('tools/module-release.env')
lines = path.read_text().splitlines()
out = []
found = False
for line in lines:
    if line.startswith('YUNKA_MODULE_RELEASE='):
        out.append('YUNKA_MODULE_RELEASE=v0.1.0')
        found = True
    else:
        out.append(line)
if not found:
    raise SystemExit('YUNKA_MODULE_RELEASE line missing')
path.write_text('\n'.join(out) + '\n')
PY
rm -f go.work.sum
go list -m all >/dev/null
make rpc-check
make contract
make contract-check
git diff --exit-code -- contracts/generated
grep -q "github.com/hvritual/yunka.io/pkg ${STABLE_VERSION}" app/go.mod
grep -q "YUNKA_MODULE_RELEASE=${STABLE_VERSION}" tools/module-release.env
git add app/go.mod app/go.sum tools/module-release.env go.work.sum
if ! git diff --cached --quiet; then
  git commit -m 'release: converge repository bookkeeping to v0.1.0'
  git push origin "HEAD:${BRANCH}"
fi
workspace_sha="$(git rev-parse HEAD)"

log "qualify stable workspace"
test -z "$(git status --porcelain)"
(cd app && go test ./cmd -run TestC116CExpertCommandCompatibilitySnapshots -count=1)
(cd app && go test ./cmd/projectflow -run TestC114QualificationRealBinaryFastFeedback -count=1)
make tidy
make dependency-check
git diff --exit-code -- go.work go.work.sum pkg/go.mod pkg/go.sum framework/go.mod framework/go.sum gateway/go.mod gateway/go.sum app/go.mod app/go.sum
make rpc-check
make contract
make contract-check
git diff --exit-code -- contracts/generated go.work.sum
for module in pkg framework gateway app; do
  echo "==> workspace ${module}"
  (cd "$module" && go test ./... && go vet ./... && go build ./...)
done
for module in pkg framework gateway; do
  echo "==> standalone ${module}"
  (
    cd "$module"
    GOPROXY=direct GOWORK=off go mod tidy -diff
    GOPROXY=direct GOWORK=off go list -m all >/dev/null
    GOPROXY=direct GOWORK=off go test ./...
    GOPROXY=direct GOWORK=off go vet ./...
    GOPROXY=direct GOWORK=off go build ./...
  )
done

log "qualify clean external consumer"
tmp="$(mktemp -d)"
(
  cd "$tmp"
  GOWORK=off go mod init yunka-v0-1-0-external-probe >/dev/null
  GOPROXY=direct GOWORK=off go get "github.com/hvritual/yunka.io/pkg@${STABLE_VERSION}"
  GOPROXY=direct GOWORK=off go get "github.com/hvritual/yunka.io/framework@${STABLE_VERSION}"
  GOPROXY=direct GOWORK=off go get "github.com/hvritual/yunka.io/gateway@${STABLE_VERSION}"
  test "$(GOPROXY=direct GOWORK=off go list -m -f '{{.Version}}' github.com/hvritual/yunka.io/pkg)" = "$STABLE_VERSION"
  test "$(GOPROXY=direct GOWORK=off go list -m -f '{{.Version}}' github.com/hvritual/yunka.io/framework)" = "$STABLE_VERSION"
  test "$(GOPROXY=direct GOWORK=off go list -m -f '{{.Version}}' github.com/hvritual/yunka.io/gateway)" = "$STABLE_VERSION"
)
rm -rf "$tmp"
test -z "$(git status --porcelain)"

log "record stable publication"
pkg_sha="$(tag_sha pkg)"
framework_sha="$(tag_sha framework)"
gateway_sha="$(tag_sha gateway)"
cat >> docs/waves/C4.1-module-publication-convergence.md <<EOF

## Stable v0.1.0 publication record

Stable module publication completed successfully in workflow run ${GITHUB_RUN_ID:-unknown} after C4.1 canonical integration at ${MAIN_MERGE_SHA}.

- pkg/v0.1.0 -> ${pkg_sha}
- framework/v0.1.0 -> ${framework_sha}
- gateway/v0.1.0 -> ${gateway_sha}
- repository release bookkeeping -> ${workspace_sha}

The stable Gate covered independent GOWORK=off tidy/test/vet/build for all publishable modules, the clean four-module source workspace, dependency-policy and generated-contract reproducibility, and a clean external consumer resolving all three modules at v0.1.0 without repository-local replacements.
EOF

git rm -f .github/workflows/c4-1-stable-v0-1-0.yml tools/c4-1-stable-v0-1-0.sh
git add docs/waves/C4.1-module-publication-convergence.md
git commit -m 'docs: record Yunka v0.1.0 stable publication'
git push origin "HEAD:${BRANCH}"
