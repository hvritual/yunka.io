# AG-04 — Exported unnamed-alias capability correction

> Class: EVIDENCE / bounded implementation record
> Current status: [STATUS](../STATUS.md)
> Contract: [APPLICATION-BOUNDARY-TYPES](../architecture/APPLICATION-BOUNDARY-TYPES.md)
> Exact candidate checks, independent review and integration: [PR #174](https://github.com/hvritual/yunka.io/pull/174), [issue #173](https://github.com/hvritual/yunka.io/issues/173)

## Proven defect and historical evidence

Candidate `4b8d7e11732dc5a03ce6ffd6ef510b8c23a3b90c`, tree
`f25f8f155050c4abde8c081705eed236144fb05a`, passed CI `34291549254`,
Production `34291549269` and the two-consumer type gate `34291549256`.
Independent review then reported comment `3963310863`: a public alias can name
an unnamed structural value, not only a named Go type. That candidate was not
merged; its successful checks remain historical evidence for its exact source,
not a qualification of the subsequent correction.

The counterexample is `type Public = struct{ hidden }`, where hidden has value
method Read and pointer method Delete. Returning Public{} through Reader does
not remove the caller's ability to assert owner.Public, assign to an addressable
local and invoke the promoted Delete. The previous early rejection of non-Named
targets missed that public surface.

## Correction and regression contract

Method-surface selection now preserves the public alias identity and inspects
exported aliases before deciding a concrete value is unnameable. Alias chains
and identical unnamed literals are recognized. Publicly nameable non-pointer
values include the pointer method set. Interfaces and existing pointers retain
their own method sets; no pointer-to-interface or pointer-to-pointer authority
is invented. Private aliases without a public spelling remain legal.

Explicitly instantiated public generic aliases retain their public spelling.
Arbitrary generic-alias instantiation is not solved by this bounded checker.
When a possible public generic alias could name the value and its pointer adds
exported methods, unresolved nameability is INCOMPLETE rather than a false PASS
or an unproven violation. Unrelated generic aliases and genuinely private values
remain legal. This limitation concerns selected capability evidence, not all
unrelated generic code in a module.

Eleven subcases across two permanent test groups cover public unnamed aliases,
alias chains, literals with a public alias, private aliases/literals, pointer
aliases, unrelated generic aliases, explicit public generic aliases, unresolved
structural/named generic spellings and private generic values. Each negative
requires the exact AG-TYPE-003 or AG-TYPE-000 diagnosis; positives remain PASS.
The original named-value, interface, opaque-field, source-coverage, toolchain and
termination regressions remain unchanged.

## Precision correction: unrelated generic structures must remain legal

Candidate `3c708f47310bc5dc2d34858a27973b1a0bb6849d`, tree
`4e75c1d1c9e05b1753d7e7cc65f44fc4628a1526`, passed CI `34293358010`,
Production `34293357978` and consumer types `34293358023`. Review comment
`3963405591` then showed that merely sharing the struct kind was too broad:
`Public[T] = struct{ Value T }` cannot name `struct{ hidden }`, and must not
block that private representation. That candidate also remained unmerged.

The possibility check now compares field count, identity, embedding and tags,
then structural field types, named origins, repeated type arguments, containers,
function signatures and interface methods. Incompatible shapes are ruled out.
Known arguments must satisfy non-dependent constraints; when all alias arguments
are inferred, Go's own `types.Instantiate(..., validate=true)` checks all
constraints, including dependent ones, and exact resulting identity. Unknown or
unused arguments remain unresolved rather than guessed. The check is bounded;
its role is to rule out impossible public spellings, not solve arbitrary generics.

Seventeen further subcases cover mismatched names/arity/embedding/tags, repeated
argument conflicts, container/function/interface differences, possible shapes,
known and dependent constraint failures, and genuinely unresolved arguments.
Unrelated or impossible aliases produce no finding; possible unresolved public
pointer authority remains AG-TYPE-000/INCOMPLETE. The earlier eleven alias cases
and all pre-alias regressions remain in the same ordinary test suite.

## Delivery boundary

Only the type-surface helper, its call site, regressions and this evidence record
change. No consumer runtime/source, dependency version, generated contract,
ordinary workflow, permission or acceptance gate is changed. The published
candidate must establish fresh exact-head CI, Production, both consumer matrices
and independent review. Non-force main integration and distinct actual-main
three-gate receipts precede the manual issue disposition. This record does not
assert its own unwritten commit or preclaim future test or merge results.
