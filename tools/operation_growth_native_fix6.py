#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()
path = ROOT / "app/cmd/change/changeset_remediation.go"
text = path.read_text()
old = '''func ReconcileRemediationWithOptions(ctx context.Context, options projectflow.Options, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
	root := options.Root
	normalizeChangeSet(&value)'''
new = '''func ReconcileRemediationWithOptions(ctx context.Context, options projectflow.Options, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
	normalizeChangeSet(&value)'''
if text.count(old) != 1:
    raise SystemExit("changeset_remediation.go: expected one unused root binding")
path.write_text(text.replace(old, new, 1))
print("unused remediation root binding removed")
