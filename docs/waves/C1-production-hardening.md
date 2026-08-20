# C1 Production Hardening

## Status

Implementation wave over the C0 baseline `b333b98bbf107021e1fe7f163ede0f0014bb4298`.

## Goal

Close the remaining production trust and startup-order boundaries without expanding the framework into C2 toolchain or C3 contract-convergence work.

C1 has three deliverables:

1. server-established RPC service identity;
2. readiness-gated local process orchestration;
3. higher-concurrency MySQL outbox regression coverage.

## Non-goals

C1 does not:

- change generated RPC files;
- trust forwarded end-user identity metadata;
- implement SPIFFE/SPIRE, a certificate authority, or secret distribution;
- make CI read-only or pin `protoc`;
- migrate protobuf sources or remove the legacy RPC generator;
- change the C0 transactional-outbox delivery contract.

## RPC trust boundary

### Contract

`gateway/rpc/transport/grpc.CredentialVerifier` is the non-generated trust-boundary seam:

```go
type CredentialVerifier interface {
    Verify(context.Context) (identity.Principal, error)
}
```

A production gRPC server installs both:

```go
grpc.UnaryInterceptor(
    yunkaGrpc.AuthenticatedUnaryServerInterceptor(chain, verifier),
)
grpc.StreamInterceptor(
    yunkaGrpc.AuthenticatedStreamServerInterceptor(chain, verifier),
)
```

The verifier must establish the Principal before authorization middleware runs. Verification failure is returned as a generic `Unauthenticated` status and does not disclose backend or token details.

The existing `UnaryServerInterceptor` remains a compatibility adapter. It derives trace and RPC metadata but intentionally does not authenticate a caller.

### Static service-token adapter

C1 provides a bootstrap adapter for deployments that do not yet have workload identity:

- metadata key: `x-yunka-service-authorization`;
- value format: `Bearer <service-token>`;
- minimum token length: 32 bytes;
- token digests are compared in constant time;
- multiple token bindings may coexist during rotation;
- transport privacy and integrity are required by default;
- outbound credentials implement gRPC `PerRPCCredentials` and require transport security.

The service credential channel is intentionally separate from the HTTP/end-user `Authorization` header. Tenant, user, and role metadata received from an RPC caller is never accepted as identity proof.

`AllowInsecureServiceCredentialsForDevelopment` is an explicit local-development escape hatch. It is not a production default.

### Future adapters

The verifier seam is intended to accept later adapters such as:

- mTLS certificate identity;
- SPIFFE/SPIRE workload identity;
- a signed, short-lived service assertion;
- a deployment-specific IAM identity provider.

Those adapters must preserve the same server-established Principal contract.

## Readiness-gated developer runtime

### Manifest schema

C1 introduces dev manifest schema version 2. Schema version 1 remains accepted for existing manifests, but readiness fields require version 2.

```json
{
  "schemaVersion": 2,
  "processes": [
    {
      "name": "api",
      "command": ["go", "run", "./cmd/api"],
      "dependsOn": ["database"],
      "readiness": {
        "url": "http://127.0.0.1:16667/_yunka/diagnostics",
        "timeout": "30s",
        "interval": "250ms",
        "expectedStatus": 200,
        "diagnosticsReady": true,
        "tokenEnv": "YUNKA_DIAGNOSTICS_TOKEN"
      }
    }
  ]
}
```

### Startup semantics

The plan remains topologically ordered. The runner now applies a barrier after starting a process that declares readiness:

```text
start dependency
    -> poll readiness
    -> require expected HTTP status
    -> optionally require core.health.ready=true
    -> start dependent
```

A process exit before readiness immediately fails the run and cancels all already-started children. Parent cancellation also cancels all children.

### Probe security

Readiness probes:

- use GET only;
- never follow redirects;
- have a bounded overall timeout and bounded per-request timeout;
- limit response-body reads;
- obtain an optional Bearer token only through a named environment variable;
- reject URL-embedded credentials and fragments;
- allow plain HTTP only to literal loopback IP addresses;
- require HTTPS for remote endpoints;
- accept expected statuses only in the 2xx range.

`diagnosticsReady=true` consumes the existing W01/W07 semantic field `core.health.ready`; it does not invent a second readiness model.

## Transactional outbox hardening

C0 established:

- `READ COMMITTED` claim transactions;
- `FOR UPDATE SKIP LOCKED` as an explicit MySQL 8 option;
- separate expired-lease and pending queues;
- ID-only locking reads;
- covering queue indexes.

C1 preserves that algorithm and expands real MySQL verification to include:

- 2 workers claiming 12 records;
- 10 workers claiming 100 records;
- zero duplicate IDs;
- complete queue partitioning;
- correct owner/status/attempt values;
- expired-lease reclaim.

## Verification

Local deterministic gate:

```bash
make tidy
make test
make race
make vet
make vuln
make build
```

Production integration gate:

```bash
export YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/yunka_test?parseTime=true&charset=utf8mb4'
make integration
```

`make verify-production` combines the normal verification gate and MySQL integration.

The regular gateway test suite includes an in-memory gRPC listener with a real TLS handshake and `PerRPCCredentials`, proving that a valid service token establishes a Principal while missing or invalid credentials fail closed.

The developer-runtime suite uses real subprocesses and HTTP endpoints to prove that a dependent process cannot start before its dependency reports ready.

## Rollout

1. Deploy the new verifier and both authenticated server interceptors to one internal RPC service.
2. Configure overlapping old/new service tokens during rotation.
3. Confirm generic `Unauthenticated` behavior and absence of user-identity metadata trust.
4. Add readiness declarations to local manifests one dependency at a time.
5. Run the MySQL integration gate against the supported production MySQL version.
6. Replace static service tokens with a workload-identity adapter when deployment infrastructure is available.

## Rollback

- RPC: remove the authenticated interceptor and restore the compatibility interceptor only for the affected deployment. This reopens the trust boundary and therefore requires an explicit incident decision.
- Readiness: remove the optional `readiness` object and return the manifest to schema version 1.
- Outbox: the C1 wave does not change the C0 Claim algorithm; only the added stress test can be reverted independently.
