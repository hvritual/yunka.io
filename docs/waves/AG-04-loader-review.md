# AG-04 — Loader and termination review corrections

> Class: EVIDENCE / bounded implementation record
> Current status: [STATUS](../STATUS.md)
> Contract: [APPLICATION-BOUNDARY-TYPES](../architecture/APPLICATION-BOUNDARY-TYPES.md)
> Exact candidate, qualification, review and integration receipts: [PR #174](https://github.com/hvritual/yunka.io/pull/174), [issue #173](https://github.com/hvritual/yunka.io/issues/173)

## Why another candidate is required

Documentation candidate `0b0e8ffadaa80bb3403ea900a6307c7489f73446`, tree
`f9a915d43e1f7287da28df18b9b302c371c512e7`, passed CI `34289291273`,
Production `34289291276` and pinned-consumer types `34289291262`.
Independent review then identified three additional defects. Those workflow
results remain valid evidence for the named candidate, not proof of defect
freedom or qualification of the corrections below. Main was not advanced on
that review, and issue #173 was not closed.

## Active imported source is not just the wildcard roots

Review comment `3963145500` demonstrated an unauthorized factory reference in an
imported `_hidden` package. The `./...` wildcard omitted that package as a root,
even though it participated in the active import graph. Hashing its source
without analyzing its references was not complete type evidence.

The loader now inventories import/dependency metadata, selects every package
belonging to the chosen source module, and promotes that exact sorted package
set to typed roots. Missing/error/duplicate/changed package identity remains
INCOMPLETE. External dependency bodies are not indiscriminately added. Test-only
or inactive entries remain explicit exclusions; an absent selected policy
subject cannot pass. Regression pairs cover `_hidden`, `.private`,
`testdata/helper` and nested ignored-directory imports: unauthorized references
must produce AG-TYPE-001, while explicitly permitted callers must pass with the
reference actually counted. This does not certify unimported ignored source,
additional modules or the AG-05 build matrix.

## Loader Go identity must match the reported toolchain

Review comment `3963145507` proved that pinned x/tools resolves `exec.Command("go")`
from the parent process PATH before assigning the child environment. Setting only
`packages.Config.Env` cannot guarantee the selected Go executable.

Before loading, the checker resolves the canonical absolute
`runtime.GOROOT()/bin/go` (or `go.exe`), requires the parent PATH resolution to be
the same regular file, and queries the version through the absolute executable.
A wrapper or different toolchain earlier on PATH is rejected as INCOMPLETE
before execution; put the checker's GOROOT/bin first on PATH to use the command.
The check is repeated around loading. No global PATH mutation, silent toolchain
substitution or dependency download is introduced. This is a fail-closed identity
precondition, not a filesystem lock against concurrent external mutation.

A real compiled forwarding wrapper writes an invocation marker if run. The
regression requires INCOMPLETE and an absent marker; the canonical-Go positive
must still pass. This distinguishes rejection of a wrong executable from merely
printing a preferred version in the report.

## Built-in panic terminates return provenance

Review comment `3963145511` demonstrated a named-result assignment after
`panic("stop")` being mistaken for reachable implementation evidence. The return
walker now identifies the actual Go built-in by type identity, stops at its
terminating expression, and handles terminating if initializers. No reachable
non-nil implementation evidence means INCOMPLETE, never PASS.

Regression cases include named/direct returns, parenthesized built-in calls,
if initializers, shadowed ordinary functions named `panic`, dead panic branches,
and genuine non-panic return paths. The latter legal cases are not rejected based
on spelling. This does not claim arbitrary interprocedural nontermination analysis.

## Delivery boundary

All new tests use the existing ordinary package/race and pinned-consumer checker
test paths. No consumer source, runtime, Executor/Authz/UoW, dependency lock,
ordinary workflow or permission policy changes. The published correction must
receive fresh exact-head CI, Production, both consumer matrices and independent
review. Only then may non-force main integration occur; its separate actual-main
three-gate receipts and final readback precede issue closure. This document does
not certify its own unwritten commit or preclaim those future results.
