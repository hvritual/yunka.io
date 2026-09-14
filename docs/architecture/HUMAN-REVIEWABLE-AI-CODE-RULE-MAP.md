# Human-Reviewable AI Code Rule Map

> Document class: **CURRENT**  
> Authority: mapping from normative engineering-quality rules to implementation work  
> Normative rule authority: [`../ENGINEERING_QUALITY_RULES.md`](../ENGINEERING_QUALITY_RULES.md)

This document exists only to keep implementation work aligned with the global rule baseline. It must not duplicate or override the normative rules.

| Rule area | Required framework capability | Delivery evidence |
| --- | --- | --- |
| Semantic production identities | naming diagnostics and durable-source identity checks | framework fixtures + two-consumer qualification |
| Generic container names / concept ownership | deterministic candidate detection plus bounded semantic review | stable finding IDs + real consumer examples |
| Package purpose / invariant comments | package-doc and governed-comment checks | authoring fixtures + generated/manual ownership exclusions |
| Durable behavior test names | test-identity checks | framework fixtures + consumer regression inventory |
| Generated vs handwritten ownership | ownership-aware quality checks | canonical generator/ownership evidence |
| Abstraction justification | structured advisory semantic finding | evidence-bound review response |
| Semantic change map | exact change metadata bound to Git/change contract | deterministic replay |
| WHY / WHAT / BOUNDARY / PROOF | review packet projection | exact candidate receipt |
| Human review packet | semantic summary before diff | deterministic artifact + digest |
| Structured semantic findings | stable advisory schema | fail-closed validation |
| Engineering debt delta | existing/new/fixed quality findings | Proof-of-Change integration |
| Waivers | owner/reason/scope/expiry contract | fail-closed expiry/mismatch tests |
| Scope completeness | reuse canonical ownership/coverage closure | framework issue #191 + cross-consumer evidence |
| Future-project propagation | init/starter policy distribution | new-project fixture + two-consumer qualification |

## Constraint

No implementation task may use its issue number, delivery round name or temporary work label as a new production package/file/symbol/test identity. GitHub/evidence references remain allowed because they are delivery history rather than production semantics.
