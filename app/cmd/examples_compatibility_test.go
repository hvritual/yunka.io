package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	runtimepkg "runtime"
	"strings"
	"testing"

	projectcmd "yunka.io/app/cmd/project"
	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/devruntime"
)

func TestC116CHappyPathExamplesMatchCanonicalSchemas(t *testing.T) {
	exampleRoot := c116ExampleRoot(t)

	projectBytes, err := os.ReadFile(filepath.Join(exampleRoot, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var project projectcmd.Config
	if err := json.Unmarshal(projectBytes, &project); err != nil {
		t.Fatal(err)
	}
	if err := projectcmd.Validate(project); err != nil {
		t.Fatalf("example project profile is invalid: %v", err)
	}
	if project.Version != projectcmd.ConfigVersion {
		t.Fatalf("project version=%d want %d", project.Version, projectcmd.ConfigVersion)
	}
	if project.Workflow.Dev.Manifest != "config/dev.runtime.json" {
		t.Fatalf("dev manifest=%q want config/dev.runtime.json", project.Workflow.Dev.Manifest)
	}
	if project.Workflow.Contract.ProtoRoot != "contracts/proto" || project.Workflow.Contract.Sources != "" {
		t.Fatalf("unexpected example contract profile: %#v", project.Workflow.Contract)
	}

	devBytes, err := os.ReadFile(filepath.Join(exampleRoot, "dev.runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest devruntime.DevManifest
	if err := json.Unmarshal(devBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != devruntime.RuntimeClosureSchemaVersion {
		t.Fatalf("dev schema=%d want %d", manifest.SchemaVersion, devruntime.RuntimeClosureSchemaVersion)
	}
	if manifest.Runtime == nil {
		t.Fatal("schema-v3 example must declare runtime evidence configuration")
	}
	if manifest.Runtime.Closure {
		t.Fatal("basic happy-path example must not require closure")
	}
	if len(manifest.Processes) != 1 || manifest.Processes[0].Name != "api" {
		t.Fatalf("unexpected example process plan: %#v", manifest.Processes)
	}
	if manifest.Processes[0].Readiness == nil {
		t.Fatal("example process must declare explicit readiness")
	}
	if err := manifest.Validate(exampleRoot, applicationgraph.Graph{}); err != nil {
		t.Fatalf("example DevManifest is invalid: %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(exampleRoot, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	for _, required := range []string{
		"yunka init",
		"yunka generate",
		"yunka check",
		"yunka dev",
		"yunka dev status",
		"yunka explain YUNKA-DX-CONTRACT-002",
		"yunka contract --help",
		"yunka assembly --help",
		"yunka module --help",
		"yunka domain --help",
		"yunka dependency --help",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("example README missing %q", required)
		}
	}
}

func c116ExampleRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtimepkg.Caller(0)
	if !ok {
		t.Fatal("cannot locate compatibility test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "docs", "examples", "c11-happy-path"))
}
