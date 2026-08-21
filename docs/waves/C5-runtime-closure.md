# C5 — Runtime Closure

## Status

Implementation wave over the C4 baseline `bad0ee4c739a6f0ca83786d13f11079e08bee956`.

## Goal

Close the local runtime loop without turning `yunka` into a deployment manager:

```text
explicit dev manifest
    -> deterministic process plan
    -> declared process graph
    -> ordered startup and W01/W07 readiness
    -> secret-free runtime report and observed graph
    -> bounded reverse-order shutdown
```

The manifest remains the only source of commands, working directories, dependencies, inherited environment-variable names, and readiness endpoints. C5 never derives commands from source code, package names, contract names, or graph relationships.

## Manifest schema v3

Schema versions 1 and 2 remain supported. Runtime closure fields require schema version 3, unless the CLI explicitly enables closure with `--closure`.

```json
{
  "schemaVersion": 3,
  "runtime": {
    "application": "yunka",
    "statePath": ".yunka/dev-runtime.json",
    "graphPath": ".yunka/runtime-graph.json",
    "contractManifest": "contracts/generated/manifest.json",
    "shutdownTimeout": "15s",
    "closure": true
  },
  "processes": []
}
```

Closure mode requires every selected process to own one existing, unique graph node. The check runs before any process starts. Runtime artifact paths must be relative to the repository and must not escape through `..` or symbolic links.

## Declared process graph

C5 adds the additive `process` node kind and `runs` edge kind. The runtime graph records only explicit facts:

- application `contains` process;
- process `depends_on` process from `dependsOn`;
- process `runs` its exact `graphNode` target.

These nodes and edges use declared evidence from `dev.manifest`. Observed process state and safe W07 core summaries use observed evidence from `dev.runtime`.

## Runtime report

When schema v3 or `--closure` is active, `yunka dev run` writes an atomic mode-`0600` report. The default path is `.yunka/dev-runtime.json`.

The report contains the application name, deterministic plan order, run state, per-process state, bounded timestamps, readiness state, a safe W07 core summary, and sanitized failure text. It never contains commands, argument values, environment values, credentials, authorization headers, request identity, diagnostics component payloads, or child output.

```bash
yunka dev status --state .yunka/dev-runtime.json
yunka dev status --state .yunka/dev-runtime.json --format json
```

Runtime state and graph files are local artifacts and `.yunka/` is ignored by Git.

## Diagnostics capture

`readiness.captureDiagnostics=true` reuses the same bounded, no-redirect, authenticated readiness request. Only these W07 core fields are retained:

- application state;
- health state, liveness, and readiness;
- route count;
- RPC client/server inventory;
- event-bus presence.

Component payloads and response bodies are not persisted. Diagnostics evidence explains the runtime; it never changes the process plan.

## Graceful shutdown

C5 supervises direct children explicitly:

1. stop starting new processes;
2. visit started children in reverse plan order;
3. send the platform-supported graceful signal;
4. wait within one total shutdown budget;
5. kill only a child that remains after the deadline;
6. preserve the original startup/readiness/exit error and report shutdown failures separately.

An unexpected child exit shuts down the remaining children. C5 does not implement restart/backoff, process-tree management, containers, systemd, Kubernetes, remote orchestration, or a public runtime-control server.

## Verification

```bash
make toolchain-check
make dependency-check
make tidy
make contract
make contract-check
make test
make race
make vet
make vuln
make build
make integration
```

The C5 gate additionally repeats runtime-closure subprocess tests and the MySQL outbox integration. Dependency metadata and committed contract artifacts must remain unchanged on the second determinism pass.

## Rollback

Revert the single C5 commit. Schema v1/v2 manifests remain valid throughout the wave, so callers can also roll back operationally by removing schema-v3 runtime fields and using the prior `yunka dev run` behavior.
