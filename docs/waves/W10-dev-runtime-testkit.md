# W10 — Developer Runtime, Doctor, and TestKit

## Goal

Turn the W09 graph and the W01-W08 runtime contracts into a predictable local development workflow:

```text
yunka doctor -> prove the environment is usable
yunka graph  -> explain the project and impact surface
yunka dev    -> resolve and run explicit local process dependencies
TestKit      -> deterministic infrastructure fakes shared by framework tests
```

## `yunka doctor`

`doctor` is read-only. It checks:

- repository root and `go.work` toolchain declaration;
- Go version against the declared toolchain;
- protoc >= 3.21;
- GCC and Git availability;
- committed W06 contract manifest readability;
- W09 contract graph compilation;
- Git working-tree status;
- optional `.yunka/dev.json` presence.

Every result is `PASS`, `WARN`, or `FAIL` and may include an explicit remediation action. `--strict` turns warnings into a non-zero exit.

`doctor` never runs `go mod tidy`, `go work sync`, generators, migrations, or other mutating commands.

## `yunka dev`

Local process orchestration is explicit configuration, not source-code guessing.

```bash
yunka dev plan --target api
yunka dev run --target api
```

`.yunka/dev.json` declares argv arrays, relative working directories, process dependencies, optional W09 graph-node references, and the names of environment variables a child may inherit.

Security rules:

- commands are argv arrays and are never passed through a shell;
- absolute or escaping working directories are rejected;
- process dependency cycles and missing dependencies fail before startup;
- graph references are validated when a W09 graph is available;
- environment inheritance can be allow-listed per process.

A process is considered started when the OS process launches. Health/readiness orchestration remains a future extension and must use W01 Health rather than arbitrary sleep-based inference.

## TestKit

TestKit is layered so leaf packages do not depend upward on the framework:

- `pkg/testkit.Clock` with explicit `Advance`;
- `pkg/testkit.Registry` implementing the complete current `registry.Registry` contract and emitting watch events;
- `framework/testkit` re-exports the leaf Clock/Registry and adds a W08 `event.Broker` with captured immutable envelopes.

TestKit exists to stop individual waves from creating partial fakes that silently drift from framework interfaces. It is not a production fallback and does not replace integration tests against real infrastructure.
