# Security toolchain baseline qualification

Document class: EVIDENCE. Issue: [#244](https://github.com/hvritual/yunka.io/issues/244).

## Decision and scope

Human approved the proposed Go 1.26.9 + golang.org/x/net v0.60.0 baseline
with the raw response `批准`, received 2026-10-15 19:39:44 +08:00. This replaces
the earlier Go 1.25.13 condition for the cs-biz route-A prerequisite. It does
not approve the complete cs-biz design, waive security or Human Full, publish
a release, or deploy production. Original cs-biz sealed rounds remain intact.

## Semantic change map

- Problem / WHY: exact producer candidate 28ec46df6b649edb8e724e126ad8968a10bd79ad
  fails current security scans on Go 1.25.13 and x/net v0.58.0.
- Current responsibilities: tools/toolchain.env owns exact tool versions;
  module graphs and dependency policy own dependency selection; doctor and CI
  consume the same supported baseline.
- Desired ownership / WHAT: retain those owners and advance their baseline to
  Go 1.26.9 and x/net v0.60.0 using canonical Go module tooling.
- Behavior: compiler/standard-library and network dependency security fixes;
  doctor fallback and CI must use the approved compiler.
- Public API / persistence / generated delta: NONE intended. Runtime, Executor,
  Authz, root UoW and business protobuf contracts remain unchanged.
- BOUNDARY: protoc 3.21.12, RPC plugin pins, scanner version and gates remain.
  Preserve go 1.25.0 language directives only if the dependency graph permits.
- PROOF: exact candidate and actual-main standard CI, real MySQL Production,
  branch baseline and applicable source/type/template gates; fresh all-module
  security scans. Distribution PR #243 must subsequently refresh safe module
  versions, sums and CLI artifact and run its independent consumer qualification.
- Preview demo: N/A; no frontend or user journey behavior changes.

## Retained baseline failure

Canonical source: 3527488325030fbf86fe52804140c936e9d937a4.
CI runs 38725755586 / 38726530372 and Production 38725755547 / 38726530297
failed at make vuln; MySQL integration was not reached. The unchanged pkg
subtree 24edaed2bd58b4160429f37f1d5aa40927bb8450 independently reproduced
reachable GO-2026-6603, 6605, 6607, 6608, 6610, 6611, 6612, 6613, 6617.
The JSON scanner emits findings independently of exit status; those retained
findings are not represented as a passing text-mode gate.

## Qualification state

BASELINE QUALIFIED / MERGED through PR #245. Accepted source bc7c0e1dd6b4e815951130c7acfbf60371445453, tree 4e3ae46845f96f520ef913bc956f6a5f458ff144, passed six candidate gates and separate actual-main CI 38730752551, Production/MySQL 38730752561, branch baseline 38730752487, consumer types 38730752460, source policy 38730752562 and template 38730752505. Distribution/artifact qualification receipts are recorded separately on PR #243; the earlier security-source receipt does not substitute for them.
Exact remote commit, run IDs and integration readback belong to issue #244
and its linked PR. Prior Go 1.25.13 distribution artifacts remain unqualified
for security and cannot be promoted by this decision alone.

## Dependency compatibility readback

x/net v0.60.0 declares go 1.26.0 and requires x/crypto v0.57.0,
x/sys v0.48.0 and x/text v0.42.0. Canonical go get/tidy consequently advances
the five module language directives and workspace to go 1.26.0. Retaining
go 1.25.0 is incompatible with that approved fix. Go MVS also selects
x/mod v0.41.0, x/sync v0.23.0 and x/tools v0.49.0 where required.
Official Go linux/amd64 archive SHA-256:
42d158b4d8f7b61ac0a830567c940a86098fb7aac52e467a5ebec03ef5cc2f8d.
