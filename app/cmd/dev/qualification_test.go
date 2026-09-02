package dev

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/pkg/devruntime"
)

const c115QualificationTimeout = 15 * time.Second

func TestC115QualificationRealDevRuntimeHappyPath(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C115_QUALIFICATION") != "1" && !strings.EqualFold(os.Getenv("CI"), "true") {
		t.Skip("C11.5 dev-runtime E2E runs in CI or with YUNKA_REQUIRE_C115_QUALIFICATION=1")
	}
	if runtime.GOOS == "windows" {
		t.Skip("C11.5 clean signal qualification is enforced on Unix CI runners")
	}

	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(repositoryRoot, "app")
	goBinary := requireC115Tool(t, "go")
	gitBinary := requireC115Tool(t, "git")
	beforeStatus := c115GitStatus(t, gitBinary, repositoryRoot)
	if beforeStatus != "" {
		t.Fatalf("C11.5 qualification requires a clean tracked worktree before build:\n%s", beforeStatus)
	}

	workRoot := t.TempDir()
	yunkaBinary := filepath.Join(workRoot, executableName("yunka-c115"))
	helperBinary := filepath.Join(workRoot, executableName("c115-runtime-helper"))
	c115RunCommand(t, appRoot, goBinary, "build", "-buildvcs=true", "-o", yunkaBinary, "./cmd")
	c115RunCommand(t, appRoot, goBinary, "build", "-o", helperBinary, "./cmd/dev/testdata/c11_5_runtime_helper/main.go")

	consumerRoot := filepath.Join(workRoot, "consumer")
	dependencyAddress := reserveC115LoopbackAddress(t)
	applicationAddress := reserveC115LoopbackAddress(t)
	eventsPath := filepath.Join(consumerRoot, ".yunka", "c115-events.log")
	profilePath := filepath.Join(consumerRoot, ".yunka", "project.json")
	manifestPath := filepath.Join(consumerRoot, "config", "dev.runtime.json")
	graphPath := filepath.Join(consumerRoot, ".yunka", "application-graph.json")
	statePath := ".yunka/c115-runtime.json"
	runtimeGraphPath := ".yunka/c115-runtime-graph.json"

	c115WriteFile(t, profilePath, `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal"},
    "dev": {"manifest": "config/dev.runtime.json"}
  }
}
`)
	c115WriteFile(t, graphPath, `{
  "schemaVersion": 1,
  "nodes": [
    {"id": "application:app", "kind": "application", "name": "app"},
    {"id": "application:dependency", "kind": "application", "name": "dependency"}
  ],
  "edges": []
}
`)
	manifest := fmt.Sprintf(`{
  "schemaVersion": 3,
  "runtime": {
    "application": "c115consumer",
    "statePath": %q,
    "graphPath": %q,
    "shutdownTimeout": "3s",
    "closure": true
  },
  "processes": [
    {
      "name": "dependency",
      "command": [%q, "--name", "dependency", "--listen", %q, "--events", %q, "--ready-delay", "300ms"],
      "graphNode": "application:dependency",
      "readiness": {
        "url": %q,
        "timeout": "5s",
        "interval": "25ms",
        "expectedStatus": 200,
        "diagnosticsReady": true,
        "captureDiagnostics": true
      }
    },
    {
      "name": "app",
      "command": [%q, "--name", "app", "--listen", %q, "--events", %q, "--ready-delay", "100ms"],
      "dependsOn": ["dependency"],
      "graphNode": "application:app",
      "readiness": {
        "url": %q,
        "timeout": "5s",
        "interval": "25ms",
        "expectedStatus": 200,
        "diagnosticsReady": true,
        "captureDiagnostics": true
      }
    }
  ]
}
`, statePath, runtimeGraphPath,
		helperBinary, dependencyAddress, eventsPath, "http://"+dependencyAddress+"/diagnostics",
		helperBinary, applicationAddress, eventsPath, "http://"+applicationAddress+"/diagnostics")
	c115WriteFile(t, manifestPath, manifest)

	if _, err := os.Stat(filepath.Join(consumerRoot, ".yunka", "dev.json")); !os.IsNotExist(err) {
		t.Fatalf("qualification fixture must not have default .yunka/dev.json: %v", err)
	}
	profileBefore := c115ReadFile(t, profilePath)
	manifestBefore := c115ReadFile(t, manifestPath)
	graphBefore := c115ReadFile(t, graphPath)

	bareLog := filepath.Join(workRoot, "bare-dev.log")
	bare := c115StartDev(t, yunkaBinary, consumerRoot, bareLog, "dev", "--root", consumerRoot, "--target", "app")
	c115WaitForOutput(t, bareLog, "DEV READY application=c115consumer", c115QualificationTimeout)
	bareReport := c115RequireRunningClosure(t, yunkaBinary, consumerRoot, statePath)
	if got, want := bareReport.Plan, []string{"dependency", "app"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bare dev target closure plan=%v want %v", got, want)
	}
	c115StopDev(t, bare)
	c115RequireStoppedReport(t, consumerRoot, statePath)
	c115RequireLifecycleOrder(t, eventsPath)
	bareOutput := string(c115ReadFile(t, bareLog))
	for _, want := range []string{
		"DEV plan processes=2 names=dependency,app",
		"DEV evidence state=.yunka/c115-runtime.json graph=.yunka/c115-runtime-graph.json",
		"DEV READY process=dependency",
		"DEV READY process=app",
		"DEV READY application=c115consumer",
	} {
		if !strings.Contains(bareOutput, want) {
			t.Fatalf("bare dev output missing %q:\n%s", want, bareOutput)
		}
	}

	if err := os.WriteFile(eventsPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(consumerRoot, filepath.FromSlash(statePath)))
	_ = os.Remove(filepath.Join(consumerRoot, filepath.FromSlash(runtimeGraphPath)))

	explicitLog := filepath.Join(workRoot, "explicit-run.log")
	explicit := c115StartDev(t, yunkaBinary, consumerRoot, explicitLog, "dev", "run", "--root", consumerRoot, "--target", "app")
	c115WaitForOutput(t, explicitLog, "DEV READY application=c115consumer", c115QualificationTimeout)
	explicitReport := c115RequireRunningClosure(t, yunkaBinary, consumerRoot, statePath)
	if !reflect.DeepEqual(explicitReport.Plan, bareReport.Plan) {
		t.Fatalf("explicit run plan=%v differs from bare dev plan=%v", explicitReport.Plan, bareReport.Plan)
	}
	c115StopDev(t, explicit)
	c115RequireStoppedReport(t, consumerRoot, statePath)
	c115RequireLifecycleOrder(t, eventsPath)

	if got := c115ReadFile(t, profilePath); !reflect.DeepEqual(got, profileBefore) {
		t.Fatal("qualification mutated .yunka/project.json")
	}
	if got := c115ReadFile(t, manifestPath); !reflect.DeepEqual(got, manifestBefore) {
		t.Fatal("qualification mutated configured DevManifest")
	}
	if got := c115ReadFile(t, graphPath); !reflect.DeepEqual(got, graphBefore) {
		t.Fatal("qualification mutated declared Application Graph")
	}
	afterStatus := c115GitStatus(t, gitBinary, repositoryRoot)
	if afterStatus != beforeStatus {
		t.Fatalf("C11.5 qualification mutated tracked framework worktree:\nbefore=%q\nafter=%q", beforeStatus, afterStatus)
	}
}

