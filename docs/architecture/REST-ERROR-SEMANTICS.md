# C9 REST Operation error semantics

> Document class: **DECISION**
> Semantic owner: `gateway/httpbinding.WriteOperationError`
> Tracking: [Yunka Issue #229](https://github.com/hvritual/yunka.io/issues/229)
> Current delivery/status authority: [`docs/STATUS.md`](../STATUS.md)

## Problem and design choice

The canonical C9 generated REST adapter previously copied error policy into
generated Go source. It handled known authorization and idempotency failures,
then mapped gRPC `Aborted` / `AlreadyExists` to HTTP 409, but fell back to
HTTP 400 for `NotFound`. As a result, a legitimate application status was
misclassified as a request error. Repeating a generator string branch for
each new status would give many generated files independent copies of a
policy that belongs to the HTTP transport boundary.

The **single current mapping authority** is the existing gateway package
`httpbinding`, exposed as
`WriteOperationError(http.ResponseWriter, error)`. C9 generated handlers
retain their existing private `writeOperationError` symbol for source
layout compatibility, but it delegates to this API. The generated REST
adapter does not own any application status switch, raw error text or new
security decision.

This is a versioned Go API through the existing `gateway` module; it is
not an additional persisted protocol DSL, Runtime or alternative Execution
Security. The RPC adapter continues to use its existing
`gateway/rpc/transport/grpc.OperationError` mapping. Both adapt the same
canonical `operation.ExecuteTyped` results.

## Priorities and public status/text contract

The first matching row wins. Public error bodies are fixed strings written
using Go `http.Error` (`text/plain; charset=utf-8` plus final newline),
preserving the existing C9 response format.

| Priority | Error owner / condition | HTTP status | Public body |
| --- | --- | --- | --- |
| 1 | Trusted `authz.DeniedError` with `ReasonUnauthenticated` or `ReasonAuthenticationMethod` | 401 | `Unauthorized` |
| 1 | Other `authz.IsDenied` decisions or denied sentinel | 403 | `Forbidden` |
| 2 | Framework `execution.ErrIdempotencyKeyRequired` | 400 | `idempotency key required` |
| 2 | Framework `execution.ErrIdempotencyInProgress` or `ErrIdempotencyCompleted` | 409 | `idempotency conflict` |
| 3 | Framework `operation.ErrExecutorUnavailable`, `ErrSecurityUnavailable`, `ErrSecurityNilContext` or `ErrIdempotencyUnavailable` | 500 | `operation execution unavailable` |
| 4 | Explicit application gRPC `codes.NotFound` | **404** | `application resource not found` |
| 4 | Explicit application gRPC `codes.Aborted` / `codes.AlreadyExists` | 409 | `application conflict` |
| 5 | `codes.InvalidArgument`, unrecognized/unsupported gRPC codes and all ordinary Go errors | 400 | `application request failed` |

An explicit Go sentinel is tested with `errors.Is` and wins even if an
application status is also present in a joined or wrapped error. Only the
standard gRPC status identity is recognized; a string resembling a
`NotFound` error **cannot** cause HTTP 404.

**Trust boundary:** raw `error.Error()`, `status.Message()`, resource
identifiers, tenant details and internal causes never reach clients.
In particular, application-origin `codes.PermissionDenied` and
`codes.Unauthenticated` alone do *not* become authoritative HTTP 403/401:
only the canonical authorization decision from `authz.IsDenied` has that
authority. Error mapping does not grant access, return success or change
transactions.

The older default of HTTP 400 remains deliberate for unknown application
errors, including plain Go errors and other gRPC statuses. This narrow
change does not invent a global gRPC/HTTP status translation matrix.
Changing the fallback or adding a new code requires a separate reviewed
protocol compatibility change and real Consumer qualification.

## Observable compatibility delta

- Intended change: a trusted application `codes.NotFound` returned from
  the canonical Executor now yields HTTP 404 instead of HTTP 400.
- Unchanged: authorization 401/403, idempotency 400/409, application
  conflict 409, framework unavailable 500, unknown application 400.
- Unchanged: the `http.Error` text response framing, HTTP route and
  request binding, generated C9 port/registration names, RPC error
  behavior, trust/authorization/tenant/UoW authority and generated ownership.
- Potential consumer impact: a client that previously interpreted
  `NotFound` as invalid input 400 must now handle 404 as absent
  resource/subscription. Do not silently translate unrelated 400 into 404.

## Permanent evidence

`gateway/httpbinding/operation_error_test.go` owns a data-driven status,
precedence, wrapping, error-string spoofing and no-leakage matrix.

`pkg/contract/c9_application_codegen_test.go` asserts exactly one
delegation call in the generated REST adapter and rejects inlined status
logic. The required real generated
`TestGeneratedHTTPBindingTransportParity` test also exercises HTTP
status delivery against the same generated gRPC/Executor/Application
fixture, including NotFound 404 and retained 409/400 semantics.

A historical Biz first-subscription 400/404 failure is a pressure
example, not evidence that Biz has consumed this new current-main
framework or passed full business/E2E validation. Biz and maintained
compatibility branches have separate SHAs, security requirements and
release/approval decisions. Specific run receipts belong to Issue
#229 and the PR, not to this durable decision contract.
