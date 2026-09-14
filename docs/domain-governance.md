# Domain Governance Coverage

> Document class: **CURRENT**  
> Authority: Domain compiler ownership coverage and explicit exemption semantics  
> Engineering-quality baseline: [`ENGINEERING_QUALITY_RULES.md`](ENGINEERING_QUALITY_RULES.md)

## Purpose

Yunka must not treat the absence of `domain.json` as proof that a domain-like source tree is intentionally outside framework ownership. A green check is meaningful only when every discovered domain-like surface has an explicit ownership disposition.

Managed ownership is derived from existing canonical source evidence; Yunka does not introduce a second list of managed domains.

## Ownership states

A direct child below the configured generated-Go root is considered domain-like when it contains one or more canonical domain-architecture directories such as `domain`, `application`, `ports`, `infrastructure`, `transport`, or `policy`.

Each domain-like surface has exactly one accepted disposition:

```text
domain.json present
  -> MANAGED

no domain.json + exact exemption
  -> EXEMPT

no domain.json + no exemption
  -> UNKNOWN
  -> UNMANAGED_DOMAIN_TOPOLOGY
  -> check fails
```

Ordinary internal packages that do not expose those domain-architecture signals are not automatically classified as Domain compiler surfaces.

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

Intentional unmanaged domain-like surfaces are declared in the project-owned exception register:

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
- `reason` is mandatory;
- `owner` and `expires` are optional metadata;
- exemption string fields must be canonical and cannot contain leading or trailing whitespace;
- `expires`, when present, uses `YYYY-MM-DD`;
- duplicate exemptions are rejected;
- an exemption that no longer matches an existing unmanaged domain-like surface is rejected as stale;
- exemptions do not disable unrelated source, architecture, security, generated-code, or runtime checks.

The exception register owns only exemptions. It does not duplicate `domain.json` or maintain a second managed-domain list.

## Generation and checking

Project-level Domain generation and checking both perform coverage validation before acting on managed domains. The lower-level Go `domain.Check` API uses the same coverage closure, so callers cannot bypass ownership validation by avoiding the CLI/project wrapper.

An `UNKNOWN` surface therefore blocks both:

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

Do not add a blanket exemption merely to restore a green check. The purpose of the contract is to make previously invisible source ownership reviewable.

## Relationship to engineering quality

Domain coverage is a prerequisite for meaningful naming, documentation and architecture-quality checks. A rule cannot claim repository coverage while a domain-like tree remains invisible to the framework.

Engineering-quality checks therefore consume this same ownership closure rather than maintaining consumer-specific path allowlists.
