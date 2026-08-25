# Domain compiler

`yunka domain` treats developer-owned PO structs as the low-friction business schema input and compiles repetitive architecture around them. The framework owns table naming, standard columns, request transactions, repositories, Service contracts, REST DTO mapping, `domain.proto`, pinned protobuf/gRPC generation, typed gRPC bridge/registration, and zero-drift checks.

## Project initialization

Choose the database prefix once:

```bash
yunka init --db-prefix biz
```

If omitted, the project default is `yk`. If `yunka init` is skipped entirely, the first `yunka domain new` creates `.yunka/project.json` with `yk`.

Every generated table is:

```text
<project-prefix>_<domain>_<po-object>
```

Examples:

```text
yk_device_coffee_machine
yk_device_device_group
biz_tenant_member
```

A domain cannot override the project prefix.

## PO-first workflow

The ordinary workflow no longer requires repeating every field on the CLI or in `domain.json`:

```bash
yunka domain new --name device
```

If `internal/device/infrastructure/persistence` already contains PO files, `domain new` adopts and scans them. If no PO exists, it bootstraps `<domain>.go` and the developer can edit that PO and run `yunka domain generate` again.

One PO object must live in one file and the filename is the object name in `snake_case`:

```text
internal/device/infrastructure/persistence/
├── coffee_machine.go   -> type CoffeeMachinePO struct
├── device_group.go     -> type DeviceGroupPO struct
└── site_binding.go     -> type SiteBindingPO struct
```

A mismatched filename is rejected by `domain new`, `domain generate`, and `domain check`.

Example developer PO:

```go
package persistence

import "time"

type CoffeeMachinePO struct {
    Serial      string    `gorm:"column:serial;type:varchar(64);index"`
    Enabled     bool      `gorm:"column:enabled;type:tinyint(1)"`
    ActivatedAt time.Time `gorm:"column:activated_at;type:datetime(3)"`

    // Persist and AutoMigrate this field, but do not expose it through
    // Domain, Service, REST, or RPC.
    InternalHash string `gorm:"column:internal_hash;type:varchar(64)" yunka:"-"`
}
```

Supported contract field types are `string`, `int64`, `uint64`, `bool`, `float64`, and `time.Time`. Unsupported/relational persistence fields are allowed when marked `yunka:"-"`; GORM still sees them through the generated final persistence record.

Framework-owned columns (`id`, `tenant_id`, `version`, `created_at`, `updated_at`, `deleted_at`) must not be redeclared in developer POs.

## What `domain new/generate` compiles

For multiple PO objects, one generation pass produces:

```text
internal/device/
├── domain.json
├── domain/
│   ├── zz_yunka_coffee_machine_entity_gen.go
│   └── zz_yunka_device_group_entity_gen.go
├── application/
│   └── zz_yunka_service_gen.go
├── ports/
│   └── zz_yunka_repositories_gen.go
├── infrastructure/persistence/
│   ├── coffee_machine.go                       # developer-owned
│   ├── device_group.go                         # developer-owned
│   ├── zz_yunka_coffee_machine_record_gen.go
│   ├── zz_yunka_device_group_record_gen.go
│   └── zz_yunka_repositories_gen.go
├── transport/rest/
│   └── zz_yunka_rest_gen.go
├── transport/rpc/
│   ├── domain.proto
│   ├── pb/
│   │   ├── domain.pb.go
│   │   └── domain_grpc.pb.go
│   └── zz_yunka_grpc_bridge_gen.go
└── wire/
    └── zz_yunka_wiring_gen.go
```

`domain.json` records the normalized PO inventory, table names, transport settings, persistence columns, stable protobuf field numbers, and reserved protobuf slots. PO files are authoritative for object/field discovery; `domain generate` reconciles the manifest from them.

## Pinned gRPC lifecycle

RPC generation is part of the same command:

```text
PO scan
  -> normalized domain contract
  -> domain.proto
  -> pinned protoc 3.21.12
  -> pinned protoc-gen-go v1.36.11
  -> pinned protoc-gen-go-grpc v1.6.2
  -> pb/domain.pb.go
  -> pb/domain_grpc.pb.go
  -> typed gRPC bridge
  -> pb.Register<Domain>ServiceServer
```

The pin values are checked against `tools/toolchain.env`. `yunka domain generate` verifies the exact protoc version, reuses exact project/repository plugins when available, and otherwise installs the pinned Go plugins into `.yunka/bin`. `protoc` plus its standard include directory must be the pinned version; `yunka doctor` remains the toolchain readiness entry point.

Generated bridge methods map protobuf request types directly to the canonical `application.Service` input types and map Service outputs back to protobuf messages. No reflection dispatcher, string service lookup, or second RPC runtime is introduced.

Generated composition is one line plus registration:

```go
bundle, err := devicewire.Build(primaryDatabase)
if err != nil { /* ... */ }

bundle.RegisterREST(httpMux)
bundle.RegisterGRPC(grpcServer)
```

`RegisterGRPC` calls the generated typed bridge registration, which calls standard `pb.Register<Domain>ServiceServer`.

## Stable protobuf numbers

PO fields receive stable protobuf numbers in `domain.json`. Adding a new PO field appends a new number without renumbering existing fields. Removing a field reserves its previous protobuf number/name so a later field cannot silently reuse the ABI slot.

Developers do not manage these numbers manually. Removing or renaming an entire PO object is intentionally not treated as an automatic compatibility-safe operation.

## Persistence and automatic schema reconciliation

Yunka generates a final GORM record for each PO:

```text
framework standard columns
+ developer-owned <Object>PO
```

`wire.Build(database)` runs `AutoMigrate` against every final record before constructing request scopes and repositories. Therefore adding a PO field or GORM index is automatically reconciled to the database at application composition/start.

Fields tagged `yunka:"-"` remain persistence-only but are still part of the final GORM record and AutoMigrate.

Tenant-scoped repositories continue to derive `tenant_id` only from trusted `identity.Principal` and execute through `framework/requestscope` transactions.

## Regenerate and check

After editing/adding/removing PO files or fields:

```bash
yunka domain generate --path internal/device
yunka domain check --root internal
```

`generate` performs PO rescan, manifest reconciliation, REST generation, pinned protobuf/gRPC generation, typed bridge/registration generation, stale generated cleanup, and wiring generation in one lifecycle.

`check` repeats the PO scan and pinned RPC generation without accepting drift. It fails on:

- PO type/file snake_case mismatch;
- object/field contract drift;
- project table-prefix drift;
- missing/stale framework artifacts;
- changed `domain.proto`;
- changed `domain.pb.go` / `domain_grpc.pb.go`;
- changed gRPC bridge/register code;
- wrong protoc/plugin versions;
- any other generated zero-drift mismatch.

## Ownership boundary

Framework-owned and never hand-edited:

```text
zz_yunka_*
domain.proto
transport/rpc/pb/*.pb.go
domain.json protobuf numbering / generated contract metadata
```

Application-owned:

```text
infrastructure/persistence/<po_object_snake_case>.go
non-generated domain policy files
business invariants
permission policy
state machines
cross-domain workflows
```

The intended developer task is to define PO objects and business rules. Directory layout, table naming, transactions, CRUD repositories, DTO conversion, protobuf generation, gRPC registration, and schema synchronization are framework responsibilities.
