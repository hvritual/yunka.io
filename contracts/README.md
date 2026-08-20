# Contract artifacts

W06 established protobuf-backed deterministic contract artifacts. C3 converges the repository's service-contract inputs through an explicit source inventory rather than treating one historical directory as the entire API surface.

## Canonical sources

The canonical service-contract inventory is:

```text
contracts/sources.json
```

C3 currently declares two independent source sets:

```text
legacy-api       -> app/cmd/rpc/pb/
gateway-runtime  -> gateway/rpc/pb/
```

Each source set carries a complete explicit file list. The compiler verifies that every `.proto` file discovered under a configured root is listed, compiles each root with a separate `protoc` invocation, then merges the normalized manifests deterministically. Independent compilation prevents same-basename imports such as `common.proto` from resolving against the wrong source root.

Internal protobuf files are not included merely because they exist in the repository. For example, `framework/infras/sms/environment.proto` is an internal data/configuration artifact rather than a service RPC contract and remains outside `contracts/sources.json`.

For compatibility with existing tooling, `yunka contract` still supports explicit single-root `--proto-dir`, `--proto-path`, and `--file` flags. Repository verification uses the canonical `--sources` inventory through the root Makefile.

## Generated artifacts

Committed outputs live under `contracts/generated/`:

- `manifest.json` — normalized protobuf/API contract used by compatibility checks.
- `openapi.json` — OpenAPI 3.1 document. Only explicitly HTTP-bound RPCs appear in `paths`; unbound RPCs are listed under `x-yunka-rpc-methods` rather than receiving invented HTTP routes.
- `client.ts` — transport-neutral TypeScript RPC client. Applications provide the HTTP/gRPC/custom transport implementation.

Do not edit these files manually. Regenerate them with:

```bash
make contract
```

and verify drift with:

```bash
make contract-check
```

`make contract-check` also runs `make rpc-contract-check`, which verifies that the protobuf descriptors registered by the committed legacy gateway generated code still match `gateway/rpc/pb/`.

## HTTP bindings

The preferred binding is standard `google.api.http` on protobuf methods. During migration, the contract compiler also accepts a source comment directive without changing wire descriptors:

```proto
// @yunka.http POST /v1/devices body=*
rpc CreateDevice(CreateDeviceRequest) returns (CreateDeviceResponse);
```

Standard annotations take precedence when both are present.

C3 does not infer routes from runtime gateway metadata. The historical `yunka api` flow imports dynamic/generated route definitions that are not committed as an authoritative protobuf mapping in this repository. Until an explicit binding is present in a canonical proto source, the RPC remains unbound and OpenAPI must not invent a path.

Other `@yunka.<key> <value>` method directives are preserved in the manifest as metadata so later waves can map contract intent to auth, resilience, observability, and Application Graph policies without introducing sidecar configuration as another source of truth.

## Legacy generated RPC code

`gateway/rpc/meta/*.pb.go` and the custom `gateway/rpc/*/*.xr_*.go` files remain generated compatibility artifacts. C3 does not hand-edit or regenerate them. The historical `gateway/rpc/gender.sh` is not considered a deterministic C3 generation path; replacement or migration requires a separate proof that the legacy generator output can be reproduced safely.
