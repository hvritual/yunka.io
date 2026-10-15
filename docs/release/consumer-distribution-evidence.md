# Consumer distribution repair evidence

> Document class: **EVIDENCE**
> Original evaluated producer: `3527488325030fbf86fe52804140c936e9d937a4`
> Consumer pressure: cs-biz #11 / PR #27; authorized upstream repair in the active ChatGPT conversation at 2026-10-15 18:56:20 +08

## Reproduced baseline failures

With official Go 1.25.13 installed and its downloaded archive SHA256 checked:

1. `GOWORK=off go build -mod=readonly ./cmd` from `app` exits 1. The current CLI cannot find `contract.BoundaryIntent`, `contract.OperationContractContext`, `contract.ResolveOperationContractContext`, `diagnostic.CodeModuleIdentity` and other current APIs in its declared `pkg v0.1.0` dependency. This is a real independent compilation defect, not an assumed tag/main difference.
2. `bash tools/module-release-check.sh` exits 1 at `github.com/hvritual/yunka.io/infras@v0.1.0: invalid version: unknown revision infras/v0.1.0`.

The initial tool bootstrap encountered missing executable modes on extracted Go tools; restoring modes from the verified archive allowed compilation. That environment issue is separate from the reproducible source/API failure above.

## Verified distribution identities

`GOWORK=off go mod download -json <module>@3527488325030fbf86fe52804140c936e9d937a4` succeeded for all four modules. `Origin.Hash` matches the full fixed commit; actual resolved versions, zip sums and go.mod sums are preserved in `tools/consumer-release-lock.json`. Infras resolves to `v0.0.0-20261008054254-352748832503`; the other three resolve to `v0.1.1-0.20261008054254-352748832503`.

This disproves the broader inference that a missing infras semantic tag makes every external version unavailable. The old v0.1.0 train is incomplete; the source-commit-derived module train is downloadable and must still pass actual combination/CLI qualification. No new tag is claimed.

## Repair boundary and proof

The repair pins module requirements to the externally downloaded package versions, regenerates dependency metadata with `make tidy`, adds a producer CLI bundle and tests the installed artifact in a disposable external module. Proto includes are contract data, not a vendored SDK Runtime. The four-module fixture covers typed capability resolution, App lifecycle, duplicate/missing exports and the canonical Outbox facade, plus canonical generation and drift rejection.

The producer's CLI source commit and evaluated SDK source commit are separate manifest fields. The candidate's actual build, archive SHA256, module graph, generated digests and command exit codes are emitted to the external output directory. GitHub PR receipts record the exact remote head and normal CI/Production/applicable source/type/template results; this file does not predict its own SHA or turn source presence into a passing run.

Initial focused checks: four Python negative-control tests and `make dependency-check` pass. Real qualification and exact-head CI/Production are pending at the time this source record is authored. External review is not performed / not required by current AGENTS policy; author inspection and tests are not independent approval. No existing test expectations or security gates are weakened.

The repository has no root LICENSE/COPYING/NOTICE file in this evaluated tree. The bundle manifest records the actual licensing-file inventory rather than inventing a grant. This candidate does not publish a release, promise SDK API stability, qualify cs-biz's operations, close #234/#188, merge unrelated PRs or deploy.
