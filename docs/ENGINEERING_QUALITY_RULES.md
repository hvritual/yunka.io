# Engineering Quality Rules

> Document class: **DECISION**  
> Authority: repository-wide human-reviewability, semantic naming, documentation, abstraction and AI-code quality baseline  
> Applies to: Yunka framework source, generated-project templates, and future Yunka consumer projects  
> Documentation governance: [`DOCUMENTATION_GOVERNANCE.md`](DOCUMENTATION_GOVERNANCE.md)

## Purpose

Yunka treats compilability, test success and task completion as necessary but insufficient evidence of engineering quality. Code produced or modified by AI must remain understandable, reviewable and maintainable by engineers who do not have the original task context.

A source artifact is acceptable only when a reviewer can determine from its package, file, type, function, comments, tests and change evidence:

- what business or technical responsibility it owns;
- why the abstraction exists;
- which invariants and side effects matter;
- which behavior changed and which behavior did not;
- how the change is proven.

Task history belongs in GitHub Issues, pull requests, commits and evidence records. It must not become the long-term identity of production code.

## Global rules

### 1. Production identities describe semantics, not task history

Production source identities MUST describe domain or technical responsibility. Historical task identifiers, delivery round names and migration labels MUST NOT become package names, production file names, exported symbols, operation IDs, database object names or durable test identities unless the term is itself a real business concept or a reviewed compatibility boundary.

Examples of prohibited identity leakage include names built around task/wave identifiers such as `CG-*`, `CE-*`, `B12`, `C10`, `round`, `wave`, `phase`, `stage`, `task`, `tmp`, `final`, `old` or similar historical labels when those labels describe delivery history rather than enduring semantics.

The rule is not a keyword blacklist. Conventional Go identities such as `NewTenant` are valid because `New` expresses constructor semantics rather than a delivery phase. Likewise, a term such as `Task` may be valid when it is a genuine persisted business concept; that case requires a scoped reviewed exception when it overlaps a deterministic delivery-history pattern.

Use semantic names such as:

```text
tenant_role_permissions_mysql_test.go
plan_catalog_mysql_test.go
tenant_isolation_pressure_test.go
```

instead of names whose primary meaning is when or in which delivery task the code was created.

### 2. Generic container names require explicit justification

Core business source SHOULD NOT use low-information container names such as:

```text
model.go
types.go
common.go
utils.go
helper.go
misc.go
manager.go
processor.go
data.go
```

when the file contains multiple independent concepts or its responsibility can be named directly.

Primary domain concepts should normally have a clear ownership boundary, for example:

```text
tenant.go
membership.go
role.go
permission.go
```

This is not a mechanical one-type-per-file rule. Cohesion and responsibility are authoritative; generic aggregation without a clear semantic boundary is the defect.

A deterministic checker may surface a generic container as an evidence observation when declaration structure provides objective evidence that several independent concept stems are aggregated. The observation is a review candidate, not proof that the file is wrong.

### 3. Names must identify responsibility

Functions, types and files MUST communicate what they do in domain or technical terms.

Avoid low-information names such as `Handle`, `Process`, `Execute`, `Do`, `Manager`, `Processor`, `Data`, `Info` or `Item` when a more precise responsibility exists.

Prefer names such as:

```text
ActivateTenant
SuspendMembership
ReplaceRolePermissions
PublishPlanVersion
SubscriptionUpgradeQuote
RolePermissionPolicy
```

### 4. Comments explain constraints, not syntax

Comments MUST add information that cannot be obtained by reading the declaration alone. Do not add comments that merely restate the name.

Comments are required when they explain one or more of:

- package responsibility and exclusions;
- exported API contract that is not obvious from the signature;
- business invariants;
- non-obvious technical decisions;
- failure semantics or side effects;
- concurrency, transaction, idempotency or security assumptions.

Example:

```go
// ReplacePermissions replaces the complete permission set of a tenant role.
// Owner roles must retain the permissions required to manage members and roles.
// Removing a required owner permission returns ErrProtectedOwnerRole and leaves
// the role unchanged.
```

Redundant comments such as `// CreateTenant creates a tenant.` do not satisfy this rule.

### 5. Core packages document purpose, boundary and invariants

A core business or framework package MUST provide package-level documentation that allows a new reviewer to understand:

- the package responsibility;
- the primary concepts it owns;
- important invariants;
- what the package intentionally does not own.

