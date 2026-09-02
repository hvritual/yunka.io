package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

type c116DExplainEnvelope struct {
	SchemaVersion int                    `json:"schemaVersion"`
	OK            bool                   `json:"ok"`
	Code          string                 `json:"code"`
	Definition    *diagnostic.Definition `json:"definition,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

func TestC116DQualificationRealBinaryDiscoverability(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C116D_QUALIFICATION") != "1" && !strings.EqualFold(os.Getenv("CI"), "true") {
		t.Skip("C11.6-D discoverability E2E runs in CI or with YUNKA_REQUIRE_C116D_QUALIFICATION=1")
	}

	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(repositoryRoot, "app")
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("C11.6-D qualification requires go")
	}
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("C11.6-D qualification requires git")
	}

	beforeStatus := c116DGitStatus(t, gitBinary, repositoryRoot)
	if beforeStatus != "" {
		t.Fatalf("C11.6-D qualification requires a clean tracked worktree before build:\n%s", beforeStatus)
	}

	workRoot := t.TempDir()
	binaryName := "yunka-c116d"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	yunkaBinary := filepath.Join(workRoot, binaryName)
	c116DRunCommand(t, appRoot, goBinary, "build", "-buildvcs=true", "-o", yunkaBinary, "./cmd")

	firstHelp, err := c116DRunCLI(yunkaBinary, appRoot, "--help")
	if err != nil {
		t.Fatalf("root help: %v\n%s", err, firstHelp)
	}
	secondHelp, err := c116DRunCLI(yunkaBinary, appRoot, "--help")
	if err != nil {
		t.Fatalf("second root help: %v\n%s", err, secondHelp)
	}
	if firstHelp != secondHelp {
		t.Fatalf("root help is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", firstHelp, secondHelp)
	}
	for _, expected := range []string{
		"Developer workflow",
		"Diagnostics and inspection",
		"Expert architecture",
		"Supplementary tooling",
		"yunka init -> yunka generate -> yunka check -> yunka dev",
	} {
		if !strings.Contains(firstHelp, expected) {
			t.Fatalf("root help missing %q:\n%s", expected, firstHelp)
		}
	}
	for _, command := range []string{
		"api", "assembly", "check", "contract", "dependency", "dev", "doc", "doctor", "domain", "explain", "generate", "graph", "init", "inspect", "module",
	} {
		if !c116DHelpHasCommand(firstHelp, command) {
			t.Fatalf("root help no longer exposes command %q:\n%s", command, firstHelp)
		}
	}
	if strings.Contains(strings.ToLower(firstHelp), "deprecated") {
		t.Fatalf("root help introduced deprecation language:\n%s", firstHelp)
	}

	expertSubcommands := map[string][]string{
		"contract":   {"check", "diff", "generate", "inspect", "lint"},
		"assembly":   {"check", "generate", "inspect"},
		"module":     {"check", "new"},
		"domain":     {"check", "generate", "new"},
		"dependency": {"check"},
	}
	for command, subcommands := range expertSubcommands {
		help, runErr := c116DRunCLI(yunkaBinary, appRoot, command, "--help")
		if runErr != nil {
			t.Fatalf("%s --help: %v\n%s", command, runErr, help)
		}
		for _, subcommand := range subcommands {
			if !c116DHelpHasCommand(help, subcommand) {
				t.Fatalf("%s help no longer exposes subcommand %q:\n%s", command, subcommand, help)
			}
		}
		if strings.Contains(strings.ToLower(help), "deprecated") {
			t.Fatalf("%s help introduced deprecation language:\n%s", command, help)
		}
	}

	wantText := "YUNKA-DX-CONTRACT-002\n  stage:    contract\n  meaning:  generated contract artifacts are stale\n  action:   Regenerate: yunka generate\n"
	firstText, err := c116DRunCLI(yunkaBinary, appRoot, "explain", diagnostic.CodeContractDrift)
	if err != nil {
		t.Fatalf("explain text: %v\n%s", err, firstText)
	}
	secondText, err := c116DRunCLI(yunkaBinary, appRoot, "explain", diagnostic.CodeContractDrift)
	if err != nil {
		t.Fatalf("second explain text: %v\n%s", err, secondText)
	}
	if firstText != wantText || secondText != wantText {
		t.Fatalf("explain text drifted:\n--- got ---\n%s\n--- want ---\n%s", firstText, wantText)
	}

	firstJSON, err := c116DRunCLI(yunkaBinary, appRoot, "explain", diagnostic.CodeContractDrift, "--format", "json")
	if err != nil {
		t.Fatalf("documented explain json form failed: %v\n%s", err, firstJSON)
	}
	secondJSON, err := c116DRunCLI(yunkaBinary, appRoot, "explain", diagnostic.CodeContractDrift, "--format", "json")
	if err != nil {
		t.Fatalf("second documented explain json form failed: %v\n%s", err, secondJSON)
	}
	if firstJSON != secondJSON {
		t.Fatalf("explain json is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", firstJSON, secondJSON)
	}
	var known c116DExplainEnvelope
	if err := json.Unmarshal([]byte(firstJSON), &known); err != nil {
		t.Fatalf("decode explain json: %v\n%s", err, firstJSON)
	}
	if !known.OK || known.Code != diagnostic.CodeContractDrift || known.Definition == nil {
		t.Fatalf("unexpected known explain envelope: %#v", known)
	}
	if known.Definition.Stage != "contract" || known.Definition.Meaning != "generated contract artifacts are stale" || len(known.Definition.Actions) != 1 || known.Definition.Actions[0].Value != "yunka generate" {
		t.Fatalf("known explain definition drifted: %#v", known.Definition)
	}

	unknownJSON, unknownErr := c116DRunCLI(yunkaBinary, appRoot, "explain", "YUNKA-DX-CONTRACT-02", "--format", "json")
	if unknownErr == nil {
		t.Fatalf("unknown diagnostic code unexpectedly succeeded: %s", unknownJSON)
	}
	var exitErr *exec.ExitError
	if !errors.As(unknownErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("unknown diagnostic exit=%v output=%s", unknownErr, unknownJSON)
	}
	var unknown c116DExplainEnvelope
	if err := json.Unmarshal([]byte(unknownJSON), &unknown); err != nil {
		t.Fatalf("decode unknown explain json: %v\n%s", err, unknownJSON)
	}
	if unknown.OK || unknown.Code != "YUNKA-DX-CONTRACT-02" || unknown.Error != "unknown diagnostic code" || unknown.Definition != nil {
		t.Fatalf("unknown code received invented/fuzzy definition: %#v", unknown)
	}

	afterStatus := c116DGitStatus(t, gitBinary, repositoryRoot)
	if afterStatus != beforeStatus {
		t.Fatalf("C11.6-D qualification mutated tracked framework worktree:\nbefore=%q\nafter=%q", beforeStatus, afterStatus)
	}
}

func c116DRunCLI(binary, dir string, args ...string) (string, error) {
	command := exec.Command(binary, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	return string(output), err
}

func c116DRunCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func c116DHelpHasCommand(help, name string) bool {
	for _, line := range strings.Split(help, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, name+" ") || strings.HasPrefix(line, name+",") {
			return true
		}
	}
	return false
}

func c116DGitStatus(t *testing.T, gitBinary, root string) string {
	t.Helper()
	command := exec.Command(gitBinary, "-C", root, "status", "--porcelain", "--untracked-files=no")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}
