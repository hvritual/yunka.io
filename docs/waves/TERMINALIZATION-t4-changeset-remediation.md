# Terminalization T4 ChangeSet + Remediation Qualification Evidence

> Document class: **EVIDENCE**  
> Evidence scope: exact T4.1/T4.2 merged candidate plus exact T4.3 qualification candidate and Biz B13 consumer pair  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Integration state at this record: **T4 complete / production-qualified / real-consumer-qualified / merged through PR #146**

## Scope

T4 closes the gap between prospective/new-structure planning, multi-subject bounded mutation, and deterministic Audit remediation proof without adding a second compiler, runtime, security model, architecture database, or LLM execution path.

The terminal control-plane sequence covered by this record is:

```text
Human intent
   ↓
prospective Operation plan / existing Change Contract
   ↓
ChangeSet on one immutable Git base
   ↓
bounded mutation
   ↓
actual Git delta + canonical semantic readback
   ↓
optional Audit remediation binding
   ↓
Audit debt delta proof
```

A remediation declaration is never treated as proof that a finding was fixed.

## T4.1 — Plan before mutation

`yunka add operation ... --plan` is read-only and reuses the same preparation path as the real structural authoring command. The plan records prospective mutations, generated effects, implementation landing path, and normalized explicit Operation semantics before any writable source is changed.

Normal `yunka add operation` remains the mutation command; the plan itself does not grant edit authority.

## T4.2 — ChangeSet v2

`yunka change set begin|check` composes existing AX7 Change Contract v1 subjects and create-Operation plan subjects on one immutable Git baseline.

The default transient ChangeSet is logically exposed as `.git/yunka/change-set.json`. Its physical storage is Git-private and resolved through Git's actual directory, so ordinary repositories, linked worktrees, and submodules do not require `.git` to be a directory. Set-wide reconciliation compares the actual Git delta with exact writable/generated boundaries and then re-reads canonical semantics, including access, permission, tenant, authentication, transaction, idempotency, composition, dependency, HTTP, RPC/service, and DTO facts.

T4.2 does not introduce broad generated-directory authority. Protobuf Go outputs remain exact-path/evidence-backed.

### T4.1/T4.2 exact qualification

- PR: #145
- exact candidate: `07a1210d06020a01380b5d51f5812408e6e553c2`
- CI #549 / run `33835565704`: **PASS**
- production #307 / run `33835565727`: **PASS** on MySQL 8.4 with clean-worktree verification
- Biz B13 pressure base: `506e9c117822855db318f8b4b6689d318a62ded1`
- Biz reverse qualification commit: `dfd500c4626c491dc5fdf39ec7fad26ccf5755d2`
- Biz reverse qualification run `33835971657`: **PASS**
- canonical merge commit: `15f19ff0845e421039ffb675937bf23c3bb8a79d`

The real consumer run proved matching protected/api-key/read-only creation to be conformant, exact protobuf generated-path ownership to remain narrow, adversarial public/optional/none semantic drift to fail closed, and the consumer worktree to be restored clean.

## T4.3 — Audit Finding to ChangeSet remediation linkage

T4.3 keeps remediation proof separate from ChangeSet mutation authority.

`yunka change set remediation bind --finding <id>` creates a Git-private binding logically exposed as `.git/yunka/change-remediation.json` that records:

- immutable ChangeSet base SHA;
- normalized ChangeSet digest;
- exact proven Audit Finding IDs.

The binding grants no additional editable path or semantic category.

`yunka change set remediation check` reruns normal ChangeSet reconciliation plus Audit debt comparison from the immutable ChangeSet base. A remediation is conformant only when:

```text
ChangeSet conformant
AND every target finding is fixed
AND remaining == 0
AND new proven debt == 0
```

Unknown findings, findings absent from the immutable base, stale/tampered ChangeSet digests, and base mismatch fail closed. Existing unrelated historical debt is not made blocking merely because one remediation target was bound.

T4.2 `change set begin|check` machine contracts remain unchanged; remediation is an independent proof sidecar rather than a new mutation-authority field.

## T4.3 framework qualification

Behavioral implementation candidate `d527d2bbfb1c9bcd89598c680b81679def68f488` first passed CI #551 and production #309 before final protocol discoverability reconciliation.

