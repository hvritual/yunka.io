# Domain scaffolding

`yunka domain` turns a business-domain manifest into the repetitive internal architecture so application developers concentrate on business rules instead of package layout, PO naming, request-scope plumbing, DTO conversion, REST/RPC delegation, or schema synchronization.

## 1. Initialize the project once

The database table prefix is a project-level decision, not a per-domain decision.

```bash
yunka init --db-prefix biz
```

This writes:

```text
.yunka/project.json
```

with:

```json
{
  "version": 1,
  "database": {
    "tablePrefix": "biz"
  }
}
```

If `--db-prefix` is omitted, the default is `yk`.

```bash
yunka init
```

If a developer skips `yunka init`, the first `yunka domain new` automatically initializes the project with the same `yk` default. Once persisted, the prefix is immutable through the scaffolder: an existing `biz` project cannot silently generate an `iot_*` domain.

## 2. Create a domain

From a downstream Go module:

```bash
yunka domain new \
  --name device \
  --object machine \
  --root internal \
  --field serial:string \
  --field enabled:bool
```

No table-prefix flag is needed. The generator reads the project prefix and creates `internal/device/domain.json` as the generated-contract source of truth.

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
│       ├── po.go                         # developer-owned, safe to edit
│       ├── zz_yunka_po_base_gen.go      # framework-owned
│       └── zz_yunka_repository_gen.go   # framework-owned
├── transport/
│   ├── rest/
│   │   └── zz_yunka_rest_gen.go
│   └── rpc/
│       ├── machine.proto
│       └── zz_yunka_rpc_gen.go
└── wire/
    └── zz_yunka_wiring_gen.go
```

All `zz_yunka_*` files and generated protobuf schemas are framework-owned artifacts and must not be edited manually. Ordinary non-generated files are application-owned.

## 3. Persistence / PO contract

Table naming is fixed to:

```text
<project-prefix>_<domain>_<business-object>
```

For a project initialized with `biz`:

```text
biz_device_machine
biz_tenant_member
biz_contract_lease
```

For an uninitialized/default project:

```text
yk_device_machine
yk_tenant_member
yk_contract_lease
```

`yunka domain check --root internal` validates every manifest against `.yunka/project.json`. The prefix is no longer repeated in every domain command.

### Framework-owned PO base

The generator owns standard persistence columns in `zz_yunka_po_base_gen.go`:

```text
id
tenant_id             # tenant-scoped domains
<manifest fields>
version
created_at
updated_at
deleted_at
```

`id`, `tenant_id`, `version`, `created_at`, `updated_at`, and `deleted_at` are reserved manifest field names.

### Developer-owned PO extension

The generated scaffold creates `infrastructure/persistence/po.go` once and never overwrites it during regeneration:

```go
type MachinePO struct {
    MachinePOBase `gorm:"embedded"`

    ExternalCode string `gorm:"column:external_code;type:varchar(64);index"`
    Source       string `gorm:"column:source;type:varchar(32)"`
}
```

Developers may add persistence-only fields, indexes, relations, or GORM metadata directly to this final PO type. Those additions do not need to be copied into a generator template and are not erased by `yunka domain generate`.

Use `domain.json` when a field belongs to the domain/service/API contract. Use editable `po.go` when a field is persistence-specific.

## 4. PO changes automatically reach the database

`wire.Build(database)` automatically calls generated `persistence.AutoMigrate(...)` against the **final editable PO type**, not only the generated base type.

Therefore this change:

```go
type MachinePO struct {
    MachinePOBase `gorm:"embedded"`
    ExternalCode string `gorm:"column:external_code;type:varchar(64);index"`
}
```

is seen by GORM on the next application composition/start and the missing column/index is reconciled automatically.

The migration target is always:

```go
&MachinePO{}
```

so developer-added PO fields participate in schema migration without modifying framework-owned files.

## 5. Tenant boundary is generated

Tenant-scoped repositories never accept tenant identity from REST/RPC input. They resolve the trusted `identity.Principal` from `context.Context` and append `tenant_id = ?` at the persistence boundary.

```text
REST/RPC request
      ↓
trusted Principal.TenantID
      ↓
application.Service
      ↓
requestscope
      ↓
repository
      ↓
WHERE tenant_id = ?
```

Use `--global` only for explicitly global domains.

## 6. Request scope and service boundary

Generated application CRUD methods use `framework/requestscope` for every operation. The generated persistence factory begins a fresh GORM transaction and builds request-owned repositories. No request transaction or repository is retained by a singleton service.

The generated `application.Service` is the canonical internal contract:

```text
REST request ─┐
              ├─ generated DTO mapping → application.Service → requestscope → repository
RPC request ──┘
```

REST handlers decode route/query/body fields into service input structs and call the matching service method directly. The RPC adapter performs the same conversion and delegates directly to the same service. Transport code does not own business rules.

The adjacent generated protobuf file is the external RPC schema for the existing pinned standard protobuf/gRPC pipeline. `yunka domain` does not introduce reflection dispatch, a second RPC runtime, or service lookup.

## 7. One-line composition

`wire.Build(database)` is the generated composition seam:

```go
bundle, err := devicewire.Build(primaryDatabase)
```

It performs:

```text
final editable PO
      ↓
AutoMigrate
      ↓
RequestScope Factory
      ↓
Repository
      ↓
Application Service
      ↓
REST Handler / RPC Adapter
```

A yunka runtime module should consume this bundle instead of rebuilding repository, transaction, DTO, or transport wiring.

## 8. Regenerate and validate

Edit `domain.json` when changing generated domain/service/transport fields, then run:

```bash
yunka domain generate --path internal/device
yunka domain check --root internal
```

`domain generate`:

- regenerates framework-owned artifacts;
- removes obsolete framework-owned transport files;
- preserves developer-owned `po.go` and other ordinary source files;
- recreates the editable PO scaffold only if it is missing.

`domain check` fails on:

- project/database-prefix mismatch;
- invalid `<prefix>_<domain>_<object>` table naming;
- missing editable PO scaffold;
- generated drift;
- stale generated artifacts.

## 9. Supported generated field types

The initial manifest field types are:

```text
string
int64
uint64
bool
float64
time
```

The manifest sorts fields deterministically. Transport DTOs, application inputs, domain entities, generated PO base fields, REST bindings, RPC bindings, and protobuf fields all derive from the same field list.

## Final developer experience

A developer should decide only:

```text
project DB prefix        once
business domain
business object
business fields
business rules
authorization policy
workflows / invariants
```

Yunka owns the repeatable mechanics:

```text
project persistence convention
internal directory layout
PO base + editable extension seam
TableName
schema AutoMigrate
request transaction
repository construction
tenant filter
service input/output contract
REST DTO mapping
RPC DTO mapping
service delegation
composition wiring
generated drift checking
```

The boundary remains deliberate: Yunka removes repetitive engineering mechanics; developers still own domain invariants, calculations, authorization policy, workflows, and product-specific behavior.
