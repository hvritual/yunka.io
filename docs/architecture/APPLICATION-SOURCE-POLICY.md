# AG-05 — Physical source inventory and build-profile import policy

> Class: CONTRACT / developer documentation
> Current delivery authority: [STATUS](../STATUS.md)
> Task and exact acceptance: [#175](https://github.com/hvritual/yunka.io/issues/175), [PR #176](https://github.com/hvritual/yunka.io/pull/176)
> Governing decision: [APPLICATION-GOVERNANCE-PLAN](APPLICATION-GOVERNANCE-PLAN.md)

## Problem and bounded result

A default `./...` expansion is not a physical source inventory. Nested modules,
ignored directory names, build constraints, test-only files and generated-marker
exemptions can hide source from an apparently green architecture check. AG-04
already closes the imported hidden-package case for its one-module type audit;
AG-05 separately accounts for every owned source file and declared build profile.

`yunka audit source` is opt-in, read-only, CLI-internal tooling. It does not infer
business domains, create a second Application/Operation graph, modify the runtime,
or claim that a small import set proves least-authority dynamic objects. AG-04
remains the type/capability checker. Template integration, architecture-debt delta
and batch consumer refactoring remain AG-06, AG-07 and AG-08 respectively.

## Backend decision and provenance

| Compared mechanism | Useful capability | Decision |
| --- | --- | --- |
| `fe3dback/go-arch-lint` | Explicit component dependencies and file/package rules | Not installed or copied; its configuration/analysis would not replace the independent physical inventory and profile receipts |
| `OpenPeeDeeP/depguard` | Import allow/deny policy and test-file selection | Not installed or copied; not a whole-module/workspace/build-coverage backend |
| Native Go command + Go parser + pinned `x/mod/modfile` | Exact active source/import metadata, standard build constraints and module/workspace parsing | The one primary AG-05 backend |

Mechanism references: https://github.com/fe3dback/go-arch-lint,
https://github.com/OpenPeeDeeP/depguard,
https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules,
https://pkg.go.dev/go/parser,
https://pkg.go.dev/golang.org/x/mod/modfile.
This is a design comparison, not a claim that the unadopted linters were run or
benchmarked. No external linter runtime or dependency version is added. The
existing `tools/toolchain.env` lock (Go 1.25.13) and app's already-direct
`golang.org/x/mod v0.37.0` remain the qualification identities. Native Go and x/mod
are upstream Go-project tools; no code from the two compared linters is embedded.

There is one source-policy input for component boundaries, module/profile scope,
explicit exclusions and test-support classification. The existing
`tools/dependency-policy.json` continues to own module-version and legacy-import
migration constraints. AG-05 neither duplicates those version lists nor relaxes
them. They are different rule responsibilities, not competing copies of one fact.

## Public command

```sh
# Prepare the approved dependency cache through the project's ordinary bootstrap.
yunka audit source --root . --policy tools/source-policy/yunka.json --format agent-json
```

`--root` contains all owned modules, the policy and every local workspace/replace
target. A relative policy is resolved against that root; an absolute policy must
still be root-contained. `--format text|json|agent-json` describes the same report;
JSON and agent-JSON are identical. `--timeout` defaults to two minutes and is
bounded at ten minutes. Invalid CLI/policy input is an error. `FAIL` and
`INCOMPLETE` both exit nonzero; only `PASS` exits zero.

The checker must start with `GOROOT` unset (or empty) and it must remain unset
while analysis runs. A nonempty inherited or later override is `AG-SRC-000 /
INCOMPLETE` before any Go tool executes; clearing a startup override inside the
running process does not establish a trusted installation. Start a fresh process
with `env -u GOROOT yunka audit source ...` on Unix. The compiled installation and
fixed host-system tool directories remain trusted prerequisites, not authenticated
binaries or a security sandbox. Parent PATH and CC/CXX/PKG_CONFIG are not inherited.

Missing cache/toolchain, unknown modules, omitted source, unsupported targets,
invalid source or input drift cannot produce an empty clean PASS. INCOMPLETE
outranks FAIL while preserving every proven violation already established.
No business code, test body, generator, build hook or user-provided package driver
is executed by the audit. Go may load native package metadata; this is not a
same-process, compiler, operating-system or network security sandbox.

## Strict policy

```json
{
  "schemaVersion": 1,
  "profiles": [
    {"name": "linux", "goos": "linux", "goarch": "amd64", "cgo": false, "tags": []},
    {"name": "integration", "goos": "linux", "goarch": "amd64", "cgo": false, "tags": ["integration"]}
  ],
  "modules": [{"path": ".", "workspace": "off", "profiles": ["linux", "integration"]}],
  "components": [
    {"name": "usecases", "path": ".", "kind": "production", "allow": ["domain"], "allowExternal": true},
    {"name": "domain", "path": "internal/domain", "kind": "production", "allow": [], "allowExternal": false}
  ]
}
```

Policy parsing rejects duplicate/unknown fields, null, trailing JSON, invalid path
identities, missing/duplicate modules/profiles/components and unknown dependency
edges. Component paths name actual directories; the most specific component owns
files/packages beneath it. Self-imports and standard-library imports are allowed;
`allow` names permitted *other* components. Explicit `denyImports` package-prefix
rules also apply to canonical `_test.go` source. `allowExternal` controls non-owned
non-standard package imports. All matching uses Go import strings/metadata, not
local alias names, service naming conventions, RPC counts or line-count scores.

A module selects an exact root-relative `go.work` or explicitly selects `off`.
Every discovered nonexcluded module must be declared. A Go-free aggregator may
have `kind: "manifest-only"` and `profiles: []`; adding Go source beneath that
module, outside declared nested modules, is INCOMPLETE rather than an exemption.
All source modules need at least one explicit GOOS/GOARCH/CGO/tags profile.

Optional `exclusions` need an exact existing path, a meaningful reason and kind:
`fixture` is deliberate non-product test input, while `dependency` identifies a
pinned external module subtree. Exclusions do not remove files from inventory.
A loaded fixture is a proven violation; a manifest selecting a fixture-only module
has an unqualified dependency context and is INCOMPLETE. Dependency bodies are not
owned-component rule subjects, but remain copied/inventoried and their active
import graph participates in production-to-test-support reachability.
No source is excluded merely because it says `Code generated ... DO NOT EDIT.`
Generated status is an observation on a still-checked file.

## Three separate evidence dimensions

**Physical inventory.** Enumerate every regular input file except Git metadata,
independent of `.gitignore`, Go wildcards, build tags or module depth. Report every
Go file and go.mod/go.sum/go.work/go.work.sum, its original hash, module/component,
exclusion reason where applicable, and active profile names. The inventory digest
binds all copied paths, executable bits and bytes, including embed/native inputs.
Symlinks/nonregular inputs, more than 100,000 files, more than 512 MiB total or a
single file over 128 MiB produce INCOMPLETE. They are not silently skipped.

**Rule applicability.** Production files participate in component dependency and
test-support rules. Canonical `_test.go` files are inventoried, parsed and loaded
using Go `-test`, but their cross-component imports are deliberately not production
layer violations. Ordinary non-test source is never classified as test support by
its directory spelling: `kind: "test-support"` and `testSupportImports` are reviewed
policy. Production may not reach those packages, or `testing`, even transitively
through an intermediary or pinned dependency. Selected deny-import rules still
apply to tests. Real-business names have no special meaning to the engine.

**Completed analysis.** Load explicit physical package directories in bounded
batches using native `go list -e -deps -test -json -mod=readonly`, not just `./...`.
Reconcile active/ignored files and explicit directories against the inventory.
All nonexcluded Go files need at least one active profile; an unsupported profile,
missing required package/file/import target or loader failure makes analysis
incomplete. A file being inventoried, parsed or listed as active is not equivalent
to all required profile/rule evidence being complete. The report separately counts
required/completed profiles and checked/excluded/uncovered files.

| Rule | Meaning |
| --- | --- |
| AG-SRC-000 | Input, parser, toolchain, metadata or budget failure; INCOMPLETE |
| AG-SRC-001 | Unknown/missing/invalid/duplicate module identity or unbound exclusion; INCOMPLETE |
| AG-SRC-002 | Missing or invalid component ownership; INCOMPLETE |
| AG-SRC-003 | Uncovered profile/file/package/import target; INCOMPLETE |
| AG-SRC-004 | Proven forbidden component/external/explicit package-prefix import |
| AG-SRC-005 | Proven production dependency on explicit test-support code or testing |
| AG-SRC-006 | Proven activation/import of an excluded fixture |
| AG-SRC-007 | Original input or private source projection drift; INCOMPLETE |
| AG-SRC-008 | Unqualified local module/workspace resolution or escaped owned input; INCOMPLETE |

## Read-only execution and limits

Go runs in a private copy, never with the original repository as its working tree.
Every local manifest target must remain inside the declared root; absolute local
paths are rebased only in the private projection with the standard manifest editor.
Original hashes stay bound to original bytes. Recheck all original inputs, including
new/deleted files and policy edits, and verify Go did not change its projection.
Missing sums/dependencies are not repaired by the auditor. Prepare them separately
using the project's approved bootstrap; never turn a repair into passing evidence
for the unmodified inputs.

Invoke the absolute configured GOROOT/bin/go and verify its actual version against
the checker build. Ignore inherited GOFLAGS, GOENV, workspace/driver/toolchain hooks
and compiler feature overrides; disable Go module/toolchain downloads and telemetry.
Explicit cache/home/temp inputs are trusted environmental prerequisites, not signed
provenance. Runtime version equality is not binary authentication or a filesystem
lock. Unix child processes have owned process groups and bounded cancellation;
Linux is the qualified native host. A non-Unix native runner fails incomplete.
Windows target source is analyzed on the Linux host; this is not Windows execution.

The matrix checks Go source/import/build selection, not full compilation, runtime
behavior, every possible build-tag combination, arbitrary external dependency
source, dynamic reflection, function effects or type capability provenance. Ordinary
CI/Production and the AG-04 type audit retain their separate responsibilities.
A hash is reproducible evidence, not a signature, reviewer approval or permission
to edit files, change a policy, close a debt item or merge a PR.

## Permanent qualification and real consumers

The read-only `ag05-source-policy` workflow triggers on **all Go-source changes**,
module/workspace manifests, its own implementation/policy/workflow and status/contract
reconciliation. It also supports manual dispatch. It has only `contents: read`,
immutable action pins, no retained checkout credentials and no commit/push/ref writes.
The exact-source bundle job is readback evidence only; it cannot substitute for the
three source-qualification matrix jobs, standard CI, Production or independent review.

Permanent tests retain exact positive/negative/INCOMPLETE expectations for hidden
packages, nested modules, relative/absolute workspaces and replacements, generated
camouflage, explicit source/profile loss, test-support reachability, loader faults,
source/projection drift, parent-PATH wrappers and process cleanup. The persisted
`tools/source-policy/tests.json` requires the named regressions in actual Go test-JSON
receipts without skips; it is a test-loss guard, not a security boundary against an
actor able to rewrite both source and verifier.

The framework policy declares the actual five modules and Linux, Windows and
explicit tagged source profiles. Existing dependency-version/legacy-import gates
remain unchanged. Its testdata exclusions name the existing deliberate compiler,
assembly and runtime-helper fixtures; loading them as product input fails.

The consumer policies remain bound to Biz `3519e7ee6e51e33984669871e4f32a55a3597d9f`
and IoT `bcd20632b666405c3f7a8fe8d53f591c78450087` with unchanged original runtime pins.
They deliberately provide coarse consumer/contract boundaries, not a claim that
all internal application organization has already been refactored.

The full frozen IoT repository contains a legacy backend replacement pointing at
`third_party/yunka/compat/go-kit-kit-log`, absent from its actual pinned runtime.
Qualification must record that exact full-repository INCOMPLETE result, not exempt
the legacy module or call the consumer conformant. A separate, explicitly narrower
projection checks every `backend-yunka` source file/profile plus its unchanged
runtime dependency tree; it is never relabeled as full-repository PASS. Further
legacy correction belongs to the consumer migration stream. No production runtime
or original consumer file is edited by qualification.

Qualification artifacts retain original/repeated/restored reports, injected
hidden-package/generated-test-support/uncovered-tag/unknown-module controls, source
hashes, checker SHA/tree/binary, actual test outcomes and restoration receipts.
Final candidate and actual-main runs must have separate identities and receipts.
Only the delivery PR/issue and STATUS may assert a tested/merged/completed state.
Removing this opt-in tool can be reverted independently without relaxing prior
runtime, ownership, type-audit or dependency-policy gates.
