package projectflow

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	modulecmd "yunka.io/app/cmd/module"
)

func TestResolveProjectConventionalDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	project, err := resolveProject(Options{Root: root, ProtoPaths: []string{"third_party/proto"}})
	if err != nil {
		t.Fatal(err)
	}
	if project.GoModule != "example.com/demo" {
		t.Fatalf("GoModule=%q", project.GoModule)
	}
	if project.CodeImport != "example.com/demo/internal" {
		t.Fatalf("CodeImport=%q", project.CodeImport)
	}
	if project.InventoryPath != "" {
		t.Fatalf("InventoryPath=%q", project.InventoryPath)
	}
	if project.ContractOut != filepath.Join(root, "contracts", "generated") {
		t.Fatalf("ContractOut=%q", project.ContractOut)
	}
	if project.ModuleRoot != filepath.Join(root, "modules") {
		t.Fatalf("ModuleRoot=%q", project.ModuleRoot)
	}
	if project.CodeOut != filepath.Join(root, "internal") {
		t.Fatalf("CodeOut=%q", project.CodeOut)
	}
	if len(project.AdditionalProtoPaths) != 1 || project.AdditionalProtoPaths[0] != filepath.Join(root, "third_party", "proto") {
		t.Fatalf("AdditionalProtoPaths=%v", project.AdditionalProtoPaths)
	}
}

func TestResolveProjectPrefersSourceInventory(t *testing.T) {
	root := t.TempDir()
	inventory := filepath.Join(root, "contracts", "sources.json")
	if err := os.MkdirAll(filepath.Dir(inventory), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventory, []byte("{\"schemaVersion\":1,\"sourceSets\":[]}"), 0o640); err != nil {
		t.Fatal(err)
	}

	project, err := resolveProject(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if project.InventoryPath != inventory {
		t.Fatalf("InventoryPath=%q want %q", project.InventoryPath, inventory)
	}
}

func TestResolveProjectRequiresContractSource(t *testing.T) {
	_, err := resolveProject(Options{Root: t.TempDir()})
	if err == nil {
		t.Fatal("expected missing contract source error")
	}
}

func TestFormatStageOrder(t *testing.T) {
	report := Report{Stages: []Stage{
		{Name: "contract", Status: "generated", Detail: "ok"},
		{Name: "modules", Status: "skipped", Detail: "none"},
	}}
	got := Format(report)
	want := "GENERATED contract  ok\nSKIPPED   modules   none\n"
	if got != want {
		t.Fatalf("Format()=%q want %q", got, want)
	}
}

func TestHappyPathGenerateCheckDeterminism(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for the C11.1 executable happy-path test")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/c11happy\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c11happy/contracts/device/v1;devicev1";
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
		Root:     filepath.Join(root, "modules"),
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
	firstReport, err := Generate(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstReport.Stages) != 3 || firstReport.Stages[2].Name != "assembly" || firstReport.Stages[2].Status != "generated" {
		t.Fatalf("unexpected first report: %#v", firstReport)
	}
	if _, err := os.Stat(filepath.Join(root, "contracts", "generated", "assembly-plan.json")); err != nil {
		t.Fatalf("assembly plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "assembly")); err != nil {
		t.Fatalf("generated assembly: %v", err)
	}

	firstSnapshot := snapshotGenerated(t, root)
	if _, err := Generate(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	secondSnapshot := snapshotGenerated(t, root)
	if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
		t.Fatalf("second generation drifted:\nfirst=%v\nsecond=%v", firstSnapshot, secondSnapshot)
	}
	if _, err := Check(context.Background(), options); err != nil {
		t.Fatal(err)
	}

	assemblyPlan := filepath.Join(root, "contracts", "generated", "assembly-plan.json")
	contents, err := os.ReadFile(assemblyPlan)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(assemblyPlan, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(assemblyPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Check(context.Background(), options); err == nil {
		t.Fatal("expected assembly drift to fail yunka check")
	}
	after, err := os.ReadFile(assemblyPlan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("yunka check mutated the drifted assembly plan")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}

func snapshotGenerated(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, relativeRoot := range []string{"contracts/generated", "internal"} {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(contents)
			result[filepath.ToSlash(relative)] = fmt.Sprintf("%x", sum[:])
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}
