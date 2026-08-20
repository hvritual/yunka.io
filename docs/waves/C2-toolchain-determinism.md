# C2 Toolchain Determinism

## Status

Implementation wave over the C1 baseline `df2c71df36bc9bb4b3cb9256dc8319cd66173f4f`.

## Goal

Make verification reproducible and make normal CI strictly non-mutating before starting C3 contract-source convergence.

C2 has four deliverables:

1. repository-owned exact toolchain lock;
2. read-only CI with no auto-repair commits;
3. exact Go/protoc verification in Make and `yunka doctor`;
4. dependency and generated-contract zero-drift gates.

## Non-goals

C2 does not:

- migrate protobuf sources or HTTP bindings;
- change contract semantics or generated artifact formats;
- replace or remove the legacy RPC generator;
- upgrade application dependencies as a dependency-convergence wave;
- add runtime features.

## Canonical toolchain lock

`tools/toolchain.env` is the repository-owned lock consumed by Make, CI, and `yunka doctor`:

```text
GO_VERSION=1.25.13
PROTOC_RELEASE=21.12
PROTOC_VERSION=3.21.12
PROTOC_LINUX_X86_64_SHA256=3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8
GOVULNCHECK_VERSION=v1.7.0
```

The protobuf archive hash is for `protoc-21.12-linux-x86_64.zip`.

The lock is source controlled. CI may consume it but may not rewrite it.

## Make gate

`make toolchain-check` is read-only and fails unless:

- `go.work` declares `toolchain go1.25.13`;
- the active `go` reports exactly `1.25.13`;
- the active `protoc` reports exactly `libprotoc 3.21.12`.

`make contract`, `make contract-check`, and `make verify` consume this gate. `govulncheck` uses the version from the same lock.

## Read-only CI

Normal CI has only:

```yaml
permissions:
  contents: read
```

It contains no repository mutation path:

- no `git add`;
- no `git commit`;
- no `git push`;
- no dependency auto-repair.

Dependency drift is a hard failure. The developer fixes the dependency state locally and submits it as an ordinary reviewed source change.

Third-party Actions are pinned to immutable commit SHAs rather than moving major tags.

## Locked protoc installation

CI downloads the exact protobuf release archive:

```text
v21.12/protoc-21.12-linux-x86_64.zip
```

Before extraction it verifies:

```text
sha256 = 3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8
```

No package-manager-selected `protobuf-compiler` version participates in contract verification.

## Doctor semantics

`yunka doctor` remains read-only. C2 adds `toolchain.lock` and changes Go/protoc validation from minimum-version checks to exact-version checks.

Examples of blocking states:

- lock says Go `1.25.13`, but `go.work` declares another toolchain;
- local Go is newer or older than `1.25.13`;
- local protoc is `3.22.0`, even though it is newer than the old minimum;
- the lock is missing, incomplete, has an unknown key, or contains an invalid protoc SHA-256.

This is deliberate: reproducibility has priority over accepting arbitrary newer compilers.

## Zero-drift CI sequence

The normal CI sequence is:

```text
checkout pinned revision
    -> read tools/toolchain.env
    -> setup exact Go
    -> download + hash-check exact protoc
    -> make toolchain-check
    -> make tidy
    -> dependency git diff == 0
    -> make contract
    -> contracts/generated git diff == 0
    -> make contract-check
    -> PR contract compatibility guard
    -> make verify
    -> repeat toolchain/tidy/contract checks
    -> whole-worktree git diff == 0
```

The second pass proves idempotence rather than merely proving that the first pass succeeded.

## Production acceptance gate

Before C2 is merged, the publication runner must execute the C1 production gate as well:

```bash
make toolchain-check
make tidy
make contract
make verify
make integration
```

with real MySQL 8.4, followed by a second deterministic drift pass. C2 must not regress the C1 RPC, readiness, or outbox baseline.

## Rollout

1. Land `tools/toolchain.env` and Make/Doctor exact checks together.
2. Convert normal CI to `contents: read` and remove auto-commit behavior in the same change.
3. Pin Actions and protoc installation.
4. Run dependency and generated-contract drift checks twice.
5. Run the complete C1 production verification before fast-forwarding the C2 branch into `main`.

## Rollback

C2 can be reverted as one commit. Reverting restores the previous permissive tooling and CI behavior but does not alter C1 runtime semantics or contract source semantics.
