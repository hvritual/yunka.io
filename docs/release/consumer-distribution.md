# Consumer distribution qualification

> Document class: **CURRENT**
> Current delivery state: [STATUS.md](../STATUS.md)

## Problem and bounded change

The cs-biz F0 consumer review found that the default v0.1.0 release probe requests an unpublished infras tag. A clean `GOWORK=off go build -mod=readonly ./cmd` also fails because the current CLI imports contract/diagnostic APIs absent from its declared pkg v0.1.0 dependency. A workspace build masks that dependency mismatch.

Missing a tag does not prove a module cannot be distributed. Go resolves all four modules at the reviewed source commit through its ordinary module protocol, including checksums and immutable pseudo-versions. These versions are development versions with no new API stability promise. They are external module downloads, not a consumer-vendored framework snapshot, local replace, or workspace dependency. Existing v0.1.0 tags remain unchanged.

The semantic change map is:

| Field | Scope |
| --- | --- |
| Problem | Workspace hides stale independent module requirements; no qualified CLI bundle/consumer lock |
| Current owners | go.mod owns each module graph; Go module distribution owns version/checksum identity; canonical CLI owns generation/check; core.App owns lifecycle |
| Desired ownership | Keep those owners; bind a tested four-module combination in tools/consumer-release-lock.json and emit a CLI artifact manifest |
| Behavior | Independent module builds use the reviewed external package graph; CLI --version identifies the artifact build |
| Public API | No SDK/runtime API changes; CLI release version replaces the hard-coded 0.0.1 |
| Persistence / database | None |
| Generated code | No hand edits; qualification generates only in disposable external consumers |
| Verification | Download checksums/source commits; no replacement/workspace; build/test/vet; actual packaged CLI generation twice, read-only check and drift rejection; normal CI/Production and applicable gates |

## Qualification and artifacts

The release lock records one evaluated SDK source commit and the exact Go-resolved version, module sum and go.mod sum for pkg/framework/gateway/infras. Updating this lock requires fresh combination qualification. Pin all four, because the independently downloaded SDK manifests may declare older minimum dependencies; do not assume one module's minimum version selects the whole evaluated train.

The CLI bundle contains its executable, canonical protobuf includes, any repository-root licensing files present, SDK lock and a manifest of source commit/tree, toolchain, binary build information and file SHA256. The manifest reports the licensing-file inventory, including an empty inventory; this tooling does not invent a license or grant. Source build happens in the producer only. The qualification consumer receives the bundle and versioned modules, with GOWORK=off and no replace directives. A candidate artifact is not a GitHub release or a cs-biz runtime qualification.

Publication, semantic-version tags and promotion remain separate actions after exact-candidate gates. This task does not complete upstream #234 (whose #188 dependency remains open), migrate Biz/IoT/cs-biz applications, introduce a third domain, change Executor/Authz/UoW, or deploy anything. The receiving cs-biz F1 task must qualify its own contracts and operations against the resulting immutable artifact lock.

## Toolchain installation condition

The existing source/type audit security contract trusts the checker's compiled-in Go installation and rejects GOROOT overrides. Therefore this bundle preserves that compiled-in root (no `-trimpath`) and records `requiredGoRoot` in its manifest. The receiver must have the exact locked Go installation at that path, put its bin directory first on PATH, and leave GOROOT unset. The supported cloud-runner installation is supplied by the pinned setup-go action. This is a toolchain condition, not permission to reference a producer checkout or local SDK source.

Qualification runs the **packaged** `audit source` on the independent consumer as well as canonical generate/check; unavailable trusted tools remain failures. This artifact does not promise arbitrary installation-path portability. Supporting a different trusted toolchain discovery policy would be a separate reviewed security decision; this repair preserves the current rejection rules.
