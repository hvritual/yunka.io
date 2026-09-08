"""Render only after exact consumer-main revalidation; never infer completion."""
from pathlib import Path
import json
import os
import subprocess

root = Path.cwd()
assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip() == '4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7'
receipt = json.loads(Path(os.environ['AG02_CONSUMER_RECEIPT']).read_text())
assert receipt['head'] == 'df6cccf32fc2557f6a22a26b4a966a58b775ff27'
assert receipt['tree'] == 'a320d20874d4054ce8a0de18482aa984274548bf'
assert receipt['mainReadback'] and receipt['postIntegrationVerified']
run = str(receipt['run'])
assert run.isdigit()

p = root / 'docs/STATUS.md'
s = p.read_text().replace('> Reconciled date: 2026-09-07', '> Reconciled date: 2026-09-08')
s = s.replace('| AG-01 / AG-01I | Mechanism regression foundation implemented |', '| AG-01 / AG-01I | Qualified / merged through PR #170 |')
old = '| AG-02 | Next dependent consumer task | Biz TenantLifecycle encapsulation, after the AG-01 main integration receipt is verified |'
assert s.count(old) == 1
s = s.replace(old, '| AG-02 / AG-02R | Complete / consumer-qualified / merged in Biz | Biz PR #16; sealed TenantLifecycle plus independently committed current-read owner-invariant correction (Biz #17). Exact sources, review, qualification and integration: [AG-02 evidence](waves/AG-02-tenant-encapsulation.md) |')
old = '| AG-03 through AG-09 | Defined, not delivered | Narrow-use-case trial, precise type rules, coverage, template integration, migration/debt-growth and continuous evolution |'
assert s.count(old) == 1
s = s.replace(old, '| AG-03 | Next independent consumer task | IoT Delivery narrow-use-case/Repository pilot; not implemented by AG-02 |\n| AG-04 through AG-09 | Defined, not delivered | Precise type rules, coverage, template integration, migration/debt-growth and continuous evolution |')
a = s.index('The earlier AG-01 head `7c50680')
b = s.index('\n\nThese tests characterize', a)
s = s[:a] + 'AG-01I is integrated at `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`; exact-main run `34136019602` revalidated the reviewed tree. Its language-mechanism evidence is separate from AG-02 consumer behavior. AG-02 retains Biz runtime `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` and separately checks current-main CLI ownership/planning at `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`. The structural batch is a manually reviewed migration, not a complete single-Operation AX7 attestation. AG-02R fixes an existing consumer persistence invariant under REPEATABLE READ; it neither changes Yunka UoW/isolation policy nor promotes a consumer defect into a framework runtime gap.' + s[b:]
p.write_text(s)

p = root / 'docs/architecture/APPLICATION-GOVERNANCE-PLAN.md'
s = p.read_text()
needle = '候选布局（仅为待资格验证目标，不授权移动生成文件）：'
assert s.count(needle) == 1
s = s.replace(needle, '布局决策：AG-02 验证了规范 Application 范围内的所有者/隐藏实现结构。以下为 TenantLifecycle 的已验证落点；其他 Application 和持久化拆分仍须独立验证，不授权手工移动生成文件：')
a = s.index('```text\ninternal/access/application/zz_yunka_*_gen.go')
b = s.index('\n```', a) + 4
s = s[:a] + '''```text
internal/access/application/zz_yunka_*_gen.go
internal/access/application/tenantlifecycle/build.go
internal/access/application/tenantlifecycle/internal/usecase/
internal/access/domain/
internal/assembly/
internal/bizruntime/
```''' + s[b:]
needle = '### AG-03 — IoT Delivery 窄用例试点'
assert s.count(needle) == 1
s = s.replace(needle, 'AG-02 落地决策：采用 `internal/access/application/tenantlifecycle/internal/usecase`，手写实现不再与生成 wrapper 同包，所有者工厂返回生成 Application 接口且实际方法集受限。该路径仍在原 Application 范围内，未扩展 Ownership/ChangePlan 白名单。运行时保持消费者原锁；当前框架 CLI 另行验证可编辑路径和明确的 unresolved implementation scope，不声称完整结构批次已经通过单 Operation AX7。\n\n验证中发现的两个问题分别修复：消费者 B12.7 验收脚本等待 Ready 状态与日志同时可见，原验收条件全部保留；AG-02R（Biz #17）修复现有 last-owner 并发判定使用旧快照的问题，以当前锁定读和双向确定性事务回归收口，不修改框架事务配置。方案的结构改进不能替代业务不变量验证。精确证据见 [AG-02 验收记录](../waves/AG-02-tenant-encapsulation.md)。\n\n' + needle)
p.write_text(s)

