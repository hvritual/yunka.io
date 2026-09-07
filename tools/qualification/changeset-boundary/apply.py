from pathlib import Path
import shutil
import sys

root = Path(sys.argv[1]).resolve()
stage = Path(__file__).resolve().parent

def replace(path, old, new):
    file = root / path
    text = file.read_text()
    assert text.count(old) == 1, (path, text.count(old), old[:100])
    file.write_text(text.replace(old, new, 1))

for source, destination in {
    'contract_capture.go': 'app/cmd/projectflow/contract_capture.go',
    'changeset_boundary.go': 'app/cmd/change/changeset_boundary.go',
    'changeset_boundary_test.go': 'app/cmd/change/changeset_boundary_test.go',
    'changeset_boundary_regression_test.go': 'app/cmd/change/changeset_boundary_regression_test.go',
}.items():
    target = root / destination
    assert not target.exists(), destination
    shutil.copyfile(stage/source, target)

# Preserve the original preview behavior while sharing immutable materialization.
file = root/'app/cmd/projectflow/contract_edit.go'
text = file.read_text()
start = text.index('\tdirectory, err := os.MkdirTemp("", "yunka-contract-edit-*")')
end = text.index('\tbefore, err := compileContract(ctx, p)', start)
text = text[:start] + '''\tp, cleanup, err := materializeContractInputs(ctx, input)
\tif err != nil {
\t\treturn ContractEdit{}, err
\t}
\tdefer cleanup()
''' + text[end:]
old = 'os.WriteFile(filepath.Join(directory, filepath.FromSlash(key)), replacement, 0600)'
assert text.count(old) == 1
text = text.replace(old, 'os.WriteFile(filepath.Join(p.Root, filepath.FromSlash(source)), replacement, 0600)', 1)
file.write_text(text)
replace('app/cmd/projectflow/contract_edit.go',
        'func (input contractInputs) digest() string {',
        '''func (input contractInputs) digest() string {
\treturn input.digestForRoot(input.project.Root)
}

func (input contractInputs) digestForRoot(root string) string {''')
replace('app/cmd/projectflow/contract_edit.go', '\tdata, _ := json.Marshal(struct {\n\t\tProject  ProjectDescriptor',
        '\tproject := describeResolvedProject(input.project)\n\tproject.Root = root\n\tdata, _ := json.Marshal(struct {\n\t\tProject  ProjectDescriptor')
replace('app/cmd/projectflow/contract_edit.go', '}{describeResolvedProject(input.project), input.includeKeys, files})', '}{project, input.includeKeys, files})')

(root/'app/cmd/boundarycore/manifest_digest.go').write_text('''package boundarycore

import "github.com/hvritual/yunka.io/pkg/contract"

// ManifestDigest names the normalized canonical model used in decisions. Input
// compilation/validation is the caller's responsibility; this is not a signature.
func ManifestDigest(manifest contract.Manifest) string {
\treturn modelDigest("boundary-manifest/v1", detachedManifest(manifest))
}
''')

