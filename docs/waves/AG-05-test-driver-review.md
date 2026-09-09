# AG-05 — Go test-driver GOROOT qualification correction

> Document class: **EVIDENCE**
> Status authority: [STATUS](../STATUS.md)
> Review: PR #176, comment `3967427989`
> Final candidate/check/review/integration authority: PR #176 and issue #175

The reviewer reported a test-launch incompatibility on exact candidate
`334a3f05d5af24398de02ff2d53344453e90a48e`: the test binary starts with GOROOT
set and the production package captures that value during init. Clearing it inside
a test cannot authorize a clean startup. This is separate from rejecting an
untrusted caller installation.

The local Go 1.23.2 probe with GOROOT unset passed without a wrapper; it did not
reproduce an unconditional injection. Go 1.25.13 source (`cmd/go/internal/test`
and `cmd/go/internal/cfg`) uses OrigEnv for user test binaries. We therefore do not
assert that every Go test always injects GOROOT or that the earlier successful
remote gates were false. The permanent paired regression explicitly supplies the
real installation GOROOT as a controlled launch condition, then proves the new
wrapper removes it before initialization.

The old workflow records reported successful qualification. They are preserved as
historical reported results, but that observation is not used to dismiss the
reviewer's fresh failure or to qualify a later candidate. The corrected head must
establish its own real test execution, inventory, ordinary CI/Production and both
consumer gates. No retrospective result is relabeled.

## Minimal correction

The app portion of `make test`, the AG-04 audit regression command and AG-05's
required test inventory use `go test -exec="/usr/bin/env -u GOROOT"`. Go still
builds/vets the same packages, supplies the same test arguments and produces the
same JSON event stream. The wrapper only removes GOROOT before exec of the test
binary, retaining its exit status. No test is skipped, removed or weakened; no
production environment guard, dependency, timeout or check policy is relaxed.
The workflows remain read-only and do not publish source or change permissions.

`TestGoTestDriverGOROOTBoundary` builds a dependency-free Go fixture. With that explicit input the unwrapped
Go driver must reproduce the specific startup-environment failure; the wrapped
invocation must pass. It is mandatory in `tools/source-policy/tests.json`.
Existing startup/late/removed-after-start fake-GOROOT tests remain mandatory;
their deliberate child process launches do not go through the execution wrapper.

## Evidence interpretation and rollback

Local Go 1.23.2 reproduction is exploratory only. The final repository head must
run the locked Go 1.25.13 gates and obtain independent review. Exact SHA/tree and
run results belong to the post-commit PR receipt, not a self-certification in this
file. Existing old failing/passing records remain scoped to their own subjects.

The launcher is a Unix qualification-host mechanism (`/usr/bin/env`); this does
not claim native Windows support. Reverting this verification correction requires
addressing the startup incompatibility, not removing the production GOROOT guard.
