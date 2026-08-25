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

The autoload package may only call:

```go
modulecatalog.MustRegister(module.GeneratedDescriptor())
```

It may not read configuration, perform I/O, start goroutines, or construct resources.

## Validate

```bash
make module-check
make verify
```

`module-check` validates generated structure and autoload purity. The C7 architecture policy also rejects dependency acquisition, reflection composition, global lookups, and request-lifetime pools under module roots.

## Lifecycle

App-owned modules may implement `Start`, `Health`, and `Shutdown`. They must remain stateless with respect to individual requests. Request identity, metadata, trace state, transactions, and repositories must remain in `request.Context` and `requestscope`.

`framework/modules/outboxruntime` is the reference lifecycle module: it declares Config + Logger + named DB + EventBus, owns one Dispatcher per App, and leaves transaction ownership with the existing outbox/request-scope APIs.

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
