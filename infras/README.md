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

A program may enable an infrastructure plugin explicitly by registering its descriptor, or by blank-importing the plugin's `autoload` package:

```go
import _ "github.com/hvritual/yunka.io/infras/modules/outboxruntime/autoload"
```

An `autoload` package is descriptor-only. Its `init` function may only register `module.GeneratedDescriptor()` in `framework/core/modulecatalog`; it may not read configuration, perform I/O, construct resources, start goroutines, or capture request state.

This repository does not use Go's runtime `plugin` / `.so` mechanism. Yunka infrastructure plugins are compile-time optional modules so they retain normal Go type safety, deterministic dependency resolution, cross-platform builds, and the existing App lifecycle/health/diagnostics model.

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
