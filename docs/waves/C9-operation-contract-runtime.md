# C9 — Operation Contract Compiler & Unified Execution Runtime

## Status

- State: **Implementation / validation**
- Baseline: `main@ed3c73b7899cd4a818d2a771006befa7bf9a5085`
- Tracking: GitHub issue #30
- Delivery branch: `agent/c9-operation-contract-runtime`

## Objective

C9 turns the C8.4-C8.7 protobuf Operation declarations into one immutable execution contract and one transport-neutral execution path.

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

## Final invariants

1. Protobuf remains the writable source of Operation intent.
2. Stable PB Operation ID is the canonical execution identity.
3. HTTP paths and gRPC full methods are transport bindings, never authorization identities.
4. Runtime never interprets protobuf descriptors/comments per request.
5. Protected Operations fail closed when the required security phase is unavailable.
6. One Operation execution performs exactly one authorization decision.
7. Domain guards run after authorization and before Application; grant scope remains opaque to the framework.
8. Canonical generated REST and gRPC adapters invoke the same `framework/operation.Executor` with the same immutable `OperationPlan`.
9. Application code contains business use-case behavior only and does not repeat Permission/Role checks.
10. C9 does not infer transactions, idempotency, audit, data predicates, or workflow semantics from method names or HTTP verbs.
11. C7 bans remain permanent: no reflection DI, service locator, global mutable runtime registry, request pooling, or singleton Runtime mutation.

## C9.1 — OperationPlan IR

`pkg/operationplan` is a leaf-safe, data-only execution IR.

```text
OperationPlan
├── operationId
├── domain
├── application
├── useCase
├── requestType
├── responseType
├── security
│   ├── public
│   ├── tenantRequired
│   ├── authentication[]
│   ├── permissions[]
│   └── permissionMode
├── composition
│   ├── boundary
│   ├── requiresOperations[]
│   └── permissionClosure[]
├── applicationRequires[]
└── bindings
    ├── rpc
    └── http[]
```

The package owns deterministic normalization, validation, canonical JSON, SHA-256 digest, encode/decode, and file loading. It imports neither `framework` nor `gateway` and contains no handlers, service instances, database types, or runtime state.

Validation rejects duplicate/empty Operation IDs, unsupported permission/composition modes, unknown/self operation dependencies, dependency cycles, missing transitive permission closure, and incomplete execution identity/binding facts.

## C9.2 — Operation Contract Compiler

`pkg/contract.CompileOperationPlans` compiles normalized Contract Manifest facts into `operationplan.Set`.

It validates:

- unique `<domain>/<application>` capability identities;
- valid and resolvable Application dependencies;
- Application dependency cycles;
- one typed Operation declaration for every typed Application method;
- globally unique stable Operation IDs;
- required Operation existence;
- cross-Application Operation calls are covered by declared Application capability dependencies;
- C8.7 transitive Permission closure.

The contract artifact pipeline now emits:

```text
contracts/generated/
├── manifest.json
├── openapi.json
├── client.ts
└── operation-plans.json
```

`operation-plans.json` is deterministic derived evidence. PB remains the source of truth.

## C9.3 — Unified Operation Executor

`framework/operation` introduces one executor with fixed semantic slots rather than a freely ordered middleware bag.

Foundation order:

```text
plan
  -> runtime metadata
  -> security
  -> application
  -> outcome
```

The executor:

- writes the stable Operation ID into `runtimecontext.Metadata.Operation`;
- calls the configured SecurityPhase once;
- fails closed for a protected plan when SecurityPhase is absent;
- invokes Application once;
- emits bounded phase/outcome observations;
- contains observer panics;
- observes and re-panics Application panics;
- does not open transactions or infer persistence semantics.

`ExecuteTyped` preserves generated compile-time Application request/response typing while the executor core remains transport-neutral.

## C9.4 — REST/gRPC convergence

`RenderC9ApplicationCode` is the canonical typed Application generator used by `yunka contract generate`.

It retains C8 non-transport compatibility artifacts such as Application Port, capability ports, and static policy metadata, but filters the old C8 REST/RPC runtime files. Canonical transport artifacts are now:

