# Documentation Governance

> Document class: **CURRENT**  
> Authority: repository-wide documentation classification and truth-ownership rules  
> Current project status authority: [`docs/STATUS.md`](STATUS.md)

## Purpose

Yunka keeps implementation plans, migration records, release evidence, durable architecture decisions, and current developer documentation in the same repository. Those artifacts serve different purposes and must not be interpreted as interchangeable sources of current truth.

This document defines how documentation is classified, which artifact owns each kind of fact, how historical records are preserved, and how humans and agents should resolve apparent conflicts.

## Core rules

1. **One fact, one current owner.** A current fact must have one designated documentation authority; other documents may link to it but must not maintain competing current-state copies.
2. **Executable evidence outranks narrative claims.** Code, generated artifacts, verification gates, and exact qualification evidence determine whether a behavioral or release claim is valid. Documentation must be reconciled to that evidence rather than redefine it.
3. **Historical records are preserved, not rewritten into the present.** A completed roadmap or wave may retain the planning state that existed when it was authored. Add an explicit classification/status banner instead of rewriting historical planning prose as if it had always described the final state.
4. **Current documents must describe the current repository.** `README.md` and other `CURRENT` documents must not present completed migration steps, deleted mechanisms, or superseded paths as still pending.
5. **Status is centralized.** [`docs/STATUS.md`](STATUS.md) is the repository documentation authority for the current framework/wave/release state.
6. **Durable memory is not a task tracker.** `PROJECT_MEMORY.md` records active governance and architecture invariants. It may point to `docs/STATUS.md`, but transient task progress and duplicated wave tables do not belong there.
7. **Evidence remains scoped to an exact candidate.** Qualification/release evidence proves the exact SHA/tree and consumer pair it names. Later repository state may inherit that evidence only when the relevant semantics are unchanged and the later release record explicitly establishes that relationship.

## Document classes

| Class | Purpose | Can answer "what is true now?" | Typical examples |
| --- | --- | --- | --- |
| `CURRENT` | Current product behavior, developer workflow, authoring guidance | Yes, for its owned subject | `README.md`, current authoring guides |
| `STATUS` | Current framework/wave/release and deferred-pressure state | Yes, authoritative for status | `docs/STATUS.md` |
| `DECISION` | Durable governance/architecture invariants and decisions | Yes, while active | `PROJECT_MEMORY.md`, future ADRs |
| `HISTORICAL` | Roadmaps, migration plans, implementation-wave records | No, unless an explicit current-status banner says otherwise | most `docs/waves/**` planning records |
| `EVIDENCE` | Exact qualification, release, CI/consumer proof | Only for the exact evidence scope | release/qualification wave records |

A document may contain historical background while remaining `CURRENT`; the decisive rule is whether its normative claims are intentionally maintained against the current repository.

## Truth ownership

| Fact type | Documentation authority | Supporting evidence |
| --- | --- | --- |
| Repository/Git workflow | `AGENTS.md` | repository state / Git refs |
| Current framework or wave status | `docs/STATUS.md` | merged refs, issues/PRs, qualification records |
| Durable architecture/governance invariants | `PROJECT_MEMORY.md` | code, architecture gates, release evidence |
| Current developer-facing behavior | `README.md` and relevant `CURRENT` authoring docs | CLI/code/tests on current tree |
| Exact release/qualification result | named `EVIDENCE` record | exact SHA/tree, CI jobs, real-consumer evidence |
| Original intent, sequencing, migration rationale | `HISTORICAL` roadmap/wave record | Git history, issue/PR discussion |

If a document outside the authority column disagrees with the authority for that fact type, treat the non-authoritative statement as stale or historical and reconcile it in a documentation-governance change.

## Classification banner

Historical roadmaps and other documents whose body can be misread as current state should carry a compact banner immediately below the title. Recommended form:

```markdown
> Document class: **HISTORICAL**  
> Delivery state: **Complete / qualified / merged**  
> Current status authority: [`docs/STATUS.md`](../STATUS.md)  
> Historical note: the original planning/status section below is preserved as authored and is not current status truth.
```

When useful, add the qualified head, merge commit, release record, or superseding document. Do not edit old planning prose merely to make every historical sentence read as though it were written after completion.

## README rule

`README.md` is a `CURRENT` developer/product entry point. It must:

- describe mechanisms and commands that exist on the current tree;
- avoid language that says an already-landed migration is still pending;
- link to `docs/STATUS.md` for current project/wave status rather than duplicate a full status ledger;
- use historical wave documents for background, not as the authority for current behavior;
- be reconciled whenever a release removes or replaces a developer-visible path.

## Wave and roadmap rule

`docs/waves/**` contains both historical plans and evidence records. The directory name alone does not make a file current.

- Planning roadmaps become `HISTORICAL` when their delivery wave closes.
- Release/qualification records should be treated as `EVIDENCE` for their exact candidates.
- A historical roadmap may keep `Planned`, `In progress`, unchecked tasks, proposed sequencing, and other original planning text if an explicit banner prevents that text from being mistaken for current status.
- New active wave documents should state their class and link to `docs/STATUS.md`; closing the wave must reconcile `docs/STATUS.md` before the roadmap becomes historical.

## Status update rule

Update `docs/STATUS.md` when any of the following occurs:

- a framework/wave becomes active, qualified, complete, superseded, or merged;
- a release candidate becomes the current merged baseline;
- a pressure item changes between open, deferred, blocking, or closed in a way that affects current planning;
- the canonical developer workflow changes;
- a later merge changes which evidence is the current release baseline.

Status changes must cite or name the exact Git/qualification evidence in the document itself. Do not claim qualification from prose alone.

## PROJECT_MEMORY rule

`PROJECT_MEMORY.md` remains the durable current-state decision memory. It should contain:

- active repository workflow/governance decisions;
- architecture boundaries that future work must preserve;
- durable qualification/release constraints when they remain architectural inputs;
- the documentation-governance hierarchy itself.

It should not become a chronological task log or duplicate the complete current wave table from `docs/STATUS.md`.

## Human and agent reading order

For a new repository task, after the mandatory repository bootstrap in `AGENTS.md`:

1. read `PROJECT_MEMORY.md` for durable active invariants;
2. read `docs/STATUS.md` for the current delivery/release baseline;
3. read `README.md` or the relevant `CURRENT` authoring guide for current user-facing behavior;
4. read relevant `EVIDENCE` records when a qualification/release claim matters;
5. read `HISTORICAL` roadmap/wave documents for intent, rationale, sequencing, and prior constraints without treating their original status fields as current truth.

## Reconciliation checklist

A documentation-governance change is complete only when:

- the target document class is unambiguous;
- current facts have one documented owner;
- historical evidence has not been silently rewritten;
- `README.md` contains no known contradictory current/migration statements in the touched scope;
- `docs/STATUS.md` agrees with merged Git state and qualification evidence;
- `PROJECT_MEMORY.md` contains only durable governance/architecture facts;
- links to current authority are valid;
- no runtime/compiler/security/transaction semantics were changed merely to make documentation consistent.