Protocol candidate `2ade49e1fc6ef9a49f1f7ced6dc9979919d3100a` then passed CI #556 / run `33837678059` and production #314 / run `33837678104`, including Agent Context schema v4 discoverability.

After PR #146 was marked ready, automated review identified a portability defect: both T4 Git-private defaults treated `<root>/.git` as a physical directory, which fails for linked worktrees/submodules where `.git` is a file. The closure preserves the logical machine paths but resolves their physical storage with Git's own `rev-parse --git-path` semantics. A real linked-worktree regression now proves both ChangeSet and remediation state can be written/read with `.git` as a regular file while Git status remains clean.

The post-review exact candidate is:

`a584da287d52271a4a05ad80167f89fdbe0d3ee0`

Exact framework evidence:

- CI #563 / run `33842121802`: **PASS** including full Verify, determinism, and linked-worktree regression
- production #321 / run `33842121900`: **PASS** on MySQL 8.4 with clean-worktree verification

No ChangeSet/remediation schema, mutation authority, Runtime, Executor, UoW, Authz, Persistence, or protobuf business semantics changed in this closure.

## Real Biz B13 remediation pressure

Consumer pressure base:

`506e9c117822855db318f8b4b6689d318a62ded1`

Post-review replay qualification branch commit:

`d2e26f9eeebce1f9989d88ebaaf07f0d3f6196a3`

Qualification run:

`33842351609` — **PASS**

Evidence artifact:

- artifact ID: `9925335952`
- digest: `sha256:891d92a07de5b555b15f22a33776060f35f8dc9339e4ba126643b366a3c0f1d4`
- size: 6159 bytes

The effective qualification tree restores the previously qualified wrapper and changes the exact Yunka candidate to `a584da28...`; the pressure script remains unchanged.

The CI-local bad baseline created the proven finding:

`AUDIT-INFRA-001:internal/deviceops/application/t4_remediation_pressure.go:yunka.io/framework/platform`

with immutable temporary consumer baseline:

`b64b5daabbe8afe1c6926ffd92fe9295c1bc3701`

and ChangeSet digest:

`8f8008b99f8689b405d7e42b7145d4b5c2c419f7be969d6f8da8ca2d01827eca`

### Pressure A — declared target remains unchanged

Result:

```text
ChangeSet conformant = true
fixed                = []
remaining            = [AUDIT-INFRA-001 target]
newDebt              = []
overall conformant   = false
```

This proves binding a finding does not self-authorize or self-attest remediation.

### Pressure B — old finding removed but replaced by new proven debt

The fixture changed from `yunka.io/framework/platform` to `yunka.io/gateway/authz`.

Result:

```text
ChangeSet conformant = true
fixed                = [AUDIT-INFRA-001 target]
remaining            = []
newDebt              = [AUDIT-AUTH-001:internal/deviceops/application/t4_remediation_pressure.go:yunka.io/gateway/authz]
overall conformant   = false
```

This proves "fixed target" is insufficient when the same bounded change introduces new proven architecture debt.

### Pressure C — actual remediation

The violating file was removed inside the declared ChangeSet scope.

Result:

```text
ChangeSet conformant = true
fixed                = [AUDIT-INFRA-001 target]
remaining            = []
newDebt              = []
Audit conformant     = true
overall conformant   = true
```

The qualification then restored the Biz worktree and removed Git-private Yunka control state.

## Final delivery and canonical integration

The final PR/EVIDENCE head was:

`8cc3967a1b2baaf5eecbbb22fa21d65eb2c80c2b`

Final pre-merge gates:

- CI #565 / run `33844574960`: **PASS**
- production #323 / run `33844574953`: **PASS** on MySQL 8.4 with clean-worktree verification

PR #146 merged this exact head as canonical merge commit:

`ba896a8776987e91d2525f6bb15a188653857980`

Post-merge main verification:

- CI run `33844917226`: **PASS**
- production run `33844917372`: **PASS** on MySQL 8.4 with clean-worktree verification

T4 is therefore complete, production-qualified, real-consumer-qualified, and canonically merged. T5 remains not started.

## Authority boundary

T4 does not authorize an AI or Advisor to decide that a change is safe to merge. It provides deterministic bounded mutation and proof inputs only.

T5 remains responsible for any future final Proof-of-Change aggregation across ChangeSet, Audit/remediation, repository gates, runtime/behavior evidence, and merge attestation. T5 is not implemented by this delivery.