```text
<domain>/transport/rest/zz_yunka_<application>_operation_executor_gen.go
<domain>/transport/rpc/zz_yunka_<application>_operation_executor_gen.go
```

Both call:

```go
operation.ExecuteTyped(ctx, executor, policy.OperationPlanX(), request, application.X)
```

Transport remains responsible for request decoding, response encoding, and trusted credential establishment. It does not own Permission/Tenant/Guard sequencing.

A real protoc fixture proves one Application invoked through REST and gRPC traverses the same Executor path and stable Operation identity. Denied calls do not reach Application.

## C9.5 — Gateway security cutover

`gateway/authz.NewExecutionSecurity` adapts `OperationPlan.Security` to the existing canonical Gateway Authorizer.

The phase performs:

```text
trusted Principal
  -> PolicyFromOperationPlan
  -> Authorizer.Authorize
  -> AuthorizedOperation context
  -> deterministic OperationGuard chain
  -> Application context
```

Permission Grants and opaque Scope values remain IAM/domain facts. The framework does not learn Customer/Site/Device or SQL scope semantics.

C8 compatibility seams remain explicitly bounded:

- `ExecutorFromOperationRuntime` allows an older caller to enter the Executor while it still supplies a C8 `OperationRuntime`;
- `PreauthorizedExecutor` supports an existing secured gRPC interceptor without evaluating authorization a second time.

They are compatibility adapters, not new composition APIs. New generated C9 transports use the direct Executor/SecurityPhase path.

## C9.6 — Plan-backed Graph and Diagnostics

`pkg/applicationgraph.AddOperationPlans` consumes the compiled plan set as declared execution evidence.

The graph receives deterministic facts for:

- Domain -> Application containment;
- Application dependencies;
- Application -> Operation containment;
- Operation dependencies;
- Operation -> Permission requirements;
- request/response message edges;
- explicit gRPC binding/service exposure;
- explicit HTTP route bindings.

Evidence is labeled `declared` with source `operation.plan`; no relationships are inferred from package names or grep.

`yunka graph build` loads `contracts/generated/operation-plans.json` in addition to the contract manifest.

`framework/diagnostics.OperationSource` exposes only safe execution metadata:

- plan schema version and digest;
- Operation ID, Domain, Application, composition class, protected/public classification;
- whether an Executor is bound;
- canonical executor phase list and observer count.

It does not expose Permissions, auth methods, Principal, Tenant IDs, grant scopes, request/response types, request payloads, credentials, or secrets.

## Explicit non-goals

C9.1-C9.6 do not introduce:

- transaction policy declarations;
- operation idempotency stores;
- security audit persistence;
- cache policy;
- workflow/BPMN runtime;
- distributed database transactions;
- generic data-scope models;
- SQL/data predicates in PB;
- automatic business-semantic inference;
- reflection dispatch or service location.

Those mechanisms may only enter later execution-policy extensions after real `biz` use cases prove the requirement.

## Permanent verification

C9 adds `make operation-check` and extends `make dsl-check` / `make verify`.

Required regression coverage includes:

- deterministic OperationPlan canonical JSON/digest;
- duplicate/unknown/cycle/permission-closure rejection;
- compiler determinism and cross-Application capability validation;
- fail-closed Executor behavior and exact phase sequencing;
- Gateway Authz SecurityPhase with one authorization and one guard pass;
- canonical C9 generator contains no legacy REST/RPC transport output;
- real protoc REST/gRPC Executor parity fixture;
- denied REST/gRPC requests never reach Application;
- OperationPlan-backed Application Graph evidence;
- diagnostics privacy boundary;
- C7/C8 architecture/authz/composition regressions;
- dependency, contract, RPC, test, race, vet, vuln, build, and determinism gates.

## Production gate

The standard CI gate is `make verify`. Where MySQL 8.4 is available, final production verification remains:

```bash
make verify-production
```

C9.1-C9.6 do not change requestscope transaction or Saga/Outbox semantics, so the existing MySQL integration suites remain the production regression authority for those mechanisms.

## Next boundary

C9.7 may add explicit execution-policy declarations only after C9.1-C9.6 are stable and reference `biz` vertical slices demonstrate repeated framework-level pressure. Candidate mechanisms are transaction policy, operation idempotency, and security audit, but none may be inferred automatically.
