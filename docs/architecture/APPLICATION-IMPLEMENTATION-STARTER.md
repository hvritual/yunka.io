# Application implementation starter — AG-06.1 / AG-06.3a

> Class: CURRENT / developer contract
> Stage authority: [STATUS](../STATUS.md)
> Parent task: [AG-06 #178](https://github.com/hvritual/yunka.io/issues/178)
> Typed dependency task: [AG-06.3 #188](https://github.com/hvritual/yunka.io/issues/188)

## Purpose

`yunka add implementation` creates editable implementation skeletons for an existing
canonical Application. AG-06.1 established the leaf-Application starter; AG-06.3a
extends that same starter to canonical cross-Application `requires` edges without
creating another Application/Operation registry or another execution path.

Existing `init`, `add application`, `add operation`, code generation,
Runtime/Executor, authorization and transaction behavior stay unchanged. A skeleton
is not a business implementation: every newly created use-case handler returns a
specific not-implemented error, never a successful response. No database, transport,
permission, event, transaction, mock fallback or sample data is inferred.

Application-level infrastructure `capabilities` remain unsupported by this starter.
They are not equivalent to cross-Application `requires` and are rejected rather
than silently omitted or guessed.

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
existing C9 renderer supplies the exact Application interface, method signatures,
PB import aliases, generated source-edge ChildCapability interfaces and port bytes.
The starter does not duplicate single/multi-Application naming or keep a second
`requires`/Operation map. Ambiguous or unsupported rendered shapes fail closed.

For a canonical edge such as:

```text
dispatch/routes
  requires inventory/stock
  dispatch.route.plan
    requiresOperations inventory.stock.reserve
```

C9 already owns a narrow generated capability such as:

```text
RoutesToInventoryStockChildCapability
  Reserve(...)
```

Only Operations explicitly required by that source edge are present. The generated
capability implementation continues to invoke `operation.ExecuteChildTyped`; the
starter does not call the target Application's hidden implementation and does not
change generated Assembly, Executor, ExecutionScope/root-UoW or permission closure.

For an Application with canonical `requires`, the owner factory accepts the exact
generated source-edge interfaces:

```go
func Build(
    stock application.RoutesToInventoryStockChildCapability,
) (application.RoutesAPI, error)
```

The hidden constructor validates required capabilities and returns an error when a
required value is nil. Each capability is then assigned only to use-case handlers
whose canonical Operation actually references an Operation owned by that dependency.
A shared broad Repository or target Application implementation is not injected into
the forwarding facade.

For leaf Applications, AG-06.1 signatures remain unchanged: `Build()` and `New()`
still have no dependency arguments and return the canonical Application interface
directly. AG-06.3a must not force leaf consumers to adopt the dependent signature.

Canonical Go import paths and selected types remain unchanged, while editable
method signatures use a separate stable import-alias namespace. Legal PB aliases
such as `service`, `New`, or `ctx` cannot collide with starter declarations. The
projection clones signature ASTs and never rewrites canonical port bytes; repeated
planning has the same contents and canonical-source hash.

For the default Go root and `shelf/catalog` the created subtree remains:

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

The canonical generated ports and capability ports stay in
`internal/<domain>/application/zz_yunka_*`. The module/import/root paths come from
the existing project profile, not a second layout registry. Non-default contained
Go roots work when their imports match the module's physical layout. An explicit
`internal` inside the owning directory makes its implementation non-importable by
sibling consumers under Go's normal rules.

All starter files are developer-owned and remain under the existing Application
scope, so ownership and change planning need no additional broad exception. The
owner `Build` is a composition entry point, not a business invocation API. Do not
register another transport handler, resolve dependencies dynamically, or call
another Application's hidden implementation from a use case.

## Type policy for dependencies

`architecture.types.json` continues to select the owner `Build` factory and its
canonical result contract. For each canonical `requires` edge, AG-06.3a also records
an exact argument contract using the generated source-edge ChildCapability type.

The existing AG-04 checker therefore validates both sides:

- `Build` must declare the exact generated argument interface, not a wider factory signature.
- The actual object passed by the permitted composition package must not expose additional public methods beyond that selected capability.
- `Build` remains callable only from its explicitly selected composition package.
- Unknown dynamic provenance remains INCOMPLETE rather than being treated as PASS.

This policy remains a reviewable developer-owned starting policy. It is not a new
runtime security boundary and is not regenerated as mutation authority.

## Preservation and failure semantics

Identical existing targets are reported `unchanged`. A differing user file,
symlink, non-regular target or invalid path rejects the whole preflight before
creating any target. Existing unplanned Go files in each destination package
directory must also have compatible package declarations; canonical external test
packages are allowed. Malformed or conflicting package clauses reject the preflight
without partial creation. New writes are exclusive and contained by Go's `os.Root`
APIs. No force/overwrite/update policy exists. Regeneration never owns or deletes
starter files.

This is not a filesystem transaction or concurrent-edit lock. A later I/O error or
concurrent destination creation can leave already-created files; the command
returns failure and does not erase potential user changes. Inspect the worktree
before retrying. Generated outputs are never written by the starter itself.

Dependency consistency also fails closed before starter creation. A required
Operation with no canonical owner, an Operation owned by an undeclared target
Application, an ambiguous Operation owner, a missing generated capability artifact,
or a malformed capability provider is rejected. The starter never fills these gaps
with a guessed interface.

Adding another Application can change canonical interface names; a subsequent
starter call reports a conflict rather than overwriting existing developer code.
Contract evolution after starter creation remains an explicit developer change;
AG-06.3b separately qualifies that consecutive-evolution workflow.

## Verification

Use canonical generation and structural checks, actual business tests, the created
AG-04 policy, and the separately reviewed repository AG-05 source policy:

```sh
yunka generate
yunka check --format agent-json
yunka audit types --root . \
  --policy internal/shelf/application/catalog/architecture.types.json \
  --format agent-json
yunka audit source --root . --format agent-json
go test ./...
```

Permanent AG-06 qualification retains all AG-06.1/06.2 tests and adds AG-06.3a
requirements. The third-domain qualification uses `dispatch/routes ->
inventory/stock`, checks the exact generated ChildCapability, compiles the editable
starter shape, rejects a dependency implementation with extra public authority,
rejects sibling imports of the hidden use-case package, and verifies malformed
canonical dependency facts fail closed.

A separate real-protoc test compiles that third-domain contract through projectflow,
plans and applies the starter, regenerates the canonical closure twice with zero
starter drift, runs canonical check, and verifies repeated apply reports every
starter file unchanged. These are named required events in the persistent
`ag06-template-qualification` workflow; unexecuted or skipped evidence is not PASS.

These are starter/structure qualifications, not a production-ready service, full
runtime/E2E requalification, or proof of existing consumer migration.

## Remaining AG-06 scope

AG-06.1 and AG-06.2 remain compatible. AG-06.3a adds only canonical
cross-Application `requires` starter support and its third-domain qualification.
Infrastructure `capabilities`, streaming/incompatible signature shapes, empty or
ambiguous Application identities remain unsupported where the existing compiler or
starter cannot prove the required shape.

AG-06.3b is still required: on the accepted typed-dependency sample, perform
consecutive contract changes, including local and dependency Operation evolution,
update the developer handler/dependency binding deliberately, replay canonical
plan/change/generate/check/verify, and prove repeated generation remains drift-free.
The AG-06 parent is not complete until that independent increment passes its own
candidate and actual-main qualification. AG-07 migration/debt-growth remains a
separate stage.

## Implementation references

- Existing repository projectflow source snapshot and C9 Application renderer.
- Existing C9 source-edge ChildCapability and generated Assembly factory contract.
- Existing AG-04 typed factory/capability checker and AG-05 source policy.
- Go traversal-resistant APIs: https://go.dev/blog/osroot
- Go os.Root file operations: https://pkg.go.dev/os#Root
