# Human-Reviewable AI Code Implementation Plan

> Document class: **CURRENT**  
> Authority: executable implementation decomposition for [`../ENGINEERING_QUALITY_RULES.md`](../ENGINEERING_QUALITY_RULES.md)  
> Status authority: GitHub Issues and [`../STATUS.md`](../STATUS.md)

## Goal

Turn the normative engineering-quality baseline into framework capabilities that make AI-authored code reviewable by humans and reusable across different Yunka consumers without embedding task-history naming in production code.

The authoritative global rules are in `docs/ENGINEERING_QUALITY_RULES.md`. This plan only decomposes implementation work; it is not a second rule source.

## Work items

### Semantic naming and durable source identity

**Goal**: detect task-history identity leakage and low-information durable names in production code and tests.

**Deliverables**:

- deterministic naming-policy representation;
- checks for task-history identifiers in package/file/exported-symbol/durable-test identities;
- configurable allowlist only for genuine business terminology;
- detection of low-information container names when they aggregate unrelated primary concepts;
- human- and agent-readable diagnostics with path, symbol, reason and remediation.

**Non-goals**:

- universal natural-language naming correctness;
- mechanical one-type-per-file enforcement;
- renaming historical evidence records.

**Acceptance**:

- positive/negative framework fixtures;
- current Biz examples detected without consumer-specific hardcoding;
- second consumer qualification;
- no false claim that advisory naming judgments are deterministic business truth.

### Package documentation and invariant comments

**Goal**: make package responsibility, exclusions, business invariants and non-obvious technical decisions discoverable without the original task context.

**Deliverables**:

- package-documentation policy for core framework/business packages;
- deterministic checks for missing package documentation in declared governed scopes;
- exported/invariant comment rules that distinguish useful contract comments from syntax restatement where deterministically possible;
- authoring guidance and examples.

**Non-goals**:

- forcing comments on every declaration;
- measuring comment quantity as quality;
- AI-generated prose as a substitute for code clarity.

**Acceptance**:

- missing package responsibility docs are detectable;
- redundant-comment examples remain advisory unless a deterministic rule can prove the violation;
- generated sources are excluded or handled by generated ownership rather than handwritten-comment requirements.

### Deterministic code-semantics checks

**Goal**: add a read-only quality surface for objectively checkable human-reviewability rules.

**Deliverables**:

- checks for generated/manual ownership mixing;
- known generic container-file patterns;
- task-history naming leakage;
- declared file-size / concept-count / package-doc thresholds where policy is deterministic;
- stable finding IDs and agent-json diagnostics;
- integration with existing audit/debt-delta infrastructure when appropriate rather than another parallel finding engine.

**Non-goals**:

- AST/LLM inference of complete DDD correctness;
- automatic source mutation;
- a second architecture Source of Truth.

**Acceptance**:

- repeatable zero-diff read-only check;
- stable findings across repeated runs;
- existing/new/fixed classification can reuse current audit debt semantics;
- no consumer path hardcoding.

### Semantic change map and human review packet

**Goal**: ensure every non-trivial AI change explains intent and semantic delta before a reviewer reads the raw diff.

**Deliverables**:

- machine-readable change metadata for Problem / Current concepts / Desired ownership / Behavior / API / Persistence / Generated / Verification;
- WHY / WHAT / BOUNDARY / PROOF projection;
- human review packet summarizing affected concepts, invariants, behavior, API/database/generated delta, proof and unresolved risk;
- binding to exact change contract / Git baseline without creating another writable business SoT.

**Non-goals**:

- replacing Git diff;
- automatic merge approval;
- allowing AI summaries to override executable evidence.

**Acceptance**:

- stale or mismatched review packets fail reconciliation;
- no-change structural refactors explicitly prove zero behavior/API/database delta;
- packet generation is deterministic for the same exact evidence set.

### Structured semantic architecture review

**Goal**: allow AI to review cohesion, responsibility clarity, abstraction value and naming fitness while keeping deterministic authority separate.

**Deliverables**:

