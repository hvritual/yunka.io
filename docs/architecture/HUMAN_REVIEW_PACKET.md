# Human Review Packet

> Document class: **DECISION**  
> Scope: semantic change map and human review evidence for bounded Yunka changes  
> Normative parent: [`../ENGINEERING_QUALITY_RULES.md`](../ENGINEERING_QUALITY_RULES.md)

## Purpose

A successful build, test run, or raw Git diff does not by itself explain why a change exists, what responsibility moves, or which semantic dimensions changed. `yunka change review` projects the existing Change Contract, Change Attestation, deterministic semantic reconciliation, architecture-debt evidence, and exact Git candidate into one review-first artifact.

The packet is presentation evidence. It never grants edit authority, changes contract semantics, marks a failing candidate conformant, waives findings, merges code, or replaces the raw diff.

## Declared review context

`yunka change review build` requires the author or agent to state:

- Problem;
- current concepts / responsibilities;
- desired ownership;
- WHY;
- WHAT;
- BOUNDARY;
- optionally affected invariants, risks, and unresolved findings.

These fields are explicitly **declared narrative**, not executable truth. They are digest-bound so accidental or unilateral packet edits are detectable, but their business correctness remains a human review decision.

## Derived facts

The framework derives the following from existing evidence rather than asking the author to restate them:

- **Behavior change** — deterministic `SemanticDelta` entries from the Change Attestation;
- **Public API change** — canonical `contract` and `transport` semantic deltas;
- **Persistence / database change** — changed source paths in deterministic persistence or migration surfaces;
- **Generated-code change** — reconciliation entries already classified as generator-owned;
- **Verification** — exact attestation conformance and gate results;
- **Unresolved findings** — attestation diagnostics and existing/new proven architecture-debt findings;
- **PROOF** — immutable base/head identity plus contract, attestation, changed-path, candidate-content, and aggregate evidence SHA-256 digests.

A structural-only refactor with no supported semantic, API, persistence, or generated evidence reports those dimensions as explicit `NONE`. `NONE` means the deterministic evidence set contains no delta in that dimension; it is not a claim that arbitrary business behavior is mathematically equivalent.

## Exact-candidate binding

The packet binds all of:

```text
Change Contract base SHA
current HEAD SHA
operation identity
Change Contract bytes
Change Attestation bytes
review narrative
reconciled changed path set
current bytes/mode/symlink target of every changed candidate path
```

`yunka change review check` reloads the packet and rebuilds the expected projection from the current Contract, Attestation, semantic evidence, Git delta, and candidate bytes. It fails when:

- HEAD moved after attestation;
- the changed path set no longer matches the attestation;
- deterministic semantic evidence no longer matches the attestation;
- candidate bytes change while the path set stays the same;
- the Contract or Attestation digest changes;
- packet narrative/evidence digests are inconsistent;
- a derived packet projection was edited or became stale.

This makes review evidence replayable without promoting the packet into another writable business source of truth.

## CLI

Build the packet only after `yunka change verify` has produced the candidate attestation:

```text
yunka change review build \
  --problem "..." \
  --current-concept "..." \
  --desired-ownership "..." \
  --why "..." \
  --what "..." \
  --boundary "..."
```

Reconcile it immediately before human review or delivery:

```text
yunka change review check
```

Both commands support `text`, `json`, and `agent-json`. Human text presents semantic summary and proof before the reviewer opens the raw diff; machine output contains the same packet fields.

## Authority boundary

The packet MUST NOT:

- replace Change Contract edit/scope authority;
- replace Change Attestation verification authority;
- infer business semantics that deterministic evidence cannot prove;
- turn AI advice into merge or mutation authority;
- silently waive architecture or engineering-quality debt;
- hide failing gates or unresolved findings;
- introduce consumer-specific path exceptions.

Semantic architecture critique remains separate work. Debt waivers, including owner/reason/scope/expiry authority, remain separate governance rather than being embedded in this review artifact.