For Go packages, `doc.go` is the preferred durable location when package documentation would otherwise be difficult to discover.

### 6. Tests are executable business and technical documentation

Durable tests MUST be named after the behavior or invariant they prove, not the implementation task that introduced them.

Prefer:

```text
TestOwnerRoleCannotLoseRequiredPermissions
TestRemovedMembershipCannotBeReactivated
TestPlanCatalogPaginationDoesNotSkipPlans
```

Task IDs may remain in historical evidence, but they should not be the primary identity of long-lived regression tests. The production-file naming gate does not reject a `_test.go` file merely because its filename retains historical context; durable test function identities such as `TestCE09Replay...`, `BenchmarkWave2...`, `FuzzTask3...` or `ExamplePhaseMigration...` are checked independently.

### 7. Generated and developer-owned semantics remain separate

Generated files and handwritten behavior MUST NOT be mixed in a way that obscures ownership. Generated source must retain an explicit generated marker and canonical generated ownership. Developer-owned business behavior must remain outside generator overwrite scope.

A refactor for readability MUST NOT silently move business logic into generator-owned files or cause generators to overwrite handwritten semantics.

### 8. Abstractions must earn their existence

A new abstraction MUST have an explicit engineering reason. At least one of the following should normally be true:

- there are multiple implementations;
- it establishes an architectural boundary;
- it isolates an external dependency;
- it expresses an important domain concept;
- it owns an independent lifecycle or policy boundary.

Single-implementation interfaces, managers, factories, resolvers, coordinators, processors, adapters, registries or engines without a demonstrated boundary are review findings rather than default good design.

### 9. AI changes require a semantic change map before mutation

Before a non-trivial AI-authored modification, the change plan MUST identify:

```text
Problem
Current concepts / responsibilities
Desired ownership
Behavior change
Public API change
Persistence / database change
Generated-code change
Verification
```

The map is evidence for the change boundary, not a second source of truth. It may live in a transient change contract, issue, PR or generated review artifact.

### 10. Every change explains WHY / WHAT / BOUNDARY / PROOF

Review evidence for a non-trivial change MUST make these four facts explicit:

- **WHY** the change exists;
- **WHAT** is changing;
- **BOUNDARY** what is intentionally not changing;
- **PROOF** which checks or tests establish the result.

A commit or PR title such as `refactor model` is not sufficient review evidence by itself.

### 11. Human review receives a semantic summary before raw diff

AI-assisted delivery SHOULD produce a human review packet containing at minimum:

```text
Change intent
Affected domain / technical concepts
Behavior delta
Structural delta
Invariants affected
Public API delta
Database / persistence delta
Generated delta
Verification evidence
Known risks or unresolved findings
```

The packet supplements Git diff; it does not replace source review or tests.

### 12. Semantic findings are structured and replayable

AI semantic review findings MUST have stable machine-readable fields rather than free-form advice only. A finding should identify:

```text
path
symbol or scope
category
severity
reason
recommended action
whether behavior change is required
```

Later runs must be able to classify a finding as new, existing or resolved.

### 13. Engineering debt is measured as a delta

A change should not increase known human-reviewability or semantic-architecture debt without an explicit waiver.

At minimum, review evidence SHOULD distinguish:

```text
existing findings
new findings
resolved findings
```

`new findings > 0` is blocking when the corresponding rule is designated blocking. A waiver must state owner, reason, scope and expiry/review condition.

### 14. Deterministic checks and semantic review have separate authority

Deterministic tooling owns objectively checkable rules such as:

- prohibited historical naming patterns;
- missing package documentation;
- generated/manual ownership mixing;
- stale generated files;
- known dependency/layer violations;
- declared size/complexity thresholds;
- required change metadata presence.

AI semantic review may evaluate questions such as cohesion, abstraction quality, responsibility clarity and naming fitness, but MUST NOT silently gain mutation, merge or business-correctness authority from an advisory result.

### 15. Scope completeness is itself a checked property

A green check is meaningful only when the relevant source is inside the declared governance coverage. New or moved source must not become invisible merely because no rule currently owns the path.

Domain ownership / coverage closure is tracked separately in framework issue #191. This document's naming and reviewability rules consume developer-owned Go source without maintaining a consumer-specific allowlist; the broader source-coverage authority remains the fail-closed coverage model established by #191.

## Deterministic source-identity enforcement

