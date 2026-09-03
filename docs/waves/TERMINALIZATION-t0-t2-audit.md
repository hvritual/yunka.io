# Framework Terminalization — T0-T2 Audit Foundation

> Document class: **CURRENT DELIVERY EVIDENCE**
> State: **T0-T2 production-qualified and real-consumer pressure-qualified candidate; unmerged**
> Delivery PR: #141
> Base: `main@1901162383832e2d5c49809d579c72919ba8cfbd`
> Final behavioral candidate: `3348c7aaf613e6443b295d746f8bc659477f9c22`

## Goal

Start framework terminalization above the already merged AX1-AX7 control plane without creating another compiler, runtime, security model, architecture Source of Truth, or LLM dependency.

The T0-T2 delivery makes the Agent protocol self-describing, establishes deterministic read-only project audit evidence, introduces a deliberately narrow first set of proven architecture violations, and classifies proven debt relative to an immutable Git baseline as existing/new/fixed.

## T0 — Agent protocol reconciliation

`yunka context --json` schema v2 exposes the merged bounded-change protocol and deterministic audit entry point:

```text
new structure -> yunka add ...
existing operation -> change plan -> change begin -> change check -> change verify
project audit -> yunka audit --format agent-json
runtime evidence -> yunka dev --event-format jsonl
```

Agent-facing check guidance uses `yunka check --format agent-json`. Root CLI discoverability is reconciled with the same protocol.

## T1 — Deterministic Audit MVP

Audit remains an `app` developer/control-plane capability; a failed isolated-module qualification proved that prematurely exporting it as a new `pkg` public API would violate the repository's independently versioned module boundary. No `replace`, workspace-only workaround, or public framework package was introduced.

`yunka audit` is read-only and deterministic. It uses canonical project/Manifest facts plus Go parser direct-import evidence. It does not use grep, package-name heuristics, SSA, DDD inference, or an LLM.

The deterministic report admits only:

- `proven_violation`
- `evidence_observation`

Design hypotheses are explicitly outside this machine-evidence layer.

The first proven rules are deliberately narrow:

- `AUDIT-APP-001`: a handwritten Application directly imports another canonically declared domain's `ports` or `infrastructure/persistence` boundary.
- `AUDIT-INFRA-001`: a handwritten Application directly imports the Yunka platform Provider surface instead of receiving declared typed infrastructure capabilities.
- `AUDIT-AUTH-001`: a handwritten Application directly imports the canonical Gateway authorization implementation instead of leaving authorization at the root execution security boundary.

Generated files and tests are excluded from these handwritten-Application rules. Cross-domain evidence requires both source and target domains to exist in the canonical Manifest.

## T2 — Architecture Debt Delta

`yunka audit --base <git-ref>` resolves the base ref to an immutable commit and reads base `go.mod`, canonical Manifest, and Go source directly from the Git object database. It does not checkout the baseline and it does not persist an architecture-baseline file.

Only stable `proven_violation` finding IDs participate in debt accounting:

```text
base + current -> existing / new / fixed
```

Evidence observations do not become blocking debt. A base/current Go module identity mismatch fails closed rather than pretending findings remain comparable across a module-identity migration.

The read-only qualification measures developer-visible worktree contents and Git status; `.git/**` implementation metadata is not treated as application source mutation.

## Repository qualification

T0-T1.3 exact candidate `3f39bf1f9015e6e9b0bef4e1f8a3f576985299bb` passed:

- CI #503 / run `33771500741`
- production #261 / run `33771500693`

Initial T2 candidate `991737b7cd9f0c5b54a1cfaf77f904448cf072d3` passed:

- CI #504 / run `33775626334`
- production #262 / run `33775626318`

Real Biz pressure then proved one compatibility gap: the candidate detected the injected cross-domain repository bypass but missed Biz's qualified compatibility import identities `yunka.io/framework/platform` and `yunka.io/gateway/authz`.

The minimal fix recognizes only the canonical GitHub module identities plus those exact qualified compatibility identities. It deliberately does not infer arbitrary imports by suffix. Final behavioral candidate `3348c7aaf613e6443b295d746f8bc659477f9c22` passed:

- CI #506 / run `33777098508`, including full Verify and determinism
- production #264 / run `33777098481`, including MySQL 8.4 and clean-worktree

## Real Biz reverse qualification

Consumer: `hvritual/biz`

- B13 pressure tree: `506e9c117822855db318f8b4b6689d318a62ded1`
- debt baseline: Biz `main@7d5afbe9cb4b849be462d9a7aed65877ed227700`
- qualification run: `33777462988`

The exact Yunka final behavioral candidate produced deterministic, read-only clean-consumer evidence:

```text
current_findings=0
existing=0
new=0
fixed=0
```

The same run then injected a temporary, uncommitted Application pressure fixture using the consumer's real import forms:

```text
github.com/hvritual/biz/internal/deviceops/ports
yunka.io/framework/platform
yunka.io/gateway/authz
```

The resulting new-debt rules were exactly:

```text
AUDIT-APP-001
AUDIT-AUTH-001
AUDIT-INFRA-001
```

The fixture was removed and Git status was reconciled to the pre-audit state. Evidence artifact ID: `9902173543`.

## Hard boundaries preserved

This delivery does not:

- change production request-path/runtime semantics;
- modify Executor, ExecutionScope/UoW, authorization, persistence, or transport ownership;
- expand protobuf business semantics;
- add an architecture baseline Source of Truth;
- introduce SSA/DDD/repository-wide heuristic architecture inference;
- call or embed an LLM;
- make historical debt blocking by default;
- claim business/domain design correctness from imports.

## Next gate

Do not proceed directly to automatic remediation. The next planned terminalization stage is T3: define an AI Architecture Advisor evidence contract that consumes deterministic Audit findings and can only produce evidence-bound recommendations/hypotheses. Yunka itself should remain LLM-independent, and any recommendation that widens authority or changes business/domain design remains human-approved rather than self-authorizing.
