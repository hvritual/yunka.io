# Contract artifacts

W06 established protobuf-backed deterministic contract artifacts. C3 converges the repository's service-contract inputs through an explicit source inventory rather than treating one historical directory as the entire API surface. C9 additionally compiles typed PB Operation declarations into immutable execution plans consumed by the unified runtime.

## Canonical sources

The canonical service-contract inventory is:

```text
contracts/sources.json
```

C6 declares one canonical source inventory rooted at `contracts/proto/`. Package and wire identities remain compatibility-controlled while physical source paths are unified.

Internal protobuf files are not included merely because they exist in the repository. For example, `framework/infras/sms/environment.proto` is an internal data/configuration artifact rather than a service RPC contract and remains outside `contracts/sources.json`.

For compatibility with existing tooling, `yunka contract` still supports explicit single-root `--proto-dir`, `--proto-path`, and `--file` flags. Repository verification uses the canonical `--sources` inventory through the root Makefile.

## Generated artifacts

Committed outputs live under `contracts/generated/`:

- `manifest.json` — normalized protobuf/API contract used by compatibility checks.
- `openapi.json` — OpenAPI 3.1 document. Only explicitly HTTP-bound RPCs appear in `paths`; unbound RPCs are listed under `x-yunka-rpc-methods` rather than receiving invented HTTP routes.
- `client.ts` — transport-neutral TypeScript RPC client. Applications provide the HTTP/gRPC/custom transport implementation.
- `operation-plans.json` — C9 deterministic execution IR compiled from typed PB Domain/Application/Operation declarations. It contains stable Operation identity, security intent, composition/dependency closure, Application capability dependencies, and explicit transport bindings. It is derived evidence; protobuf remains the writable source of truth.

Do not edit these files manually. Regenerate them with:

```bash
make contract
```

and verify drift with:

```bash
make contract-check
```

`make contract-check` also runs `make rpc-contract-check`, which verifies the committed modern generated descriptors against the canonical `contracts/proto/` inventory. `operation-plans.json` participates in the same drift and determinism checks as the other committed artifacts.

## Operation execution contract

For a typed Application, C9 compiles PB declarations into `pkg/operationplan` and generates static Go OperationPlan accessors plus Executor-backed REST/gRPC adapters. Runtime code does not inspect descriptors or comments per request.

The stable PB Operation ID is the execution identity. gRPC full method names and HTTP method/path pairs are bindings only. Gateway authorization remains the security authority; the framework Executor sequences the configured security phase before Application invocation.

Canonical C9 generation does not emit the historical C8 REST/RPC transport adapters. The older `RenderApplicationCode` API remains only as a bounded compatibility/test surface while `yunka contract generate` uses the C9 generator.

## HTTP bindings

The preferred binding is standard `google.api.http` on protobuf methods. During migration, the contract compiler also accepts a source comment directive without changing wire descriptors:

```proto
// @yunka.http POST /v1/devices body=*
rpc CreateDevice(CreateDeviceRequest) returns (CreateDeviceResponse);
```

Standard annotations take precedence when both are present.

C3 does not infer routes from runtime gateway metadata. The historical `yunka api` flow imports dynamic/generated route definitions that are not committed as an authoritative protobuf mapping in this repository. Until an explicit binding is present in a canonical proto source, the RPC remains unbound and OpenAPI must not invent a path.

Other `@yunka.<key> <value>` method directives are preserved in the manifest as metadata so later waves can map contract intent to auth, resilience, observability, and Application Graph policies without introducing sidecar configuration as another source of truth.

## C6 canonical RPC source root

`contracts/proto` is the single inventoried protobuf root. Physical source paths may move, but protobuf package names, message/service full names, field and enum numbers, and RPC method names remain compatibility-controlled. Standard Go output is generated with exact pinned `protoc-gen-go` and `protoc-gen-go-grpc` versions. Existing gateway business code continues importing `yunka.io/gateway/rpc/meta`; the old XR generator, duplicate roots, and custom memory dispatcher have been removed.
