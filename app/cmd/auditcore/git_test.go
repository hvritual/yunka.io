package auditcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGitFileAtCommitUsesRepositoryRelativePathForNestedProject(t *testing.T) {
	repository := t.TempDir()
	project := filepath.Join(repository, "backend-yunka")
	mustWriteGitTest(t, filepath.Join(project, "go.mod"), "module example.com/nested\n\ngo 1.25.0\n")
	mustWriteGitTest(t, filepath.Join(repository, "README.md"), "outside project\n")
	commit := commitGitTest(t, repository)

	contents, err := ReadGitFileAtCommit(project, commit, "go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "module example.com/nested\n\ngo 1.25.0\n" {
		t.Fatalf("contents=%q", contents)
	}
	if _, err := ReadGitFileAtCommit(project, commit, "README.md"); err == nil {
		t.Fatal("project-relative immutable read must not reach repository sibling files")
	}
}

func TestCollectGoSourceAtCommitKeepsProjectRelativeEvidenceForNestedProject(t *testing.T) {
	repository := t.TempDir()
	project := filepath.Join(repository, "backend-yunka")
	mustWriteGitTest(t, filepath.Join(project, "internal", "device", "application", "service.go"), "package application\n\nimport _ \"fmt\"\n")
	mustWriteGitTest(t, filepath.Join(project, "internal", "device", "application", "service_test.go"), "package application\n")
	mustWriteGitTest(t, filepath.Join(repository, "internal", "outside", "outside.go"), "package outside\n")
	commit := commitGitTest(t, repository)

	snapshot, err := CollectGoSourceAtCommit(project, "internal", commit)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SourceRoot != "internal" {
		t.Fatalf("source root=%q", snapshot.SourceRoot)
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("files=%#v", snapshot.Files)
	}
	for _, file := range snapshot.Files {
		if strings.HasPrefix(file.Path, "backend-yunka/") {
			t.Fatalf("repository prefix leaked into project evidence: %#v", file)
		}
		if strings.Contains(file.Path, "outside") {
			t.Fatalf("repository sibling leaked into project snapshot: %#v", file)
		}
	}
	if snapshot.Files[0].Path != "internal/device/application/service.go" || snapshot.Files[1].Path != "internal/device/application/service_test.go" {
		t.Fatalf("files=%#v", snapshot.Files)
	}
}

func mustWriteGitTest(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitGitTest(t *testing.T, root string) string {
	t.Helper()
	gitAuditTest(t, root, "init")
	gitAuditTest(t, root, "config", "user.email", "audit-git@example.invalid")
	gitAuditTest(t, root, "config", "user.name", "Audit Git Test")
	gitAuditTest(t, root, "add", ".")
	gitAuditTest(t, root, "commit", "-m", "baseline")
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func gitAuditTest(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
