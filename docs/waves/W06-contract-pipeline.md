# W06 — Contract Pipeline

## Goal

Make protobuf the canonical API contract and establish a deterministic pipeline from source
contract to normalized manifest, OpenAPI, client SDK, and compatibility guard.

## Scope

### Included

1. Compile `.proto` files through `protoc` into `FileDescriptorSet`.
2. Decode descriptors without adding a new runtime protobuf dependency to `pkg`.
3. Produce deterministic `manifest.json`.
4. Produce OpenAPI 3.1 JSON from explicit HTTP bindings.
5. Produce a transport-neutral TypeScript client for every RPC method.
6. Support standard `google.api.http` and a migration-only `@yunka.http` comment directive.
7. Lint duplicate HTTP routes and unresolved contract types.
8. Classify contract changes and fail on breaking changes.
9. Detect stale generated artifacts.
10. Expose the workflow through `yunka contract` and CI.

### Not included

- Replacing the legacy RPC generator.
- Editing generated RPC files by hand.
- API marketplace/developer portal/billing.
- GraphQL.
- Inventing HTTP routes for RPC methods that have no explicit binding.
- Automatically applying contract auth/resilience directives to runtime policy; W06 only
  preserves generic yunka directives for later integration.

## Commands

```bash
yunka contract lint
yunka contract generate
yunka contract inspect
yunka contract diff --baseline old/manifest.json --current contracts/generated/manifest.json
yunka contract check
```

`make contract` regenerates committed artifacts. `make contract-check` fails when committed
artifacts differ from protobuf source.

## Contract guard

Breaking changes include:

- service or RPC method removal;
- request/response type changes;
- streaming mode changes;
- protobuf field removal, renumbering, rename/JSON-name change, type/cardinality/presence change;
- enum value removal or numeric change;
- removal/change of an existing HTTP method/path/body binding;
- adding a proto2 required field.

Compatible additions such as new methods, messages, enum values, and non-required fields are
reported but do not fail the guard.

## CI flow

```text
Proto source
   ↓
contract compile + lint
   ↓
render artifacts in memory
   ↓
compare committed generated artifacts
   ↓
base-manifest compatibility diff
   ↓
make verify
```

On the first W06 PR, the base branch has no manifest and compatibility comparison is skipped.
Once W06 lands, every later PR compares against the base commit's committed manifest.

## Acceptance

- Current legacy `app/cmd/rpc/pb` compiles without modification.
- Generated artifacts are deterministic.
- OpenAPI does not fabricate HTTP routes for unbound methods.
- TypeScript client contains all current RPC operations.
- A field type change and HTTP binding change are detected as breaking.
- Generated artifact drift fails `contract check`.
- `pkg/contract` passes unit tests, race tests, and vet.
