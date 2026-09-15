# Engineering-quality baseline distribution

## Purpose

Yunka's repository-wide engineering rules are normative in `docs/ENGINEERING_QUALITY_RULES.md`. Consumer projects must receive the same human-reviewability baseline without receiving Yunka's implementation history, current status, issue identifiers, or delivery-wave state.

`yunka init` therefore distributes a versioned, self-contained consumer projection rather than copying framework governance documents wholesale.

## Consumer artifacts

For a Go project, `yunka init` installs these files only when they are missing:

```text
AGENTS.md
.yunka/ENGINEERING_QUALITY.md
.yunka/engineering-quality-baseline.json
.yunka/engineering-quality.json
```

The baseline identity is defined by:

```text
policyVersion = yunka.engineering-quality/v1
provenance    = yunka:docs/ENGINEERING_QUALITY_RULES.md
policyIdentity = SHA256(policyVersion + provenance + canonical consumer rule digest)
```

`policyIdentity` is repository-independent. Biz, IoT Delivery and a fresh project therefore use the same rule identity without consumer-specific rule forks.

The consumer rule document is self-contained and can be read without network access or a local Yunka checkout after initialization.

## Normative rules versus framework history

Consumer policy contains durable engineering rules only. It does not copy:

- `docs/STATUS.md`;
- `PROJECT_MEMORY.md`;
- issue or pull-request identifiers;
- framework delivery-wave names;
- current framework task state.

A consumer's own project requirements, source ownership, canonical contracts, generated ownership and project configuration remain the enforcement inputs. The distributed policy is not a second business or architecture Source of Truth.

## Enforcement configuration

`.yunka/engineering-quality.json` is the deterministic Audit policy. A fresh project receives the accepted proven blocking rules for semantic source identity, package/exported-contract documentation, architecture bypasses, authorization bypasses and generated ownership/drift.

No file-size or structural-complexity threshold is enabled by default. Those review budgets require an explicit project decision.

If `.yunka/engineering-quality.json` already exists, `yunka init` validates it but does not replace it. This preserves consumer configuration ownership while the global `policyIdentity` remains stable.

## Idempotency and ownership

Initialization uses create-if-missing semantics. Re-running `yunka init` does not overwrite:

- an existing `AGENTS.md`;
- an existing enforcement policy;
- the installed consumer rule document;
- an existing baseline provenance record.

If an existing baseline has a different version/identity, initialization reports that an explicit upgrade review is required and leaves the old baseline untouched.

If the installed consumer rule document no longer matches the canonical text for the recorded version, initialization preserves the edit and reports explicit reconciliation instead of silently replacing it.

## Upgrade behavior

Policy upgrades are intentionally not automatic.

A later Yunka release may advertise a newer `policyVersion`, but the project must review and reconcile its local instructions/rules/policy explicitly. `yunka init` is not a hidden policy-upgrade mechanism.

This preserves a clean ownership boundary:

```text
Yunka owns canonical version/provenance identity.
Project owners own accepted local configuration and reviewed upgrades.
```

## Qualification

The distribution gate qualifies the same installer against:

1. a fresh Go project;
2. a pinned Biz checkout;
3. a pinned IoT Delivery checkout.

Real consumer repositories are read-only qualification inputs. Their module identity and any existing quality policy are projected into temporary projects, then the same baseline installer is exercised twice to prove deterministic identity and idempotency.

The qualification does not claim repository-level unbypassable enforcement. It proves deterministic distribution and the authority boundaries described above.
