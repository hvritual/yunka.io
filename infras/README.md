# Yunka Infras

`github.com/hvritual/yunka.io/infras` is the separately versioned infrastructure-extension module for Yunka applications.

## Boundary

`infras` owns optional, reusable infrastructure capabilities and their Yunka module adapters. It is not a second application framework and it does not own business domains.

Dependency direction is one-way:

```text
application
   ↓
infras plugin
   ↓
framework stable contracts/runtime
```

`framework` must not import `infras`. This keeps the core runtime independent from optional infrastructure distributions and allows `infras` to evolve and release on its own module/tag lifecycle.

Existing `framework/infras/**` paths are compatibility/internal surfaces for now. This module does not migrate or delete them in the initial introduction. New public infrastructure capabilities should target the root `infras` module instead of expanding `framework/infras`.

## Versioning

The Go module path is:

```text
github.com/hvritual/yunka.io/infras
```

Releases use the module tag namespace:

```text
infras/vX.Y.Z
```

This is the same multi-module versioning model used by `gateway`: the module has an independent import path and tag namespace even when a release train chooses the same semantic version as other Yunka modules.

## Plugin model

Infrastructure capabilities use Yunka's existing typed module catalog. There is no second plugin runtime.

A composition path that intentionally uses the default module catalog may enable a lifecycle-only infrastructure plugin by registering its descriptor explicitly or blank-importing the plugin's `autoload` package:

```go
import _ "github.com/hvritual/yunka.io/infras/modules/outboxruntime/autoload"
```

An `autoload` package is descriptor-only. Its `init` function may only register `module.GeneratedDescriptor()` in `framework/core/modulecatalog`; it may not read configuration, perform I/O, construct resources, start goroutines, or capture request state.

This repository does not use Go's runtime `plugin` / `.so` mechanism. Yunka infrastructure plugins are compile-time optional modules so they retain normal Go type safety, deterministic dependency resolution, cross-platform builds, and the existing App lifecycle/health/diagnostics model.

## Typed capability export and consumer binding

Infrastructure modules that must be called directly by Application code use the typed capability contract rather than a service locator.

The provider defines one stable Go interface contract and a typed capability key. The package/type strings must be the real import identity of that contract:

```go
var CacheDefault = modulecatalog.MustCapabilityKey[cachecontract.Cache](
    "cache.default",
    "example.com/contracts/cache",
    "Cache",
)
```

Its composed descriptor declares the export and its built module instance implements `CapabilityExporter`:

```go
func CapabilityDescriptor() modulecatalog.Descriptor {
    descriptor := GeneratedDescriptor() // optional generated base descriptor
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

Do not edit `zz_yunka_module_gen.go`. The current declarative `module.yunka.json` schema describes module input `Requirements` but does not yet generate `Descriptor.Provides`, so an exporting plugin currently adds the `Provides` wrapper in handwritten Go.

The consuming Application declares the same logical key and Go interface identity in protobuf:

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

Assembly generation projects that declaration into a normal typed dependency field. Consumer-owned factory code receives the generated dependency and passes it into the Application implementation:

```go
func (factory factories) BuildDeviceQuery(
    dependencies generatedassembly.DeviceQueryDependencies,
) (deviceapplication.QueryApplication, error) {
    return newDeviceQuery(dependencies.CacheDefault), nil
}
```

For generated Assembly, separately versioned infrastructure implementations are explicit composition inputs:

```go
result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{
    Platform: platformProvider,
    AdditionalModules: []modulecatalog.Descriptor{
        rediscache.CapabilityDescriptor(),
    },
    Factories: factories{},
    Executor: executor,
    Transports: transports,
})
```

Generated Assembly does not discover optional infrastructure from blank imports. A blank-imported generated `autoload` package also cannot add typed exports that are absent from the generated descriptor, so exporting plugins should use the explicit descriptor path above until declarative `Provides` authoring is added.

At bootstrap, Yunka validates descriptor/export equality and provider uniqueness, snapshots an immutable `CapabilitySet`, resolves the generated typed constructor dependency, and discards the resolver from business runtime. Missing, duplicate, undeclared, mismatched, or non-assignable capabilities fail before transport registration/App start. Provider module/App lifecycle ownership is unchanged, and separate App instances receive isolated capability values.

## Initial plugin

`infras/modules/outboxruntime` is the first public infrastructure-plugin facade. It delegates to the canonical `framework/modules/outboxruntime` implementation and descriptor, so adopting the new import surface does not create a second Outbox runtime or change transaction/event semantics.

This facade establishes the distribution boundary first. Moving implementation ownership out of `framework` is a separate compatibility decision and is intentionally not part of the initial module introduction.

## Future candidates

Future capabilities may include, when real consumer pressure proves a stable generic boundary:

- cache/Redis adapters;
- object storage;
- search/indexing;
- background job runtime adapters;
- additional messaging/broker adapters;
- other process-scoped infrastructure capabilities.

Business audit policy, tenant/customer/device models, workflow/BPMN, business data-scope rules, and domain-specific semantics do not belong in this module merely because an operations platform needs them.

## Verification

The root repository gates include this module in `test`, `race`, `vet`, `vuln`, `tidy`, and `build`. Dependency policy and CI drift checks also track `infras/go.mod` and `infras/go.sum`.

Before an `infras/vX.Y.Z` release, run the repository qualification gates and `make module-release-check` against the intended released tag set.
