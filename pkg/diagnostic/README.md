# Diagnostic

`pkg/diagnostic` is the dependency-neutral developer diagnostics presentation contract introduced by C11.3.

It owns stable developer-tooling identities and rendering only:

- `YUNKA-DX-*` codes;
- the canonical `Definition` catalog for stable code, stage, meaning, static location, and static non-executing remediation metadata;
- severity/stage/summary/detail presentation;
- project-relative locations;
- structured, non-executing remediation actions;
- deterministic text/JSON normalization and rendering;
- the AX3 `agent-json` projection that maps the same normalized facts into stable `cause`, `target`, `remediation`, and `retry` slots.

Diagnostic producers must reference the catalog for stable identity facts rather than maintaining parallel code/stage dictionaries. Dynamic facts remain with their canonical owners: compiler/checker error detail, Doctor environment observations, and current-environment remediation text are still produced by those validators/adapters.

The Agent projection does not infer new framework semantics. `cause` is the canonical summary/detail, `target` is the normalized project-relative location, `remediation` is the existing non-executing action list, and `retry` is the command that produced the failed diagnostic. The legacy `json` contract is unchanged; callers opt into the projection with `--format agent-json`.

`LookupDefinition` performs only whitespace trimming and case normalization. It never fuzzy-matches or invents a meaning for an unknown code. `yunka explain` is a read-only view over this catalog.

This package does **not** own contract lint rules, module validation, assembly drift, Doctor environment probes, authorization, execution, transactions, runtime health, or any other canonical validation/runtime semantics. Those remain with their existing owners and are adapted at developer-facing boundaries.
