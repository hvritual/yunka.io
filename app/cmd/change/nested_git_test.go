package change

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestNestedProjectGitShowReadsRepositoryPrefixedCanonicalFile(t *testing.T) {
	repository := t.TempDir()
	project := filepath.Join(repository, "backend-yunka")
	manifest := filepath.Join(project, "contracts", "generated", "manifest.json")
	writeNestedGitTest(t, manifest, "{\"schemaVersion\":4}\n")
	writeNestedGitTest(t, filepath.Join(repository, "README.md"), "repository sibling\n")
	base := commitNestedGitTest(t, repository)

	contents, err := gitShow(project, base, "contracts/generated/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "{\"schemaVersion\":4}\n" {
		t.Fatalf("contents=%q", contents)
	}
}

func TestNestedProjectGitChangesReturnsOnlyProjectRelativeDelta(t *testing.T) {
	repository := t.TempDir()
	project := filepath.Join(repository, "backend-yunka")
	service := filepath.Join(project, "internal", "device", "application", "service.go")
	readme := filepath.Join(repository, "README.md")
	writeNestedGitTest(t, service, "package application\n")
	writeNestedGitTest(t, readme, "baseline\n")
	base := commitNestedGitTest(t, repository)

	writeNestedGitTest(t, service, "package application\n\nvar changed = true\n")
	writeNestedGitTest(t, filepath.Join(project, "internal", "device", "application", "new.go"), "package application\n")
	writeNestedGitTest(t, readme, "outside changed\n")
	writeNestedGitTest(t, filepath.Join(repository, "outside.txt"), "outside untracked\n")

	changes, err := gitChanges(project, base)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, change := range changes {
		if strings.HasPrefix(change.Path, "backend-yunka/") {
			t.Fatalf("repository prefix leaked into project evidence: %#v", change)
		}
		got = append(got, change.Status+" "+change.Path)
	}
	sort.Strings(got)
	want := []string{
		"A internal/device/application/new.go",
		"M internal/device/application/service.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("changes=\n%s\nwant=\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func writeNestedGitTest(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitNestedGitTest(t *testing.T, root string) string {
	t.Helper()
	gitNestedTest(t, root, "init")
	gitNestedTest(t, root, "config", "user.email", "nested-git@example.invalid")
	gitNestedTest(t, root, "config", "user.name", "Nested Git Test")
	gitNestedTest(t, root, "add", ".")
	gitNestedTest(t, root, "commit", "-m", "baseline")
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func gitNestedTest(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
