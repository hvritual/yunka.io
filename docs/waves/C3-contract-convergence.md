# C3 Contract Convergence

## Status

Implementation wave over the C2 baseline `9a18d17a55b9b2aa9baff777c2f31b908b5398c9`.

## Goal

Make the committed contract artifacts cover every explicitly selected service-contract source while preserving the existing W06 compatibility baseline and the C2 deterministic toolchain.

C3 closes a concrete blind spot: W06 compiled only `app/cmd/rpc/pb/`, while the running gateway RPC surface is generated from `gateway/rpc/pb/` and was therefore absent from `contracts/generated/manifest.json`.

C3 does not treat directory layout as architecture. The legacy API protobuf and gateway runtime protobuf are distinct source sets and are compiled independently before their normalized contracts are merged.

## Canonical source inventory

`contracts/sources.json` is the repository-owned inventory for service contracts. Each source set declares:

- a stable source-set name;
- a repository-relative protobuf root;
- the complete, explicit list of `.proto` files in that root;
- optional repository-relative import roots.

The C3 inventory contains:

```text
legacy-api
  app/cmd/rpc/pb/
    api.proto
    api_common.proto
    common.proto
    unit.proto

gateway-runtime
  gateway/rpc/pb/
    api_common.proto
    common.proto
    gateway.proto
```

The compiler rejects an inventory when a configured source root contains an unlisted `.proto` file. Adding a service contract therefore cannot silently bypass the canonical manifest.

`framework/infras/sms/environment.proto` is intentionally not in this inventory: it is an internal protobuf data/configuration artifact, not a service RPC contract. Inclusion is explicit rather than based on a repository-wide `*.proto` glob.

## Independent compilation

Source sets are compiled with separate `protoc` invocations. This is required because both current roots contain files such as `common.proto` and `api_common.proto`; combining import paths in one invocation could resolve an import against the wrong root.

For every source set C3:

1. validates that root/import paths remain inside the repository;
2. verifies the explicit file list exactly matches discovered `.proto` files;
3. compiles with the C2-locked `protoc 3.21.12`;
4. canonicalizes manifest file names to repository-relative paths;
5. merges normalized messages, enums, and services;
6. fails closed when two source sets claim the same canonical file or the same protobuf message, enum, or service full name.

Source-set order does not affect the output. The aggregate descriptor identity is derived from the source-set name, root, and individual descriptor digest in sorted source-set order.

## Compatibility behavior

C3 is additive relative to C2:

- the existing `ApiService` and `UnitService` contracts remain present;
- the gateway runtime service `io.yunka.gateway.rpc.GatewayService` and its namespaced types are added;
- existing field numbers, request/response types, service methods, and streaming modes are not changed;
- the C2 -> C3 contract diff must contain no breaking change.

The manifest remains schema version 1. Source inventory is build metadata and does not require a manifest schema change.

## HTTP binding boundary

C3 does not invent HTTP routes.

The current repository has no committed `google.api.http` bindings and no `@yunka.http` directives for these RPC sources. Historical gateway HTTP metadata is dynamically imported from generated/runtime route metadata by the legacy `yunka api` flow; those route definitions are not a committed protobuf contract source in this repository.

Therefore unbound RPCs remain under `x-yunka-rpc-methods` and OpenAPI `paths` may remain empty. A later migration may add explicit HTTP bindings only when an authoritative committed route mapping exists.

## Generated runtime descriptor guard

`gateway/rpc/meta/*.pb.go` remains generated legacy runtime code and is not rewritten by C3.

`make rpc-contract-check` compiles `gateway/rpc/pb/` with the locked protoc and compares its normalized descriptor contract with the protobuf descriptors registered by the committed generated `gateway/rpc/meta` package. Any wire-shape/service-shape drift blocks verification.

This establishes the direction of authority:

```text
gateway/rpc/pb
      |
      | locked protoc
      v
canonical descriptor contract
      |
      | must equal
      v
gateway/rpc/meta generated descriptors
```

The destructive historical `gateway/rpc/gender.sh` remains legacy-only. C3 does not claim a deterministic replacement generator until the full custom generator and legacy `protoc-gen-go` output can be reproduced byte-for-byte or migrated deliberately.

## CLI and Make

Canonical repository verification uses:

```bash
make contract
make rpc-contract-check
make contract-check
```

`make contract` and `make contract-check` pass `contracts/sources.json` plus the repository root to `yunka contract`.

For compatibility, the CLI still accepts the historical single-root flags:

```text
--proto-dir
--proto-path
--file
```

The new canonical mode is selected with:

```text
--sources <inventory.json>
--repo-root <repository-root>
```

## Verification

C3 is complete only when all of the following pass:

```bash
make toolchain-check
make tidy
make contract
make contract-check
make test
make race
make vet
make vuln
make build
```

Additional C3 gates:

```text
source inventory completeness/path containment
source-set ordering determinism
duplicate ownership fail-closed
gateway generated descriptor sync
C2 -> C3 contract compatibility: no breaking changes
second make contract: byte-identical / clean worktree
```

Production regression remains:

```bash
export YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/yunka_test?parseTime=true&charset=utf8mb4'
make integration
```

## Non-goals

C3 does not:

- infer or invent dynamic HTTP routes;
- rename protobuf packages;
- renumber fields;
- delete the legacy `app/cmd/rpc/pb` source set;
- hand-edit or regenerate committed RPC generated files;
- remove the legacy RPC generator;
- upgrade dependency versions;
- add runtime features.

Those changes require independent evidence and review rather than being hidden inside contract convergence.

## Rollback

C3 is additive. Rollback restores the C2 single-root contract command and C2 generated artifacts. The running gateway RPC implementation is unchanged by C3, so rollback does not require a runtime protocol migration.