replace('app/cmd/change/changeset.go', 'ChangeSetSchemaVersion = 2', 'ChangeSetSchemaVersion = 3')
replace('app/cmd/change/changeset.go', 'type CreateOperationChange struct {', 'type CreateOperationChange struct {\n\tBoundaryProof *CreateBoundaryProof `json:"boundaryProof,omitempty"`')
replace('app/cmd/change/changeset.go',
'''\t\tprotobufPaths, err := bindCreateProtobufGoGeneratedPaths(descriptor.Root, plan)''',
'''\t\tif err := bindCreateBoundaryBase(descriptor.Root, baseSHA, &create); err != nil {
\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
\t\t}
\t\tprotobufPaths, err := bindCreateProtobufGoGeneratedPaths(descriptor.Root, plan)''')
replace('app/cmd/change/changeset.go',
'''\tnormalizeChangeSet(&value)
\treturn value, descriptor.Root, nil
}''',
'''\tnormalizeChangeSet(&value)
\tif err := validateChangeSet(value); err != nil {
\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
\t}
\treturn value, descriptor.Root, nil
}''')
replace('app/cmd/change/changeset.go',
'''\tdata, err := add.Render(plan, add.FormatAgentJSON)''',
'''\t// Retain detached evidence, not aliases to mutable caller-owned slices.
\tencoded, err := json.Marshal(plan)
\tif err != nil {
\t\treturn CreateOperationChange{}, err
\t}
\tvar detached add.Report
\tif err := json.Unmarshal(encoded, &detached); err != nil {
\t\treturn CreateOperationChange{}, err
\t}
\tplan = detached
\tdata, err := add.Render(plan, add.FormatAgentJSON)''')
replace('app/cmd/change/changeset.go', '\tvalue := CreateOperationChange{', '\tvalue := CreateOperationChange{\n\t\tBoundaryProof: &CreateBoundaryProof{SchemaVersion: 1, Plan: plan},')
replace('app/cmd/change/changeset.go', 'if value.SchemaVersion != ChangeSetSchemaVersion {', 'if value.SchemaVersion != ChangeSetSchemaVersion && value.SchemaVersion != 2 {')
replace('app/cmd/change/changeset.go',
'''\t\t\tif subject.Create.PlanDigest == "" || subject.Create.Expected.Semantics.UseCase == "" || len(subject.Create.EditablePaths) == 0 {''',
'''\t\t\tif value.SchemaVersion != ChangeSetSchemaVersion {
\t\t\t\treturn staleBoundary("legacy ChangeSet create subject %s must be replanned with a current boundary proof", operationID)
\t\t\t}
\t\t\tif err := validateCreateBoundaryProof(value.BaseSHA, *subject.Create); err != nil {
\t\t\t\treturn err
\t\t\t}
\t\t\tif subject.Create.PlanDigest == "" || subject.Create.Expected.Semantics.UseCase == "" || len(subject.Create.EditablePaths) == 0 {''')

replace('app/cmd/change/changeset_reconcile.go', 'ChangeSetCheckSchemaVersion    = 2', 'ChangeSetCheckSchemaVersion    = 3')
replace('app/cmd/change/changeset_reconcile.go', 'type ChangeSetCheckReport struct {', 'type ChangeSetCheckReport struct {\n\tBoundary *ChangeSetBoundaryReport `json:"boundary,omitempty"`')
replace('app/cmd/change/changeset_reconcile.go',
'''\treturn ChangeSetCheckReport{
\t\tContractSources: sources,''',
'''\tboundary, err := reconcileChangeSetBoundaries(ctx, options, value)
\tif err != nil {
\t\treturn ChangeSetCheckReport{}, err
\t}
\tif boundary != nil {
\t\tgitReport.Violations = append(gitReport.Violations, boundary.Violations...)
\t\tsortChangeViolations(gitReport.Violations)
\t}
\treturn ChangeSetCheckReport{
\t\tBoundary: boundary,
\t\tContractSources: sources,''')

