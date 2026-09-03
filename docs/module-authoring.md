# Typed module authoring

C7.4 makes the compiler-checked module path the ordinary development workflow.

## Create a module

Run the command from any workspace directory and point `--root` at a directory owned by the intended Go module:

```bash
cd app
go run ./cmd module new \
  --root ../framework/modules \
  --name billing \
  --database primary \
  --rpc inventory \
  --event-bus \
  --depends-on tenant
```

Infrastructure-extension modules belong under the separately versioned `infras` Go module rather than expanding `framework/infras`. Use `../infras/modules` as the generation root when the capability is an optional reusable infrastructure plugin:

```bash
cd app
go run ./cmd module new \
  --root ../infras/modules \
  --name cache \
  --logger
```

The generator writes atomically:

```text
modules/billing/
├── config.go
├── dependencies.go
├── module.go
├── zz_yunka_module_gen.go
└── autoload/
    └── register.go
```

`zz_yunka_module_gen.go` is generated and must not be edited manually. Configuration, dependency contracts, and business behavior remain ordinary reviewed Go source.

## Capability rules

A module may receive only capabilities declared by its immutable descriptor:

- one configuration key;
- a module-scoped logger;
- named GORM databases;
- the App-owned EventBus;
- named gRPC connections;
- explicit module dependencies.

The module must not acquire DSNs, TLS material, root configuration, environment values, database/RPC factories, a global App, or another service at runtime. Database and RPC construction belongs to `platform.Provider`; request transactions and repositories belong to `requestscope`.

## Enable a module

Import the generated autoload package once:

```go
import _ "yunka.io/framework/modules/billing/autoload"
```

A separately versioned infrastructure plugin uses the same runtime contract and a different distribution module, for example:

```go
import _ "github.com/hvritual/yunka.io/infras/modules/outboxruntime/autoload"
```

The autoload package may only call:

```go
modulecatalog.MustRegister(module.GeneratedDescriptor())
```

It may not read configuration, perform I/O, start goroutines, or construct resources. `infras` does not introduce Go `.so`/runtime-plugin loading; infrastructure plugins remain normal compile-time Go dependencies so module identity, type checking, dependency resolution, lifecycle, and diagnostics stay deterministic.

## Validate

```bash
make module-check
make verify
```

`module-check` validates generated structure and autoload purity. The C7 architecture policy also rejects dependency acquisition, reflection composition, global lookups, and request-lifetime pools under module roots. Repository architecture tests additionally enforce the `framework -> infras` dependency prohibition and descriptor-only autoload packages under `infras/modules`.

## Lifecycle

App-owned modules may implement `Start`, `Health`, and `Shutdown`. They must remain stateless with respect to individual requests. Request identity, metadata, trace state, transactions, and repositories must remain in `request.Context` and `requestscope`.

`framework/modules/outboxruntime` is the canonical Outbox lifecycle implementation. `infras/modules/outboxruntime` initially exposes it as a separately versioned plugin facade, preserving one implementation and one descriptor while establishing the new infrastructure distribution boundary.

## Infras ownership rule

Use `infras` for optional reusable infrastructure capability distributions and adapters. `framework` remains the transport-neutral execution/lifecycle/security/contract runtime and must not import `infras`; dependencies point from infrastructure plugins toward stable framework contracts, never the reverse.

Existing `framework/infras/**` packages are compatibility/internal surfaces and are not automatically migrated. New public Redis/cache, object-storage, search, background-job, broker, or similar capabilities should start in `infras` only after a stable generic boundary is proven.

## Migration checklist

1. Declare every capability in `GeneratedDescriptor`.
2. Accept only compiler-checked `Dependencies`.
3. Remove environment/global/service-locator access.
4. Keep App-owned services free of current-request state.
5. Add two-App isolation and lifecycle tests.
6. Preserve existing protobuf and persistence contracts.
7. Run double tidy, module/architecture/dependency gates, RPC/Contract zero-drift, race, full verify, and relevant MySQL integration.

## Rollback

Each C7.4 wave is one formal commit. Revert the corresponding wave commit. Do not restore the C7.3 legacy Runtime, reflection container, or service locator as a compatibility fallback.
