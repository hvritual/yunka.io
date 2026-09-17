#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()
path = ROOT / "app/cmd/change/changeset_test.go"
text = path.read_text()
old = '''func TestBuildChangeSetRejectsCreatePlanBoundToDifferentBase(t *testing.T) {
	fixture := newPressureFixture(t)
	plan := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	_, _, err := BuildChangeSet(fixture.Root, "HEAD^", nil, []string{plan})
	if err == nil || !strings.Contains(err.Error(), "boundary decision base") || !strings.Contains(err.Error(), "differs from ChangeSet base") {
		t.Fatalf("mismatched create-plan base escaped ChangeSet binding: %v", err)
	}
}
'''
new = '''func TestBuildChangeSetRejectsCreatePlanBoundToDifferentBase(t *testing.T) {
	fixture := newPressureFixture(t)
	// Advance only the Git identity. HEAD^ remains the complete generated
	// baseline, while the plan is intentionally bound to the newer HEAD.
	gitPressure(t, fixture.Root, "commit", "--allow-empty", "-m", "advance authoring head")
	plan := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
	_, _, err := BuildChangeSet(fixture.Root, "HEAD^", nil, []string{plan})
	if err == nil || !strings.Contains(err.Error(), "boundary decision base") || !strings.Contains(err.Error(), "differs from ChangeSet base") {
		t.Fatalf("mismatched create-plan base escaped ChangeSet binding: %v", err)
	}
}
'''
if text.count(old) != 1:
    raise SystemExit("changeset_test.go: expected hardclose1 mismatch regression")
path.write_text(text.replace(old, new, 1))
print("hardclose1 BaseSHA mismatch regression baseline fixed")
