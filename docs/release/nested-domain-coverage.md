# Nested Domain source coverage

> Document class: **EVIDENCE**
> Current status authority: [STATUS](../STATUS.md)
> Exact candidate and qualification: task-branch Git, PR and original CI artifacts

WHY / Problem: cs-biz's registered handwritten interaction domain stops matching
the direct-child inventory after moving to internal/modules/interaction. Removing
its exemption instead produces a false pass. The original failing native CI and
five Linux directory controls remain on cs-biz PR #82.

Current responsibility: domain coverage derives identities and generation scope
from domain.json or an anchored domain/ tree. Check and generated-artifact audit
previously repeated a direct-child walk, allowing their physical scope to diverge.

WHAT / Desired ownership: one canonical surface discovery traverses grouping
directories, stops at each domain owner, and supplies the same roots to coverage,
managed generation/check, generated-artifact audit and generated-path ownership.
Existing logical directory-name identities and exemption schema remain unchanged.
Duplicate identities cannot use one exemption to cover two physical domains.
An ordinary domain's child packages stay inside that owner's source tree.

Behavior change: previously invisible nested domains become visible and must
satisfy the same ownership and generated checks as flat domains. Directory aliases
and nested Go modules fail explicitly. Flat-domain rendering is unchanged.

Public API change: no runtime or SDK API change and no new configuration registry.
CLI coverage now includes nested roots; no automatic runtime module registration
or business implementation is inferred from directory grouping.

Persistence / database change: none. Generated-code change: no generated files
are hand-edited; nested managed output is still produced by the canonical
renderer from its actual Go import path. Existing layouts retain their output.

BOUNDARY: no runtime, authorization, transaction, SDK source, protobuf DSL,
business contract or exemption-authority changes. The existing approved compiler
upgrade candidate remains separate from the new discovery semantics. This is a
candidate distribution, not a tag publication or main integration claim.

PROOF: mixed flat/nested handwritten coverage, unregistered nested rejection,
duplicate identity rejection, nested managed generate/check/drift/ownership,
directory alias rejection and nested-module rejection must run on Linux. Existing
CI, Production/MySQL, real Biz/IoT coverage, source/type/template and independent
packaged-consumer qualification remain required. The downstream lock changes
only after producer commit/tree, archive, manifest, SDK and toolchain identities
are checked against the newly qualified artifact. Windows checks are reported
separately; a host unable to create symbolic links cannot establish that case.

Engineering debt disposition: the known nested-coverage blind spot is the repair
target. New deterministic blocking debt must remain zero; unrelated historical
debt is not rewritten. Self-check and automated gates are not independent approval.
