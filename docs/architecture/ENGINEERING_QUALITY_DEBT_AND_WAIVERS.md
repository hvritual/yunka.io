# Engineering-quality debt and waivers

## Purpose

Engineering-quality debt is evidence about how a candidate changes the maintainability and semantic reviewability of a repository. It does not replace deterministic Audit findings, semantic architecture review findings, Git diff, runtime verification, or source ownership policy.

The framework uses two existing finding sources:

1. deterministic Audit findings, classified against an immutable Git baseline as `existing / new / fixed`;
2. advisory semantic architecture findings, classified by the validated semantic-review contract as `existing / new / resolved`.

`QualityDebtProof` is only a Proof-of-Change projection over those sources. It does not create new finding identities or rewrite either source.

## Blocking authority

Only deterministic findings that are both:

- `proven_violation`; and
- explicitly enabled in the canonical engineering-quality `blockingRules`

may fail quality conformance automatically.

Historical debt remains visible but is not reclassified as new debt. Non-blocking deterministic findings and all semantic-review findings remain visible and advisory.

A semantic-review finding can never become blocking merely because its severity is `high`.

## Exact waiver contract

A waiver is temporary acceptance of one exact **current blocking new deterministic finding**. It is stored in Git-private state by default at:

```text
.git/yunka/engineering-quality-waivers.json
```

Each waiver must contain:

- a human `owner`;
- a non-empty `reason`;
- exact scope containing base SHA, head SHA, finding ID, finding SHA-256, rule and path;
- RFC3339 `expiresAt`;
- a non-empty `reviewCondition`.

The waiver ID and the waiver-set digest are derived from the normalized contract. Permanent, unowned, duplicate, stale, expired or scope-mismatched waivers fail closed.

A waiver does **not** suppress:

- Git change-contract violations;
- generated/source ownership violations outside its exact finding;
- semantic-contract violations;
- runtime/test failures;
- advisory findings;
- any deterministic finding that is not the exact finding digest named by the waiver.

## Verification behavior

`yunka change verify` performs the following quality sequence after canonical Yunka verification succeeds:

1. build deterministic Audit debt against the Change Contract base SHA;
2. preserve the full deterministic delta as architecture-debt evidence;
3. optionally compare validated semantic-review baseline/current attestations;
4. load the exact-candidate waiver set when present;
5. construct a digest-bound `QualityDebtProof`;
6. fail the `quality-debt` gate only when unwaived blocking new deterministic findings remain or waiver evidence is invalid.

The `architecture-debt` gate proves that deterministic debt evidence was collected. The `quality-debt` gate owns the blocking decision.

## Waiver commands

Waiver operations are nested under `change review` because they are review governance, not mutation authority.

Create a waiver only after inspecting the rendered blocking debt:

```text
yunka change review waiver create \
  --finding <AUDIT-FINDING-ID> \
  --owner <human-owner> \
  --reason <temporary-acceptance-reason> \
  --expires-at <RFC3339> \
  --review-condition <condition-requiring-review>
```

Validate the current Git-private waiver file against the exact candidate:

```text
yunka change review waiver check
```

Moving HEAD, changing the baseline, changing the finding evidence, changing rule/path identity, or passing the expiry timestamp invalidates the waiver.

## Human review packet

When a Change Attestation contains a valid `QualityDebtProof`, the human review packet projects:

- deterministic existing/new/fixed counts;
- blocking-new count;
- waived and unwaived blocking counts;
- advisory existing/new/resolved counts;
- exact blocking and advisory-new finding IDs;
- the quality-debt proof SHA-256.

Waived findings remain visible in `unresolvedFindings` with owner, expiry and review condition. A waiver is therefore an explicit reviewed exception, not suppression.

## Authority boundary

Quality debt and waivers do not grant mutation, patch, deployment, merge, or semantic-correctness authority. They only affect the engineering-quality blocking gate for the exact deterministic finding named by an otherwise valid waiver.
