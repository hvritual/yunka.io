# C9 — Operation Contract Compiler & Unified Execution Runtime

## Status

- State: **Complete / merged**
- Original tracking: GitHub issue #30
- Original delivery: PR #31
- C9.1-C9.6 are now part of the active `main` baseline and were subsequently extended by C9.7 execution semantics and C9.8 canonical internal Operations.

## Objective

C9 turns protobuf Operation declarations into one immutable execution contract and one transport-neutral execution path.

```text
PB OperationDeclaration
        |
        v
Operation Contract Compiler
        |
        v
immutable OperationPlan
        |
        v
Unified Operation Executor
        |
        +--> Gateway security phase
        +--> Application invocation
        +--> terminal observation

REST / gRPC
        |
        +--> trusted transport context
        +--> decode
        +--> generated OperationPlan
        +--> Executor.Execute
        +--> encode
```

C9 does not move authorization into `framework`. `gateway/authz` remains the canonical authorization boundary. The framework owns deterministic execution orchestration only.

## Final C9.1-C9.6 invariants

1. Protobuf is the writable source of Operation intent.
2. Stable Operation ID is the canonical execution identity.
3. HTTP paths and gRPC full methods are transport bindings, never authorization identities.
4. Runtime never interprets protobuf descriptors/comments per request.
5. Protected Operations fail closed when the required security phase is unavailable.
6. One root Operation execution performs exactly one authorization decision.
7. Domain guards run after authorization and before Application; grant scope remains opaque to the framework.
8. Canonical generated REST and gRPC adapters invoke the same `framework/operation.Executor` with the same immutable `OperationPlan`.
9. Application code contains business use-case behavior only and does not repeat Permission/Role checks.
10. Execution semantics are explicit contract facts; they are not inferred from method names or HTTP verbs.
11. C7 bans remain permanent: no reflection DI, service locator, global mutable runtime registry, request pooling, or singleton Runtime mutation.

## C9.1 — OperationPlan IR

`pkg/operationplan` is the leaf-safe, data-only execution IR. C9.1 established deterministic normalization, validation, canonical JSON, SHA-256 digest, encode/decode and file loading without importing runtime/gateway state.

The C9.1 shape contained stable operation/application identity, security, composition, Application dependencies and transport bindings. C9.7 later extended the IR with explicit transaction/idempotency execution policy; C9.8 later made transport bindings optional for canonical internal Operations.

## C9.2 — Operation Contract Compiler

`pkg/contract.CompileOperationPlans` compiles normalized Contract Manifest facts into deterministic `operationplan.Set` evidence.

The compiler validates:

- unique `<domain>/<application>` capability identities;
- valid and resolvable Application dependencies;
- Application dependency cycles;
- typed Operation/Application method consistency;
- globally unique stable Operation IDs;
- required Operation existence;
- cross-Application Operation calls covered by declared Application capability dependencies;
- transitive Permission closure.

The contract artifact pipeline emits:

```text
contracts/generated/
├── manifest.json
├── openapi.json
├── client.ts
└── operation-plans.json
```

These files are deterministic derived evidence. Protobuf remains the writable source of contract intent.

## C9.3 — Unified Operation Executor

`framework/operation` provides a fixed semantic executor rather than a freely ordered middleware bag. C9 established the shared transport-neutral execution boundary and stable Operation metadata; C9.7 later expanded the fixed phase machine to own idempotency and ExecutionScope/transaction semantics.

`ExecuteTyped` preserves generated compile-time request/response typing while the executor core remains transport-neutral.

## C9.4 — REST/gRPC convergence

Canonical generated REST and gRPC adapters enter the same Executor/OperationPlan path.

Transport owns:

- request decoding;
- response encoding;
- trusted credential establishment;
- transport-specific metadata propagation.

Transport does not own a second Permission/Tenant/Guard execution model.

C9.8 additionally allows an Operation to have no REST/gRPC binding at all while remaining canonical and callable through typed local child capabilities.

## C9.5 — Gateway security cutover

`gateway/authz.NewExecutionSecurity` adapts immutable OperationPlan security facts to the canonical Gateway Authorizer:

```text
trusted Principal
  -> PolicyFromOperationPlan
  -> Authorizer.Authorize
  -> AuthorizedOperation context
  -> deterministic OperationGuard chain
  -> Application context
```

Permission Grants and opaque Scope values remain IAM/domain facts. Framework code does not learn Customer/Site/Device or SQL scope semantics.

## C9.6 — Plan-backed Graph and Diagnostics

OperationPlan contributes declared evidence for:

- Domain -> Application containment;
- Application dependencies;
- Application -> Operation containment;
- Operation dependencies;
- Operation -> Permission requirements;
- request/response message edges;
- explicit gRPC exposure;
- explicit HTTP route bindings.

Evidence is labeled and deterministic; architecture is not inferred from package or method naming.

Diagnostics expose safe bounded execution metadata only and do not expose Permissions, auth methods, Principal, tenant IDs, grant scopes, request payloads, credentials or secrets.

## Subsequent extensions

### C9.7 — Execution Semantics Closure

C9.7 added explicit transaction/idempotency policy, root ExecutionScope/UoW ownership, join-only request repository views, typed local child execution, Saga/Outbox transaction joining and durable MySQL-backed idempotency.

See `docs/waves/C9.7-execution-semantics-closure.md`.

### C9.8 — Canonical Internal Operations

C9.8 removed the false equivalence between Operation and RPC Method. Application-level internal Operations compile into the same OperationPlan/Application Graph/capability model without external transport exposure; internal-only DTOs do not leak into external OpenAPI/TypeScript projections unless transport-reachable.

See `docs/waves/C9.8-canonical-internal-operations.md`.

## Permanent verification

C9 established `make operation-check` and extended `make dsl-check` / `make verify`.

The active repository gate also covers C9.7/C9.8 regressions through architecture, contract, OperationPlan, ApplicationGraph, RPC, race, vet, vulnerability and build checks. Real MySQL verification remains `make verify-production`.

## Current boundary

C9.1-C9.8 runtime/product semantics are implemented. C9.9 is now the closure wave that reconciles exact `main` verification, docs, durable memory, Framework Pressure, GitHub issues and the real `hvritual/biz` consumer before C10 introduces any new framework surface.

Remaining open pressure is not an excuse for speculative expansion. In particular, Saga step topology evidence (FP-C9-005) remains deferred until repeated real operational pressure proves the need.
