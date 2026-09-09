# AG-05 — CGO tool lookup review correction

> Document class: **EVIDENCE**
> Current status: [STATUS](../STATUS.md)
> Contract: [Application source policy](../architecture/APPLICATION-SOURCE-POLICY.md)
> Final exact-head qualification, review and integration: [PR #176](https://github.com/hvritual/yunka.io/pull/176) and issue #175

## Reviewed source and correction

Review comment `3966291018` identifies inherited PATH use in sourceaudit at
`45e40278d880e85e9d7ed6928dd7c26f65462f8c` (tree
`556a97f7660b5e19280870af61f933fba2a3eb20`). An active CGO profile could select a
caller-installed compiler wrapper despite the read-only audit contract. The old
candidate's green CI and metadata checks did not prove compiler lookup isolation.

The runner now constructs PATH from GOROOT/bin and fixed Unix /usr/bin and /bin
only. Caller CC/CXX/PKG_CONFIG overrides remain discarded. Before CGO metadata
execution, it requires an executable host cc in a trusted system tool directory
and sets CC to its absolute path. Missing/untrusted tools fail the profile with
AG-SRC-000 / INCOMPLETE; it never disables CGO or searches caller paths as fallback.
This explicit precondition matters because metadata-only Go commands need not
invoke the C compiler on every version/path. A successful syntax inventory alone
is not evidence that the required native compiler exists.

System installation directories, Go installation and dependency caches are trusted
prerequisites. This is not binary authentication, a compiler sandbox, or proof that
an installed host compiler can build all cross-target applications. No product
runtime/Executor/UoW, generated artifact, dependency version, existing CI/Production
or repository permission changes are part of this correction.

## Permanent regressions

- `TestSourceToolPathDoesNotInheritCallerPaths` verifies both CGO settings omit
  caller search paths and compiler overrides.
- `TestCGOProfileIgnoresParentCompilerWrappers` uses a real C-importing module,
  fresh Go cache and test-owned gcc/cc/clang/C++/pkg-config wrappers. The marker
  must stay absent while source inventory succeeds and stays unchanged.
- `TestMissingCGOToolDoesNotFallBackToParentPATH` checks unavailable compiler
  metadata returns INCOMPLETE and source restoration, not a false PASS.
- `TestCGOToolEnvironmentRequiresTrustedExecutable` rejects relative, untrusted
  executable and missing compiler paths; non-CGO remains independent of a C tool.

All four are included in the mandatory test-JSON inventory in
`tools/source-policy/tests.json`, not only an ad-hoc script. No tests or checks are
removed or downgraded. The same actual locked-head source must pass canonical CI,
real-MySQL Production, AG-04 consumers and AG-05 source qualification, followed by
independent re-review and separate main acceptance. Local Go 1.23.2 exploration
using that installation's vendored x/mod is explicitly NOT locked Go 1.25.13 /
x/mod v0.37.0 qualification. Source/manifest hashes and receipts distinguish them.

## Completion boundary

This document records the reviewed issue and implemented regression contract;
it does not claim qualification of the unwritten commit containing itself. PR #176
must bind the eventual exact SHA/tree, successful gates, resolved review and main
readback. Historical IoT full-repository INCOMPLETE remains visible and separate
from its current-module projection. AG-06 and later tasks are not implemented here.