func c115RequireRunningClosure(t *testing.T, yunkaBinary, consumerRoot, statePath string) devruntime.RuntimeReport {
	t.Helper()
	status := c115RunCommand(t, consumerRoot, yunkaBinary, "dev", "status", "--root", consumerRoot, "--closure")
	for _, want := range []string{"runtime application=c115consumer state=running", "dependency", "app", "ready=true"} {
		if !strings.Contains(status, want) {
			t.Fatalf("closure status missing %q:\n%s", want, status)
		}
	}
	report, err := devruntime.LoadRuntimeReport(consumerRoot, statePath)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != devruntime.RuntimeRunRunning {
		t.Fatalf("runtime state=%s want running", report.State)
	}
	if len(report.Processes) != 2 {
		t.Fatalf("runtime process count=%d want 2", len(report.Processes))
	}
	for _, process := range report.Processes {
		if process.State != devruntime.ProcessReady || !process.Ready || process.Diagnostics == nil || !process.Diagnostics.Live || !process.Diagnostics.Ready {
			t.Fatalf("process %s is not closure-ready: %#v", process.Name, process)
		}
	}
	return report
}

func c115RequireStoppedReport(t *testing.T, consumerRoot, statePath string) {
	t.Helper()
	report, err := devruntime.LoadRuntimeReport(consumerRoot, statePath)
	if err != nil {
		t.Fatal(err)
	}
	if report.State != devruntime.RuntimeRunStopped {
		t.Fatalf("final runtime state=%s reason=%q want stopped", report.State, report.Reason)
	}
	for _, process := range report.Processes {
		if process.State != devruntime.ProcessStopped {
			t.Fatalf("final process %s state=%s want stopped", process.Name, process.State)
		}
	}
}

