# Contract artifacts

W06 establishes protobuf as the API contract source of truth and generates deterministic
artifacts under `contracts/generated/`.

## Source

The initial legacy source root is:

```text
app/cmd/rpc/pb/
```

The contract compiler invokes `protoc` only to build a `FileDescriptorSet`; yunka then
normalizes the descriptor into its own vendor-neutral manifest. Existing generated RPC files
are not edited by W06.

## Generated artifacts

- `generated/manifest.json` — normalized protobuf/API contract used by compatibility checks.
- `generated/openapi.json` — OpenAPI 3.1 document. Only explicitly HTTP-bound RPCs appear in
  `paths`; unbound RPCs are listed under `x-yunka-rpc-methods` rather than receiving invented
  HTTP routes.
- `generated/client.ts` — transport-neutral TypeScript RPC client. Applications provide the
  HTTP/gRPC/custom transport implementation.

Do not edit these files manually. Regenerate them with:

```bash
make contract
```

and verify drift with:

```bash
make contract-check
```

## HTTP bindings

The preferred binding is standard `google.api.http` on protobuf methods. During migration,
W06 also accepts a source comment directive without changing wire descriptors:

```proto
// @yunka.http POST /v1/devices body=*
rpc CreateDevice(CreateDeviceRequest) returns (CreateDeviceResponse);
```

Standard annotations take precedence when both are present.

Other `@yunka.<key> <value>` method directives are preserved in the manifest as metadata so
later waves can map contract intent to auth, resilience, observability, and Application Graph
policies without introducing sidecar configuration as another source of truth.
