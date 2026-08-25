# Domain scaffolding

`yunka domain` turns a business-domain manifest into the repetitive internal architecture so application developers concentrate on business rules rather than package layout, persistence naming, DTO conversion, request-scope plumbing, or transport delegation.

## Create a domain

From a downstream Go module:

```bash
yunka domain new \
  --name device \
  --object machine \
  --root internal \
  --table-prefix biz \
  --field serial:string \
  --field enabled:bool
```

The command creates `internal/device/domain.json` as the source of truth and generates:

```text
internal/device/
├── domain.json
├── domain/
│   └── zz_yunka_entity_gen.go
├── application/
│   └── zz_yunka_contract_gen.go
├── ports/
│   └── zz_yunka_repository_gen.go
├── infrastructure/
│   └── persistence/
│       ├── zz_yunka_po_gen.go
│       └── zz_yunka_repository_gen.go
├── transport/
│   ├── rest/
│   │   └── zz_yunka_rest_gen.go
│   └── rpc/
│       ├── machine.proto
│       └── zz_yunka_rpc_gen.go
└── wire/
    └── zz_yunka_wiring_gen.go
```

All `zz_yunka_*` files and generated protobuf schemas are framework-owned artifacts and must not be edited manually. Extend a domain with ordinary non-generated Go files beside them.

## Persistence / PO contract

The manifest fixes table naming as:

```text
<table-prefix>_<domain>_<business-object>
```

For example:

```text
biz_device_machine
biz_tenant_member
biz_contract_lease
```

`yunka domain check --root internal` requires every manifest under the same domain root to use one fixed table prefix. This prevents one application from silently mixing prefixes such as `biz_device_*` and `iot_tenant_*`.

A tenant-scoped generated PO owns the standard columns:

```text
id
tenant_id
<business fields>
version
created_at
updated_at
deleted_at
```

`id`, `tenant_id`, `version`, `created_at`, `updated_at`, and `deleted_at` are reserved manifest field names. The PO is persistence-only and generated mapping keeps it separate from the domain entity.

Tenant-scoped repositories never accept tenant identity from REST/RPC input. They resolve the trusted `identity.Principal` from `context.Context` and append `tenant_id = ?` at the persistence boundary. Use `--global` only for explicitly global domains.

## Request scope and service boundary

Generated application CRUD methods use `framework/requestscope` for every operation. The generated persistence factory begins a fresh GORM transaction and builds request-owned repositories. No request transaction or repository is retained by a singleton service.

The generated `application.Service` is the canonical internal contract. Transport adapters do not own business logic:

```text
REST request ─┐
              ├─ generated DTO mapping → application.Service → requestscope → repository
RPC request ──┘
```

REST handlers decode route/query/body fields into service input structs and delegate directly to the matching service method. The RPC adapter performs the same conversion and delegates directly to the service; the adjacent generated protobuf file is the canonical external RPC schema for the existing pinned standard protobuf/gRPC generation pipeline. `yunka domain` does not introduce a second RPC runtime or a reflection-based dispatcher.

## One-line composition

`wire.Build(database)` creates the request-scope repository factory, default CRUD service, REST handler, and RPC adapter:

```go
bundle, err := devicewire.Build(primaryDatabase)
```

A yunka runtime module should use this bundle as a composition seam rather than rebuilding repositories, DTO adapters, or transaction handling itself.

## Regenerate and validate

Edit only `domain.json` when changing generated schema/transport metadata, then run:

```bash
yunka domain generate --path internal/device
yunka domain check --root internal
```

`domain generate` removes obsolete framework-owned generated transport/artifact files when a manifest disables or changes generated surfaces; it never deletes ordinary developer-owned source files.

`domain check` recomputes every generated artifact and fails on drift, stale generated files, inconsistent root table prefixes, or a table name that does not equal the required `<prefix>_<domain>_<object>` convention.

## Supported field types

The initial generator supports:

```text
string
int64
uint64
bool
float64
time
```

The manifest sorts fields deterministically. Transport DTOs, application inputs, domain entities, GORM POs, REST bindings, RPC bindings, and protobuf fields are all generated from the same field list.

## Boundary

`yunka domain` intentionally does not generate business policy. Developers own non-generated files for invariants, calculations, authorization policy, workflows, and domain-specific use cases. The framework owns repetitive structural and transport/persistence mechanics.