func c115RequireLifecycleOrder(t *testing.T, eventsPath string) {
	t.Helper()
	lines := strings.Fields(string(c115ReadFile(t, eventsPath)))
	want := []string{"start:dependency", "ready:dependency", "start:app", "ready:app", "stop:app", "stop:dependency"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("runtime lifecycle events=%v want %v", lines, want)
	}
}

type c115DevProcess struct {
	command *exec.Cmd
	log     *os.File
}

func c115StartDev(t *testing.T, binary, dir, logPath string, args ...string) *c115DevProcess {
	t.Helper()
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, args...)
	command.Dir = dir
	command.Stdout = log
	command.Stderr = log
	if err := command.Start(); err != nil {
		_ = log.Close()
		t.Fatalf("start %s %s: %v", binary, strings.Join(args, " "), err)
	}
	return &c115DevProcess{command: command, log: log}
}

func c115StopDev(t *testing.T, process *c115DevProcess) {
	t.Helper()
	if process == nil || process.command == nil || process.command.Process == nil {
		t.Fatal("dev process is unavailable")
	}
	if err := process.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal dev process: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- process.command.Wait() }()
	select {
	case err := <-result:
		_ = process.log.Close()
		if err != nil {
			t.Fatalf("dev process did not shut down cleanly: %v", err)
		}
	case <-time.After(c115QualificationTimeout):
		_ = process.command.Process.Kill()
		_ = process.log.Close()
		t.Fatal("dev process did not stop after SIGTERM")
	}
}

func c115WaitForOutput(t *testing.T, path, needle string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(contents), needle) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	contents, _ := os.ReadFile(path)
	t.Fatalf("timed out waiting for %q in %s:\n%s", needle, path, contents)
}

func reserveC115LoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func c115RunCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func requireC115Tool(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("C11.5 qualification requires %s", name)
	}
	return path
}

func c115GitStatus(t *testing.T, gitBinary, root string) string {
	t.Helper()
	command := exec.Command(gitBinary, "-C", root, "status", "--porcelain", "--untracked-files=no")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func c115WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func c115ReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func TestC115QualificationFixtureJSONIsStable(t *testing.T) {
	var envelope map[string]any
	if err := json.Unmarshal([]byte(`{"schemaVersion":3}`), &envelope); err != nil || envelope["schemaVersion"] != float64(3) {
		t.Fatalf("qualification JSON sanity failed: %v %#v", err, envelope)
	}
}
