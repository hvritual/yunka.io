package change

import (
	"path/filepath"
	"testing"
)

func TestRepositoryRootGitChangesRemainProjectRelative(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "service.go")
	writeNestedGitTest(t, path, "package root\n")
	base := commitNestedGitTest(t, root)
	writeNestedGitTest(t, path, "package root\n\nvar changed = true\n")

	changes, err := gitChanges(root, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Status != "M" || changes[0].Path != "service.go" {
		t.Fatalf("changes=%#v", changes)
	}
}
