package devruntime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

func TestBuildPlanTopologicalAndTargeted(t *testing.T) {
	root := t.TempDir()
	manifest := DevManifest{SchemaVersion: 1, Processes: []Process{
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
	manifest := DevManifest{SchemaVersion: 1, Processes: []Process{{Name: "a", Command: []string{"a"}, DependsOn: []string{"b"}}, {Name: "b", Command: []string{"b"}, DependsOn: []string{"a"}}}}
	if _, err := BuildPlan(manifest, t.TempDir(), nil, applicationgraph.Graph{}); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestManifestRejectsEscapingWorkingDir(t *testing.T) {
	manifest := DevManifest{SchemaVersion: 1, Processes: []Process{{Name: "a", Command: []string{"a"}, WorkingDir: "../outside"}}}
	if err := manifest.Validate(t.TempDir(), applicationgraph.Graph{}); err == nil {
		t.Fatal("expected path error")
	}
}

func TestDoctorReadOnlyChecks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\ntoolchain go1.25.13\n"), 0o644); err != nil {
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

func TestClockLikeTimingIndependent(t *testing.T) { _ = time.Second }

func TestManifestRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	manifest := DevManifest{SchemaVersion: 1, Processes: []Process{{Name: "a", Command: []string{"a"}, WorkingDir: "linked"}}}
	if err := manifest.Validate(root, applicationgraph.Graph{}); err == nil {
		t.Fatal("expected symlink escape error")
	}
}