body = '''# AG-02 / AG-02R — Real consumer encapsulation and owner-invariant qualification

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: one real Biz Application, a necessary consumer persistence correction, and framework documentation; no framework runtime/compiler upgrade

## Exact sources and boundaries

[Biz PR #16](https://github.com/hvritual/biz/pull/16) integrates `df6cccf32fc2557f6a22a26b4a966a58b775ff27`, tree `a320d20874d4054ce8a0de18482aa984274548bf`, from base `7d5afbe9cb4b849be462d9a7aed65877ed227700`. Three separate normal local-Git commits implement encapsulation (`0924157e3ed72cae3cf364955bd041c895087adf`), readiness-harness correction (`b0f250e7570e6adae7b108a1ac27f92bd5e6ea78`), and owner-invariant correction (`df6cccf32fc2557f6a22a26b4a966a58b775ff27`). Runner-local git am reproduced their exact commit/tree identities before testing.

The consumer runtime remains Yunka `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`. A separate governance CLI checkout is Yunka `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`. These are distinct qualification subjects, not an implicit dependency upgrade.

TenantLifecycle's handwritten concrete service is unexported and resides in `internal/access/application/tenantlifecycle/internal/usecase`, outside the generated wrappers' package-private namespace. Owner Build returns the existing generated Application interface. Tests inspect actual method sets and private fields; a pilot source rule limits production owner imports to the composition root. Existing generated child wrappers and root ExecutionScope/UoW joins are retained. PB/API, Operation identity, schema, dependency and generator inputs do not change.

Both new locations stay inside the canonical Application scope. Current CLI Ownership reports them editable; Change Plan leaves the handwritten filename unresolved within that scope. There is no broader whitelist. This is a manually reviewed multi-file structural batch, not a complete single-Operation AX7 attestation or generic architecture analyzer.

## Executed qualification and preserved failures

Initial [run 34185711218](https://github.com/hvritual/biz/actions/runs/34185711218) passed the initial encapsulation head on locked Go 1.25.13/protoc 3.21.12/MySQL 8.4: consumer-certify, check/generate/check with zero drift, full Go tests, internal race, vet/build, six targeted tests, nineteen then-existing MySQL integration tests, legal/illegal/aliased hidden-import probes and current CLI readback. Artifact `10040462449`, ZIP SHA256 `2a06651991013e58f47eed008f17911b4923ba6e1568bda91477da0d49de3a36`, was downloaded, CRC and all 23 manifest members verified. It proves only that exact initial source and does not erase later failures.

B12.7 run `34185974204` observed Ready in the canonical state file before the independently polled DEV READY log. The corrected consumer gate waits for BOTH within the unchanged bound, retaining final grep, Diagnostics, six graph bindings and clean shutdown. A deterministic regression of the actual workflow shell proves delayed-log success and missing-log/not-ready rejection. Workflow schedules, permissions and acceptance requirements are not relaxed.

Run `34186461994` then failed the existing role-revoke/member-suspend concurrency test: both destructive calls succeeded. [Biz #17](https://github.com/hvritual/biz/issues/17) classifies this as an existing consumer persistence defect. The owner-role lock serialized mutation, but later plain SELECTs reused an earlier REPEATABLE READ snapshot. AG-02R keeps the owner-role lock and switches assignment and actual active-owner-row reads to current locking reads, without altering framework isolation or UoW ownership.

Final-source [run 34187156307](https://github.com/hvritual/biz/actions/runs/34187156307), job `101937695522`, verifies BOTH old-repository snapshot-order overlays fail for the expected invariant, while the fix passes. Six targeted tests, twenty integration functions (22 pass events including two subcases), twenty repetitions of both the original cross-path race and deterministic snapshot regression (80 pass events including subcases), and the actual full B12.7 runtime bodies passed without test failures/skips. The same run passed consumer-certify, generation zero drift, full tests/race/vet/build and source cleanliness.

That run's overall conclusion is FAILURE solely at final Git push: the Runner token lacked workflow-write permission. It is not relabeled SUCCESS; its post-push manifest/receipt stage was not reached. Artifact `10040953482`, ZIP SHA256 `6ac5bcbc7cd5299e3dbeffe912fe24ec2394ef128bbe9927e7290068dea0114f`, was downloaded and its CRC/digest and JSON outcomes checked. The existing normal local-Git object was independently read back with the same SHA/tree, then the authorized Connector non-force advanced the task ref to that SAME object. No product commit was recreated through an API.

Nine fresh existing PR workflows passed the final source, including B12.7 run `34187433520`, concurrency run `34187433457` and C9 run `34187433447`. Exact-head independent automated review and integration disposition are recorded on PR #16; tests do not constitute a human APPROVE.

The [post-integration consumer verification and framework-documentation qualification run RUN_ID](https://github.com/hvritual/yunka.io/actions/runs/RUN_ID) reads the integrated Biz SHA/tree, replays generation, all consumer tests, repeated owner regressions and the actual runtime bodies, and emits a consumer main receipt BEFORE this document is rendered. The enclosing docs-only Yunka commit requires its own full framework verification, independent review and final main readback; this file does not fabricate a self-referential enclosing commit result.

## Limits and recovery

Only TenantLifecycle is sealed. Member/Role retain their prior organization; AG-02R only corrects their shared persistence invariant. The import policy is pilot-specific, Linux/GNU-timeout probes are not an OS sandbox, and general templates/type-analysis/structure-migration proof remain later AG tasks.

Prior staging blob `575cf9925c4a826abf8934df3c8b643000798f35` failed Git pack decompression and was not used. The implementation was rebuilt from a verified clean baseline. A later transport-copy discrepancy was corrected before checking immutable patch digest `9951e36a546a3f67396a8770dfd9aa0989d480dc17fb6049da287f023cd28138` and exact commit/tree. Transport integrity is not behavioral qualification.

Rollback may revert the structural or readiness commits separately; reverting AG-02R restores the known owner-invariant defect and requires an explicit risk decision. No deployment or data migration occurred. AG-03 is the next independent IoT Delivery narrow-use-case task; AG-04 through AG-09 are not delivered here.
'''.replace('RUN_ID', run)
(root / 'docs/waves/AG-02-tenant-encapsulation.md').write_text(body)
