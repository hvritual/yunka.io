# HTTP binding semantic change map

Refs #177 / #230; base d7902c98cc124d0b55fb7081df015633c550a0a0. This is transient implementation/review evidence, not a second contract source.

Problem: generated typed Application REST adapters omit repeated query fields, disagree with protobuf JSON names and published OpenAPI, and silently skip unsupported shapes. Compilation and deterministic generation alone do not prove equivalent Application inputs.

Current owners: canonical PB descriptors -> contract Manifest; C9 and retained compatibility generator each write binding code; OpenAPI separately derives parameters; TypeScript owns DTO and binding projections. Executor/Authz/root UoW own execution and stay unchanged.

Desired ownership: one internal, non-persisted HTTP binding plan in pkg/contract, derived from the existing Method/HTTPBinding/Field facts. Shared typed binding code emitter consumes it; lint and typed OpenAPI/TypeScript use that same support boundary. No new DSL, plugin API, gateway binder, dependency or Kernel interface.

Behavior delta: repeated scalar query values retain order, duplicates and empty string values; protobuf source and JSON aliases are accepted, ambiguous mixed aliases/singular duplicates rejected; invalid numeric/bool/encoding input reports HTTP 400 rather than being dropped; path owns its fields and overrides body, body:* owns all non-path fields and therefore has no query parameters. Application-specific list/tenant constraints remain in the Application/security layer. Unsupported URL enum/message/map/oneof or non-simple paths fail with operation/field/binding diagnostics. Existing protojson body:* behavior is retained.

Public API: no Executor/Runtime/API shape change; typed contract artifacts may add derived parameter metadata and exact path variable names. Descriptor-derived oneof membership may be retained additively only to reject unsupported URL access; legacy untyped artifact bytes must remain unchanged.

Persistence: none. Generated delta: from canonical generator only, never patch consumer zz_yunka files. Product scope: pkg/contract binding/compiler/projection files and tests, minimal Makefile registration for the new real-runtime fixture, HTTP binding documentation and STATUS. Control workflows/payloads never enter product diff.

Proof: install only the permanent runtime regression on untouched base first, preserve a real repeated-value failure rather than a build/network failure; apply bounded fix; run table-driven support/negative tests, real generated HTTP/gRPC/Executor parity with locked protoc, source/JSON names, invalid values, empty/missing/duplicates/limits, path/body conflict, unauthenticated and cross-tenant cases. Preserve existing tests, repeat generation byte-for-byte, run canonical CI/Production and applicable consumer qualifications. Attempt locked original Biz contract reverse qualification without changing the actual consumer pin or checkout. Each implementation increment must have exact candidate/run identity, non-force integration and separate actual-main readback; unresolved behavior remains OPEN, not declared complete.
