#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path): return (ROOT / path).read_text()
def write(path, text): (ROOT / path).write_text(text)
def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:180]!r}")
    write(path, text.replace(old, new, 1))

# Remediation is another base/current evidence consumer. Keep root-only wrappers
# for API compatibility, but route compiler-aware callers through the same
# projectflow.Options used by ChangeSet reconciliation and Audit boundary growth.
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''import (
\t"crypto/sha256"''',
    '''import (
\t"context"
\t"crypto/sha256"''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''\t"yunka.io/app/cmd/auditcore"
)''',
    '''\t"yunka.io/app/cmd/auditcore"
\t"yunka.io/app/cmd/projectflow"
)''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''func BuildRemediationBinding(root string, value ChangeSet, findingIDs []string) (RemediationBinding, error) {
\tif err := ensureCleanWorktree(root); err != nil {''',
    '''func BuildRemediationBinding(root string, value ChangeSet, findingIDs []string) (RemediationBinding, error) {
\treturn BuildRemediationBindingWithOptions(projectflow.Options{Root: root}, value, findingIDs)
}

func BuildRemediationBindingWithOptions(options projectflow.Options, value ChangeSet, findingIDs []string) (RemediationBinding, error) {
\troot := options.Root
\tif err := ensureCleanWorktree(root); err != nil {''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''\treport, err := audit.BuildWithBase(root, value.BaseSHA)''',
    '''\treport, err := audit.BuildWithBaseOptions(options, value.BaseSHA)''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''func ReconcileRemediation(root string, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
\tnormalizeChangeSet(&value)''',
    '''func ReconcileRemediation(root string, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
\treturn ReconcileRemediationWithOptions(context.Background(), projectflow.Options{Root: root}, value, binding)
}

func ReconcileRemediationWithOptions(ctx context.Context, options projectflow.Options, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
\troot := options.Root
\tnormalizeChangeSet(&value)''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''\tchangeReport, err := ReconcileChangeSet(root, value)''',
    '''\tchangeReport, err := ReconcileChangeSetWithOptions(ctx, options, value)''')
replace_once(
    "app/cmd/change/changeset_remediation.go",
    '''\tauditReport, err := audit.BuildWithBase(root, value.BaseSHA)''',
    '''\tauditReport, err := audit.BuildWithBaseOptions(options, value.BaseSHA)''')

# Public remediation CLI must expose the same compiler profile flags as ChangeSet
# check/verify. This prevents a CLI path from silently losing external DSL inputs.
replace_once(
    "app/cmd/change/changeset_command.go",
    '''\t\tFlags: []cli.Flag{
\t\t\tcli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
\t\t\tcli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
\t\t\tcli.StringSliceFlag{Name: "finding", Usage: "exact proven Audit finding ID to remediate; repeatable"},''',
    '''\t\tFlags: []cli.Flag{
\t\t\tcli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
\t\t\tsourceProtocFlag(), sourceIncludesFlag(),
\t\t\tcli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
\t\t\tcli.StringSliceFlag{Name: "finding", Usage: "exact proven Audit finding ID to remediate; repeatable"},''')
replace_once(
    "app/cmd/change/changeset_command.go",
    '''\t\t\tbinding, err := BuildRemediationBinding(descriptor.Root, value, c.StringSlice("finding"))''',
    '''\t\t\tcompiler := sourceCompilerOptions(c)
\t\t\tcompiler.Root = descriptor.Root
\t\t\tbinding, err := BuildRemediationBindingWithOptions(compiler, value, c.StringSlice("finding"))''')
replace_once(
    "app/cmd/change/changeset_command.go",
    '''\t\tFlags: []cli.Flag{
\t\t\tcli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
\t\t\tcli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
\t\t\tcli.StringFlag{Name: "binding", Value: DefaultRemediationBindingPath, Usage: "remediation binding path"},''',
    '''\t\tFlags: []cli.Flag{
\t\t\tcli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
\t\t\tsourceProtocFlag(), sourceIncludesFlag(),
\t\t\tcli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
\t\t\tcli.StringFlag{Name: "binding", Value: DefaultRemediationBindingPath, Usage: "remediation binding path"},''')
replace_once(
    "app/cmd/change/changeset_command.go",
    '''\t\t\treport, err := ReconcileRemediation(descriptor.Root, value, binding)''',
    '''\t\t\tcompiler := sourceCompilerOptions(c)
\t\t\tcompiler.Root = descriptor.Root
\t\t\treport, err := ReconcileRemediationWithOptions(context.Background(), compiler, value, binding)''')

# Remediation regressions use the fixture's real external include profile instead
# of exercising the compatibility root-only wrapper.
text = read("app/cmd/change/changeset_remediation_test.go")
text = text.replace('BuildRemediationBinding(fixture.Root, value,', 'BuildRemediationBindingWithOptions(fixture.compilerOptions(), value,')
text = text.replace('ReconcileRemediation(fixture.Root, value, binding)', 'ReconcileRemediationWithOptions(context.Background(), fixture.compilerOptions(), value, binding)')
text = text.replace('ReconcileRemediation(fixture.Root, tampered, binding)', 'ReconcileRemediationWithOptions(context.Background(), fixture.compilerOptions(), tampered, binding)')
write("app/cmd/change/changeset_remediation_test.go", text)
replace_once(
    "app/cmd/change/changeset_remediation_test.go",
    '''import (
\t"os"''',
    '''import (
\t"context"
\t"os"''')

print("remediation compiler-profile threading prepared")
