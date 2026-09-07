from pathlib import Path
import sys

root=Path(sys.argv[1]).resolve()
def replace(path, old, new):
    p=root/path
    text=p.read_text()
    assert text.count(old)==1,(path,text.count(old),old[:100])
    p.write_text(text.replace(old,new,1))

# CLI DTO options are local declaration names, not canonical qualified type IDs.
replace('app/cmd/change/changeset_boundary_test.go',
        'secondOptions.RequestType, secondOptions.ResponseType = firstPlan.Identity["requestType"], firstPlan.Identity["responseType"]',
        '''requestType, responseType := firstPlan.Identity["requestType"], firstPlan.Identity["responseType"]
\tsecondOptions.RequestType = requestType[strings.LastIndex(requestType, ".")+1:]
\tsecondOptions.ResponseType = responseType[strings.LastIndex(responseType, ".")+1:]''')
# Use a valid versioned project profile; the intended negative condition is its
# absence from the immutable base, not a malformed configuration document.
replace('app/cmd/change/changeset_boundary_test.go',
        '"{\\\"schemaVersion\\\":1}\\n"',
        '`{"version":2,"database":{"tablePrefix":"yk"},"workflow":{"contract":{"protoRoot":"contracts/proto","generated":"contracts/generated"},"modules":{"root":"modules"},"generatedGo":{"root":"internal"},"dev":{"manifest":".yunka/dev.json"}}}`')

# Keep the old source-scope assertion intact BEFORE corrupting the bound plan.
# With a forged path list the new earlier proof check must ALSO reject it.
replace('app/cmd/change/contract_sources_test.go',
'''\t\tf.set.Subjects[0].Create.EditablePaths = append(f.set.Subjects[0].Create.EditablePaths, f.paths["other"])
\t\tsourceViolation(t, f.check(t), "contract-source", f.paths["other"])''',
'''\t\tsourceViolation(t, f.check(t), "contract-source", f.paths["other"])
\t\tf.set.Subjects[0].Create.EditablePaths = append(f.set.Subjects[0].Create.EditablePaths, f.paths["other"])
\t\tif _, err := ReconcileChangeSetWithOptions(context.Background(), projectflow.Options{Root: f.root}, f.set); err == nil || !strings.Contains(err.Error(), "STALE_BOUNDARY_PROOF") {
\t\t\tt.Fatalf("tampered create scope must fail proof binding before source checks: %v", err)
\t\t}''')

# The Git-private-storage test is about path/layout/permissions, not a fabricated
# executable creation proof. Use a valid existing-only subject and retain every
# linked-worktree, round-trip, permissions and clean-state assertion.
p=root/'app/cmd/change/git_private_state_test.go'
text=p.read_text()
start=text.index('\tcreate := &CreateOperationChange{')
end=text.index('\n\tpath, err := WriteChangeSet',start)
text=text[:start]+'''\texisting := &ChangeContract{
\t\tSchemaVersion: ChangeContractSchemaVersion,
\t\tBaseSHA: baseSHA,
\t\tIntent: IntentImplementation,
\t\tOperation: ChangeOperation{OperationID: "test.existing", Domain: "test", Application: "app"},
\t\tEditablePaths: []string{"internal/test/application/existing.go"},
\t}
\tchangeSet := ChangeSet{
\t\tSchemaVersion: ChangeSetSchemaVersion,
\t\tBaseSHA: baseSHA,
\t\tSubjects: []ChangeSetSubject{{Kind: ChangeSubjectExistingOperation, Existing: existing}},
\t}
''' +text[end:]
text=text.replace('\n\t"yunka.io/app/cmd/add"\n','\n')
p.write_text(text)

# A set must not be issued with conflicting include profiles which no single
# subsequent check could satisfy. Validate this before persistence, not only later.
replace('app/cmd/change/changeset.go',
'''\tnormalizeChangeSet(&value)
\tif err := validateChangeSet(value); err != nil {
\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
\t}
\treturn value, descriptor.Root, nil''',
'''\tnormalizeChangeSet(&value)
\tif err := validateCreateProfiles(descriptor.Root, value); err != nil {
\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
\t}
\tif err := validateChangeSet(value); err != nil {
\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: err}
\t}
\treturn value, descriptor.Root, nil''')
p=root/'app/cmd/change/changeset_boundary.go'
p.write_text(p.read_text()+'''
func validateCreateProfiles(root string, value ChangeSet) error {
\tvar profile []string
\tinitialized := false
\tfor _, subject := range value.Subjects {
\t\tif subject.Create == nil {
\t\t\tcontinue
\t\t}
\t\tif subject.Create.BoundaryProof == nil {
\t\t\treturn staleBoundary("create subject is missing a compiler profile proof")
\t\t}
\t\tnext, err := boundaryIncludeIdentity(root, subject.Create.BoundaryProof.Plan.ProtoPaths)
\t\tif err != nil {
\t\t\treturn err
\t\t}
\t\tif initialized && jsonValue(profile) != jsonValue(next) {
\t\t\treturn staleBoundary("create subjects use incompatible compiler include profiles; replan against one explicit profile")
\t\t}
\t\tprofile, initialized = next, true
\t}
\treturn nil
}
''')
# Include profiles cannot be mixed even when both independently compile to the
# same model. Test the binding helper without duplicating the compiler fixtures.
p=root/'app/cmd/change/changeset_boundary_test.go'
p.write_text(p.read_text()+'''
func TestIssue161SetBoundaryRejectsMixedIncludeProfiles(t *testing.T) {
\tfixture, value, _ := proofSetFixture(t)
\tsecond := cloneProofSet(t, value).Subjects[0]
\tsecond.Create.BoundaryProof.Plan.ProtoPaths = append(second.Create.BoundaryProof.Plan.ProtoPaths, t.TempDir())
\tvalue.Subjects = append(value.Subjects, second)
\tif err := validateCreateProfiles(fixture.Root, value); !errors.Is(err, boundarycore.ErrStaleBoundaryProof) {
\t\tt.Fatalf("incompatible profiles must fail before a set is written: %v", err)
\t}
}
''')
