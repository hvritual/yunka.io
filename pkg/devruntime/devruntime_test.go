package devruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

func TestBuildPlanTopologicalAndTargeted(t *testing.T) {
	root := t.TempDir()
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{
		{Name: "db", Command: []string{"db"}},
		{Name: "api", Command: []string{"api"}, DependsOn: []string{"db"}, GraphNode: "service:svc"},
		{Name: "worker", Command: []string{"worker"}, DependsOn: []string{"db"}},
	}}
	graph := applicationgraph.Graph{SchemaVersion: 1, Nodes: []applicationgraph.Node{{ID: "service:svc", Kind: applicationgraph.NodeService, Name: "svc"}}}
	plan, err := BuildPlan(manifest, root, []string{"api"}, graph)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plan.Names(), []string{"db", "api"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names=%v want %v", got, want)
	}
}

func TestBuildPlanRejectsCycle(t *testing.T) {
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{
		{Name: "a", Command: []string{"a"}, DependsOn: []string{"b"}},
		{Name: "b", Command: []string{"b"}, DependsOn: []string{"a"}},
	}}
	if _, err := BuildPlan(manifest, t.TempDir(), nil, applicationgraph.Graph{}); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestManifestRejectsDuplicateDependency(t *testing.T) {
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{
		{Name: "db", Command: []string{"db"}},
		{Name: "api", Command: []string{"api"}, DependsOn: []string{"db", " db "}},
	}}
	if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil || !strings.Contains(err.Error(), "duplicate dependency") {
		t.Fatalf("err=%v", err)
	}
}

func TestManifestRejectsEscapingWorkingDir(t *testing.T) {
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{{Name: "a", Command: []string{"a"}, WorkingDir: "../outside"}}}
	if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil {
		t.Fatal("expected path error")
	}
}

func TestManifestRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{{Name: "a", Command: []string{"a"}, WorkingDir: "linked"}}}
	if err := manifest.Validate(root, applicationgraph.Graph{}); err == nil {
		t.Fatal("expected symlink escape error")
	}
}

func TestManifestSchemaV2ReadinessValidation(t *testing.T) {
	valid := DevManifest{SchemaVersion: DevSchemaVersion, Processes: []Process{{
		Name: "api", Command: []string{"api"}, Readiness: &Readiness{
			URL: "http://127.0.0.1:16667/_yunka/diagnostics", Timeout: "30s", Interval: "250ms", ExpectedStatus: http.StatusOK,
		},
	}}}
	if err := valid.Validate(t.TempDir(), applicationgraph.Graph{}); err != nil {
		t.Fatal(err)
	}

	cases := []Readiness{
		{URL: "http://example.com/ready"},
		{URL: "file:///tmp/ready"},
		{URL: "https://user:password@example.com/ready"},
		{URL: "https://example.com/ready#fragment"},
		{URL: "https://example.com/ready", Timeout: "6m"},
		{URL: "https://example.com/ready", Interval: "1ms"},
		{URL: "https://example.com/ready", ExpectedStatus: http.StatusTemporaryRedirect},
		{URL: "https://example.com/ready", TokenEnv: "INVALID-NAME"},
	}
	for index, readiness := range cases {
		manifest := DevManifest{SchemaVersion: DevSchemaVersion, Processes: []Process{{Name: "api", Command: []string{"api"}, Readiness: &readiness}}}
		if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil {
			t.Fatalf("case %d accepted: %+v", index, readiness)
		}
	}
}

