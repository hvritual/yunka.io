from pathlib import Path
import os,re,subprocess
root=Path.cwd()
assert subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()=='4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7'
assert not subprocess.check_output(['git','status','--porcelain'],text=True).strip()
sha=os.environ['BIZ_SHA'];tree=os.environ['BIZ_TREE'];pr=os.environ['BIZ_PR'];run=os.environ['QUAL_RUN']
assert re.fullmatch('[0-9a-f]{40}',sha) and re.fullmatch('[0-9a-f]{40}',tree)
assert pr.isdigit() and run.isdigit()
p=root/'docs/STATUS.md';text=p.read_text()
old='| AG-02 | Next dependent consumer task | Biz TenantLifecycle encapsulation, after the AG-01 main integration receipt is verified |'
assert old in text
text=text.replace(old,f'| AG-02 | Real-consumer pilot qualified | Biz `{sha}` (tree `{tree}`), qualified in run `{run}`; [evidence](waves/AG-02-biz-encapsulation.md), Biz PR #{pr}. Hidden TenantLifecycle implementation and current owner-invariant reads; exact PR/Git receipts distinguish qualification from integration. Runtime source remains pinned to `6ba99c1440dc`; no framework runtime/compiler change |')
text=text.replace('| AG-01 / AG-01I | Mechanism regression foundation implemented |','| AG-01 / AG-01I | Complete / merged through PR #170 |',1)
text=text.replace('> Reconciled date: 2026-09-07','> Reconciled date: 2026-09-08',1)
needle='These tests characterize the language boundary, including expected examples of insufficient encapsulation.'
insert=f'''AG-01I integration was completed as `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`; exact-main run `34136019602` passed. AG-02 continues from that verified foundation rather than reopening it. The real Biz pilot keeps its existing pinned runtime and canonical generated artifacts. Its owner stays under `<domain>/application`, so the current control-plane placement probe requires no expanded whitelist. Full qualification exposed Biz #18, a consumer stale-snapshot owner-invariant defect; both deterministic baseline transaction schedules reproduced it before the narrow locking-read repair. This is not a new Yunka runtime defect. AG-03 is the next dependent pilot after actual Biz integration is verified.\n\n'''
assert needle in text;text=text.replace(needle,insert+needle,1);p.write_text(text)
p=root/'docs/architecture/APPLICATION-GOVERNANCE-PLAN.md';text=p.read_text()
needle='### AG-03 — IoT Delivery 窄用例试点'
insert='''AG-02 的兼容落点采用 `internal/access/application/tenantlifecycle/internal/usecase`，将手写实现移出生成 wrapper 的同包私有范围，同时留在现有 Change Plan/Audit 的 Application 根内。前文 sibling 目录是候选示例，不是强制另建布局或第二份事实源。真实验收中发现的消费者持久化问题按独立问题记录和确定性复现处理，不能降低 last-owner 等行为门槛来完成目录迁移。\n\n'''
assert needle in text;text=text.replace(needle,insert+needle,1);p.write_text(text)
p=root/'docs/waves/AG-02-biz-encapsulation.md'
p.write_text(f'''# AG-02 — Biz TenantLifecycle encapsulation qualification

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: one consumer Application; not generic architecture certification

## Exact subjects and qualification

- Consumer base: `hvritual/biz@7d5afbe9cb4b849be462d9a7aed65877ed227700`.
- Qualified consumer head: `{sha}`; tree `{tree}`.
- Runtime/generator source: `yunka.io@6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`, unchanged in consumer locks.
- Current governance CLI probe: `yunka.io@4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`.
- Qualification: [Biz run {run}](https://github.com/hvritual/biz/actions/runs/{run}).
- Review/integration disposition: [Biz PR #{pr}](https://github.com/hvritual/biz/pull/{pr}).

The CLI probe is not a consumer upgrade or latest-framework runtime qualification.
The named Git/action receipts distinguish candidate qualification, actual review
and main integration; none is inferred from another.

## Implementation

The handwritten service is hidden in
`internal/access/application/tenantlifecycle/internal/usecase/tenant_lifecycle.go`.
`tenantlifecycle.Build` returns the generated Application interface and is called
by the existing runtime assembly factory. Seven business methods retain their
validation, DTO/ID, CAS, requestscope and child-call behavior; the original error
sentinel remains available at the old contract seam. Existing lifecycle tests are
retained via the factory, and shared Member test helpers remain test-only.

The owner stays under the canonical Application root. Current Ownership accepts
the new production paths and Change Plan preserves the matching scope. No path
matcher, generated file, compiler, runtime or second binding registry is changed.
This is placement compatibility, not automatic semantic approval of any refactor.

Real generated child wrappers are checked for their actual narrow method sets;
real runtime Assembly must return the hidden implementation. Four import probes
in a disposable tracked-source copy accept canonical ports/owner factory and
reject direct/aliased hidden imports for the exact expected compiler diagnostic.
Infrastructure failures do not count as architecture detections.

## Consumer correctness blocker — Biz #18

Full qualification discovered the existing cross-path owner race: role revocation
and Member suspension could both succeed. The unchanged consumer baseline then
failed both deterministic two-transaction snapshot schedules for exactly the
missing `ErrLastTenantOwner` rejection. The previous AG-02 layout change did not
modify those persistence methods.

The repository repair uses a locking read of joined active-owner rows under the
same owner-role lock and caller-owned transaction. Plain counts can retain an
old REPEATABLE READ snapshot despite later lock acquisition; the repair does not
change process-wide isolation, invent an authorization decision or own a new UoW.
See [MySQL consistent reads](https://dev.mysql.com/doc/refman/8.4/en/innodb-consistent-read.html).

## Executed gates and limits

Qualification used locked Go 1.25.13/protoc 3.21.12, canonical repeated generation,
workspace/GOWORK=off consumer certification, consumer verify, targeted unit/race
checks, and full MySQL 8.4 integration. Required existing lifecycle, tenant
isolation, REST/gRPC, bootstrap/rollback, concurrency and last-owner assertions
remain. JSON evidence rejects failures/skips and missing required test events;
its exact counts and four import outcomes are stored in the qualification artifact.

PB/module sources, generated contracts and assembly, dependency locks and runtime
configuration have no delta. Product Git commits precede tests and publication;
remote task-ref readback and artifact member hashes bind the actual candidate.
Control workflows do not enter either product tree. Framework changes in this
reconciliation are documentation only and require their own ordinary verification.

The previous unreferenced AG-02 bundle blob failed pack inflation and was not used
for product publication. Run `34189305040` caught an incomplete role test double;
run `34189439919` caught the real owner race. Both failed candidates were withheld.
Their results are preserved rather than relabeled PASS.

Member/Role implementation migration, universal factory-call checks, templates,
full-source coverage, structural migration and debt-growth protocols remain later
AG tasks. The import harness has POSIX execution scope. This is not a security
sandbox, generic architecture scanner or proof of every business boundary.

## Rollback and continuation

Revert the complete consumer change including assembly bindings, relocated tests
and the separately identified owner repair only with its failing evidence visible.
No schema/data migration or production deployment is included. AG-03 follows after
actual main integration of the qualified consumer is confirmed.
''')
paths=['docs/STATUS.md','docs/architecture/APPLICATION-GOVERNANCE-PLAN.md','docs/waves/AG-02-biz-encapsulation.md']
subprocess.run(['git','add',*paths],check=True)
assert set(subprocess.check_output(['git','diff','--cached','--name-only'],text=True).splitlines())==set(paths)
subprocess.run(['git','diff','--cached','--check'],check=True)
