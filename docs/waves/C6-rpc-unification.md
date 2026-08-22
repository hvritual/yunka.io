# C6 — RPC Unification

## Status

Implementation started over `main@0408c860671e5d5b464d04ea73505680fc6a1609`.

C6 replaces the historical generator/runtime without requiring edits to existing framework Service business methods. The migration preserves the business-facing Go ABI while replacing generated implementation and transport internals.

## Final invariant

```text
contracts/proto/**
    -> one canonical inventory
    -> pinned protoc
    -> pinned protoc-gen-go
    -> pinned protoc-gen-go-grpc
    -> modern typed protobuf/gRPC output
    -> compatibility facades only where source ABI requires them
```

There is one protobuf fact model and one grpc-go runtime. Compatibility code may preserve existing import paths and method signatures, but it may not generate messages, descriptors, network transports, service locators, reflection injection, object pools, or hidden registration.

## Subwaves

### C6.0 — Service ABI Freeze

- freeze existing Service method signatures and message field/getter usage;
- retain C5 contract manifest as the semantic compatibility baseline;
- reject business coupling to `XXX_*`, old descriptor registration, generated factory maps, and HTTP-only Runtime behavior in RPC methods;
- require consumer compilation with business source hashes unchanged before final C6 completion.

### C6.1 — Standard Generation Foundation

- make `contracts/proto` the only canonical inventory;
- preserve protobuf package names, field numbers, enum numbers, service names, and method names;
- preserve `yunka.io/gateway/rpc/meta` for existing business imports;
- lock `protoc-gen-go` and `protoc-gen-go-grpc` exactly;
- add deterministic `make rpc-generate` and read-only `make rpc-check`;
- replace gateway legacy protobuf output with modern generated output;
- keep the old XR wrappers only until typed compatibility bridges land.

### C6.2 — Typed Compatibility Bridge

- adapt standard typed gRPC service interfaces to the existing Service ABI;
- preserve legacy client method signatures through a typed facade;
- map `*Special(..., nodeIP)` to an explicit target-aware client factory;
- carry trusted identity, metadata, tracing, deadlines, transaction hooks, and finish hooks through an explicit Runtime bridge;
- use `bufconn` or typed fakes instead of generated memory transport.

### C6.3 — Atomic Legacy Removal

Delete `app/cmd/rpc`, `gateway/rpc/gender.sh`, all `*.xr_*.go`, generated memory transport, generated method/handle/server registries, message `sync.Pool`, old protobuf RPC imports, and obsolete compatibility allowlists. No dual generator remains.

### C6.4 — Consumer Proof

Compile real framework Service consumers without modifying business files, prove old/new wire interoperability, and pass all production gates.

## Foundation commands

```bash
make rpc-generate
make rpc-check
make rpc-compat-check
make verify
```

`rpc-generate` is the only mutating RPC generation command. `rpc-check` is read-only and fails on drift. Normal CI never repairs or commits generated files.

## C6.2 implementation contract

C6.2 introduces three narrow seams:

```text
authenticated gRPC context
        -> RuntimeFactory
        -> Provider[GatewayBusinessService]
        -> unchanged Service method
        -> FinishRequest / release exactly once
```

```text
historical GatewayServiceClient methods
        -> typed client facade
        -> standard generated GatewayServiceClient
        -> default or explicit-target client factory
```

```text
grpc.Server
        -> typed RegisterGatewayService
        -> standard generated registration
```

The current module container is visible only through `ServicePool.GetService/PutService` inside `ModuleGatewayProvider`. The bridge copies trusted Principal, transport metadata, and trace state into `WorkRuntime`, clears the Service runtime before returning it, and contains release panics. No reflection, package `init`, handler registry, or `sync.Pool` is introduced by the bridge.

The historical constructor accepting `invoke.RpcClient` remains source-compatible during C6.2 and is isolated behind a unary-only `grpc.ClientConnInterface` adapter. New composition uses `NewGatewayServiceClientWithFactory`; per-target connection ownership belongs to the injected factory. C6.3 removes the legacy constructor transport, remaining XR files, and generated memory dispatch before the final C6 commit.

## C6.3/C6.4 completion contract

C6.3 performs an atomic deletion of the old generator module, duplicate protobuf roots, all XR output, memory dispatch, string service registration, message factories/pools, and the legacy invoke client/server abstraction. grpc-go is the only network and local-test runtime.

C6.4 protects consumers at three levels:

1. semantic contract compatibility remains non-breaking against the C5 baseline;
2. the actual `RoleIntercept` RPC method bodies are normalized and SHA-256 locked, while compile-time assertions prove they implement the standard generated server interface without edits;
3. an external-package fixture with the historical `core.BaseService` plus method-signature shape runs through the typed provider, standard server, `bufconn`, and handwritten client facade.

`make rpc-consumer-check` and `make rpc-legacy-check` are permanent, read-only gates. No fallback generator or alternate dispatcher is retained.