func TestLegacyManifestRejectsReadiness(t *testing.T) {
	manifest := DevManifest{SchemaVersion: LegacyDevSchemaVersion, Processes: []Process{{
		Name: "api", Command: []string{"api"}, Readiness: &Readiness{URL: "http://127.0.0.1:16667/ready"},
	}}}
	if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil || !strings.Contains(err.Error(), "schemaVersion 2") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadDevManifestDefaultsToLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dev.json")
	if err := os.WriteFile(path, []byte(`{"processes":[{"name":"api","command":["api"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadDevManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != LegacyDevSchemaVersion {
		t.Fatalf("schema=%d", manifest.SchemaVersion)
	}
}

func TestManifestRejectsUnsupportedSchema(t *testing.T) {
	manifest := DevManifest{SchemaVersion: DevSchemaVersion + 1, Processes: []Process{{Name: "api", Command: []string{"api"}}}}
	if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil {
		t.Fatal("unsupported schema accepted")
	}
}

func TestDoctorReadOnlyChecks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\ntoolchain go1.25.13\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolchain := "GO_VERSION=1.25.13\nPROTOC_RELEASE=21.12\nPROTOC_VERSION=3.21.12\nPROTOC_LINUX_X86_64_SHA256=3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8\nGOVULNCHECK_VERSION=v1.7.0\n"
	if err := os.WriteFile(filepath.Join(root, "tools", "toolchain.env"), []byte(toolchain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "contracts", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "contracts", "generated", "manifest.json"), []byte(`{"schemaVersion":1,"files":[],"messages":[],"enums":[],"services":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lookPath := func(name string) (string, error) { return "/tools/" + name, nil }
	run := func(_ context.Context, name string, args ...string) (string, error) {
		switch filepath.Base(name) {
		case "go":
			return "go version go1.25.13 linux/amd64", nil
		case "protoc":
			return "libprotoc 3.21.12", nil
		case "gcc":
			return "gcc (GCC) 13.2.0", nil
		case "git":
			if len(args) > 0 && args[0] == "--version" {
				return "git version 2.45.0", nil
			}
			return "", nil
		}
		return "", nil
	}
	report := Doctor(context.Background(), DoctorOptions{Root: root, LookPath: lookPath, Run: run})
	if report.Failed(false) {
		t.Fatalf("unexpected failure: %+v", report.Checks)
	}
	foundWarn := false
	for _, check := range report.Checks {
		if check.Name == "dev.manifest" && check.Status == CheckWarn {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatal("expected optional dev manifest warning")
	}
}

func TestInheritedEnvironmentAllowList(t *testing.T) {
	got := inheritedEnvironment([]string{"A=1", "B=2"}, []string{"B"})
	if !reflect.DeepEqual(got, []string{"B=2"}) {
		t.Fatalf("env=%v", got)
	}
}

func TestWaitForDiagnosticsReadiness(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		ready := calls.Add(1) >= 2
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"core": map[string]any{"health": map[string]bool{"ready": ready}}})
	}))
	defer server.Close()

	process := Process{Name: "api", Readiness: &Readiness{
		URL: server.URL, Timeout: "2s", Interval: "10ms", DiagnosticsReady: true, TokenEnv: "YUNKA_DIAGNOSTICS_TOKEN",
	}}
	err := waitForReadiness(context.Background(), process, RunOptions{
		HTTPClient: server.Client(), Environ: []string{"YUNKA_DIAGNOSTICS_TOKEN=test-token"},
	}, make(chan processExit, 1))
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestWaitForReadinessRequiresConfiguredToken(t *testing.T) {
	process := Process{Name: "api", Readiness: &Readiness{
		URL: "http://127.0.0.1:16667/ready", TokenEnv: "YUNKA_DIAGNOSTICS_TOKEN",
	}}
	err := waitForReadiness(context.Background(), process, RunOptions{Environ: []string{"OTHER=value"}}, make(chan processExit, 1))
	if err == nil || !strings.Contains(err.Error(), "YUNKA_DIAGNOSTICS_TOKEN") {
		t.Fatalf("err=%v", err)
	}
}

func TestReadinessDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	ready, err := probeReadiness(context.Background(), redirect.Client(), &Readiness{URL: redirect.URL}, "")
	if ready || err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("ready=%v err=%v", ready, err)
	}
}

