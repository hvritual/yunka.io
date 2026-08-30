# Diagnostic

`pkg/diagnostic` is the dependency-neutral developer diagnostics presentation contract introduced by C11.3.

It owns stable developer-tooling identities and rendering only:

- `YUNKA-DX-*` codes;
- severity/stage/summary/detail;
- project-relative locations;
- structured, non-executing remediation actions;
- deterministic text/JSON normalization and rendering.

It does **not** own contract lint rules, module validation, assembly drift, Doctor environment probes, authorization, execution, transactions, runtime health, or any other canonical validation/runtime semantics. Those remain with their existing owners and are adapted at developer-facing boundaries.
