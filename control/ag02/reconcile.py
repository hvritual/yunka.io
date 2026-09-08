from pathlib import Path
import os
import subprocess

root = Path.cwd()
assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip() == '4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7'
run = os.environ['AG02_INTEGRATION_RUN']
assert run.isdigit()

p = root / 'docs/STATUS.md'
s = p.read_text()
s = s.replace('> Reconciled date: 2026-09-07', '> Reconciled date: 2026-09-08')
s = s.replace('| AG-01 / AG-01I | Mechanism regression foundation implemented |', '| AG-01 / AG-01I | Qualified / merged through PR #170 |')
old = '| AG-02 | Next dependent consumer task | Biz TenantLifecycle encapsulation, after the AG-01 main integration receipt is verified |'
assert s.count(old) == 1
s = s.replace(old, '| AG-02 | Complete / consumer-qualified / merged in Biz | Biz PR #16; TenantLifecycle hidden implementation and composition-only owner; unchanged consumer runtime lock. Exact sources, qualification and integration: [AG-02 evidence](waves/AG-02-tenant-encapsulation.md) |')
old = '| AG-03 through AG-09 | Defined, not delivered | Narrow-use-case trial, precise type rules, coverage, template integration, migration/debt-growth and continuous evolution |'
assert s.count(old) == 1
s = s.replace(old, '| AG-03 | Next independent consumer task | IoT Delivery narrow-use-case/Repository pilot; not implemented by AG-02 |\n| AG-04 through AG-09 | Defined, not delivered | Precise type rules, coverage, template integration, migration/debt-growth and continuous evolution |')
a = s.index('The earlier AG-01 head `7c50680')
b = s.index('\n\nThese tests characterize', a)
s = s[:a] + 'AG-01I is integrated at `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`; exact-main run `34136019602` revalidated the reviewed tree after non-force fast-forward. Its mechanism evidence is separate from AG-02 consumer behavior. AG-02 preserves Biz runtime `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb` and separately qualifies current-main CLI ownership/planning at `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`. It does not upgrade consumer dependencies or add a framework compiler/runtime rule. The structural batch remains a manually reviewed migration, not a claimed complete single-Operation AX7 attestation.' + s[b:]
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
s = s.replace(needle, 'AG-02 落地决策：采用 `internal/access/application/tenantlifecycle/internal/usecase`，手写实现不再与生成 wrapper 同包，所有者工厂只返回生成 Application 接口。该路径仍在原 Application 范围内，未扩展 Ownership/ChangePlan 白名单；构造入口、实际方法集和非法隐藏包导入均有消费者回归。运行时保持消费者原锁；当前框架 CLI 另行验证路径可编辑及明确的 unresolved implementation scope，不声称完整结构批次已通过单 Operation AX7。验收过程中修正了消费者 B12.7 的 Ready 状态/日志观察时序，保留全部验收条件。精确证据见 [AG-02 验收记录](../waves/AG-02-tenant-encapsulation.md)。\n\n' + needle)
p.write_text(s)

body = '''# AG-02 — Biz TenantLifecycle encapsulation qualification

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: one real consumer Application; no framework runtime/compiler upgrade

## Exact sources and implementation

Biz PR #16 integrates `b0f250e7570e6adae7b108a1ac27f92bd5e6ea78`, tree `99201f2e54069b8d478d04f02830bde99539f871`, from base `7d5afbe9cb4b849be462d9a7aed65877ed227700`. The encapsulation commit is `0924157e3ed72cae3cf364955bd041c895087adf`; the follow-up fixes a consumer runtime-verification race. Both formal commits were created with local Git and reproduced with exact SHA/tree equality on the Runner before qualification and publication.

Consumer runtime remains Yunka `6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb`. The separate governance CLI checkout is Yunka `4dd2a264c73a2ce6b85ad7ae6238046762e7e0f7`. These are distinct qualification subjects, not an implicit framework upgrade.

Handwritten TenantLifecycle moves into `internal/access/application/tenantlifecycle/internal/usecase`, with an unexported concrete service. The owner factory returns the generated Application interface. Its actual method set and fields are checked, and production owner imports are restricted by a pilot source rule to the existing composition root. Generated child wrappers and root ExecutionScope/UoW behavior are unchanged. No PB/API, Operation identity, schema, authorization, CAS, dependency or generator changes.

The implementation remains inside the canonical Application scope. Current CLI Ownership reports both new locations editable, while Change Plan correctly leaves the handwritten filename unresolved within that scope. No broad allowlist is added.

## Executed consumer qualification

[Initial run 34185711218](https://github.com/hvritual/biz/actions/runs/34185711218), job `101933505538`, passed locked Go 1.25.13/protoc 3.21.12, MySQL 8.4, consumer-certify, check/generate/check with zero drift, full Go tests, internal race, vet/build and current CLI readback. Six targeted test functions and all 19 MySQL integration tests passed without fail/skip. Linux overlay probes accept the legal owner and reject direct/aliased hidden imports for the exact internal-package error.

Initial artifact `10040462449`, ZIP SHA256 `2a06651991013e58f47eed008f17911b4923ba6e1568bda91477da0d49de3a36`, was downloaded; CRC and all 23 member hashes were verified, including candidate identity and per-test JSON. This proves its exact initial head, not the later follow-up automatically.

[Final qualification run 34186461994](https://github.com/hvritual/biz/actions/runs/34186461994) reruns the consumer suite on the follow-up head and executes the actual B12.7 workflow build/runtime bodies: Ready, Diagnostics, six Application graph links and shutdown. Normal PR checks and independent automated review disposition are recorded on [Biz PR #16](https://github.com/hvritual/biz/pull/16). Tests are not human approval.

The [consumer integration run INTEGRATION_RUN](https://github.com/hvritual/biz/actions/runs/INTEGRATION_RUN) verifies review/check evidence, performs a non-force fast-forward, reads actual main back and replays tests on that exact main. This enclosing Yunka change is documentation only and requires its own qualification/review and main readback; no unexecuted enclosing commit verification is claimed here.

## Limits and preserved failures

Only TenantLifecycle is sealed; Member/Role retain their existing organization. The pilot import policy is not a generic type/data-flow analyzer. Linux/GNU-timeout probes are not an OS sandbox or all-platform certification. This manually reviewed structural batch is not a complete single-Operation AX7 attestation. Template enforcement, generic migration proof and continuous-growth checks remain later AG tasks.

Original B12.7 run `34185974204` observed canonical Ready state before the independently polled `DEV READY` log; its immediate grep failed. The corrected consumer harness waits for BOTH facts within the original bound and retains all final assertions. A deterministic regression of the actual shell fragment proves delayed-log success and missing-log/not-ready rejection. Workflow schedules, permissions, runtime semantics and acceptance requirements are unchanged.

Previous staging blob `575cf9925c4a826abf8934df3c8b643000798f35` had a readable bundle header but failed Git pack decompression. It was not used. The implementation was rebuilt from a verified clean baseline. A later transport-copy discrepancy was corrected before verifying the immutable patch digest `9951e36a546a3f67396a8770dfd9aa0989d480dc17fb6049da287f023cd28138` and exact Git commit/tree. Transport checks do not replace behavioral qualification.

Rollback reverts the consumer batch including owner wiring and test-harness correction. No deployment, data migration, signed authority or self-approval is claimed. AG-03 is the next independent IoT Delivery narrow-use-case task.
'''.replace('INTEGRATION_RUN', run)
(root / 'docs/waves/AG-02-tenant-encapsulation.md').write_text(body)
