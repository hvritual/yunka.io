# Generated C9 HTTP/gRPC semantic conformance

> Document class: **CURRENT**
> Source authority: canonical PB, OperationPlan and the existing C9 generators/Executor
> Evidence authority: executable `TestGeneratedHTTPBindingTransportParity` and Git-bound CI artifacts
> Tracking: [Issue #231](https://github.com/hvritual/yunka.io/issues/231)
> Delivery state authority: [STATUS](../STATUS.md)

## Semantic contract, not byte-for-byte transport comparison

Generated REST and gRPC adapters must preserve the same **Application-visible
protobuf request, Operation identity and authorization/idempotency outcome**
for the supported input. Successful payloads are compared after ProtoJSON
decoding into the canonical protobuf response, using `proto.Equal`, not
comparisons of HTTP JSON bytes against gRPC wire bytes.

For a rejected operation, each transport retains its documented external
representation. REST may expose HTTP 404 for application NotFound while gRPC
returns `codes.NotFound`; REST 409 and gRPC `codes.Aborted` can both
represent an in-progress idempotent claim. The status normalization policy
is owned by [REST Error Semantics](REST-ERROR-SEMANTICS.md), not copied into
a test-only error adapter.

Malformed HTTP percent-encoding, mixed query aliases, duplicate singular
query fields and unsupported PB/URL combinations have **no equivalent typed
gRPC request**. For these cases, the matrix explicitly records a null
equivalence dimension and tests either rejection before Application or
compile-time `UNSUPPORTED_HTTP_BINDING`. The test must not claim that
transports are equal where one is intentionally incapable of representing
the request.

## Existing canonical runtime exercised

One small, domain-neutral canonical PB fixture generates the real C9
REST and gRPC entrypoints. A real `httptest.Server` and in-memory
`bufconn` gRPC server use the same controlled test Application and
one canonical `operation.Executor`, plus real `authz.ExecutionSecurity`.

The suite covers:

- Repeated GET values under proto/JSON names, original ordering,
  duplicates and empty strings; omitted fields and scalar/base64 width.
- Path ownership over query and body; body:`*` ownership of non-path fields.
- Invalid/overflow/ambiguous/malformed input rejected before Application.
- Unknown capability and more than 128 values reaching *both* Application
  validations, with identical request semantics.
- NotFound, Aborted, AlreadyExists, Internal, PermissionDenied and plain
  errors classified under each transport without leaking details.
- Unauthenticated and cross-tenant denial before any Application call.
- An explicitly idempotent `Change` Operation. A deterministic
  `execution.MemoryIdempotencyStore` through the *real* coordinator
  proves required key, in-progress and completed claim suppression.
  Different initial keys are used for HTTP and RPC to prevent the test
  itself from manufacturing a duplicate across first executions.
- A **test-only Executor fault injector** asserts safe failure
  for the framework unavailable sentinel. It cannot generate a
  successful Application result or circumvent the security path.
- Unsupported URL enum/message/map/oneof, repeated path and named-body
  compiler combinations (proven as compiler negatives, not runtime success).

This suite is not a production credential verifier, real Consumer
subscription/provisioning path, MySQL substitute or independent Runtime.
The separate `make verify-production` still owns MySQL integration.

## Machine-readable and human-readable proof

The required `make dsl-check` executes the real generated fixture. It emits
one machine result for each executed case; the parent test **requires an
exact set of named cases** and rejects missing/duplicated, unsupported-as-
supported, negative-equivalence or unsafe diagnostic records. Compilation
negatives are independently derived from the same compiler.

The resulting `schemaVersion: 1` JSON report includes:

- `candidateSHA`, `candidateTree`: exact checked-out Git commit/tree.
- `protobufSHA256`, `descriptorSHA256`: source fixture and real protoc
  compiled canonical descriptor identities.
- `generatedSHA256`, `generatedFileCount`: deterministic SHA-256 over
  the ordered paths and contents of the actual C9 generated files, checked
  against a second generation.
- `cases[]`: `case`, `supported`, `request-equivalent`,
  `response-equivalent`, `httpStatus`, `grpcStatus` and `diagnostic`.
  JSON null means a comparison is **not applicable**, not a success.
- `summary` and `caseCount` for human review.

The report is written only to `$RUNNER_TEMP/yunka-protocol-conformance/matrix.json`
or the explicit `YUNKA_CONFORMANCE_EVIDENCE_DIR` (not into tracked source).
PR and main CI plus Production independently validate the file against the
**actual checkout commit/tree** and upload artifact
`c9-http-grpc-semantic-matrix`. The readback verifier
`tools/verify_http_grpc_conformance.py` only validates evidence provenance,
shape and integrity; it does not own a second runtime protocol policy.

Missing or malformed reports, omitted cases, false equivalence, absent
unsupported negatives or exact Git ref mismatch fail delivery. The
report-validator mutation tests deliberately exercise these denials.

## Limits and controlled follow-ups

The supported URL representation boundary remains
[HTTP Binding Semantics](HTTP-BINDING-SEMANTICS.md); this Issue only
adds continuing qualification. New unrepresentable protobuf combinations
should fail at compile time, not be introduced as a silent second binder.
Generic Consumer version/pin qualification, third clean project and a
full protocol/version matrix remain independently scoped to the
parent roadmap (#235). Historical real Biz incidents and their exact
old-base reproductions support the defect lineage, but **do not mean
Biz has upgraded to the latest Yunka main** or that a business
lifecycle passed production E2E.
