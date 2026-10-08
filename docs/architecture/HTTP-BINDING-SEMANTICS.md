# Typed HTTP parameter semantics

> Document class: **CURRENT**
> Authority: canonical protobuf and the compiler in `pkg/contract`
> Delivery/qualification state: [STATUS.md](../STATUS.md), issues #177 and #230

## One derived binding plan

The compiler derives URL/body ownership from the existing protobuf request and
explicit HTTP binding. The plan is temporary compiler data, not another writable
DSL or runtime field registry. Typed compatibility and canonical Executor REST
adapters share the same assignment emitter. Lint, typed OpenAPI and TypeScript
projection reject the same unsupported combinations before publishing them.
Untyped descriptive inventories retain their legacy schema and output contract.

| Input source | Supported mapping |
| --- | --- |
| Path | Whole-segment scalar variables, using the exact template variable name |
| Query | Non-path scalar or repeated scalar fields when no body is declared |
| `body: "*"` | Existing ProtoJSON request decoding; all non-path fields belong to the body |

The path wins when the body contains a different value for the matched resource
field. Query values never overwrite a path field. A whole-body binding has no
query-owned request fields; extra query keys do not replace body values.
This is field ownership, not authorization: tenant/user query or body values
cannot establish a trusted Principal. The unchanged Executor/security boundary
retains that authority.

## Query encoding and failure behavior

The JSON name from protobuf is the published query name; its protobuf source
name is also accepted. Custom `json_name` follows the same rule. Supplying both
aliases for one field is rejected as ambiguous rather than applying an implicit
merge or precedence. Singular fields accept at most one occurrence.

Repeated fields use repeated keys, not comma splitting. Values preserve their
order, duplicates and empty string entries. Absence remains absence. The binder
does not own a consumer's list-size limit, capability taxonomy or tenant model;
unknown values and over-limit lists still reach the same Application validation
as an equivalent gRPC request.

Strings, booleans, signed/unsigned 32/64-bit integer families, floats/doubles and
bytes are supported. Numeric conversion checks the protobuf width and signedness.
Bytes accept standard/URL-safe base64, padded or unpadded. Malformed URL encoding,
invalid values, singular duplicates and mixed aliases return HTTP 400 without
invoking the Application. Error-response mapping after execution is unchanged
and belongs to independent issue #229.

## Explicit support boundary

URL enum, message, map and non-synthetic oneof fields are explicitly unsupported
in this increment. Repeated path fields, complex path templates, named request
body mappings and `response_body` mappings remain unsupported. A typed projection
returns `UNSUPPORTED_HTTP_BINDING` with Operation/method/binding/field context;
it must not silently skip the field. The same message/map/enum/oneof shapes remain
usable through the already-supported ProtoJSON whole-body mapping when its path
fields are supported.

The derived Field `oneof` fact comes from descriptors; proto3 optional synthetic
oneofs are distinguished from real unions. It is additive, omitted when false,
and removed only from the legacy untyped projection. Fresh canonical compilation
is the authority; stale persisted manifests are not independent proof of support.

## Compatibility and validation

Existing typed adapters must be regenerated through the canonical generator.
Do not patch consumer `zz_yunka_*` files. Previously ignored repeated values now
reach the Application; previously ignored invalid scalar values are errors. URL
shapes that never had a correct generated implementation now fail explicitly.
A consumer selecting a supported body binding is a contract change, not evidence
that the original GET defect was repaired.

The permanent real-runtime test `TestGeneratedHTTPBindingTransportParity` is
required by `make dsl-check`. It runs locked protoc, generated HTTP and gRPC
adapters, a shared Executor, and real authorization with a controlled fixture
Principal. It tests the compiler/transport boundary, not a production credential
verifier or arbitrary business behavior. Standard `make verify` and
`make verify-production` remain separate required delivery gates.
