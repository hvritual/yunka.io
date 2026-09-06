package change

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

// The old pressure fixture ignored all of .yunka/, masking issue #151.
// Exercise the public default commands without that consumer workaround.
func TestIssue151DefaultProtocolWithoutIgnore(t *testing.T) {
	fixture := newPressureFixture(t)
	if err := os.Remove(filepath.Join(fixture.Root, filepath.FromSlash(DefaultChangeContractPath))); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.Root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	issue151Workspace(t, fixture)
	// The legacy pressure helper generates only contract/assembly artifacts.
	// Use the full public generation path to include standard protobuf Go.
	if _, err := projectflow.GenerateIncremental(context.Background(), projectflow.Options{Root: fixture.Root, ProtoPaths: []string{fixture.ProtoPath}}, true); err != nil {
		t.Fatalf("prepare complete canonical baseline: %v", err)
	}
	gitPressure(t, fixture.Root, "add", "-A")
	gitPressure(t, fixture.Root, "commit", "-m", "unignored default-protocol baseline")

	run := func(args ...string) error {
		t.Helper()
		app := cli.NewApp()
		app.Commands = []cli.Command{Command()}
		app.ExitErrHandler = func(*cli.Context, error) {}
		return app.Run(append([]string{"yunka", "change"}, args...))
	}
	begin := []string{"begin", "--root", fixture.Root, "--proto-path", fixture.ProtoPath, "--operation", "tenant.suspend", "--intent", "both"}
	check := []string{"check", "--root", fixture.Root, "--format", "agent-json"}
	verify := []string{"verify", "--root", fixture.Root, "--proto-path", fixture.ProtoPath, "--format", "agent-json"}
	if err := run(begin...); err != nil {
		t.Fatal(err)
	}
	if err := run(check...); err != nil {
		t.Fatalf("ISSUE151_DEFAULT_ROUNDTRIP: default begin/check failed without ignore rules: %v", err)
	}
	if err := run(verify...); err != nil {
		t.Fatalf("default verify (including Go tests): %v", err)
	}
	physical, _, err := resolveGitPrivateStatePath(fixture.Root, "", DefaultChangeAttestationPath)
	if err != nil {
		t.Fatal(err)
	}
	first := []byte(readPressureFile(t, physical))
	var attestation ChangeAttestation
	if err := json.Unmarshal(first, &attestation); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"git-delta", "yunka-check", "semantic-delta", "architecture-debt", "go-test"} {
		if got := gateStatus(attestation.Gates, name); got != "pass" {
			t.Fatalf("gate %s=%s; no skipped gates permitted", name, got)
		}
	}
	if !attestation.Conformant {
		t.Fatal("default attestation is not conformant")
	}
	if err := run(verify...); err != nil {
		t.Fatalf("repeat verify: %v", err)
	}
	if !bytes.Equal(first, []byte(readPressureFile(t, physical))) {
		t.Fatal("repeat attestation is not byte deterministic")
	}
	if err := run(check...); err != nil {
		t.Fatalf("check after attestation: %v", err)
	}
	if status := gitPressure(t, fixture.Root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("default control files polluted Git delta: %s", status)
	}

	// Even a valid contract at the old path is ordinary Git delta, not an
	// implicit directory/file allowlist. Unrelated files must remain blocked.
	value, _, err := LoadChangeContract(fixture.Root, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".yunka/undeclared.txt", ".yunka/change-contract.json", "outside.txt"} {
		writePressureFile(t, filepath.Join(fixture.Root, path), "undeclared\n")
		if path == ".yunka/change-contract.json" {
			if _, err := WriteChangeContract(fixture.Root, path, value); err != nil {
				t.Fatal(err)
			}
		}
		if err := run(check...); err == nil {
			t.Fatalf("undeclared path %s escaped", path)
		}
		report, err := ReconcileGitDelta(fixture.Root, value)
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, report, "scope", path)
		if err := os.Remove(filepath.Join(fixture.Root, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := run(begin...); err != nil {
		t.Fatalf("next default begin polluted by previous control evidence: %v", err)
	}
}

// Join the real framework workspace only inside the disposable consumer. No
// fake test runner, skipped Go gate, or change to framework module files.
func issue151Workspace(t *testing.T, fixture pressureFixture) {
	t.Helper()
	repository := filepath.Dir(filepath.Dir(fixture.ProtoPath))
	// make rpc-tools installs the locked plugins at the repository root, not
	// inside this disposable consumer. Expose those same binaries to the test;
	// explicit PROTOC_GEN_GO/PROTOC_GEN_GO_GRPC overrides remain authoritative.
	t.Setenv("PATH", filepath.Join(repository, ".yunka", "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	command := exec.Command("go", "work", "edit", "-json")
	command.Dir = repository
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("workspace inventory: %v\n%s", err, output)
	}
	var work struct {
		Go        string
		Toolchain string
		Use       []struct{ DiskPath string }
	}
	if err := json.Unmarshal(output, &work); err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	fmt.Fprintf(&content, "go %s\n", work.Go)
	if work.Toolchain != "" {
		fmt.Fprintf(&content, "toolchain %s\n", work.Toolchain)
	}
	content.WriteString("use (\n")
	for _, use := range work.Use {
		path := use.DiskPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(repository, path)
		}
		fmt.Fprintf(&content, "%q\n", filepath.ToSlash(path))
	}
	fmt.Fprintf(&content, "%q\n)\n", filepath.ToSlash(fixture.Root))
	path := filepath.Join(t.TempDir(), "go.work")
	writePressureFile(t, path, content.String())
	t.Setenv("GOWORK", path)
}

func TestIssue151PrivateStateLayouts(t *testing.T) {
	repository := t.TempDir()
	gitPressure(t, repository, "init")
	gitPressure(t, repository, "config", "user.name", "Issue 151 regression")
	gitPressure(t, repository, "config", "user.email", "issue151@example.invalid")
	writePressureFile(t, filepath.Join(repository, "README.md"), "fixture\n")
	gitPressure(t, repository, "add", "README.md")
	gitPressure(t, repository, "commit", "-m", "baseline")
	nested := filepath.Join(repository, "backend")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	gitPressure(t, repository, "worktree", "add", "-b", "linked", linked, "HEAD")
	t.Cleanup(func() { _ = exec.Command("git", "-C", repository, "worktree", "remove", "--force", linked).Run() })
	for _, root := range []string{repository, nested, linked} {
		t.Run(filepath.Base(root), func(t *testing.T) {
			value := ChangeContract{SchemaVersion: ChangeContractSchemaVersion, BaseSHA: gitPressure(t, root, "rev-parse", "HEAD")}
			path, err := WriteChangeContract(root, "", value)
			if err != nil || path != DefaultChangeContractPath {
				t.Fatalf("write path=%s err=%v", path, err)
			}
			loaded, _, err := LoadChangeContract(root, "")
			if err != nil || loaded.BaseSHA != value.BaseSHA {
				t.Fatalf("load=%#v err=%v", loaded, err)
			}
			if _, err := WriteChangeAttestation(root, "", ChangeAttestation{SchemaVersion: ChangeAttestationSchemaVersion}); err != nil {
				t.Fatal(err)
			}
			for _, logical := range []string{DefaultChangeContractPath, DefaultChangeAttestationPath} {
				physical, _, err := resolveGitPrivateStatePath(root, "", logical)
				if err != nil {
					t.Fatal(err)
				}
				assertPrivateStateFile(t, physical)
			}
			if status := gitPressure(t, root, "status", "--porcelain"); status != "" {
				t.Fatalf("private state polluted worktree: %s", status)
			}
		})
	}
}

func TestIssue151ExplicitPathsRemainLiteral(t *testing.T) {
	root := t.TempDir() // Explicit output does not require a Git repository.
	for _, path := range []string{".yunka/change-contract.json", "custom/contract.json", filepath.Join(t.TempDir(), "external.json")} {
		value := ChangeContract{SchemaVersion: ChangeContractSchemaVersion, BaseSHA: "explicit"}
		if _, err := WriteChangeContract(root, path, value); err != nil {
			t.Fatal(err)
		}
		loaded, physical, err := LoadChangeContract(root, path)
		if err != nil || loaded.BaseSHA != "explicit" {
			t.Fatalf("explicit roundtrip: %#v %v", loaded, err)
		}
		assertPrivateStateFile(t, physical)
	}
	path := filepath.Join(root, ".yunka", "change-attestation.json")
	if _, err := WriteChangeAttestation(root, path, ChangeAttestation{}); err != nil {
		t.Fatal(err)
	}
	assertPrivateStateFile(t, path)
}
