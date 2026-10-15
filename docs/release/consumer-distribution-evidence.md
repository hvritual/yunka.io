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

The initial remote distribution candidate `558289969d79cf13536940cbc4dd762f814e6847` passed packaged consumer qualification in run `38725755733`. Its AG-05 run `38725755747` correctly rejected `tools/release-consumer/distribution_test.go` as lacking module/component ownership (`AG-SRC-001` / `AG-SRC-002`), preserved in artifact `11968032030`. The repair explicitly classifies only the disposable fixture subtree, like the existing standalone runtime fixtures; no production component/profile or assertion is removed. Actual compilation/execution remains mandatory in the new qualification workflow. A later candidate requires fresh same-head gates; the earlier success is historical only.

Additional packaged-CLI inspection exposed that `-trimpath` erased runtime.GOROOT, so existing source audit correctly returned INCOMPLETE before executing tools. The packaging repair removes that build flag, records the required trusted Go installation root, and requires the packaged source audit to pass on the external consumer. It does not change source/type audit code or the prohibition on inherited/late GOROOT overrides. The receiver must satisfy the explicit toolchain-root condition; arbitrary-path portability is not claimed.

The repository has no root LICENSE/COPYING/NOTICE file in this evaluated tree. The bundle manifest records the actual licensing-file inventory rather than inventing a grant. This candidate does not publish a release, promise SDK API stability, qualify cs-biz's operations, close #234/#188, merge unrelated PRs or deploy.

## Approved security refresh

Human approved Go 1.26.9 / x/net v0.60.0 at 2026-10-15 19:39:44 +08.
Independent PR #245 accepted source bc7c0e1dd6b4e815951130c7acfbf60371445453
passed all six candidate and separate main gates, including real MySQL.
This refresh locks externally downloaded modules from that accepted source;
old 3527488 download identities and Go1.25.13 artifacts remain historical.
x/net requires go1.26.0; the independent consumer now derives its language
directive from the producer. In addition to ordinary workspace CI security,
the qualifier scans all five isolated producer modules and the external
consumer with the unchanged locked govulncheck and current database.
No advisory is suppressed; module-only/non-reachable messages remain in logs.
New exact candidate/artifact receipts must be established independently.
