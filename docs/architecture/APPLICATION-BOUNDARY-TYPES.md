# AG-04 — Typed factory and capability boundaries

> Class: CONTRACT / developer documentation
> Current delivery authority: [STATUS](../STATUS.md)
> Task: [#173](https://github.com/hvritual/yunka.io/issues/173)
> Exact qualification/review/integration: the task's delivery PR and workflow receipts

## Purpose and scope

The AG-02 Biz and AG-03 IoT pilots established two different needs: owning factories
must not become arbitrary business-call entry points, and a small interface must
not conceal an object with a wider method set. The typed audit checks these
explicit boundaries. It does not score DDD, infer business domains, execute
Applications, or modify the compiler/Executor/authorization/UoW.

The two reference sources are Biz `3519e7ee6e51e33984669871e4f32a55a3597d9f`
(TenantLifecycle) and IoT Delivery `bcd20632b666405c3f7a8fe8d53f591c78450087`
(saved views). Their runtime pins remain respectively `6ba99c1440dc...` and
`057ebcf88a87...`; running a current checker is not upgrading either runtime.
The existing consumer parser gates remain in place. This task supplies a reusable
opt-in type check, not full replacement of their behavioral/runtime tests.

## Public command

```sh
# Prepare approved dependencies using the project's normal bootstrap first.
yunka audit types --root ./my-module --policy ./boundary-policy.json --format agent-json
```

A relative policy path is resolved against `--root`. Flags: `--tags` selects one
active Go build profile; `--timeout` defaults to two minutes (maximum ten).
`--format text`, `json` and `agent-json` describe the same report.
Analysis fixes GOWORK=off, ignores inherited GOFLAGS/GOENV/package-driver hooks,
uses the checker's own Go installation, disables toolchain/module downloads and
loads with `-mod=readonly`. Cached dependencies and one valid Go module are
required. Missing dependencies/unsupported source never mean an empty clean audit.
It does not install tools or silently edit dependency files.

The command reports `PASS`, `FAIL`, or `INCOMPLETE`. Any incomplete required evidence
makes the overall state INCOMPLETE, even when other violations are already proven;
those proven findings remain in the report. Both non-pass states return nonzero.
Invalid policy/CLI input returns an error before analysis. A report includes the
policy digest, loaded source packages, parsed-source hashes, build scope, number
of checked factories/references, and sorted rule/location diagnostics.

Old `yunka audit`, debt schema, ChangeSet and Attestation behavior do not change.
This command does not authorize edits, count a migration as fixed, or approve a
merge. Integrating type findings into debt-growth/migration proofs belongs to AG-07.

## Explicit policy, derived facts

```json
{
  "schemaVersion": 1,
  "factories": [{
    "symbol": {"package": "example.com/shop/internal/orders", "name": "Build"},
    "allowedCallerPackages": ["example.com/shop/internal/bootstrap"],
    "results": [{
      "index": 0,
      "contract": {"package": "example.com/shop/internal/orders", "name": "Application"}
    }],
    "arguments": [{
      "index": 0,
      "contract": {"package": "example.com/shop/internal/orders/ports", "name": "Repository"}
    }]
  }]
}
```

Symbols are exact package-level Go function/type identities. Import aliases and
Go type aliases do not change identity. No wildcard scopes or guessed receiver
names are accepted. Method factories, type-parameter contracts, empty policies,
unknown/duplicate JSON fields, duplicate subjects/slots and missing declared
symbols do not silently pass. A policy selects one or more result/argument slots;
ordinary values/configuration outside those slots are not forced into interfaces.

`allowedCallerPackages` is explicit reviewed policy, not a claim that those packages
were automatically discovered to be safe. It permits factory references only in
those exact packages. The factory's own package has no implicit exemption. Broader
business semantics inside an allowed package still require review. This deliberate
policy input does not duplicate Application/Operation definitions or an architecture
graph; the actual methods, declarations and source references come from Go types.

Example policies in `tools/application-boundaries/biz.json` and `iot.json` constrain
the two real pilots. The IoT legacy composition facade remains an allowed package;
its unrelated wide Service is not declared fixed merely by this policy.

## Rules and evidence

| Rule | Evidence checked |
| --- | --- |
| AG-TYPE-001 | Selected factory referenced from a package not in its explicit composition policy |
| AG-TYPE-002 | Selected function slot does not declare the exact interface contract (aliases preserve identity) |
| AG-TYPE-003 | Actual value exposes exported methods beyond the contract, including promoted embedding and extra unwrapping methods |
| AG-TYPE-004 | A selected opaque capability's concrete representation exposes direct or promoted exported fields |
| AG-TYPE-005 | Factory function escapes a direct call; unsupported indirect-call provenance is INCOMPLETE |
| AG-TYPE-000 | Missing source/subject/type or unsupported/unresolved provenance; INCOMPLETE |

A small-interface conversion is followed to the original value; it does not reduce
that object's authority. Concrete types have fixed method sets. Interface-returning
constructors are followed through available module-local source bodies, tuple
results, direct argument binding, and single-initialization interface locals.
Private methods are not counted as publicly exposed capability methods. Promoted
fields are resolved using Go selectors: a private embedded value may expose a
public field, whereas an ambiguous selector is not treated as accessible.

The checker accepts independent non-embedded private wrappers. It follows actual
constructor implementations instead of merely believing their interface signature.
It does not inspect the business effects of a declared method: the selected
contract itself is an approved capability boundary, not automatically safe by name.
Extra public fields are forbidden only for selected opaque capability objects,
not arbitrary application/domain records.

## Explicit limits (not false success)

Return analysis supports ordinary constructors with declarations, assignments,
expression statements, blocks, if/error-return paths and direct returns. It prunes
constant boolean dead branches and stops after structurally terminating blocks/if
arms. Interface locals with multiple assignments,
address escape, unresolved dynamic constructors, generic/method/variadic interface
return summaries, recursion/depth overflow, unsupported constructor control flow,
and absent dependency bodies are INCOMPLETE when they affect selected evidence.
Reassignment is not treated as proof that an overwritten wide value was returned.

Only one source module and active build are loaded. Nested modules, the complete
OS/architecture/tag matrix, ignored/non-built source, universal dataflow, reflection,
unsafe and same-process adversarial isolation are not certified. These cannot be
turned into "zero violations in the entire repository" by reading a PASS report.
Source hashes cover parsed module source plus go.mod/go.sum; dependency source,
compiler provenance and policy authorship are not authenticated by an unkeyed digest.
Source is checked for drift, but the tool is not a filesystem lock or security sandbox.

`NewAnalyzer` is the go/analysis adapter used by analysistest. A single analysis pass
has only its own package body; missing cross-package constructor bodies remain
INCOMPLETE there. The public module command supplies all loaded source-package bodies.
No full-program SSA engine or second framework runtime is introduced.

## Permanent qualification

The normal Go tests include exact `analysistest` diagnostics, legal narrow wrappers,
wide-object casts, aliases, helper/tuple returns, exported state, factory handles,
unknown/reassigned/address-escaped interfaces, policy validation, public CLI exit
states, deterministic JSON and read-only loading. A nonzero compiler/download error
is not accepted as a successful architecture negative.

`ag04-consumer-types` is a persistent read-only workflow, with `contents: read` and
no retained checkout credentials. It checks out the two immutable consumer commits
and their original runtimes, builds the candidate checker and prepares dependencies.
`tools/qualify_ag04_consumers.py` then checks each real module twice, injects an actual
capability-widening mutant and unauthorized factory caller in disposable source,
requires the matching type rule, restores source, rechecks PASS and verifies clean
source/ref/tree. Artifacts bind checker Git identity, policy, source and outcomes.
This is real-consumer static qualification, not re-execution of consumer runtime/E2E.
The workflow includes its own file, checker, policy, script and dependency paths in
both PR and main triggers; it never commits, pushes, publishes a candidate or changes
permissions. Existing CI/Production remains required and unchanged.

Exact candidate tests, review disposition and eventual integration are separate
facts in the delivery PR. This document makes no self-referential qualification
claim. Removing this bounded opt-in feature can be reverted independently; neither
consumer implementation nor the earlier audit/ownership gates need be weakened.

## Mechanism sources

- Go type system and method sets: https://go.dev/ref/spec#Method_sets
- Pinned Go tools module (same v0.47.0 already present in repository dependency graph):
  https://github.com/golang/tools/tree/v0.47.0/go/packages
- Exact diagnostic testing:
  https://github.com/golang/tools/tree/v0.47.0/go/analysis/analysistest

Only the already locked dependency is used; no upstream runtime/architecture template
is copied into Yunka. Source APIs are used for typed facts, not for inferring business
roles from directory or method names.
