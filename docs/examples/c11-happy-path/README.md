# C11 developer happy path example

This directory is a copyable **shape example** for the qualified C11 developer workflow. It does not replace the canonical owners of project, contract, module, assembly, diagnostics, or runtime semantics.

Canonical owners remain:

- `.yunka/project.json` / `app/cmd/project` for developer workflow locations;
- protobuf contracts for writable business contract truth;
- generated module facts and AssemblyPlan for composition truth;
- `pkg/diagnostic` for stable `YUNKA-DX-*` diagnostic identities;
- DevManifest / `pkg/devruntime` for explicitly declared local processes, dependencies, readiness, and runtime evidence.

## 1. Initialize the project profile

Start with the normal developer entry point:

```sh
yunka init
```

For a project that keeps its dev manifest outside `.yunka/dev.json`, use `project.json` in this directory as the shape for `.yunka/project.json`. Its only dev-runtime responsibility is locating:

```text
config/dev.runtime.json
```

If the project uses `contracts/sources.json`, keep the inventory selected by `yunka init`; `workflow.contract.sources` and `workflow.contract.protoRoot` are mutually exclusive.

## 2. Declare the local runtime

Use `dev.runtime.json` in this directory as the shape for `config/dev.runtime.json`.

The example deliberately uses schema version 3 so runtime state/graph evidence is available, but it does **not** enable closure. It declares one process and one explicit HTTP readiness barrier.

Replace these application-owned values with the project's real values:

- `command`: the argv array that starts the local process;
- `readiness.url`: the real loopback/local readiness endpoint;
- optional runtime paths if the project does not use the defaults shown here.

Yunka does not infer commands, ports, endpoints, secrets, dependencies, or readiness from names.

## 3. Run the normal workflow

```sh
yunka generate
yunka check
yunka dev
```

`yunka dev` resolves `workflow.dev.manifest`, starts the existing canonical `pkg/devruntime` supervisor, waits for the declared readiness barrier, and surfaces canonical runtime evidence.

While the runtime is live, inspect its persisted report with:

```sh
yunka dev status --state .yunka/dev-runtime.json
```

Use `Ctrl-C` / SIGTERM to stop the supervised runtime through the existing shutdown path.

## 4. Explain a diagnostic code

When `generate`, `check`, or `doctor` emits a stable code, look it up directly instead of searching ad-hoc documentation:

```sh
yunka explain YUNKA-DX-CONTRACT-002
yunka explain YUNKA-DX-CONTRACT-002 --format json
```

`yunka explain` reads the same canonical `pkg/diagnostic` Definition catalog used by the diagnostic producers. It does not fuzzy-guess unknown codes and never executes remediation actions.

## 5. Expert interfaces remain available

The normal developer loop remains:

```text
init -> generate -> check -> dev
```

For explicit architecture work, the existing expert interfaces are still available and behavior-compatible:

```sh
yunka contract --help
yunka assembly --help
yunka module --help
yunka domain --help
yunka dependency --help
```

These expert commands are not deprecated by C11.6. Their command names, aliases, flags, and subcommands are protected by C11.6-C compatibility snapshots.