func TestRunBlocksDependentUntilDependencyReady(t *testing.T) {
	probeStarted := make(chan struct{})
	releaseProbe := make(chan struct{})
	var probeOnce sync.Once
	readinessServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		probeOnce.Do(func() { close(probeStarted) })
		<-releaseProbe
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"core":{"health":{"ready":true}}}`))
	}))
	defer readinessServer.Close()

	dependentStarted := make(chan struct{}, 1)
	notifyServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		select {
		case dependentStarted <- struct{}{}:
		default:
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer notifyServer.Close()

	helper := os.Args[0]
	manifest := DevManifest{SchemaVersion: DevSchemaVersion, Processes: []Process{
		{
			Name:      "dependency",
			Command:   []string{helper, "-test.run=TestDevruntimeHelperProcess", "--", "block"},
			Readiness: &Readiness{URL: readinessServer.URL, Timeout: "5s", Interval: "10ms", DiagnosticsReady: true},
		},
		{
			Name:      "dependent",
			Command:   []string{helper, "-test.run=TestDevruntimeHelperProcess", "--", "notify", notifyServer.URL},
			DependsOn: []string{"dependency"},
		},
	}}
	plan, err := BuildPlan(manifest, t.TempDir(), []string{"dependent"}, applicationgraph.Graph{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runResult := make(chan error, 1)
	go func() {
		runResult <- Run(ctx, plan, RunOptions{Environ: append(os.Environ(), "GO_WANT_DEVRUNTIME_HELPER=1")})
	}()

	select {
	case <-probeStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("readiness probe did not start")
	}
	select {
	case <-dependentStarted:
		t.Fatal("dependent started before readiness completed")
	default:
	}
	close(releaseProbe)
	select {
	case <-dependentStarted:
	case err := <-runResult:
		t.Fatalf("runner stopped before dependent start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("dependent did not start after readiness")
	}
	cancel()
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not stop after cancellation")
	}
}

func TestRunFailsWhenProcessExitsBeforeReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	plan := Plan{Processes: []Process{{
		Name:      "dependency",
		Command:   []string{os.Args[0], "-test.run=TestDevruntimeHelperProcess", "--", "exit"},
		Readiness: &Readiness{URL: server.URL, Timeout: "5s", Interval: "10ms"},
	}}}
	err := Run(context.Background(), plan, RunOptions{Environ: append(os.Environ(), "GO_WANT_DEVRUNTIME_HELPER=1")})
	if err == nil || !strings.Contains(err.Error(), "dependency") || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("err=%v", err)
	}
}

func TestNormalizeProcessDoesNotMutateCallerCommand(t *testing.T) {
	original := Process{Name: " api ", Command: []string{" go ", "run"}, Readiness: &Readiness{URL: " http://127.0.0.1:8080/ready "}}
	normalized := normalizeProcess(original)
	if normalized.Name != "api" || normalized.Command[0] != "go" || normalized.Readiness.URL != "http://127.0.0.1:8080/ready" {
		t.Fatalf("normalized=%+v", normalized)
	}
	if original.Command[0] != " go " || original.Readiness.URL != " http://127.0.0.1:8080/ready " {
		t.Fatalf("caller data mutated: %+v", original)
	}
}

func TestPrefixWriter(t *testing.T) {
	var buffer bytes.Buffer
	writer := &prefixWriter{prefix: "[x] ", writer: &buffer}
	if _, err := writer.Write([]byte("one\ntwo\n")); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "[x] one\n[x] two\n" {
		t.Fatalf("output=%q", got)
	}
}

func TestDevruntimeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_DEVRUNTIME_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "exit":
		os.Exit(17)
	case "notify":
		if len(args) != 2 {
			os.Exit(2)
		}
		request, err := http.NewRequest(http.MethodPost, args[1], nil)
		if err != nil {
			os.Exit(3)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			os.Exit(4)
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode > 299 {
			os.Exit(5)
		}
	case "block":
	default:
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

func TestDoctorRejectsExactToolchainVersionDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\ntoolchain go1.25.13\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolchain := "GO_VERSION=1.25.13\nPROTOC_RELEASE=21.12\nPROTOC_VERSION=3.21.12\nPROTOC_LINUX_X86_64_SHA256=3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8\nGOVULNCHECK_VERSION=v1.7.0\n"
	if err := os.WriteFile(filepath.Join(root, "tools", "toolchain.env"), []byte(toolchain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "contracts", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "contracts", "generated", "manifest.json"), []byte(`{"schemaVersion":1,"files":[],"messages":[],"enums":[],"services":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lookPath := func(name string) (string, error) { return "/tools/" + name, nil }
	run := func(_ context.Context, name string, args ...string) (string, error) {
		switch filepath.Base(name) {
		case "go":
			return "go version go1.25.13 linux/amd64", nil
		case "protoc":
			return "libprotoc 3.22.0", nil
		case "gcc":
			return "gcc (GCC) 13.2.0", nil
		case "git":
			if len(args) > 0 && args[0] == "--version" {
				return "git version 2.45.0", nil
			}
			return "", nil
		}
		return "", nil
	}
	report := Doctor(context.Background(), DoctorOptions{Root: root, LookPath: lookPath, Run: run})
	if !report.Failed(false) {
		t.Fatalf("expected exact-version failure: %+v", report.Checks)
	}
	for _, check := range report.Checks {
		if check.Name == "tool.protoc" {
			if check.Status != CheckFail || !strings.Contains(check.Action, "3.21.12") {
				t.Fatalf("unexpected protoc check: %+v", check)
			}
			return
		}
	}
	t.Fatal("tool.protoc check not found")
}

func TestDoctorRejectsToolchainLockGoWorkMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\ntoolchain go1.25.13\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	toolchain := "GO_VERSION=1.25.12\nPROTOC_RELEASE=21.12\nPROTOC_VERSION=3.21.12\nPROTOC_LINUX_X86_64_SHA256=3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8\nGOVULNCHECK_VERSION=v1.7.0\n"
	if err := os.WriteFile(filepath.Join(root, "tools", "toolchain.env"), []byte(toolchain), 0o644); err != nil {
		t.Fatal(err)
	}
	lookPath := func(name string) (string, error) { return "/tools/" + name, nil }
	run := func(_ context.Context, name string, args ...string) (string, error) {
		switch filepath.Base(name) {
		case "go":
			return "go version go1.25.12 linux/amd64", nil
		case "protoc":
			return "libprotoc 3.21.12", nil
		case "gcc":
			return "gcc (GCC) 13.2.0", nil
		case "git":
			if len(args) > 0 && args[0] == "--version" {
				return "git version 2.45.0", nil
			}
			return "", nil
		}
		return "", nil
	}
	report := Doctor(context.Background(), DoctorOptions{Root: root, LookPath: lookPath, Run: run})
	if !report.Failed(false) {
		t.Fatalf("expected lock mismatch failure: %+v", report.Checks)
	}
	for _, check := range report.Checks {
		if check.Name == "toolchain.lock" {
			if check.Status != CheckFail || !strings.Contains(check.Detail, "go.work=go1.25.13") {
				t.Fatalf("unexpected lock check: %+v", check)
			}
			return
		}
	}
	t.Fatal("toolchain.lock check not found")
}
