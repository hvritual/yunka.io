package projectflow

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"yunka.io/pkg/fastfeedback"
)

const c114FastHitBudget = 2 * time.Second

type c114CLIEnvelope struct {
	SchemaVersion int     `json:"schemaVersion"`
	Command       string  `json:"command"`
	OK            bool    `json:"ok"`
	Stages        []Stage `json:"stages,omitempty"`
	Diagnostics   []struct {
		Code string `json:"code"`
	} `json:"diagnostics,omitempty"`
}

func TestC114QualificationRealBinaryFastFeedback(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C114_QUALIFICATION") != "1" && !strings.EqualFold(os.Getenv("CI"), "true") {
		t.Skip("C11.4 fast-feedback E2E runs in CI or with YUNKA_REQUIRE_C114_QUALIFICATION=1")
	}

	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(repositoryRoot, "app")
	protoc := strings.TrimSpace(os.Getenv("PROTOC"))
	if protoc == "" {
		protoc, err = exec.LookPath("protoc")
		if err != nil {
			t.Fatal("C11.4 qualification requires protoc")
		}
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("C11.4 qualification requires go")
	}
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("C11.4 qualification requires git")
	}

	beforeStatus := c114GitStatus(t, gitBinary, repositoryRoot)
	if beforeStatus != "" {
		t.Fatalf("C11.4 qualification requires a clean tracked worktree before build:\n%s", beforeStatus)
	}

	workRoot := t.TempDir()
	binaryName := "yunka-c114"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	yunkaBinary := filepath.Join(workRoot, binaryName)
	c114RunCommand(t, appRoot, nil, goBinary, "build", "-o", yunkaBinary, "./cmd")

	consumerRoot := filepath.Join(workRoot, "consumer")
	c114WriteFile(t, filepath.Join(consumerRoot, "go.mod"), "module example.com/c114consumer\n\ngo 1.25.0\n")
	c114WriteFile(t, filepath.Join(consumerRoot, "contracts", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c114consumer/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };

message GetDeviceRequest { string id = 1; }
message GetDeviceResponse { string id = 1; }

service DeviceApplication {
  option (yunka.dsl.v1.application) = {
    name: "management"
    operations: {
      id: "device.get"
      use_case: "get_device"
      public: true
      request_type: "device.v1.GetDeviceRequest"
      response_type: "device.v1.GetDeviceResponse"
      application_method: "GetDevice"
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)

	c114RunCommand(t, consumerRoot, nil, yunkaBinary,
		"module", "new", "--name", "device", "--root", filepath.Join(consumerRoot, "modules"), "--no-config", "--no-logger")

	baseGenerate := []string{
		"generate",
		"--root", consumerRoot,
		"--protoc", protoc,
		"--proto-path", filepath.Join(repositoryRoot, "contracts", "proto"),
		"--format", "json",
	}
	baseCheck := []string{
		"check",
		"--root", consumerRoot,
		"--protoc", protoc,
		"--proto-path", filepath.Join(repositoryRoot, "contracts", "proto"),
		"--format", "json",
	}

	firstGenerate, _, err := c114RunCLI(yunkaBinary, consumerRoot, baseGenerate...)
	if err != nil {
		t.Fatalf("first canonical generate: %v\n%s", err, firstGenerate)
	}
	firstEnvelope := c114DecodeEnvelope(t, firstGenerate)
	if !firstEnvelope.OK || c114HasStage(firstEnvelope, "fast-generate") {
		t.Fatalf("first generate must be canonical, got %s", firstGenerate)
	}
	c114RequireStages(t, firstEnvelope, "contract", "modules", "assembly")

	cachePath := filepath.Join(consumerRoot, filepath.FromSlash(fastfeedback.CacheRelativePath))
	metadata, err := fastfeedback.Load(cachePath)
	if err != nil {
		t.Fatalf("load first-generation evidence: %v", err)
	}
	if !metadata.Engine.Verified {
		t.Fatalf("qualification binary did not record a verified engine identity: %#v", metadata.Engine)
	}
	if !strings.HasPrefix(metadata.Engine.ID, "vcs:") && !strings.HasPrefix(metadata.Engine.ID, "module:") {
		t.Fatalf("unexpected engine identity %q", metadata.Engine.ID)
	}

	fastGenerate, generateDuration, err := c114RunCLI(yunkaBinary, consumerRoot, baseGenerate...)
	if err != nil {
		t.Fatalf("fast generate: %v\n%s", err, fastGenerate)
	}
	fastGenerateEnvelope := c114DecodeEnvelope(t, fastGenerate)
	c114RequireOnlyFastStage(t, fastGenerateEnvelope, "fast-generate", "unchanged")
	c114RequireBudget(t, "fast generate", generateDuration)

	fastCheck, checkDuration, err := c114RunCLI(yunkaBinary, consumerRoot, baseCheck...)
	if err != nil {
		t.Fatalf("fast check: %v\n%s", err, fastCheck)
	}
	fastCheckEnvelope := c114DecodeEnvelope(t, fastCheck)
	c114RequireOnlyFastStage(t, fastCheckEnvelope, "fast-check", "ok")
	c114RequireBudget(t, "fast check", checkDuration)

	fullGenerateArgs := append(append([]string(nil), baseGenerate...), "--full")
	fullGenerate, _, err := c114RunCLI(yunkaBinary, consumerRoot, fullGenerateArgs...)
	if err != nil {
		t.Fatalf("forced full generate: %v\n%s", err, fullGenerate)
	}
	fullGenerateEnvelope := c114DecodeEnvelope(t, fullGenerate)
	if !fullGenerateEnvelope.OK || c114HasStage(fullGenerateEnvelope, "fast-generate") {
		t.Fatalf("--full generate unexpectedly used fast path: %s", fullGenerate)
	}
	c114RequireStages(t, fullGenerateEnvelope, "contract", "modules", "assembly")

	fullCheckArgs := append(append([]string(nil), baseCheck...), "--full")
	fullCheck, _, err := c114RunCLI(yunkaBinary, consumerRoot, fullCheckArgs...)
	if err != nil {
		t.Fatalf("forced full check: %v\n%s", err, fullCheck)
	}
	fullCheckEnvelope := c114DecodeEnvelope(t, fullCheck)
	if !fullCheckEnvelope.OK || c114HasStage(fullCheckEnvelope, "fast-check") {
		t.Fatalf("--full check unexpectedly used fast path: %s", fullCheck)
	}
	c114RequireStages(t, fullCheckEnvelope, "contract", "modules", "assembly")

	manifestPath := filepath.Join(consumerRoot, "contracts", "generated", "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifest, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	driftOutput, _, driftErr := c114RunCLI(yunkaBinary, consumerRoot, baseCheck...)
	if driftErr == nil {
		t.Fatalf("corrupt generated output unexpectedly passed check: %s", driftOutput)
	}
	var exitErr *exec.ExitError
	if !errors.As(driftErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("drift check exit=%v output=%s", driftErr, driftOutput)
	}
	driftEnvelope := c114DecodeEnvelope(t, driftOutput)
	if driftEnvelope.OK || !c114HasDiagnostic(driftEnvelope, "YUNKA-DX-CONTRACT-002") {
		t.Fatalf("expected canonical contract drift diagnostic, got %s", driftOutput)
	}

	recoveryGenerate, _, err := c114RunCLI(yunkaBinary, consumerRoot, baseGenerate...)
	if err != nil {
		t.Fatalf("recovery generate: %v\n%s", err, recoveryGenerate)
	}
	recoveryEnvelope := c114DecodeEnvelope(t, recoveryGenerate)
	if !recoveryEnvelope.OK || c114HasStage(recoveryEnvelope, "fast-generate") {
		t.Fatalf("output drift must force canonical regeneration: %s", recoveryGenerate)
	}
	c114RequireStages(t, recoveryEnvelope, "contract", "modules", "assembly")

	recoveredCache, err := fastfeedback.Load(cachePath)
	if err != nil {
		t.Fatalf("load refreshed evidence: %v", err)
	}
	if !recoveredCache.Engine.Verified {
		t.Fatalf("recovery evidence lost verified engine identity: %#v", recoveredCache.Engine)
	}

	postRecoveryGenerate, postGenerateDuration, err := c114RunCLI(yunkaBinary, consumerRoot, baseGenerate...)
	if err != nil {
		t.Fatalf("post-recovery fast generate: %v\n%s", err, postRecoveryGenerate)
	}
	c114RequireOnlyFastStage(t, c114DecodeEnvelope(t, postRecoveryGenerate), "fast-generate", "unchanged")
	c114RequireBudget(t, "post-recovery fast generate", postGenerateDuration)

	postRecoveryCheck, postCheckDuration, err := c114RunCLI(yunkaBinary, consumerRoot, baseCheck...)
	if err != nil {
		t.Fatalf("post-recovery fast check: %v\n%s", err, postRecoveryCheck)
	}
	c114RequireOnlyFastStage(t, c114DecodeEnvelope(t, postRecoveryCheck), "fast-check", "ok")
	c114RequireBudget(t, "post-recovery fast check", postCheckDuration)

	afterStatus := c114GitStatus(t, gitBinary, repositoryRoot)
	if afterStatus != beforeStatus {
		t.Fatalf("C11.4 qualification mutated tracked framework worktree:\nbefore=%q\nafter=%q", beforeStatus, afterStatus)
	}
}

func c114RunCLI(binary, dir string, args ...string) (string, time.Duration, error) {
	started := time.Now()
	command := exec.Command(binary, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	return string(output), time.Since(started), err
}

func c114RunCommand(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	if env != nil {
		command.Env = append(os.Environ(), env...)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func c114DecodeEnvelope(t *testing.T, output string) c114CLIEnvelope {
	t.Helper()
	var envelope c114CLIEnvelope
	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("decode CLI JSON: %v\n%s", err, output)
	}
	return envelope
}

func c114HasStage(envelope c114CLIEnvelope, name string) bool {
	for _, stage := range envelope.Stages {
		if stage.Name == name {
			return true
		}
	}
	return false
}

func c114RequireStages(t *testing.T, envelope c114CLIEnvelope, names ...string) {
	t.Helper()
	got := make([]string, 0, len(envelope.Stages))
	for _, stage := range envelope.Stages {
		got = append(got, stage.Name)
	}
	sort.Strings(got)
	want := append([]string(nil), names...)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("stages=%v want %v", got, want)
	}
}

func c114RequireOnlyFastStage(t *testing.T, envelope c114CLIEnvelope, name, status string) {
	t.Helper()
	if !envelope.OK || len(envelope.Stages) != 1 || envelope.Stages[0].Name != name || envelope.Stages[0].Status != status {
		t.Fatalf("expected %s/%s only, got %#v", name, status, envelope)
	}
}

func c114HasDiagnostic(envelope c114CLIEnvelope, code string) bool {
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func c114RequireBudget(t *testing.T, label string, duration time.Duration) {
	t.Helper()
	if duration > c114FastHitBudget {
		t.Fatalf("%s latency %s exceeds budget %s", label, duration, c114FastHitBudget)
	}
}

func c114GitStatus(t *testing.T, gitBinary, root string) string {
	t.Helper()
	command := exec.Command(gitBinary, "-C", root, "status", "--porcelain", "--untracked-files=no")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func c114WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestC114QualificationBudgetIsConservative(t *testing.T) {
	if c114FastHitBudget < time.Second {
		t.Fatalf("qualification budget %s is too aggressive for shared CI runners", c114FastHitBudget)
	}
	if c114FastHitBudget > 5*time.Second {
		t.Fatalf("qualification budget %s is too weak to protect the C11 fast-feedback goal", c114FastHitBudget)
	}
}
