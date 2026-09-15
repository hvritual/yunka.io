# Semantic Architecture Review

> Document class: **CURRENT**  
> Normative engineering-quality authority: [`../ENGINEERING_QUALITY_RULES.md`](../ENGINEERING_QUALITY_RULES.md)  
> Deterministic architecture authority remains Audit / Change Contract / Change Attestation.

## Purpose

Semantic architecture review covers questions that deterministic checks cannot prove completely, including responsibility clarity, semantic colocation, abstraction value, naming fitness, and duplicated concept ownership.

The review is **advisory-only**. It explains concerns to a human. It does not authorize source mutation, waiver, merge, release, or changes to deterministic Audit facts.

## Command surface

```text
yunka advisor semantic request
yunka advisor semantic validate
yunka advisor semantic compare
```

`request` exports exact source evidence. `validate` validates an external reviewer response. `compare` compares two validated attestations as `existing / new / resolved`.

Yunka does not invoke an LLM in this command path. An external reviewer may be an AI or a human process, but its response has no authority until it passes the semantic review contract, and passing the contract still grants advisory authority only.

## Exact evidence identity

Every request binds:

- exact Git `HEAD` identity;
- every explicitly selected source path;
- the exact UTF-8 source bytes and SHA-256 for each path;
- a deterministic source-set SHA-256;
- a deterministic `sourceIdentity` over the Git/source evidence;
- optionally, a verified Human Review Packet exact-candidate identity:
  - base SHA;
  - head SHA;
  - candidate SHA-256;
  - review evidence SHA-256.

When `--review-packet` is supplied, Yunka first runs the existing review-packet reconciliation. A stale or tampered review packet cannot become semantic-review evidence.

Semantic review never widens Change Contract editable paths or generated ownership.

## Finding contract

A finding contains:

```text
id
path
symbolOrScope
category
severity
reason
recommendedAction.kind
recommendedAction.detail
behaviorChangeRequired
sourceIdentity
```

`id` is framework-owned. It is derived from:

```text
category + path + symbolOrScope
```

A reviewer may omit the ID and let Yunka canonicalize it. A supplied mismatched ID is rejected.

Supported categories are intentionally narrow:

- `semantic_colocation`
- `ambiguous_responsibility`
- `unjustified_abstraction`
- `naming_fitness`
- `duplicated_concept_ownership`

Severity is advisory prioritization only:

- `low`
- `medium`
- `high`

Supported recommendation kinds are non-executable:

- `investigate`
- `discuss_design`
- `consider_refactor`
- `consider_rename`
- `document_decision`

`behaviorChangeRequired=true` means the reviewer believes acting on the recommendation would require behavior review. It does **not** authorize that change.

## Authority boundary

The request fixes:

```text
authority = advisory_only
mutationAuthorized = false
mergeAuthorized = false
```

The response also fixes:

```text
authority = advisory_only
```

Unknown fields fail strict JSON decoding. Unsupported categories, severities, action kinds, source identities, paths, digests, or stable finding IDs fail validation.

The recommendation surface has no patch payload, command execution, file-write, merge, waiver, or approval field. Explicit authority language such as `safe_to_merge` or `apply_patch` in the recommended action is rejected.

A semantic finding cannot rewrite or suppress deterministic Audit findings. It is stored in a separate semantic attestation and remains advisory even when the same concern is repeated across reviews.

## External response shape

A reviewer response uses the request digest and source identity it received:

```json
{
  "schemaVersion": 1,
  "authority": "advisory_only",
  "requestDigest": "<request sha256>",
  "findings": [
    {
      "id": "",
      "path": "internal/tenant/application/service.go",
      "symbolOrScope": "Service",
      "category": "ambiguous_responsibility",
      "severity": "medium",
      "reason": "The service owns unrelated lifecycle and quota decisions.",
      "recommendedAction": {
        "kind": "discuss_design",
        "detail": "Review whether quota policy needs a separate owner."
      },
      "behaviorChangeRequired": false,
      "sourceIdentity": "<exact source identity sha256>"
    }
  ]
}
```

The empty `id` is canonicalized by Yunka. All other finding evidence must be explicit.

## Replay and debt-style comparison

A validated response becomes a digest-bound semantic attestation. Attestations can be compared by stable finding ID:

- `existing`: identity exists in baseline and current review;
- `new`: identity exists only in current review;
- `resolved`: identity exists only in baseline review.

This classification is evidence for later engineering-quality governance. It is not itself a blocking gate. Any future decision to make a rule blocking belongs to deterministic policy and explicit waiver governance, not to this semantic review surface.

## Failure conditions

Validation fails closed when, among other cases:

- source evidence is empty;
- source bytes or source digest were changed;
- Git/change identity is inconsistent;
- a finding references a path outside the exact request evidence;
- `sourceIdentity` does not match the request;
- the response attempts authority escalation;
- category/severity/action kind is unknown;
- unknown JSON fields are present;
- attestation digest was tampered;
- recommendation text attempts merge or mutation authority.

## Human projection

Text output includes category, severity, path, symbol/scope, reason, recommended action, behavior-change requirement, and stable finding ID. A reviewer therefore does not need the original AI conversation to understand the advisory concern.
