# AG-01I — Boundary regression integrity and integration evidence

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: mechanism regression foundation and review closure; not a consumer refactor or an arbitrary-project analyzer

The original AG-01 head `7c50680f0381505d9cc55a029a2a92baa6b0a140` passed existing CI/production. Independent automated review on PR #170 nevertheless identified three P1s: descendant processes could outlive `exec.CommandContext`/`WaitDelay`, STATUS did not record the AG stream, and the lasting boundaries were not linked from PROJECT_MEMORY. All three are addressed in the delivery flow; review is not a self-issued APPROVE.

AG-01I also records a named mechanism inventory. Run `34128240351` exercises a disposable copy in which the old suite accepts 10 of 11 scenarios, while the inventory check rejects the same removal with its exact diagnostic. The registry and tests are policy inputs, not a defense against an actor who can rewrite the whole verifier.

Process cleanup uses a fresh Unix process group and kills it on cancellation and after Wait, including early parent exit with inherited pipes. Linux regressions create a real descendant and require it to be gone or terminated (a zombie awaiting the host reaper is not a running process). Fixture code does not deliberately escape its process group. Non-Unix is explicitly INCOMPLETE; no Windows job-object guarantee is made.

Exact final candidate identity, per-case JSON, original process-leak reproduction, complete `make verify-production` on locked Go/protoc and MySQL 8.4, dependency/contract regeneration, source-tree cleanliness and remote product-ref readback are recorded by [delivery run 34130711408](https://github.com/hvritual/yunka.io/actions/runs/34130711408). The worker creates the candidate with normal local Git before testing and only pushes after all required checks. The artifact contains candidate SHA/tree and a verified Git bundle; no enclosing/self-referential SHA is invented in this file.

Failed control runs remain historical evidence: `34129121261` stopped before commit on two Markdown trailing spaces; `34129651617` passed targeted checks but caught a repeated-test /proc exit race. Only ENOENT/ESRCH (including wrapped errors) now mean process termination; permission and other I/O errors still fail. No failed candidate was pushed or merged.

The targeted run requires six named parent tests, all 11 mechanism scenarios, all 13 inventory mutations and both Linux cleanup cases, with zero skips. The inherited package gates retain ordinary, repeated and race testing. Mere command failure is never accepted as an expected architecture diagnostic.

Publication of the candidate does not itself mean main integration. PR #170 and the separate integration receipt record exact reviewer disposition, non-force fast-forward, matching tree and main readback. No runtime, authz, UoW, dependency lock, normal CI or consumer API/data semantics change. Control workflows/payloads stay outside the product tree.
