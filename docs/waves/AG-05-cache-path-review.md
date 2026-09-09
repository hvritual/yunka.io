# AG-05 — Reject writable locations overlapping source

> Document class: **EVIDENCE**
> Current state: [STATUS](../STATUS.md)
> Exact delivery, review and acceptance: [PR #176](https://github.com/hvritual/yunka.io/pull/176), issue #175

Review `3967046000` on `dfd3cc0c18bb386b98bf9365b4b31371f31b0d76`
reproduced a root-contained GOCACHE writing into the original repository during
metadata loading. Post-run AG-SRC-007 correctly noticed drift but did not undo the
read-only violation. A private working directory alone does not isolate caches.

The auditor now rejects original-root-contained writable cache/home/temp locations
before creating its private workspace or invoking Go. The same prerequisite checks
the actual environment passed to every command. Cache paths overlapping the root,
relative or unresolvable locations, and symlinked ancestors leading back into the
root are not accepted. Missing leaf directories are resolved through their existing
ancestors without creating anything. Known HOME-based Go cache defaults and all
GOPATH entries are included, not only a nonempty explicit GOCACHE.

The command does not silently repair, initialize or move the caller's cache. An
external prepared cache is still supported. Rejection is AG-SRC-000 / INCOMPLETE
with zero completed profiles and unchanged original source, not an after-the-fact
successful analysis with a dirty worktree. Administratively managed host caches
remain trusted prerequisites; this is not filesystem locking or a concurrency
sandbox. Process-global environment mutation by unrelated goroutines is not an
endorsed configuration mechanism.

Five permanent test parents (fifteen named events) cover a real fresh root-contained
Go build cache, zero runner calls for five writable inputs, multi-entry GOPATH,
missing cache beneath an external symlink, relative paths, implicit cache defaults
and a real external-cache PASS control. A normal GOPATH/src checkout is allowed;
the effective GOPATH/pkg/mod location is separately resolved and a symlink back
into original source is rejected. The original source digest and absence of
created cache files are checked. All names are in the recurring test inventory.

The exact root-cache test fails against the unchanged prior checker: it emits only
AG-SRC-007 after writing source. The corrected checker rejects before the write.
Local Go 1.23.2 RED/GREEN is exploratory evidence only. Final locked Go 1.25.13,
MySQL Production, source-policy and prior type-consumer gates plus independent
exact-head review precede non-force integration. Separate actual-main receipts
are required before task completion; historical passing runs are not inherited.