- structured finding schema with path, symbol/scope, category, severity, reason, recommendation and behavior-change flag;
- supported categories such as semantic colocation, unjustified abstraction, ambiguous responsibility and naming fitness;
- evidence binding to exact source SHA/change identity;
- explicit advisory authority boundary.

**Non-goals**:

- automatic mutation from semantic advice;
- automatic merge authorization;
- claiming AI review is deterministic business correctness.

**Acceptance**:

- unknown fields/categories/authority escalation fail closed;
- same finding can be tracked as new/existing/resolved;
- empty deterministic evidence cannot be expanded into mutation authority.

### Engineering-quality debt delta and waiver contract

**Goal**: prevent AI refactors from improving one area while silently introducing new reviewability debt elsewhere.

**Deliverables**:

- existing/new/fixed quality finding delta;
- blocking policy for accepted deterministic rules;
- explicit waiver schema with owner, reason, scope and expiry/review condition;
- Proof-of-Change / review-packet projection of the delta.

**Non-goals**:

- making every advisory AI finding blocking;
- silently grandfathering new debt;
- permanent waiver without ownership or review condition.

**Acceptance**:

- new blocking debt rejects conformance;
- unchanged historical debt remains visible but does not become new debt;
- fixed debt is recorded;
- expired/mismatched waiver fails closed.

### Framework and consumer rule propagation

**Goal**: make the quality baseline a normal constraint for Yunka itself and future generated/bootstrapped projects.

**Deliverables**:

- Yunka repository bootstrap references the authoritative quality rules;
- new project initialization/starter output carries an equivalent consumer-facing engineering-quality baseline or an explicit canonical reference appropriate for standalone consumers;
- generated project instructions distinguish normative rules from framework implementation history;
- cross-consumer qualification against Biz plus at least one independent consumer.

**Non-goals**:

- copying current Yunka task/status history into consumer repositories;
- consumer-specific rule forks;
- hidden remote dependency on the Yunka working tree.

**Acceptance**:

- a newly initialized project exposes the baseline before its first AI-authored source change;
- consumer rule text has stable semantic identity and deterministic version/provenance;
- Biz and a second consumer exercise the same policy without hardcoded repository names.

### Reviewability baseline for existing repositories

**Goal**: establish a bounded migration method for existing framework and consumer code without turning historical debt into an unbounded rewrite.

**Deliverables**:

- inventory of existing findings;
- touched-scope no-new-debt policy;
- bounded refactor recipe for generic aggregate files and task-named tests;
- prioritized real examples including `biz/internal/access/domain/model.go` while preserving behavior;
- explicit relationship to Domain ownership / coverage issue #191.

**Non-goals**:

- repository-wide rename in one change;
- changing business behavior merely to satisfy naming style;
- suppressing legacy debt by deleting checks.

**Acceptance**:

- baseline distinguishes existing from new debt;
- representative structural refactor proves behavior/API/persistence/generated delta is NONE;
- consumer qualification remains green after refactor.

## Dependency order

The implementation dependency is semantic rather than numbered:

```text
authoritative rules
    ↓
deterministic naming/docs/ownership checks
    ↓
change-map + review-packet contract
    ↓
structured semantic review
    ↓
quality debt delta / waiver
    ↓
framework + generated-project propagation
    ↓
bounded existing-repository migration
```

Issue execution may proceed in parallel only where the acceptance evidence is independent. No task identifier from this plan should become a durable code symbol or file identity.

## Completion condition

The overall initiative is complete only when:

- the authoritative rules are discoverable from repository instructions;
- deterministic checks cover the accepted objective subset;
- semantic AI review remains evidence-bound and advisory;
- non-trivial AI changes emit reviewable WHY / WHAT / BOUNDARY / PROOF evidence;
- engineering-quality debt is measurable as existing/new/fixed;
- new projects receive the baseline by default;
- at least two real consumers qualify the same policy without consumer-specific hardcoding;
- historical task names remain in GitHub/evidence history rather than becoming production-code identity.
