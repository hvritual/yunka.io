#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path): return (ROOT / path).read_text()
def write(path, text): (ROOT / path).write_text(text)
def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:160]!r}")
    write(path, text.replace(old, new, 1))

# Persist the canonical DSL support inside the pressure fixture's Git baseline.
# Audit/ChangeSet immutable-base recompilation must not depend on transient test
# process proto-path arguments.
replace_once(
    "app/cmd/change/pressure_qualification_test.go",
    '''\trepositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../.."))
\tprotoPath := filepath.Join(repositoryRoot, "contracts", "proto")

\tif _, err := add.AddApplication''',
    '''\trepositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../.."))
\tprotoPath := filepath.Join(repositoryRoot, "contracts", "proto")
\tdsl, err := os.ReadFile(filepath.Join(protoPath, "yunka", "dsl", "v1", "options.proto"))
\tif err != nil { t.Fatal(err) }
\twritePressureFile(t, filepath.Join(root, "contracts", "proto", "yunka", "dsl", "v1", "options.proto"), string(dsl))

\tif _, err := add.AddApplication''')

# Existing add-package positive fixtures now declare explicit architecture intent.
for path in ["app/cmd/add/ag06_2_test.go", "app/cmd/add/compile_test.go", "app/cmd/add/main_test.go"]:
    text = read(path)
    text = text.replace('Composition: "none",\n\t})', 'Composition: "none", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant",\n\t})')
    text = text.replace('Composition: "local",\n\t})', 'Composition: "local", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant",\n\t})')
    write(path, text)

# Plan qualification: boundary is part of explicit semantics and therefore part of
# deterministic plan/apply equality.
replace_once(
    "app/cmd/add/plan_test.go",
    '''\t\tHTTPBody:       "*",
\t}''',
    '''\t\tHTTPBody:       "*",
\t\tBoundaryContext: "tenant.lifecycle",
\t\tBoundaryAggregate: "tenant",
\t}''')
replace_once(
    "app/cmd/add/plan_test.go",
    '''\twantSemantics := &OperationSemantics{
\t\tUseCase:''',
    '''\twantSemantics := &OperationSemantics{
\t\tBoundary:           &OperationBoundarySemantics{Context: "tenant.lifecycle", Aggregate: "tenant"},
\t\tUseCase:''')
replace_once(
    "app/cmd/add/plan_test.go",
    '''\t\tAuthentication: []string{"service", "jwt", "api-key"}, Transaction: "local", Idempotency: "none", Composition: "local",
\t}''',
    '''\t\tAuthentication: []string{"service", "jwt", "api-key"}, Transaction: "local", Idempotency: "none", Composition: "local",
\t\tBoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant",
\t}''')
# The existing-landing negative fixture should reach the landing conflict rather
# than being short-circuited by missing boundary intent.
replace_once(
    "app/cmd/add/plan_test.go",
    '''\t\tAccess: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
\t})''',
    '''\t\tAccess: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
\t\tBoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant",
\t})''')

# ChangeSet semantic-drift regression should exercise the final check, not ask the
# authoring gate to create an already-incompatible candidate. Create the planned
# valid Operation, then bypass the authoring entry point by editing canonical source.
replace_once(
    "app/cmd/change/changeset_reconcile_test.go",
    '''\tif _, err := add.AddOperation(changeSetCreateOptions(fixture, false)); err != nil {
\t\tt.Fatalf("apply drifted create operation: %v", err)
\t}
\tgeneratePressureProject(t, fixture)
''',
    '''\tif _, err := add.AddOperation(changeSetCreateOptions(fixture, true)); err != nil {
\t\tt.Fatalf("apply planned create operation: %v", err)
\t}
\tmutateRPCOption(t, fixture, "Archive", "tenant_required: true", "tenant_required: false")
\tgeneratePressureProject(t, fixture)
''')

# A source-scope fixture with one UNKNOWN peer still represents existing boundary
# debt, so automatic growth must remain disabled. Give the unrelated existing peer
# the same explicit context/aggregate; its public security remains intentionally
# different and is not used as the candidate's common witness.
replace_once(
    "app/cmd/change/contract_sources_test.go",
    '''option (yunka.dsl.v1.operation) = { id: "scope.other" use_case: "other" public: true execution: {transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE} };''',
    '''option (yunka.dsl.v1.operation) = { id: "scope.other" use_case: "other" public: true execution: {transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE} boundary: { context: "scope.lifecycle" aggregate: "scope" } };''')

print("remaining fixture and immutable-base support fixes prepared")
