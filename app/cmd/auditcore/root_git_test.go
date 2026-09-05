package auditcore

import (
	"path/filepath"
	"testing"
)

func TestReadGitFileAtCommitPreservesRepositoryRootProject(t *testing.T) {
	root := t.TempDir()
	mustWriteGitTest(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25.0\n")
	commit := commitGitTest(t, root)

	contents, err := ReadGitFileAtCommit(root, commit, "go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "module example.com/root\n\ngo 1.25.0\n" {
		t.Fatalf("contents=%q", contents)
	}
}
