# AG-02 / AG-02R — Biz encapsulation and actual-main acceptance

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: TenantLifecycle consumer pilot and its proven qualification corrections

## Exact integrated subjects

- Consumer base: `hvritual/biz@7d5afbe9cb4b849be462d9a7aed65877ed227700`.
- Consumer main / reviewed head: `3519e7ee6e51e33984669871e4f32a55a3597d9f`.
- Consumer tree: `3f8926b459fb78688fe0f39e9c4acfb6d93b3b47`.
- Consumer PR: [Biz #16](https://github.com/hvritual/biz/pull/16), merged by non-force fast-forward to the qualified existing commit.
- Runtime/generator pin: Yunka `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`, unchanged.
- Separate current control-plane placement probe: Yunka `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`.
- Final candidate qualification: [run 34188061220](https://github.com/hvritual/biz/actions/runs/34188061220).
- Actual-main acceptance after integration: [run 34192294589](https://github.com/hvritual/biz/actions/runs/34192294589).

The current CLI probe is not qualification of an upgraded consumer runtime.
All four product commits were created using local/Runner-local Git. Runner-token
publication lacked workflow-write permission; the authorized Connector moved the
ref to the exact existing qualified object without recreating a commit or forcing
history. Qualification, independent review and integration remain separate facts.

## Delivered implementation

The handwritten service is unexported and lives in
`internal/access/application/tenantlifecycle/internal/usecase/service.go`.
The owning `tenantlifecycle.Build` returns the canonical generated Application
interface and is wired by existing `bizruntime` factories. The original request
error identity, business methods, DTO/ID handling, CAS, requestscope joins and
generated source-edge child Operations are preserved. Shared Member test helpers
remain test-only; the original TenantLifecycle assertions remain executable.

The owner remains under the existing canonical Application root. Current Ownership
accepts the production paths and Change Plan retains that root without expanding
path authorization or relocating generated files. This manually reviewed
structural batch is not represented as complete single-Operation AX7 conformance.

Method-set tests examine actual built Application and actual generated wrappers,
not just assignment to a small interface. Three overlay compiler probes permit
the public owner and reject ordinary/aliased hidden imports with the exact expected
diagnostic. No tracked consumer source is changed by those probes. The import
harness is Linux-oriented and is not an OS or same-process security sandbox.

## Necessary qualification corrections

### Runtime readiness evidence

Run `34185974204` observed canonical Ready state before the independent DEV READY
observer log. The old immediate grep failed. B12.7 now waits for BOTH facts within
the original bounded polling loop. Final log, Diagnostics, six Application graph
bindings and clean shutdown assertions remain. A deterministic regression executes
the actual workflow shell for delayed log, missing log and log-without-ready cases.
No application or Yunka runtime behavior was altered to make this check pass.

### Consumer last-owner snapshot defect — Biz #17

Run `34186461994` reproduced concurrent role revoke and Member suspend both
succeeding. The common owner-role lock did not refresh subsequent ordinary reads
of assignments and active-owner counts in an already-established REPEATABLE READ
snapshot. The consumer repair keeps that lock and uses current locking reads of
assignments and the joined active-owner rows in the SAME caller-owned transaction.
No global isolation, schema, permissions or root-UoW owner was changed.

The deterministic MySQL regression establishes the loser's snapshot before the
winner commits, in both revoke-first and suspend-first schedules. It requires
ErrLastTenantOwner and exactly one remaining effective owner. The existing
simultaneous runtime regression remains unchanged and is repeated as well.

### Recurring gate coverage

Independent review of `df6cccf32fc2557f6a22a26b4a966a58b775ff27` found two omissions.
The final commit retains B12.6's original TestB126 selection and additionally runs
`TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot`; both event path lists include
that test file. B12.7's push/PR paths both include the readiness-regression script.
These are explicit consumer CI changes; triggers and acceptance conditions were
not weakened and no framework CI or runtime change was made.

## Exact verification and review

The final source used Go 1.25.13, protoc 3.21.12 and real disposable MySQL 8.4:

- consumer certification in the local workspace and GOWORK=off;
- canonical check/generate/check with zero dependency/generated drift;
- full Go tests, internal race, vet and build;
- six targeted test passes and three real import probes;
- twenty integration test functions plus two snapshot subcases (22 pass events);
- twenty repetitions of the original concurrent test and deterministic snapshot
  test, including both schedules (80 pass events, not 80 independent features);
- actual B12.7 Ready/Diagnostics/six-Application graph/clean-shutdown body.

Required test events are checked and no test-level failure/skip is accepted.
Package-level records for packages without test files are not described as skipped
business tests or counted as extra scenarios. Actual-main acceptance repeats the
same gates against the integrated head and reads back main SHA/tree afterward.

All nine ordinary final-head PR workflows passed: C9 pressure, B12 multi-tenant,
Member, Role, bootstrap, concurrency, runtime, pressure-disposition and dependency
workspace isolation. Exact final automated review comment `5579459226` reports no
major issues for `3519e7ee6e`; both actionable review threads were answered and
resolved. This is automated independent review, not a fabricated human APPROVE.

Final candidate artifact `10041258566` has ZIP SHA256
`89f5a8ab91d6dbe87fe2c711879026c5e58689e8d03c0ccbd46180a859a4be3c`.
Its CRC, all 20 manifest members, Git bundle objects, candidate SHA/tree and test
JSON were read back. Its original receipt correctly records blocked Runner
publication, not main integration. The later actual-main run's unsigned receipt
and Git/PR readback establish integration separately.

## Preserved failures and bounded claims

The corrupt original unreferenced staging bundle was not imported. Initial
qualification applied only to its exact source. Run `34187156307` passed its gates
but failed overall at workflow-write publication; it is not relabeled SUCCESS.
Readiness and owner-invariant failures remain evidence, not suppressed flakes.

A separate temporary reconstruction used during recovery had a different tree and
is not the reviewed source above. Its runs and extra test counts cannot be used to
certify `3519e7ee`. The already-reviewed four-commit PR is the canonical delivery.
Biz #18 records a duplicate reproduction of #17, not another framework defect.

PB/API/Operation identities, generated contracts/modules/assembly, dependency pins,
configuration and database schema remain unchanged. Member/Role encapsulation,
generic type rules, all-source coverage, templates and migration/debt-growth
protocols remain later AG tasks. No production deployment or data migration was
performed. Framework changes for this handoff are documentation only and undergo
separate standard verification before main integration.

## Rollback and continuation

Revert the complete reviewed change set with owner wiring, moved tests and harness
updates together; keep the known owner-defect evidence visible. No database
rollback is implied because there was no schema/data migration. AG-03 is the next
independent consumer pilot once this framework handoff is integrated.

## Actual-main acceptance correction

The first post-integration control run `34191937549` completed the consumer,
MySQL, repeated-owner and full runtime gates, but its targeted package selector
omitted `internal/architecture`; its required six-test inventory correctly failed.
It is not marked successful. Run `34192294589` adds that existing package without
lowering the inventory and repeats all gates; the six required test names are
explicitly matched. Product source and both qualified identities are unchanged.
