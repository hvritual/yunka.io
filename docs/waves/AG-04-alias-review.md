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

## Delivery boundary

Only the type-surface helper, its call site, regressions and this evidence record
change. No consumer runtime/source, dependency version, generated contract,
ordinary workflow, permission or acceptance gate is changed. The published
candidate must establish fresh exact-head CI, Production, both consumer matrices
and independent review. Non-force main integration and distinct actual-main
three-gate receipts precede the manual issue disposition. This record does not
assert its own unwritten commit or preclaim future test or merge results.
