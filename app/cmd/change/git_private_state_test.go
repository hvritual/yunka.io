package change

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/add"
)

func TestT4GitPrivateStateSupportsLinkedWorktree(t *testing.T) {
	repository := t.TempDir()
	runGitPrivateStateTest(t, repository, "init")
	runGitPrivateStateTest(t, repository, "config", "user.email", "t4-worktree@example.invalid")
	runGitPrivateStateTest(t, repository, "config", "user.name", "T4 Worktree Qualification")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitPrivateStateTest(t, repository, "add", "README.md")
	runGitPrivateStateTest(t, repository, "commit", "-m", "fixture baseline")

	linked := filepath.Join(t.TempDir(), "linked")
	runGitPrivateStateTest(t, repository, "worktree", "add", "-b", "linked", linked, "HEAD")
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", repository, "worktree", "remove", "--force", linked).Run()
	})

	gitFile, err := os.Stat(filepath.Join(linked, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if !gitFile.Mode().IsRegular() {
		t.Fatalf("linked worktree .git must be a regular file, mode=%v", gitFile.Mode())
	}
	baseSHA := strings.TrimSpace(runGitPrivateStateTest(t, linked, "rev-parse", "HEAD"))

	create := &CreateOperationChange{
		Operation: ChangeOperation{OperationID: "test.create", Domain: "test", Application: "app"},
		PlanDigest: "plan-digest",
		Expected: CreateOperationExpectation{
			Service: "TestApplication",
			RPC:     "Create",
			Semantics: add.OperationSemantics{
				UseCase:            "create_test",
				Permissions:        []string{},
				Authentication:     []string{},
				RequiresOperations: []string{},
			},
		},
		EditablePaths:   []string{"contracts/proto/test.proto"},
		GeneratedPaths:  []string{},
		GeneratedScopes: []string{},
	}
	changeSet := ChangeSet{
		SchemaVersion: ChangeSetSchemaVersion,
		BaseSHA:       baseSHA,
		Subjects: []ChangeSetSubject{{
			Kind:   ChangeSubjectCreateOperation,
			Create: create,
		}},
	}

	path, err := WriteChangeSet(linked, "", changeSet)
	if err != nil {
		t.Fatal(err)
	}
	if path != DefaultChangeSetPath {
		t.Fatalf("ChangeSet logical path=%q", path)
	}
	physicalSet, _, err := resolveGitPrivateStatePath(linked, "", DefaultChangeSetPath)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateStateFile(t, physicalSet)
	loadedSet, loadedSetPath, err := LoadChangeSet(linked, "")
	if err != nil {
		t.Fatal(err)
	}
	if loadedSetPath != DefaultChangeSetPath || loadedSet.BaseSHA != baseSHA {
		t.Fatalf("ChangeSet roundtrip path=%q value=%#v", loadedSetPath, loadedSet)
	}

	binding := RemediationBinding{
		SchemaVersion:   RemediationBindingSchemaVersion,
		BaseSHA:         baseSHA,
		ChangeSetDigest: "change-set-digest",
		FindingIDs:      []string{"AUDIT-TEST-001:fixture"},
	}
	bindingPath, err := WriteRemediationBinding(linked, "", binding)
	if err != nil {
		t.Fatal(err)
	}
	if bindingPath != DefaultRemediationBindingPath {
		t.Fatalf("remediation logical path=%q", bindingPath)
	}
	physicalBinding, _, err := resolveGitPrivateStatePath(linked, "", DefaultRemediationBindingPath)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateStateFile(t, physicalBinding)
	loadedBinding, loadedBindingPath, err := LoadRemediationBinding(linked, "")
	if err != nil {
		t.Fatal(err)
	}
	if loadedBindingPath != DefaultRemediationBindingPath || loadedBinding.ChangeSetDigest != binding.ChangeSetDigest {
		t.Fatalf("remediation roundtrip path=%q value=%#v", loadedBindingPath, loadedBinding)
	}

	if status := strings.TrimSpace(runGitPrivateStateTest(t, linked, "status", "--porcelain")); status != "" {
		t.Fatalf("Git-private state polluted linked worktree: %q", status)
	}
}

func assertPrivateStateFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("private state %s mode=%v", path, info.Mode())
	}
}

func runGitPrivateStateTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
