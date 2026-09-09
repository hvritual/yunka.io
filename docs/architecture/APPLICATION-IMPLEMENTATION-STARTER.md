# Application implementation starter — AG-06.1

> Class: CURRENT / developer contract
> Stage authority: [STATUS](../STATUS.md)
> Parent task: [AG-06 #178](https://github.com/hvritual/yunka.io/issues/178)

## Purpose

`yunka add implementation` creates editable implementation skeletons for an existing
canonical leaf Application. It is the first independent AG-06 delivery, not the
whole template rollout. Existing `init`, `add application`, `add operation`, code
generation, Runtime/Executor, authorization and transaction behavior stay unchanged.

A skeleton is not a business implementation: every newly created use-case handler
returns a specific not-implemented error, never a successful response. No database,
transport, permission, event, transaction, mock fallback or sample data is inferred.

## Plan first, create explicitly

```sh
yunka add implementation --root . \
  --composition-package example.com/bookshop/internal/bootstrap \
  --format agent-json shelf/catalog

# Review the paths, exact contents, hashes, interface and factory policy, then:
yunka add implementation --root . \
  --composition-package example.com/bookshop/internal/bootstrap \
  --apply --format agent-json shelf/catalog
```

The default call is read-only. Apply recompiles current canonical inputs and
preflights every destination; an earlier JSON report is not mutation authority.
An existing ChangeSet still requires its own plan/ownership/exact-path admission.
`--protoc` and repeated `--proto-path` use the same projectflow source compiler.
Contract inventories continue to own their own include paths.

Application-key semantics are validated by the existing contract lint and matched
to the current canonical manifest. The starter separately checks physical Go import
paths: dotted keys such as `shelf/catalog.v2` are accepted unchanged; a key whose
segment cannot form a Go import path is rejected, not silently encoded or renamed.

A caller package must be explicit, valid, within the current module and outside the
new owner subtree. This is reviewed policy, not automatic proof that its business
logic is composition-only. A typo or absent caller package is not auto-created.

## Canonical derivation and ownership

The current source snapshot is compiled by the existing projectflow resolver. The
existing C9 renderer supplies the exact interface, method signatures, PB import
aliases and port bytes; the starter does not duplicate single/multi-Application
naming or read stale generated outputs as the source of truth. A rendered service
comment identifies its port within the declared domain; ambiguous or unsupported
port shapes are rejected rather than guessed.

Canonical Go import paths and selected types remain unchanged, while editable
method signatures use a separate stable import-alias namespace. Legal PB aliases
such as `service`, `New`, or `ctx` cannot collide with starter declarations. The
projection clones signature ASTs and never rewrites canonical port bytes; repeated
planning has the same contents and canonical-source hash.

For the default Go root and `shelf/catalog` the created subtree is:

```text
internal/shelf/application/catalog/
  build.go                       # owner.Build returns the canonical interface
  architecture.types.json        # explicit AG-04 starter policy, editable
  internal/usecase/
    wiring.go                    # private facade, explicit delegation only
    listbooks_handler.go         # a separate handler per declared method
  README.md                      # implementation and verification obligations
  docs/task-template.md
  docs/adr-template.md
```

The canonical generated ports stay in `internal/shelf/application/zz_yunka_*`.
The module/import/root paths come from the existing project profile, not a second
layout registry. Non-default contained Go roots work when their imports match the
module's physical layout. An explicit `internal` inside the owning directory makes
its implementation non-importable by sibling consumers under Go's normal rules.

All starter files are developer-owned and remain under the existing Application
scope, so ownership and change planning need no additional broad exception.
`Build` has no resources or resolver and delegates construction to the hidden
package. Use cases start with no dependencies; add only the required narrow ports
to the appropriate handler during a reviewed implementation task. Do not restore
a shared broad Repository on the forwarding facade.

## Preservation and failure semantics

Identical existing targets are reported `unchanged`. A differing user file,
symlink, non-regular target or invalid path rejects the whole preflight before
creating any target. Existing unplanned Go files in each destination package
directory must also have compatible package declarations; canonical external test
packages are allowed. Malformed or conflicting package clauses reject the preflight
without partial creation. New writes are exclusive and contained by Go's `os.Root`
APIs (the repository already requires Go 1.25.13). No force/overwrite/update policy
exists. Regeneration never owns or deletes starter files.

This is not a filesystem transaction or concurrent-edit lock. A later I/O error or
concurrent destination creation can leave already-created files; the command
returns failure and does not erase potential user changes. Inspect the worktree
before retrying. Generated outputs are never written by the starter itself.

The policy records factory/contract identities selected from canonical facts and
the explicitly supplied caller. It is a reviewable starting policy, not silently
regenerated authority: contract changes may require intentional policy updates.
Adding another Application can change canonical interface names; a subsequent
starter call will report a conflict rather than overwrite existing code.

## Verification

Use the canonical generation and structural checks, actual business tests, and the
created AG-04 policy. Run the separately reviewed repository source policy through
AG-05; the per-owner type policy does not certify all source or build profiles.

```sh
yunka generate
yunka check --format agent-json
yunka audit types --root . \
  --policy internal/shelf/application/catalog/architecture.types.json \
  --format agent-json
go test ./...
```

Permanent tests check default read-only CLI, explicit creation, deterministic
planning, canonical single/multi-Application symbols, custom paths, forbidden
inputs and target preservation. Real-protoc tests regenerate the canonical closure
twice and check that editable starter files remain unchanged and stay within the
existing ownership/change scope. A separately labeled DTO shape projection builds
and executes the scaffold, requires explicit not-implemented errors, then checks
actual extra methods, unauthorized factory use and the compiler's hidden-import
rejection. Restored source and type reports must equal the originals.

These are starter/structure qualifications, not a production-ready service, full
runtime/E2E requalification, or proof of existing consumer migration.

## Remaining AG-06 scope

AG-06.1 intentionally rejects declared infrastructure and cross-Application
requirements, composed Operations, streaming or incompatible signature shapes,
empty Applications and ambiguous port identity. It does not silently discard
these facts. AG-06.2 will integrate default templates and source policies into
init/context/normal authoring. AG-06.3 will qualify typed dependency templates and
consecutive contract evolution. Those tasks and AG-07 migration/debt-growth remain
separate; this starter's delivery does not mark the parent stage complete.

## Implementation references

- Existing repository projectflow source snapshot and C9 Application renderer.
- Go traversal-resistant APIs: https://go.dev/blog/osroot
- Go os.Root file operations: https://pkg.go.dev/os#Root
