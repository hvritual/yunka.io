package projectflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	modulecmd "yunka.io/app/cmd/module"
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

func TestProfiledHappyPathGenerateAndCheck(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for the C11.2 profiled happy-path test")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/profiled\n\ngo 1.25.0\n")
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
	writeTestFile(t, filepath.Join(root, "api", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/profiled/contracts/device/v1;devicev1";
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
	if err := modulecmd.GenerateWithOptions(modulecmd.Options{
		Name:     "device",
		Root:     filepath.Join(root, "src", "modules"),
		NoConfig: true,
		Logger:   false,
	}); err != nil {
		t.Fatal(err)
	}
	options := Options{
		Root:       root,
		Protoc:     protoc,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
	}
	report, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Stages) != 3 || report.Stages[2].Status != "generated" {
		t.Fatalf("unexpected profiled report: %#v", report)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "contracts", "assembly-plan.json")); err != nil {
		t.Fatalf("profiled assembly plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "generated", "assembly")); err != nil {
		t.Fatalf("profiled generated Go: %v", err)
	}
	if _, err := Check(context.Background(), options); err != nil {
		t.Fatalf("profiled check: %v", err)
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