replace('README.md',
'This is entry-point protection only: serialized ChangeSets do not yet carry a complete boundary proof, and `change set check`/`yunka check` do not yet detect every direct-edit Operation Growth.',
'ChangeSet create subjects now retain and independently revalidate the complete proof as described below. Canonical direct-edit growth outside those subjects is not yet universally enforced by `yunka check`.')
readme = '''#### Persisted ChangeSet boundary proofs

ChangeSet writers use **schema v3**. Each create subject stores a `boundaryProof` containing the full schema-v2 Operation plan (including the complete decision and counter-evidence) plus a checkout-independent immutable-base input digest. `change set begin` requires the actual captured source/include/profile inputs to equal its Git baseline, not merely share its HEAD string. Ignored canonical metadata or source cannot be attributed to a commit that does not contain it.

`change set check` always revalidates create proofs, even when no `.proto` delta triggers the quick source gate. It compiles private snapshots of raw Git base files and current source inputs, checks the explicit include profile, and recomputes each complete decision. Missing/legacy, altered, stale, non-reuse or wrong-base evidence fails with `STALE_BOUNDARY_PROOF`; recomputing an unkeyed saved digest cannot authorize a different result. The check report is schema v3 and exposes `boundary` entries and violations, with full set before/after Manifest and current input digests. Existing scope, ownership, semantic and source-declaration checks remain required.

Multiple independent creates may share one baseline. For each check, only the other declared **base-absent** Operations are projected out of the freshly compiled current model; every such sibling must separately pass. Existing Operations, DTOs, files, Application metadata and dependencies are never erased to manufacture a single-addition proof. Each candidate must retain an unchanged base peer; new siblings cannot witness one another. Under the unchanged single-addition policy, simultaneous existing canonical edits, new DTO identities, moves and migration batches require separate tasks/policies rather than silent approval. Implementation-only existing subjects can coexist.

Supply the same explicit `--proto-path` profile used for planning; a saved proof never selects an external directory without the check caller's input. External include bytes, including comments, remain bound to the stored base digest. Absolute checkout roots are excluded from that base-content identity, so a committed change can be verified in a relocated checkout. A create check is canonical-model/explicit-input conformance, not a signature or a requirement that current source text exactly match a scaffold: harmless current-source comments remain possible. Compiler executables and implicit standard includes are outside this digest contract.

Existing-only schema-v2 ChangeSets remain readable and retain their quick checks. Legacy create subjects must be replanned; there is no downgrade bypass. Full direct-edit growth detection, existing-operation growth/moves, first-Operation initialization, waivers and boundary-debt governance remain separate. These proofs do not grant generic mutation, merge, runtime or business-ontology authority.

'''
replace('README.md', '### Operation-scoped Agent Context\n', readme+'### Operation-scoped Agent Context\n')
replace('PROJECT_MEMORY.md', '## Operation execution baseline\n', '''- Persisted create-Operation ChangeSet evidence must bind the complete plan and be independently recomputed against immutable base and current canonical input. An unkeyed digest, a copied HEAD or a legacy schema cannot authorize growth. Independent additions may share a baseline only when every candidate retains its own unchanged base witness; other declared additions must never hide existing-model drift.

## Operation execution baseline
''')
# Replace stale claims in CURRENT/STATUS prose, preserving the historic delivery record.
file = root/'docs/STATUS.md'
text = file.read_text()
text = text.replace('direct-edit growth detection, complete ChangeSet boundary proof and boundary-debt policy remain separate.', 'direct-edit growth detection and boundary-debt policy remain separate; persisted create-proof verification is added by the increment below.')
text = text.replace('Full ChangeSet boundary-proof binding and canonical growth checks remain disconnected.', 'Persisted create-ChangeSet proof binding is connected by the increment below; universal canonical growth checks remain separate.')
text = text.replace('Full saved-ChangeSet boundary proof, check/audit growth detection, boundary-debt policy and new-boundary initialization remain unimplemented.', 'Saved create-ChangeSet boundary proofs are connected by the increment below; universal check/audit growth detection, boundary-debt policy and new-boundary initialization remain unimplemented.')
section = '''## Persisted ChangeSet create-boundary proofs — issue #161.4

**State: IMPLEMENTED / IN REVIEW as an independent increment on PR #168; issue #161 remains OPEN.**

ChangeSet v3 retains each full create plan and complete boundary decision, plus captured immutable-base compiler-input identity. Begin independently compiles the raw Git baseline and rejects ignored/current canonical input that is not actually represented by that baseline. Check recompiles captured immutable/current sources and revalidates each complete decision regardless of whether a protobuf delta triggered the ordinary quick gate. Schema-v3 reports expose exact set model/input digests and per-Operation boundary results. The existing policy, scope, ownership, source and semantic checks are not weakened.

Multiple independent new Operations can share the immutable baseline: only other declared base-absent Operations are projected out, all sibling proofs must independently pass, and existing canonical declarations remain visible to every proof. Existing canonical edits alongside creation, migrations and novel client-contract evidence are not silently blessed by the inherited single-addition policy. Existing-only v2 sets remain compatible; legacy create subjects fail closed and require replanning. Caller-supplied include profiles must agree with the saved evidence, and explicit include content drift invalidates the base proof. Checkout location alone does not invalidate base content.

This increment does not implement universal direct-edit growth detection, existing-operation growth policy, first-Operation initialization, structured waivers, or existing/new/fixed boundary debt. It does not sign evidence, certify business ontology, freeze arbitrary external editors or verify consumer runtime. Exact qualification and delivery status belong to this increment's PR/evidence; no main merge or full #161 completion is implied.

'''
assert text.count('## Current pressure frontier\n') == 1
file.write_text(text.replace('## Current pressure frontier\n', section+'## Current pressure frontier\n', 1))
