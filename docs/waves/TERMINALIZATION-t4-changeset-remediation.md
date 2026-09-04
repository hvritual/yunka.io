# Terminalization T4 ChangeSet + Remediation Qualification Evidence

> Document class: **EVIDENCE**  
> Evidence scope: exact T4.1/T4.2 merged candidate plus exact T4.3 qualification candidate and Biz B13 consumer pair  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Integration state at this record: **T4.1/T4.2 merged; T4.3 production-qualified and real-consumer-qualified in PR #146, not yet merged**

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

The default transient ChangeSet is Git-private at `.git/yunka/change-set.json`. Set-wide reconciliation compares the actual Git delta with exact writable/generated boundaries and then re-reads canonical semantics, including access, permission, tenant, authentication, transaction, idempotency, composition, dependency, HTTP, RPC/service, and DTO facts.

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

`yunka change set remediation bind --finding <id>` creates a Git-private binding at `.git/yunka/change-remediation.json` that records:

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

The final protocol candidate is:

`2ade49e1fc6ef9a49f1f7ced6dc9979919d3100a`

It adds Agent Context schema v4 discoverability for prospective Operation planning, ChangeSet begin/check, Operation apply, and remediation bind/check without widening runtime or mutation semantics.

Exact final-candidate framework evidence:

- CI #556 / run `33837678059`: **PASS**
- production #314 / run `33837678104`: **PASS** on MySQL 8.4 with clean-worktree verification

An intermediate discoverability-only candidate failed because it changed an existing root-help sentence required by a compatibility test; the final candidate restored that wording without weakening the test or changing T4.3 behavior.

## Real Biz B13 remediation pressure

Consumer pressure base:

`506e9c117822855db318f8b4b6689d318a62ded1`

Final replay qualification branch commit:

`2306a74a98bf594bb0a9c0f28e783c8c76f50b9c`

Qualification run:

`33838460057` — **PASS**

Evidence artifact:

- artifact ID: `9924084745`
- digest: `sha256:5f6c27d3a0fe9c4ae87e8b3137aadc74df72437d7364b83b7fc2111ab626b0d4`
- size: 6166 bytes

The wrapper changed only the exact Yunka candidate SHA from the earlier behavior replay; the pressure body remained unchanged.

The CI-local bad baseline created the proven finding:

`AUDIT-INFRA-001:internal/deviceops/application/t4_remediation_pressure.go:yunka.io/framework/platform`

with immutable temporary consumer baseline:

`b9c675b6c776d008e199a9cf957306d22fa87f45`

and ChangeSet digest:

`1d59ae480d47d3d318478a5cd4c07b97e6134b17586ba7b23e76244f85c2e0eb`

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

## Authority boundary

T4 does not authorize an AI or Advisor to decide that a change is safe to merge. It provides deterministic bounded mutation and proof inputs only.

T5 remains responsible for any future final Proof-of-Change aggregation across ChangeSet, Audit/remediation, repository gates, runtime/behavior evidence, and merge attestation. T5 is not implemented by this delivery.
