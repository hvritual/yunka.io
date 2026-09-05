package gitproject

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTranslatesNestedProjectPaths(t *testing.T) {
	repository := t.TempDir()
	gitPathTest(t, repository, "init")
	project := filepath.Join(repository, "backend-yunka")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := Resolve(project)
	if err != nil {
		t.Fatal(err)
	}
	if paths.RepositoryRoot != filepath.Clean(repository) {
		t.Fatalf("repository root=%q want=%q", paths.RepositoryRoot, repository)
	}
	if paths.ProjectRoot != filepath.Clean(project) {
		t.Fatalf("project root=%q want=%q", paths.ProjectRoot, project)
	}
	if paths.ProjectPrefix != "backend-yunka" {
		t.Fatalf("project prefix=%q", paths.ProjectPrefix)
	}

	repositoryPath, err := paths.ToRepository("contracts/generated/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if repositoryPath != "backend-yunka/contracts/generated/manifest.json" {
		t.Fatalf("repository path=%q", repositoryPath)
	}
	projectPath, inside, err := paths.ToProject(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !inside || projectPath != "contracts/generated/manifest.json" {
		t.Fatalf("project path=%q inside=%t", projectPath, inside)
	}
	if _, inside, err := paths.ToProject("README.md"); err != nil || inside {
		t.Fatalf("outside repository path inside=%t err=%v", inside, err)
	}
}

func TestResolveRepositoryRootPreservesPathDomain(t *testing.T) {
	repository := t.TempDir()
	gitPathTest(t, repository, "init")
	paths, err := Resolve(repository)
	if err != nil {
		t.Fatal(err)
	}
	if paths.ProjectPrefix != "." {
		t.Fatalf("project prefix=%q", paths.ProjectPrefix)
	}
	repositoryPath, err := paths.ToRepository("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if repositoryPath != "go.mod" {
		t.Fatalf("repository path=%q", repositoryPath)
	}
	projectPath, inside, err := paths.ToProject("go.mod")
	if err != nil || !inside || projectPath != "go.mod" {
		t.Fatalf("project path=%q inside=%t err=%v", projectPath, inside, err)
	}
}

func TestPathTranslationRejectsEscapes(t *testing.T) {
	repository := t.TempDir()
	gitPathTest(t, repository, "init")
	paths, err := Resolve(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := paths.ToRepository("../outside"); err == nil {
		t.Fatal("expected project escape to fail")
	}
	if _, _, err := paths.ToProject("../outside"); err == nil {
		t.Fatal("expected repository escape to fail")
	}
}

func gitPathTest(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
