# AG-05 — Reject inherited Go installation overrides

> Document class: **EVIDENCE**
> Current status: [STATUS](../STATUS.md)
> Exact final-head qualification/review/integration: [PR #176](https://github.com/hvritual/yunka.io/pull/176), issue #175

Review `3966816758` on `7e35d08063b9ef4c0fa64017b437596b1e11cf4f`
identified that `runtime.GOROOT()` can honor a caller-provided startup value.
An absolute path and a matching self-reported Go version do not authenticate that
installation. The earlier CGO PATH repair and green tests are not evidence that
this distinct override was safe.

The source-audit runner now rejects a nonempty GOROOT at package initialization
or at tool invocation, before resolving or calling any candidate Go executable.
It retains the initial value so clearing it later cannot promote that installation
to trusted status. It does not mutate global process environment, guess installation
paths or add another execution backend. Valid callers start a fresh checker process
without the override. Ordinary source/profile/consumer semantics are unchanged.

Permanent regressions run the real test executable with a fake startup GOROOT:
both inherited and removed-after-start cases must report exact AG-SRC-000,
zero completed profiles and unchanged source; the fake executable's marker must
not appear. A late override is rejected before even an injected runner is called.
An unset/empty override remains a legal PASS control. All five parent/subcase names
are included in the existing required test inventory. The subprocess is bounded
and owns its process group through the existing cleanup path.

Local Go 1.23.2 exploration is not the locked Go 1.25.13/xmod v0.37.0 qualification.
The PR must bind this amendment's own four successful gates and completed independent
review before non-force integration, then record separate main acceptance. This
source document cannot certify its own unwritten commit. The original attached
42c1422e patch is preserved as earlier exploration; its broader rewrite is not
cherry-picked over the already published and tested 7e35 CGO implementation.
