package projectflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveProjectUsesExplicitProfileLocations(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/profiled\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(root, "api", "proto", "service.proto"), "syntax = \"proto3\";\npackage demo;\n")
	writeTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "api/proto", "generated": "build/contracts"},
    "modules": {"root": "src/modules"},
    "generatedGo": {"root": "src/generated"},
    "dev": {"manifest": ".yunka/local-dev.json"}
  }
}
`)

	project, err := resolveProject(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !project.Profiled {
		t.Fatal("expected explicit project profile")
	}
	if project.ProtoDir != filepath.Join(root, "api", "proto") {
		t.Fatalf("ProtoDir=%q", project.ProtoDir)
	}
	if project.ContractOut != filepath.Join(root, "build", "contracts") {
		t.Fatalf("ContractOut=%q", project.ContractOut)
	}
	if project.ModuleRoot != filepath.Join(root, "src", "modules") {
		t.Fatalf("ModuleRoot=%q", project.ModuleRoot)
	}
	if project.CodeOut != filepath.Join(root, "src", "generated") {
		t.Fatalf("CodeOut=%q", project.CodeOut)
	}
	if project.CodeImport != "example.com/profiled/src/generated" {
		t.Fatalf("CodeImport=%q", project.CodeImport)
	}
	if project.DevManifest != filepath.Join(root, ".yunka", "local-dev.json") {
		t.Fatalf("DevManifest=%q", project.DevManifest)
	}
}

func TestResolveProjectProfileMissingContractPathFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "fallback.proto"), "syntax = \"proto3\";\npackage fallback;\n")
	writeTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "configured/missing", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal"},
    "dev": {"manifest": ".yunka/dev.json"}
  }
}
`)

	_, err := resolveProject(Options{Root: root})
	if err == nil {
		t.Fatal("expected configured missing path to fail")
	}
	if !strings.Contains(err.Error(), "workflow.contract.protoRoot") || !strings.Contains(err.Error(), "missing directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveProjectRejectsGeneratedImportConflict(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/profiled\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal", "import": "example.com/wrong/internal"},
    "dev": {"manifest": ".yunka/dev.json"}
  }
}
`)

	_, err := resolveProject(Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "conflicts with go.mod-derived import") {
		t.Fatalf("expected generated import conflict, got %v", err)
	}
}

func TestResolveProjectAllowsExplicitImportWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal", "import": "example.com/external/internal"},
    "dev": {"manifest": ".yunka/dev.json"}
  }
}
`)

	project, err := resolveProject(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if project.GoModule != "" || project.CodeImport != "example.com/external/internal" {
		t.Fatalf("unexpected explicit import resolution: %#v", project)
	}
}
