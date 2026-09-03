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

## Module input requirements

A module may receive only platform inputs declared by `Descriptor.Requirements`:

- one configuration key;
- a module-scoped logger;
- named GORM databases;
- the App-owned EventBus;
- named gRPC connections;
- explicit module dependencies.

The module must not acquire DSNs, TLS material, root configuration, environment values, database/RPC factories, a global App, or another service at runtime. Database and RPC construction belongs to `platform.Provider`; request transactions and repositories belong to `requestscope`.

## Export a typed process/App capability

`Descriptor.Requirements` describes what a module consumes. `Descriptor.Provides` describes what a built module exposes to Application constructors. They point in opposite directions and must not be conflated.

Define a stable interface contract and typed key in a package importable by both provider and consumer:

```go
var CacheDefault = modulecatalog.MustCapabilityKey[cachecontract.Cache](
    "cache.default",
    "example.com/contracts/cache",
    "Cache",
)
```

The current declarative `module.yunka.json` schema does not yet contain a `Provides` field. Do not edit generated `zz_yunka_module_gen.go`; wrap the generated descriptor in handwritten Go instead:

```go
func CapabilityDescriptor() modulecatalog.Descriptor {
    descriptor := GeneratedDescriptor()
    descriptor.Provides = []modulecatalog.CapabilityContract{
        CacheDefault.Contract(),
    }
    return descriptor
}

func (module *Module) ExportCapabilities() []modulecatalog.CapabilityExport {
    return []modulecatalog.CapabilityExport{
        CacheDefault.Export(module.cache),
    }
}
```

The wrapper preserves generated input requirements/build wiring while making the output contract explicit and reviewable. Descriptor promises and runtime exports must match exactly.

## Consume a typed capability from an Application

Application protobuf declares infrastructure constructor dependencies separately from cross-Application `requires` and Operation `requires_operations`:

```protobuf
option (yunka.dsl.v1.application) = {
  name: "query"
  capabilities: {
    name: "cache.default"
    go_package: "example.com/contracts/cache"
    go_type: "Cache"
  }
};
```

`yunka generate` carries that fact through Manifest/AssemblyPlan and generates a concrete Go interface field such as `CacheDefault cachecontract.Cache` in the Application dependency struct. The consumer-owned `ApplicationFactories` implementation receives that typed field; Application runtime code never receives `CapabilitySet`.

Generated Assembly composes separately versioned provider descriptors explicitly:

```go
AdditionalModules: []modulecatalog.Descriptor{
    rediscache.CapabilityDescriptor(),
}
```

Resolution is bootstrap-only and fail-closed before transport registration/App start. There is no `app.Get(string)`, reflection DI, or public untyped lookup.

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

For generated Assembly, typed exporting plugins must be supplied through `BootstrapOptions.AdditionalModules`; generated Assembly deliberately does not discover the package-global/default catalog. Until declarative module specs can synthesize `Descriptor.Provides`, the generated `autoload` package is suitable for lifecycle-only descriptors but does not replace the explicit capability descriptor path described above.

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

1. Declare module inputs in `Descriptor.Requirements` and exported process/App capabilities in `Descriptor.Provides`.
2. Export only declared values through `CapabilityExporter`; keep provider construction compiler checked.
3. Declare each consuming Application capability explicitly in protobuf with matching logical key and Go package/type identity.
4. Remove environment/global/service-locator access.
5. Keep App-owned services free of current-request state.
6. Add missing/duplicate/mismatch and two-App isolation tests.
7. Preserve existing protobuf, Operation/UoW/authz, and persistence semantics.
8. Run double tidy, module/architecture/dependency gates, RPC/Contract zero-drift, race, full verify, and relevant MySQL integration.

## Rollback

Each C7.4 wave is one formal commit. Revert the corresponding wave commit. Do not restore the C7.3 legacy Runtime, reflection container, or service locator as a compatibility fallback.
