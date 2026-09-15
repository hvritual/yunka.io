package auditcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeGitCommitPreservesNestedProjectIdentity(t *testing.T) {
	repository := t.TempDir()
	project := filepath.Join(repository, "nested")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMaterializeTestFile(t, filepath.Join(project, "go.mod"), "module example.com/nested\n\ngo 1.25.0\n")
	writeMaterializeTestFile(t, filepath.Join(project, "value.txt"), "baseline\n")
	gitMaterializeTest(t, repository, "init")
	gitMaterializeTest(t, repository, "config", "user.email", "audit@example.invalid")
	gitMaterializeTest(t, repository, "config", "user.name", "Yunka Audit Test")
	gitMaterializeTest(t, repository, "add", ".")
	gitMaterializeTest(t, repository, "commit", "-m", "baseline")
	shaBytes, err := exec.Command("git", "-C", repository, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(shaBytes))
	writeMaterializeTestFile(t, filepath.Join(project, "value.txt"), "current\n")

	baseline, cleanup, err := MaterializeGitCommit(project, sha)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	contents, err := os.ReadFile(filepath.Join(baseline, "value.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "baseline\n" {
		t.Fatalf("materialized contents=%q", string(contents))
	}
	if _, err := os.Stat(filepath.Join(baseline, ".git")); !os.IsNotExist(err) {
		t.Fatalf("baseline projection unexpectedly contains Git metadata: %v", err)
	}
}

func writeMaterializeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitMaterializeTest(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