Issue #193 establishes the first deterministic implementation of the naming rules above inside the existing `auditcore` finding and debt-delta model.

### `AUDIT-NAME-001` — historical delivery identity

`AUDIT-NAME-001` is a `proven_violation`. It evaluates developer-owned, non-generated Go source identities that can be determined without guessing business semantics:

- package identity;
- production Go filename identity;
- exported type, function, method, variable and constant identity;
- durable `Test`, `Benchmark`, `Fuzz` and `Example` identity.

The rule detects explicit task IDs and delivery-history prefixes such as `B12`, `C10`, `CG-07`, `CE-06`, `YU-14`, `EC-RI-04`, `Round2`, `WaveReplay`, `PhaseMigration`, `Stage3`, `TaskRunner`, `TmpStore`, `FinalMigration` and `OldAdapter` when they are the leading identity.

A finding includes stable finding ID, exact source path, exact symbol or source identity, reason, remediation and source/canonical evidence. Existing violations are handled by the normal debt baseline; only new proven violations become new debt.

### `AUDIT-NAME-002` — generic container cohesion observation

`AUDIT-NAME-002` is an `evidence_observation`, not a proven violation. A filename such as `model.go` or `types.go` is not sufficient evidence by itself. The deterministic checker only raises an observation when declaration structure objectively shows several exported type concept stems and no single concept dominates the file.

This observation is intentionally excluded from blocking debt delta. Human or AI semantic review decides whether the concepts are actually unrelated and whether a split improves ownership.

### Generated source

Source marked as generated by Go's generated-source convention is excluded from both naming rules. Generator-owned compatibility names must be fixed at their source contract or generator rather than by editing generated output.

### Scoped reviewed exceptions

A developer-owned identity that intentionally overlaps a deterministic historical pattern may declare a local exception only when it is a real business concept or a reviewed compatibility boundary. The exception is a Go documentation directive attached to the exact declaration or package/file scope:

```go
// Stage2State persists the external workflow vocabulary term Stage2.
// yunka:audit-name-exception business-concept Stage2 is part of the persisted public vocabulary.
type Stage2State struct{}
```

or:

```go
// yunka:audit-name-exception compatibility-boundary OldAdapter is required by the v1 public compatibility contract.
type OldAdapter struct{}
```

Accepted exception categories are exactly:

```text
business-concept
compatibility-boundary
```

A category without a non-empty reason is invalid and does not suppress the finding. There is no repository-global keyword allowlist, consumer-specific path exemption or silent waiver. An exception suppresses only the naming check at its declared scope and does not waive architecture, authorization, coverage or other findings.

## Required change-review contract

For framework and consumer changes, the minimum review contract is:

```text
Requirement / defect
    ↓
Current-code understanding
    ↓
Semantic change map
    ↓
Bounded implementation
    ↓
Naming / documentation / ownership checks
    ↓
Behavior and architecture verification
    ↓
Engineering debt delta
    ↓
Human review packet
    ↓
Human decision / normal delivery flow
```

Passing compilation and tests alone does not establish human-reviewable engineering quality.

## Enforcement model

These rules are normative immediately. Tooling may initially classify some rules as advisory while deterministic enforcement is implemented.

The implementation MUST preserve these authority boundaries:

- documentation rules do not invent business semantics;
- semantic review is advisory unless a deterministic accepted rule makes a finding blocking;
- generated artifacts remain derived evidence, not new writable Sources of Truth;
- no consumer-specific path list may become the framework's universal quality model;
- enforcement must be qualified against more than one real consumer before being called cross-project.

Open implementation work is tracked in the linked engineering-quality issues. A rule being documented here does not permit the repository to claim an enforcement mechanism that has not yet landed and qualified.

## Review checklist

A human or agent reviewing a change should be able to answer all of the following without reconstructing the original task conversation:

- Can I tell what each touched package/file/symbol owns?
- Do names describe enduring semantics rather than delivery history?
- Are comments present where invariants, side effects or non-obvious decisions require explanation?
- Are generated and handwritten ownership boundaries obvious?
- Are new abstractions justified by a real boundary or multiple implementations?
- Do tests describe durable behavior?
- Is the behavior/API/database/generated delta explicit?
- Is verification linked to the exact change?
- Did the change add new engineering debt?
- Can an engineer maintain this code after the original AI/task context is gone?

If the answer to a blocking item is no, the change is not yet a human-reviewable engineering artifact.
