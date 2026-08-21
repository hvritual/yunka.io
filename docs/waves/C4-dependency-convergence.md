# C4 Dependency Convergence

## Status

Implementation wave over the C3 baseline `308b5cf48a77c68b086c72cd1323390042ca47b7`.

## Goal

Remove the historical dependency graph workaround that survived C2/C3 while preserving the compatibility boundaries proven by C1-C3.

C4 is not a broad upgrade. Its convergence order is:

1. identify the dependency that selects the historical graph;
2. isolate the unavoidable compatibility package path in a repository-owned module;
3. remove the monolithic genproto workaround;
4. constrain remaining legacy protobuf ownership;
5. keep product module boundaries stable unless evidence requires change.

## Corrected root cause

The first C4 control run established that changing only the first-party `pkg/aliLogStore/manager.go` import was insufficient. `github.com/aliyun/aliyun-log-go-sdk v0.1.127` itself imports `github.com/go-kit/kit/log` and `github.com/go-kit/kit/log/level` and requires `github.com/go-kit/kit v0.10.0`.

The upstream monolithic kit module declares a large historical graph including 2019 unsplit etcd and gRPC `v1.26.0`, which selects the historical monolithic `google.golang.org/genproto` tree. The upstream Aliyun SDK main branch still carries the same old kit dependency, so a version-only SDK upgrade is not a valid convergence strategy.

C4 therefore introduces `compat/go-kit-kit-log`, a repository-owned workspace module whose module path is `github.com/go-kit/kit`. It implements only the logging API required by the SDK and delegates to the split `github.com/go-kit/log v0.2.1` module. Root workspace commands resolve the historical path to this main module. Because `go mod tidy` evaluates one product module at a time, C4 also applies the same version-scoped local replacement for `github.com/go-kit/kit v0.10.0` in each product module that reaches the SLS SDK. This prevents tidy from reading the external monolithic kit go.mod while keeping the replacement narrow, local, and pinned to the exact upstream version.

First-party code imports `github.com/go-kit/log` directly. The old package path is owned solely by the compatibility module for third-party SDK compilation.

## Compatibility module invariants

`compat/go-kit-kit-log` must:

- be listed in root `go.work` as a workspace main module;
- have exact module path `github.com/go-kit/kit`;
- live at the exact repository path `compat/go-kit-kit-log`;
- depend only on the split `github.com/go-kit/log` surface and its minimal dependencies;
- expose only the root `log` and `log/level` API required by the pinned SLS SDK;
- be the target of the exact version-scoped local replacement `github.com/go-kit/kit v0.10.0 => <repository>/compat/go-kit-kit-log` in root `go.work` and the affected product module files;
- never be targeted by an unversioned replacement, an external module replacement, or a path outside this repository;
- never be imported directly by first-party product code.

Removal requires retiring the direct Aliyun SLS SDK adapter or adopting an upstream SDK that no longer imports the old package path.

## Genproto convergence

C4 removes `replace google.golang.org/genproto => ...` from the workspace and all five product module files. The converged build list contains the split modules:

- `google.golang.org/genproto/googleapis/api`;
- `google.golang.org/genproto/googleapis/rpc`.

The monolithic `google.golang.org/genproto` module is forbidden. Dependency convergence is incomplete if the workspace requires a version replacement to avoid ambiguous ownership.

## Protobuf compatibility islands

C4 does not rewrite legacy generated protobuf or change public RPC signatures. `github.com/golang/protobuf` remains temporarily valid only for bounded compatibility owners:

- `app/cmd/rpc/` legacy generator and generated helper code;
- `gateway/rpc/meta/` committed legacy protobuf guarded by C3 descriptor sync;
- `framework/infras/sms/` committed generated protobuf;
- `pkg/invoke/`, `pkg/selector/rpc_client.go`, and `pkg/util/`, whose public interfaces still accept legacy protobuf messages.

`github.com/gogo/protobuf` has no approved new runtime ownership; only the isolated legacy generator subtree may import it directly. SLS pointer construction no longer uses protobuf helpers because ordinary Go pointers are sufficient.

## Dependency policy

`tools/dependency-policy.json` is the durable C4 graph policy. `yunka dependency check` / `make dependency-check` inspect the actual workspace module build list, workspace ownership, module files, and repository Go imports.

The gate rejects:

- legacy unsplit `go.etcd.io/etcd`;
- legacy `github.com/grpc-ecosystem/grpc-gateway` v1;
- monolithic `google.golang.org/genproto`;
- a reintroduced genproto replacement or any other external module replacement;
- a missing, unversioned, or misdirected local replacement for `github.com/go-kit/kit v0.10.0`;
- a non-local `github.com/go-kit/kit` workspace module;
- drift in convergence-critical module versions;
- first-party use of the historical go-kit package path;
- legacy protobuf imports outside approved compatibility paths.

## Module boundary decision

C4 retains the five product modules and adds one compatibility-only workspace module. `app/cmd/rpc` remains separate because that boundary isolates the legacy generator and its old protobuf tooling from the normal application CLI. The compatibility module is not a sixth product module and must not accumulate unrelated packages.

## Verification

```bash
make toolchain-check
make tidy
make dependency-check
make tidy
make dependency-check
make contract-check
make test
make race
make vet
make vuln
make build
```

Production regression also runs the C1 MySQL 8.4 integration suite and repeated outbox concurrency coverage. The second `make tidy` must be byte-stable. Contract artifacts must remain unchanged from C3.

## Rollback

Rollback C4 as one unit. Removing only the version-scoped local kit replacements would make single-module tidy read the external monolithic kit graph again; restoring only the genproto replace would hide rather than solve that regression.

## Non-goals

C4 does not:

- run a broad `go get -u` upgrade sweep;
- fork or vendor the complete Aliyun SLS SDK;
- migrate generated RPC code to a new protobuf generator;
- change `pkg/invoke` public RPC message signatures;
- remove the legacy RPC generator;
- merge product modules for cosmetic reasons;
- add C5 runtime behavior.

## Targeted SLS SDK convergence

C4 pins `github.com/aliyun/aliyun-log-go-sdk v0.1.127`. The previous
`v0.1.64` release required `google.golang.org/protobuf v1.25.0`, whose module
metadata selected the pre-split `google.golang.org/genproto` module even though
first-party packages did not import it. The targeted SDK upgrade moves that
edge to protobuf `v1.28.1`, which does not require monolithic genproto. The
repository-owned `compat/go-kit-kit-log` module remains the bounded adapter for
the SDK's historical go-kit import path.

Existing legacy RPC interfaces and committed generated transports continue to
use `github.com/golang/protobuf` only at the exact files recorded in the
dependency policy. New files and broad directory-level compatibility
exceptions remain forbidden.
