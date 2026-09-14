# Domain Governance Coverage

> Document class: **CURRENT**  
> Authority: Domain compiler ownership coverage and explicit exemption semantics  
> Engineering-quality baseline: [`ENGINEERING_QUALITY_RULES.md`](ENGINEERING_QUALITY_RULES.md)

## Purpose

Yunka must not treat the absence of `domain.json` as proof that a domain-like source tree is intentionally outside framework ownership. A green check is meaningful only when every discovered Domain compiler surface has an explicit ownership disposition.

Managed ownership is derived from existing canonical source evidence; Yunka does not introduce a second list of managed domains.

## Ownership states

A direct child below the configured generated-Go root is considered an unmanaged Domain compiler candidate only when it contains the canonical `domain/` source anchor. Other architecture directories such as `application`, `ports`, `infrastructure`, `transport`, or `policy` are collected as supporting topology signals after that anchor is present.

This distinction is deliberate. Yunka's contract/application compiler can legitimately generate `application`, `policy`, or `transport` trees for a domain that is not owned by the Domain compiler. Those trees must not be converted into Domain ownership implicitly merely because their directory names look architectural.

Each Domain compiler surface has exactly one accepted disposition:

```text
domain.json present
  -> MANAGED

domain/ anchor present + no domain.json + exact exemption
  -> EXEMPT

domain/ anchor present + no domain.json + no exemption
  -> UNKNOWN
  -> UNMANAGED_DOMAIN_TOPOLOGY
  -> check fails
```

Ordinary internal packages and application/compiler output without a `domain/` anchor are not automatically classified as Domain compiler surfaces.

## Managed domains

`domain.json` remains the canonical ownership fact for a Domain compiler managed domain. Managed identity is never copied into another writable inventory.

A managed domain remains subject to the existing deterministic checks for:

- persistence-only Domain Manifest compatibility;
- developer-owned PO contract;
- canonical entity-owned generated files;
- repository/GORM generated artifacts;
- stale generated files;
- exact generated-content drift.

An exemption for a domain that already contains `domain.json` is a configuration conflict and is rejected.

## Explicit exemptions

Intentional unmanaged Domain compiler surfaces are declared in the project-owned exception register:

```text
.yunka/domain-coverage.json
```

Example:

```json
{
  "schemaVersion": 1,
  "exemptions": [
    {
      "domain": "legacy",
      "reason": "migration is tracked separately",
      "owner": "platform",
      "expires": "2026-12-31"
    }
  ]
}
```

Rules:

- `domain` names exactly one direct child of the checked root;
- the target must be an unmanaged Domain compiler surface with a `domain/` anchor;
- `reason` is mandatory;
- `owner` and `expires` are optional metadata;
- exemption string fields must be canonical and cannot contain leading or trailing whitespace;
- `expires`, when present, uses `YYYY-MM-DD`;
- duplicate exemptions are rejected;
- an exemption that no longer matches an existing unmanaged Domain compiler surface is rejected as stale;
- exemptions do not disable unrelated source, architecture, security, generated-code, or runtime checks.

The exception register owns only exemptions. It does not duplicate `domain.json` or maintain a second managed-domain list.

## Generation and checking

Project-level Domain generation and checking both perform coverage validation before acting on managed domains. The lower-level Go `domain.Check` API uses the same coverage closure, so callers cannot bypass ownership validation by avoiding the CLI/project wrapper.

An `UNKNOWN` Domain compiler surface therefore blocks both:

```text
yunka generate
yunka check
yunka domain check
```

The check is read-only. It never creates a manifest or exemption automatically because doing so would convert an unresolved ownership decision into framework-owned intent.

## Migration guidance

When `UNMANAGED_DOMAIN_TOPOLOGY` is reported, choose one explicit disposition:

1. adopt the source tree as a managed Domain and create/migrate the canonical `domain.json` through the supported Domain authoring flow; or
2. record a reviewed exemption with a concrete reason while a separate migration remains outstanding.

Do not add a blanket exemption merely to restore a green check. In particular, do not exempt an application-only compiler output merely because it lives under `internal/<name>`; without a `domain/` anchor it is outside this Domain compiler coverage contract.

## Relationship to engineering quality

Domain coverage is a prerequisite for meaningful naming, documentation and architecture-quality checks. A rule cannot claim Domain compiler coverage while a source tree containing domain models remains invisible to the framework.

Engineering-quality checks therefore consume this same ownership closure rather than maintaining consumer-specific path allowlists.
